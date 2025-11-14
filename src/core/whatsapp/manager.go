package whatsapp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

// EventPublisher defines the interface for publishing events
type EventPublisher interface {
	Publish(topic string, messages ...*message.Message) error
}

// Device represents a WhatsApp device
type Device struct {
	ID     string
	Client *whatsmeow.Client
	QRCode string
	Status string
}

// Manager manages multiple WhatsApp devices
type Manager struct {
	devices   map[string]*Device
	container *sqlstore.Container
	deviceDB  *DeviceDatabase
	logger    waLog.Logger
	publisher EventPublisher
	mu        sync.RWMutex
}

// NewManager creates a new WhatsApp manager
func NewManager(dbPath string, logger waLog.Logger) (*Manager, error) {
	container, err := sqlstore.New(context.Background(), "sqlite3", dbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Create device mapping database
	deviceDB, err := NewDeviceDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create device database: %w", err)
	}

	return &Manager{
		devices:   make(map[string]*Device),
		container: container,
		deviceDB:  deviceDB,
		logger:    logger,
	}, nil
}

// SetEventPublisher sets the event publisher for the manager
func (m *Manager) SetEventPublisher(publisher EventPublisher) {
	m.publisher = publisher
}

// extractMessagePayload extracts message details from WhatsApp event
// Now includes automatic media download from WhatsApp for media messages
func (m *Manager) extractMessagePayload(deviceID string, v *events.Message, client *whatsmeow.Client) MessagePayload {
	ctx := context.Background()

	// Initialize payload with basic info
	chatName := ""
	senderName := v.Info.PushName
	isGroup := v.Info.Chat.Server == "g.us"

	// Get sender name from contacts
	if contact, err := client.Store.Contacts.GetContact(ctx, v.Info.Sender); err == nil && contact.Found {
		if contact.FullName != "" {
			senderName = contact.FullName
		} else if contact.PushName != "" {
			senderName = contact.PushName
		}
	}

	// Get chat name
	if isGroup {
		// For group chats, get the group name
		if groupInfo, err := client.GetGroupInfo(ctx, v.Info.Chat); err == nil {
			chatName = groupInfo.Name
		}
	} else {
		// For DM, use the contact name (same as chat recipient)
		if contactTo, err := client.Store.Contacts.GetContact(ctx, v.Info.Chat); err == nil && contactTo.Found {
			if contactTo.FullName != "" {
				chatName = contactTo.FullName
			} else if contactTo.PushName != "" {
				chatName = contactTo.PushName
			}
		}
		// Fallback: if no contact found, use sender name for DM
		if chatName == "" {
			chatName = senderName
		}
	}

	payload := MessagePayload{
		MessageID:  v.Info.ID,
		ChatID:     v.Info.Chat.String(),
		ChatName:   chatName,
		Sender:     v.Info.Sender.String(),
		SenderName: senderName,
		Timestamp:  v.Info.Timestamp.UnixMilli(),
		PushName:   v.Info.PushName,
		FromMe:     v.Info.IsFromMe,
		IsGroup:    isGroup,
	}

	// Extract message content based on type
	msg := v.Message
	if msg.Conversation != nil {
		payload.Type = "text"
		payload.Text = &TextContent{
			Content: *msg.Conversation,
		}
	} else if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
		payload.Type = "text"
		payload.Text = &TextContent{
			Content: *msg.ExtendedTextMessage.Text,
		}
	} else if msg.ImageMessage != nil {
		payload.Type = "image"

		// Generate base filename for the media
		baseFilename := fmt.Sprintf("image_%s_%d", v.Info.ID, v.Info.Timestamp.Unix())

		// Download media from WhatsApp
		mediaData, err := m.downloadMedia(ctx, deviceID, client, msg.ImageMessage, baseFilename)
		if err != nil {
			m.logger.Errorf("Failed to download image media for device %s, message %s: %v", deviceID, v.Info.ID, err)

			// Publish failure event
			m.publishEvent("message.media.download_failed", MessageMediaDownloadFailedEvent{
				DeviceID:  deviceID,
				MessageID: v.Info.ID,
				ChatID:    v.Info.Chat.String(),
				MediaType: "image",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})

			// Fallback: populate URL field only (backward compatibility)
			imageInfo := &ImageContent{
				Mimetype: msg.ImageMessage.GetMimetype(),
				Caption:  msg.ImageMessage.GetCaption(),
			}
			if msg.ImageMessage.URL != nil {
				imageInfo.URL = *msg.ImageMessage.URL
				// Also store in MediaData.WhatsAppURL for consistency
				imageInfo.Media = &MediaData{
					WhatsAppURL: *msg.ImageMessage.URL,
					Mimetype:    msg.ImageMessage.GetMimetype(),
				}
			}
			payload.Image = imageInfo
		} else {
			// Success: populate both Media (new) and URL (deprecated) fields
			imageInfo := &ImageContent{
				Media:    mediaData,
				Mimetype: mediaData.Mimetype,
				Caption:  msg.ImageMessage.GetCaption(),
			}
			// Include deprecated URL field for backward compatibility
			if msg.ImageMessage.URL != nil {
				imageInfo.URL = *msg.ImageMessage.URL
				mediaData.WhatsAppURL = *msg.ImageMessage.URL
			}
			payload.Image = imageInfo
		}
		payload.HasMedia = true
	} else if msg.DocumentMessage != nil {
		payload.Type = "document"

		// Use original filename as base, or generate one
		baseFilename := msg.DocumentMessage.GetFileName()
		if baseFilename == "" {
			baseFilename = fmt.Sprintf("document_%s_%d", v.Info.ID, v.Info.Timestamp.Unix())
		}

		// Download media from WhatsApp
		mediaData, err := m.downloadMedia(ctx, deviceID, client, msg.DocumentMessage, baseFilename)
		if err != nil {
			m.logger.Errorf("Failed to download document media for device %s, message %s: %v", deviceID, v.Info.ID, err)

			// Publish failure event
			m.publishEvent("message.media.download_failed", MessageMediaDownloadFailedEvent{
				DeviceID:  deviceID,
				MessageID: v.Info.ID,
				ChatID:    v.Info.Chat.String(),
				MediaType: "document",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})

			// Fallback: populate URL field only
			docInfo := &DocumentContent{
				Mimetype: msg.DocumentMessage.GetMimetype(),
				Caption:  msg.DocumentMessage.GetCaption(),
				Filename: msg.DocumentMessage.GetFileName(),
			}
			if msg.DocumentMessage.URL != nil {
				docInfo.URL = *msg.DocumentMessage.URL
				docInfo.Media = &MediaData{
					WhatsAppURL: *msg.DocumentMessage.URL,
					Mimetype:    msg.DocumentMessage.GetMimetype(),
					Filename:    msg.DocumentMessage.GetFileName(),
				}
			}
			payload.Document = docInfo
		} else {
			// Success: populate both Media and URL fields
			docInfo := &DocumentContent{
				Media:    mediaData,
				Mimetype: mediaData.Mimetype,
				Caption:  msg.DocumentMessage.GetCaption(),
				Filename: mediaData.Filename,
			}
			if msg.DocumentMessage.URL != nil {
				docInfo.URL = *msg.DocumentMessage.URL
				mediaData.WhatsAppURL = *msg.DocumentMessage.URL
			}
			payload.Document = docInfo
		}
		payload.HasMedia = true
	} else if msg.VideoMessage != nil {
		payload.Type = "video"

		// Generate base filename
		baseFilename := fmt.Sprintf("video_%s_%d", v.Info.ID, v.Info.Timestamp.Unix())

		// Download media from WhatsApp
		mediaData, err := m.downloadMedia(ctx, deviceID, client, msg.VideoMessage, baseFilename)
		if err != nil {
			m.logger.Errorf("Failed to download video media for device %s, message %s: %v", deviceID, v.Info.ID, err)

			// Publish failure event
			m.publishEvent("message.media.download_failed", MessageMediaDownloadFailedEvent{
				DeviceID:  deviceID,
				MessageID: v.Info.ID,
				ChatID:    v.Info.Chat.String(),
				MediaType: "video",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})

			// Fallback: populate URL field only
			videoInfo := &VideoContent{
				Mimetype: msg.VideoMessage.GetMimetype(),
				Caption:  msg.VideoMessage.GetCaption(),
			}
			if msg.VideoMessage.URL != nil {
				videoInfo.URL = *msg.VideoMessage.URL
				videoInfo.Media = &MediaData{
					WhatsAppURL: *msg.VideoMessage.URL,
					Mimetype:    msg.VideoMessage.GetMimetype(),
				}
			}
			payload.Video = videoInfo
		} else {
			// Success: populate both Media and URL fields
			videoInfo := &VideoContent{
				Media:    mediaData,
				Mimetype: mediaData.Mimetype,
				Caption:  msg.VideoMessage.GetCaption(),
			}
			if msg.VideoMessage.URL != nil {
				videoInfo.URL = *msg.VideoMessage.URL
				mediaData.WhatsAppURL = *msg.VideoMessage.URL
			}
			payload.Video = videoInfo
		}
		payload.HasMedia = true
	} else if msg.AudioMessage != nil {
		payload.Type = "audio"

		// Generate base filename
		baseFilename := fmt.Sprintf("audio_%s_%d", v.Info.ID, v.Info.Timestamp.Unix())

		// Download media from WhatsApp
		mediaData, err := m.downloadMedia(ctx, deviceID, client, msg.AudioMessage, baseFilename)
		if err != nil {
			m.logger.Errorf("Failed to download audio media for device %s, message %s: %v", deviceID, v.Info.ID, err)

			// Publish failure event
			m.publishEvent("message.media.download_failed", MessageMediaDownloadFailedEvent{
				DeviceID:  deviceID,
				MessageID: v.Info.ID,
				ChatID:    v.Info.Chat.String(),
				MediaType: "audio",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})

			// Fallback: populate URL field only
			audioInfo := &AudioContent{
				Mimetype: msg.AudioMessage.GetMimetype(),
			}
			if msg.AudioMessage.URL != nil {
				audioInfo.URL = *msg.AudioMessage.URL
				audioInfo.Media = &MediaData{
					WhatsAppURL: *msg.AudioMessage.URL,
					Mimetype:    msg.AudioMessage.GetMimetype(),
				}
			}
			payload.Audio = audioInfo
		} else {
			// Success: populate both Media and URL fields
			audioInfo := &AudioContent{
				Media:    mediaData,
				Mimetype: mediaData.Mimetype,
			}
			if msg.AudioMessage.URL != nil {
				audioInfo.URL = *msg.AudioMessage.URL
				mediaData.WhatsAppURL = *msg.AudioMessage.URL
			}
			payload.Audio = audioInfo
		}
		payload.HasMedia = true
	} else if msg.StickerMessage != nil {
		payload.Type = "sticker"

		// Generate base filename
		baseFilename := fmt.Sprintf("sticker_%s_%d", v.Info.ID, v.Info.Timestamp.Unix())

		// Download media from WhatsApp
		mediaData, err := m.downloadMedia(ctx, deviceID, client, msg.StickerMessage, baseFilename)
		if err != nil {
			m.logger.Errorf("Failed to download sticker media for device %s, message %s: %v", deviceID, v.Info.ID, err)

			// Publish failure event
			m.publishEvent("message.media.download_failed", MessageMediaDownloadFailedEvent{
				DeviceID:  deviceID,
				MessageID: v.Info.ID,
				ChatID:    v.Info.Chat.String(),
				MediaType: "sticker",
				Error:     err.Error(),
				Timestamp: time.Now().UnixMilli(),
			})

			// Fallback: populate URL field only
			stickerInfo := &StickerContent{
				Mimetype: msg.StickerMessage.GetMimetype(),
			}
			if msg.StickerMessage.URL != nil {
				stickerInfo.URL = *msg.StickerMessage.URL
				stickerInfo.Media = &MediaData{
					WhatsAppURL: *msg.StickerMessage.URL,
					Mimetype:    msg.StickerMessage.GetMimetype(),
				}
			}
			payload.Sticker = stickerInfo
		} else {
			// Success: populate both Media and URL fields
			stickerInfo := &StickerContent{
				Media:    mediaData,
				Mimetype: mediaData.Mimetype,
			}
			if msg.StickerMessage.URL != nil {
				stickerInfo.URL = *msg.StickerMessage.URL
				mediaData.WhatsAppURL = *msg.StickerMessage.URL
			}
			payload.Sticker = stickerInfo
		}
		payload.HasMedia = true
	} else if msg.ContactMessage != nil {
		payload.Type = "contact"
		contactInfo := &ContactContent{
			DisplayName: msg.ContactMessage.GetDisplayName(),
			VCard:       msg.ContactMessage.GetVcard(),
		}
		payload.Contact = contactInfo
	} else if msg.ContactsArrayMessage != nil {
		payload.Type = "contacts"
		contacts := make([]string, 0, len(msg.ContactsArrayMessage.Contacts))
		for _, contact := range msg.ContactsArrayMessage.Contacts {
			contacts = append(contacts, contact.GetVcard())
		}
		contactsInfo := &ContactsContent{
			DisplayName: msg.ContactsArrayMessage.GetDisplayName(),
			Contacts:    contacts,
		}
		payload.Contacts = contactsInfo
	} else if msg.LocationMessage != nil {
		payload.Type = "location"
		locationInfo := &LocationContent{
			Latitude:  float64(msg.LocationMessage.GetDegreesLatitude()),
			Longitude: float64(msg.LocationMessage.GetDegreesLongitude()),
			Name:      msg.LocationMessage.GetName(),
			Address:   msg.LocationMessage.GetAddress(),
			URL:       msg.LocationMessage.GetURL(),
			IsLive:    false,
		}
		payload.Location = locationInfo
	} else if msg.LiveLocationMessage != nil {
		payload.Type = "live_location"
		locationInfo := &LocationContent{
			Latitude:  float64(msg.LiveLocationMessage.GetDegreesLatitude()),
			Longitude: float64(msg.LiveLocationMessage.GetDegreesLongitude()),
			Name:      msg.LiveLocationMessage.GetCaption(),
			IsLive:    true,
		}
		payload.Location = locationInfo
	} else if msg.ReactionMessage != nil {
		payload.Type = "reaction"
		reactionInfo := &ReactionContent{
			Emoji:           msg.ReactionMessage.GetText(),
			TargetMessageID: msg.ReactionMessage.GetKey().GetId(),
		}
		if msg.ReactionMessage.GetKey().GetParticipant() != "" {
			reactionInfo.TargetSender = msg.ReactionMessage.GetKey().GetParticipant()
		} else if msg.ReactionMessage.GetKey().GetRemoteJid() != "" {
			reactionInfo.TargetSender = msg.ReactionMessage.GetKey().GetRemoteJid()
		}
		payload.Reaction = reactionInfo
	} else if msg.ProtocolMessage != nil {
		payload.Type = "protocol"
		protocolType := "unknown"
		targetMessageID := ""

		switch msg.ProtocolMessage.GetType() {
		case waProto.ProtocolMessage_REVOKE:
			protocolType = "revoke"
			if msg.ProtocolMessage.Key != nil {
				targetMessageID = msg.ProtocolMessage.Key.GetId()
			}
		case waProto.ProtocolMessage_EPHEMERAL_SETTING:
			protocolType = "ephemeral_setting"
		case waProto.ProtocolMessage_EPHEMERAL_SYNC_RESPONSE:
			protocolType = "ephemeral_sync_response"
		case waProto.ProtocolMessage_HISTORY_SYNC_NOTIFICATION:
			protocolType = "history_sync_notification"
		case waProto.ProtocolMessage_APP_STATE_SYNC_KEY_SHARE:
			protocolType = "app_state_sync_key_share"
		case waProto.ProtocolMessage_INITIAL_SECURITY_NOTIFICATION_SETTING_SYNC:
			protocolType = "initial_security_notification_setting_sync"
		case waProto.ProtocolMessage_APP_STATE_FATAL_EXCEPTION_NOTIFICATION:
			protocolType = "app_state_fatal_exception_notification"
		}

		protocolInfo := &ProtocolContent{
			Type:            protocolType,
			TargetMessageID: targetMessageID,
		}
		payload.Protocol = protocolInfo
	} else if msg.PollCreationMessage != nil {
		payload.Type = "poll"
		options := make([]PollOption, 0, len(msg.PollCreationMessage.GetOptions()))
		for _, opt := range msg.PollCreationMessage.GetOptions() {
			options = append(options, PollOption{
				Name: opt.GetOptionName(),
			})
		}
		pollInfo := &PollContent{
			Name:            msg.PollCreationMessage.GetName(),
			Options:         options,
			SelectableCount: int(msg.PollCreationMessage.GetSelectableOptionsCount()),
		}
		payload.Poll = pollInfo
	} else if msg.PollUpdateMessage != nil {
		payload.Type = "poll_vote"
		pollVoteInfo := &PollVoteContent{
			PollMessageID:   msg.PollUpdateMessage.GetPollCreationMessageKey().GetId(),
			SelectedOptions: []string{},
		}

		// Try to decrypt the poll vote to get selected options
		if vote := msg.PollUpdateMessage.GetVote(); vote != nil {
			// The vote contains encrypted selected option names
			// For now, we'll just indicate that a vote was cast
			// Full decryption would require calling client.DecryptPollVote()
			// which needs the full event context, not just the message
			m.logger.Debugf("Poll vote received but decryption requires full event context")
			pollVoteInfo.SelectedOptions = []string{"[encrypted - requires decryption]"}
		}

		payload.PollVote = pollVoteInfo
	} else {
		payload.Type = "unknown"
	}

	return payload
}

// handleHistorySync processes history sync events from WhatsApp
func (m *Manager) handleHistorySync(deviceID string, client *whatsmeow.Client, historySync *events.HistorySync) {
	// Log the history sync event
	syncType := historySync.Data.GetSyncType().String()
	m.logger.Infof("Received history sync for device %s: type=%s, conversations=%d",
		deviceID, syncType, len(historySync.Data.GetConversations()))

	// Extract messages from the history sync
	var messages []MessagePayload
	var chatID string

	// Process conversations in the history sync
	for _, conversation := range historySync.Data.GetConversations() {
		chatJID := conversation.GetId()
		chatID = chatJID // Save for event (will be last processed chat)

		// Process messages in this conversation
		for _, historyMsg := range conversation.GetMessages() {
			webMsg := historyMsg.GetMessage()
			if webMsg == nil {
				continue
			}

			// Create MessageInfo structure for extracting payload
			msgInfo := &events.Message{
				Info: types.MessageInfo{
					MessageSource: types.MessageSource{
						Chat:   types.NewJID(chatJID, types.DefaultUserServer),
						Sender: types.NewJID(webMsg.Key.GetParticipant(), types.DefaultUserServer),
					},
					ID:        webMsg.Key.GetId(),
					Timestamp: time.UnixMilli(int64(webMsg.GetMessageTimestamp())),
					PushName:  webMsg.GetPushName(),
				},
				Message: webMsg.GetMessage(),
			}

			// Extract message payload
			payload := m.extractMessagePayload(deviceID, msgInfo, client)
			messages = append(messages, payload)
		}
	}

	// If we have messages, publish the history synced event
	if len(messages) > 0 {
		m.logger.Infof("Publishing %d messages from history sync for device %s", len(messages), deviceID)
		m.publishEvent("message.history.synced", MessageHistorySyncedEvent{
			DeviceID: deviceID,
			ChatID:   chatID,
			Messages: messages,
			Count:    len(messages),
			// Note: RequestID is empty for automatic syncs, only set for manual requests
		})
	} else {
		m.logger.Debugf("No messages in history sync for device %s (type=%s)", deviceID, syncType)
	}
}

// setupEventHandlers sets up event handlers for a WhatsApp client
func (m *Manager) setupEventHandlers(deviceID string, client *whatsmeow.Client) {
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Handle incoming message from WhatsApp
			messagePayload := m.extractMessagePayload(deviceID, v, client)
			m.publishEvent("message.received", MessageReceivedEvent{
				DeviceID: deviceID,
				Message:  messagePayload,
				Source:   "whatsapp",
			})

		case *events.Disconnected:
			// Handle device disconnect (internet issue)
			m.publishEvent("device.disconnected", DeviceDisconnectedEvent{
				DeviceID: deviceID,
				Reason:   "connection_lost",
			})

		case *events.LoggedOut:
			// Handle device logout
			m.mu.Lock()
			delete(m.devices, deviceID)
			m.mu.Unlock()

			// Delete device mapping from database
			if err := m.deviceDB.DeleteMapping(deviceID); err != nil {
				m.logger.Errorf("Failed to delete device mapping: %v", err)
			} else {
				m.logger.Infof("Deleted device mapping: %s", deviceID)
			}

			m.publishEvent("device.logout", DeviceLogoutEvent{
				DeviceID: deviceID,
				Reason:   v.Reason.String(),
			})

		case *events.Connected:
			// Handle successful connection
			deviceJID := ""
			if client.Store.ID != nil {
				deviceJID = client.Store.ID.User
			}
			m.publishEvent("device.connected", DeviceConnectedEvent{
				DeviceID:  deviceID,
				DeviceJID: deviceJID,
				Status:    "connected",
			})

		case *events.Receipt:
			// Handle read receipts
			m.publishEvent("message.read", MessageReadEvent{
				DeviceID:   deviceID,
				ChatID:     v.Chat.String(),
				Sender:     v.Sender.String(),
				MessageIDs: v.MessageIDs,
				Timestamp:  v.Timestamp.UnixMilli(),
				Type:       v.Type.GoString(),
			})

		case *events.HistorySync:
			// Handle history sync (automatic on first connection or manual request)
			m.handleHistorySync(deviceID, client, v)
		}
	})
}

// CreateDevice creates a new WhatsApp device and returns QR code
// If deviceID is empty, a new unique ID will be generated
func (m *Manager) CreateDevice(ctx context.Context, deviceID string) (*Device, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate a unique ID if not provided
	if deviceID == "" {
		deviceID = fmt.Sprintf("device_%d", len(m.devices)+1)
	}

	// Check if device ID already exists
	if _, exists := m.devices[deviceID]; exists {
		return nil, fmt.Errorf("device with ID %s already exists", deviceID)
	}

	// Create a new device store
	deviceStore := m.container.NewDevice()
	client := whatsmeow.NewClient(deviceStore, m.logger)

	// Setup event handlers
	m.setupEventHandlers(deviceID, client)

	// Get QR channel
	qrChan, err := client.GetQRChannel(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get QR channel: %w", err)
	}

	// Connect to WhatsApp
	err = client.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	device := &Device{
		ID:     deviceID,
		Client: client,
		Status: "pending",
	}

	// Publish device created event
	m.publishEvent("device.created", DeviceCreatedEvent{
		DeviceID: deviceID,
		Status:   "pending",
	})

	// Wait for QR code
	go func() {
		for evt := range qrChan {
			if evt.Event == "code" {
				// Generate QR code PNG image
				qrPNG, err := qrcode.Encode(evt.Code, qrcode.High, 1024)
				if err != nil {
					m.logger.Errorf("Failed to generate QR code: %v", err)
					continue
				}
				device.QRCode = base64.StdEncoding.EncodeToString(qrPNG)
				device.Status = "qr_ready"

				// Publish QR code ready event
				m.publishEvent("device.qr_ready", DeviceQRReadyEvent{
					DeviceID: deviceID,
					Status:   "qr_ready",
				})
			} else if evt.Event == "success" {
				device.Status = "connected"
				m.mu.Lock()
				m.devices[device.ID] = device
				m.mu.Unlock()

				// Save device mapping to database
				deviceJID := ""
				if device.Client.Store.ID != nil {
					deviceJID = device.Client.Store.ID.User
					if err := m.deviceDB.SaveMapping(deviceID, deviceJID); err != nil {
						m.logger.Errorf("Failed to save device mapping: %v", err)
					} else {
						m.logger.Infof("Saved device mapping: %s -> %s", deviceID, deviceJID)
					}
				}

				// Publish device connected event
				m.publishEvent("device.connected", DeviceConnectedEvent{
					DeviceID:  deviceID,
					DeviceJID: deviceJID,
					Status:    "connected",
				})
			}
		}
	}()

	// Wait for initial QR code
	for evt := range qrChan {
		if evt.Event == "code" {
			// Generate QR code PNG image
			qrPNG, err := qrcode.Encode(evt.Code, qrcode.High, 1024)
			if err != nil {
				m.logger.Errorf("Failed to generate QR code: %v", err)
				continue
			}
			device.QRCode = base64.StdEncoding.EncodeToString(qrPNG)
			device.Status = "qr_ready"
			break
		}
	}

	return device, nil
}

// publishEvent publishes an event to the event bus
func (m *Manager) publishEvent(topic string, payload interface{}) {
	if m.publisher == nil {
		return
	}

	data, err := json.Marshal(payload)
	if err != nil {
		m.logger.Errorf("Failed to marshal event payload: %v", err)
		return
	}

	msg := message.NewMessage(fmt.Sprintf("%d", len(m.devices)), data)
	if err := m.publisher.Publish(topic, msg); err != nil {
		m.logger.Errorf("Failed to publish event: %v", err)
	}
}

// GetDevices returns all connected devices
func (m *Manager) GetDevices() []*Device {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]*Device, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, device)
	}
	return devices
}

// GetDevice returns a device by ID
func (m *Manager) GetDevice(id string) (*Device, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	return device, ok
}

// GetGroupInfo retrieves group information for a given device and group JID
func (m *Manager) GetGroupInfo(deviceID string, groupJID string) (*types.GroupInfo, error) {
	m.mu.RLock()
	device, ok := m.devices[deviceID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("device not found: %s", deviceID)
	}

	if device.Status != "connected" {
		return nil, fmt.Errorf("device not connected: %s", deviceID)
	}

	// Parse the group JID
	jid, err := types.ParseJID(groupJID)
	if err != nil {
		return nil, fmt.Errorf("invalid group JID: %w", err)
	}

	// Get group info from WhatsApp
	ctx := context.Background()
	groupInfo, err := device.Client.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, fmt.Errorf("failed to get group info: %w", err)
	}

	return groupInfo, nil
}

// RestoreDevices recovers all previously connected devices from the database
// This should be called on server startup to restore device connections
func (m *Manager) RestoreDevices(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get all device mappings from the database
	mappings, err := m.deviceDB.GetAllMappings()
	if err != nil {
		return fmt.Errorf("failed to get device mappings: %w", err)
	}

	m.logger.Infof("Found %d device mapping(s), attempting to restore connections", len(mappings))

	// Get all devices from WhatsApp store
	allDevices, err := m.container.GetAllDevices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get devices from WhatsApp store: %w", err)
	}

	// Create a map of JID to device store for quick lookup
	deviceStoreMap := make(map[string]*store.Device)
	for _, deviceStore := range allDevices {
		if deviceStore.ID != nil {
			deviceStoreMap[deviceStore.ID.User] = deviceStore
		}
	}

	// Restore devices based on mappings
	for _, mapping := range mappings {
		deviceStore, exists := deviceStoreMap[mapping.DeviceJID]
		if !exists {
			m.logger.Warnf("Device mapping found but WhatsApp store missing: %s -> %s", mapping.DeviceID, mapping.DeviceJID)
			continue
		}

		// Skip if device has no JID (never logged in)
		if deviceStore.ID == nil {
			m.logger.Debugf("Skipping device %s (never logged in)", mapping.DeviceID)
			continue
		}

		// Create client for the stored device
		client := whatsmeow.NewClient(deviceStore, m.logger)

		// Setup event handlers
		m.setupEventHandlers(mapping.DeviceID, client)

		// Connect to WhatsApp
		err := client.Connect()
		if err != nil {
			m.logger.Errorf("Failed to connect device %s: %v", mapping.DeviceID, err)

			// Publish failed reconnection event
			m.publishEvent("device.restore_failed", DeviceRestoreFailedEvent{
				DeviceID: mapping.DeviceID,
				Error:    err.Error(),
			})
			continue
		}

		// Create device object
		device := &Device{
			ID:     mapping.DeviceID,
			Client: client,
			Status: "connected",
		}

		m.devices[mapping.DeviceID] = device
		m.logger.Infof("Successfully restored device: %s (JID: %s)", mapping.DeviceID, mapping.DeviceJID)

		// Publish device restored event
		m.publishEvent("device.restored", DeviceRestoredEvent{
			DeviceID:  mapping.DeviceID,
			DeviceJID: mapping.DeviceJID,
			Status:    "connected",
		})
	}

	m.logger.Infof("Device restoration complete. %d device(s) now active", len(m.devices))
	return nil
}

// Close closes all devices and the manager
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, device := range m.devices {
		device.Client.Disconnect()
	}

	// Close device database
	if m.deviceDB != nil {
		if err := m.deviceDB.Close(); err != nil {
			return fmt.Errorf("failed to close device database: %w", err)
		}
	}

	return nil
}

// SendTextMessageRequest represents a text message request
type SendTextMessageRequest struct {
	DeviceID       string `json:"device_id"`
	ChatID         string `json:"chat_id"`
	Message        string `json:"message"`
	ReplyMessageID string `json:"reply_message_id,omitempty"`
	IsForwarded    bool   `json:"is_forwarded"`
	Duration       uint32 `json:"duration,omitempty"`
}

// SendImageMessageRequest represents an image message request
type SendImageMessageRequest struct {
	DeviceID       string `json:"device_id"`
	ChatID         string `json:"chat_id"`
	FileURL        string `json:"file_url"`
	Caption        string `json:"caption,omitempty"`
	ReplyMessageID string `json:"reply_message_id,omitempty"`
	IsForwarded    bool   `json:"is_forwarded"`
	ViewOnce       bool   `json:"view_once"`
	Duration       uint32 `json:"duration,omitempty"`
}

// SendFileMessageRequest represents a file message request
type SendFileMessageRequest struct {
	DeviceID       string `json:"device_id"`
	ChatID         string `json:"chat_id"`
	FileURL        string `json:"file_url"`
	Caption        string `json:"caption,omitempty"`
	ReplyMessageID string `json:"reply_message_id,omitempty"`
	IsForwarded    bool   `json:"is_forwarded"`
	Duration       uint32 `json:"duration,omitempty"`
}

// SendPresenceRequest represents a presence update request
type SendPresenceRequest struct {
	DeviceID    string `json:"device_id"`
	ChatID      string `json:"chat_id"`
	IsForwarded bool   `json:"is_forwarded"`
}

// SendReactionRequest represents a reaction request
type SendReactionRequest struct {
	DeviceID    string `json:"device_id"`
	ChatID      string `json:"chat_id"`
	MessageID   string `json:"message_id"`
	Emoji       string `json:"emoji"`
	IsForwarded bool   `json:"is_forwarded"`
}

// FetchMessageHistoryRequest represents a request to fetch message history
type FetchMessageHistoryRequest struct {
	DeviceID        string `json:"device_id"`
	ChatID          string `json:"chat_id"`
	BeforeMessageID string `json:"before_message_id,omitempty"` // Message ID to fetch before (for pagination)
	Count           int    `json:"count"`                       // Number of messages to fetch (recommended: 50)
}

// MessageResponse represents the response for message operations
type MessageResponse struct {
	MessageID string `json:"message_id"`
}

// MessageHistoryResponse represents the response for message history requests
type MessageHistoryResponse struct {
	RequestID string           `json:"request_id"` // Unique ID to track this request
	Messages  []MessagePayload `json:"messages"`
	Count     int              `json:"count"`
}

// SendTextMessage sends a text message
func (m *Manager) SendTextMessage(ctx context.Context, req SendTextMessageRequest) (*MessageResponse, error) {
	device, ok := m.GetDevice(req.DeviceID)
	if !ok {
		return nil, fmt.Errorf("device not found: %s", req.DeviceID)
	}

	jid, err := types.ParseJID(req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	msg := &waProto.Message{
		Conversation: proto.String(req.Message),
	}

	resp, err := device.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		m.publishEvent("message.text.failed", MessageTextFailedEvent{
			DeviceID: req.DeviceID,
			ChatID:   req.ChatID,
			Error:    err.Error(),
		})
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Publish event for message sent from HTTP handler
	m.publishEvent("message.text.sent", MessageTextSentEvent{
		DeviceID:  req.DeviceID,
		ChatID:    req.ChatID,
		MessageID: resp.ID,
		Source:    "http",
	})

	return &MessageResponse{
		MessageID: resp.ID,
	}, nil
}

// SendImageMessage sends an image message
func (m *Manager) SendImageMessage(ctx context.Context, req SendImageMessageRequest) (*MessageResponse, error) {
	device, ok := m.GetDevice(req.DeviceID)
	if !ok {
		return nil, fmt.Errorf("device not found: %s", req.DeviceID)
	}

	jid, err := types.ParseJID(req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Download image from URL
	httpResp, err := http.Get(req.FileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}
	defer httpResp.Body.Close()

	imageData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	// Upload image to WhatsApp
	uploaded, err := device.Client.Upload(ctx, imageData, whatsmeow.MediaImage)
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(req.Caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(httpResp.Header.Get("Content-Type")),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(imageData))),
			ViewOnce:      proto.Bool(req.ViewOnce),
		},
	}

	resp, err := device.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		m.publishEvent("message.image.failed", MessageImageFailedEvent{
			DeviceID: req.DeviceID,
			ChatID:   req.ChatID,
			Error:    err.Error(),
		})
		return nil, fmt.Errorf("failed to send image: %w", err)
	}

	// Publish event for image message sent from HTTP handler
	m.publishEvent("message.image.sent", MessageImageSentEvent{
		DeviceID:  req.DeviceID,
		ChatID:    req.ChatID,
		MessageID: resp.ID,
		Source:    "http",
	})

	return &MessageResponse{
		MessageID: resp.ID,
	}, nil
}

// SendFileMessage sends a file message
func (m *Manager) SendFileMessage(ctx context.Context, req SendFileMessageRequest) (*MessageResponse, error) {
	device, ok := m.GetDevice(req.DeviceID)
	if !ok {
		return nil, fmt.Errorf("device not found: %s", req.DeviceID)
	}

	jid, err := types.ParseJID(req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Download file from URL
	httpResp, err := http.Get(req.FileURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer httpResp.Body.Close()

	fileData, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Upload file to WhatsApp
	uploaded, err := device.Client.Upload(ctx, fileData, whatsmeow.MediaDocument)
	if err != nil {
		return nil, fmt.Errorf("failed to upload file: %w", err)
	}

	msg := &waProto.Message{
		DocumentMessage: &waProto.DocumentMessage{
			Caption:       proto.String(req.Caption),
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(httpResp.Header.Get("Content-Type")),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(fileData))),
		},
	}

	resp, err := device.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		m.publishEvent("message.file.failed", MessageFileFailedEvent{
			DeviceID: req.DeviceID,
			ChatID:   req.ChatID,
			Error:    err.Error(),
		})
		return nil, fmt.Errorf("failed to send file: %w", err)
	}

	// Publish event for file message sent from HTTP handler
	m.publishEvent("message.file.sent", MessageFileSentEvent{
		DeviceID:  req.DeviceID,
		ChatID:    req.ChatID,
		MessageID: resp.ID,
		Source:    "http",
	})

	return &MessageResponse{
		MessageID: resp.ID,
	}, nil
}

// SendPresence sends a presence update (typing indicator)
func (m *Manager) SendPresence(ctx context.Context, req SendPresenceRequest) error {
	device, ok := m.GetDevice(req.DeviceID)
	if !ok {
		return fmt.Errorf("device not found: %s", req.DeviceID)
	}

	jid, err := types.ParseJID(req.ChatID)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	err = device.Client.SendPresence(ctx, types.PresenceAvailable)
	if err != nil {
		m.publishEvent("message.presence.failed", MessagePresenceFailedEvent{
			DeviceID: req.DeviceID,
			ChatID:   req.ChatID,
			Error:    err.Error(),
		})
		return fmt.Errorf("failed to send presence: %w", err)
	}

	err = device.Client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	if err != nil {
		m.publishEvent("message.presence.failed", MessagePresenceFailedEvent{
			DeviceID: req.DeviceID,
			ChatID:   req.ChatID,
			Error:    err.Error(),
		})
		return fmt.Errorf("failed to send chat presence: %w", err)
	}

	m.publishEvent("message.presence.sent", MessagePresenceSentEvent{
		DeviceID: req.DeviceID,
		ChatID:   req.ChatID,
	})

	return nil
}

// SendReaction sends a reaction to a message
func (m *Manager) SendReaction(ctx context.Context, req SendReactionRequest) (*MessageResponse, error) {
	device, ok := m.GetDevice(req.DeviceID)
	if !ok {
		return nil, fmt.Errorf("device not found: %s", req.DeviceID)
	}

	jid, err := types.ParseJID(req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	msg := &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(req.ChatID),
				FromMe:    proto.Bool(false),
				ID:        proto.String(req.MessageID),
			},
			Text:              proto.String(req.Emoji),
			SenderTimestampMS: proto.Int64(0),
		},
	}

	resp, err := device.Client.SendMessage(ctx, jid, msg)
	if err != nil {
		m.publishEvent("message.reaction.failed", MessageReactionFailedEvent{
			DeviceID:  req.DeviceID,
			ChatID:    req.ChatID,
			MessageID: req.MessageID,
			Error:     err.Error(),
		})
		return nil, fmt.Errorf("failed to send reaction: %w", err)
	}

	m.publishEvent("message.reaction.sent", MessageReactionSentEvent{
		DeviceID:        req.DeviceID,
		ChatID:          req.ChatID,
		TargetMessage:   req.MessageID,
		ReactionMessage: resp.ID,
	})

	return &MessageResponse{
		MessageID: resp.ID,
	}, nil
}

// FetchMessageHistory requests message history from WhatsApp
// This method sends a history sync request to the user's primary WhatsApp device.
// The response will come asynchronously as events.HistorySync events.
//
// Note: This feature has limitations:
// - Only works if you have an active primary device (phone) connected
// - The primary device must have the message history available
// - Response time depends on network and primary device availability
// - Recommended batch size is 50 messages per request
//
// For pagination, use the BeforeMessageID field to fetch messages before a specific message.
func (m *Manager) FetchMessageHistory(ctx context.Context, req FetchMessageHistoryRequest) (*MessageHistoryResponse, error) {
	device, ok := m.GetDevice(req.DeviceID)
	if !ok {
		return nil, fmt.Errorf("device not found: %s", req.DeviceID)
	}

	if device.Status != "connected" {
		return nil, fmt.Errorf("device not connected: %s", req.DeviceID)
	}

	// Validate count (recommend 50, max 100 for safety)
	if req.Count <= 0 {
		req.Count = 50
	} else if req.Count > 100 {
		req.Count = 100
	}

	// Parse chat JID
	chatJID, err := types.ParseJID(req.ChatID)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Generate unique request ID for tracking
	requestID := uuid.New().String()

	// Construct MessageInfo for the history sync request
	var messageInfo *types.MessageInfo

	if req.BeforeMessageID != "" {
		// If BeforeMessageID is provided, create a MessageInfo for that message
		// Note: In a production system, you'd want to retrieve this from your message store
		// For now, we create a minimal MessageInfo with the message ID and chat
		messageInfo = &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat: chatJID,
			},
			ID: req.BeforeMessageID,
			// Timestamp is optional for history sync - WhatsApp will use the message ID
		}
	} else {
		// If no BeforeMessageID, fetch from the beginning
		// Create a MessageInfo with current timestamp to fetch most recent messages
		messageInfo = &types.MessageInfo{
			MessageSource: types.MessageSource{
				Chat: chatJID,
			},
			ID:        "", // Empty ID means fetch most recent
			Timestamp: time.Now(),
		}
	}

	// Build the history sync request message
	historySyncMsg := device.Client.BuildHistorySyncRequest(messageInfo, req.Count)

	// Publish event that history sync was requested
	m.publishEvent("message.history.requested", MessageHistoryRequestedEvent{
		DeviceID:        req.DeviceID,
		ChatID:          req.ChatID,
		BeforeMessageID: req.BeforeMessageID,
		Count:           req.Count,
		RequestID:       requestID,
	})

	// Send the history sync request to the user's own JID (primary device)
	// The whatsmeow.SendRequestExtra{Peer: true} tells WhatsApp to route this to the primary device
	ownJID := device.Client.Store.ID.ToNonAD()
	_, err = device.Client.SendMessage(ctx, ownJID, historySyncMsg, whatsmeow.SendRequestExtra{Peer: true})
	if err != nil {
		// Publish failure event
		m.publishEvent("message.history.failed", MessageHistoryFailedEvent{
			DeviceID:  req.DeviceID,
			ChatID:    req.ChatID,
			RequestID: requestID,
			Error:     err.Error(),
		})
		return nil, fmt.Errorf("failed to send history sync request: %w", err)
	}

	// Return response indicating the request was sent
	// Note: The actual history will come via events.HistorySync events
	// In a production system, you'd want to:
	// 1. Store the requestID for correlation
	// 2. Set up a timeout/channel to wait for the response
	// 3. Return the actual messages once received
	//
	// For now, we return a response indicating the request was initiated
	return &MessageHistoryResponse{
		RequestID: requestID,
		Messages:  []MessagePayload{}, // Empty for now - will be populated via events
		Count:     0,
	}, nil
}

// Media Download Constants
const (
	// MaxMediaSize is the maximum allowed size for downloaded media (10MB)
	MaxMediaSize = 10 * 1024 * 1024
)

// detectMimetype detects the MIME type of media data using magic bytes
// Returns the detected MIME type or "application/octet-stream" if unknown
func detectMimetype(data []byte) string {
	if len(data) < 12 {
		return "application/octet-stream"
	}

	// Check magic bytes for common media types
	// JPEG: FF D8 FF
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 &&
		data[4] == 0x0D && data[5] == 0x0A && data[6] == 0x1A && data[7] == 0x0A {
		return "image/png"
	}

	// GIF: 47 49 46 38 (GIF8)
	if len(data) >= 4 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "image/gif"
	}

	// WebP: 52 49 46 46 ... 57 45 42 50 (RIFF...WEBP)
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	// MP4: Check for ftyp box (starts at byte 4)
	// Format: ... 66 74 79 70 (ftyp)
	if len(data) >= 12 {
		for i := 0; i < 8 && i+7 < len(data); i++ {
			if data[i] == 0x66 && data[i+1] == 0x74 && data[i+2] == 0x79 && data[i+3] == 0x70 {
				return "video/mp4"
			}
		}
	}

	// PDF: 25 50 44 46 (%PDF)
	if len(data) >= 4 && data[0] == 0x25 && data[1] == 0x50 && data[2] == 0x44 && data[3] == 0x46 {
		return "application/pdf"
	}

	// OGG: 4F 67 67 53 (OggS)
	if len(data) >= 4 && data[0] == 0x4F && data[1] == 0x67 && data[2] == 0x67 && data[3] == 0x53 {
		// Could be audio or video, but WhatsApp typically uses it for audio
		return "audio/ogg"
	}

	// MP3: Check for ID3 header (49 44 33) or MPEG frame sync (FF FB or FF F3)
	if len(data) >= 3 {
		if data[0] == 0x49 && data[1] == 0x44 && data[2] == 0x33 {
			return "audio/mpeg"
		}
		if data[0] == 0xFF && (data[1] == 0xFB || data[1] == 0xF3) {
			return "audio/mpeg"
		}
	}

	return "application/octet-stream"
}

// getExtensionFromMimetype returns the file extension for a given MIME type
func getExtensionFromMimetype(mimetype string) string {
	switch mimetype {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "video/3gpp":
		return ".3gp"
	case "audio/ogg":
		return ".ogg"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4":
		return ".m4a"
	case "audio/aac":
		return ".aac"
	case "application/pdf":
		return ".pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return ".docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return ".xlsx"
	case "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return ".pptx"
	case "application/msword":
		return ".doc"
	case "application/vnd.ms-excel":
		return ".xls"
	case "application/vnd.ms-powerpoint":
		return ".ppt"
	case "application/zip":
		return ".zip"
	case "application/x-rar-compressed":
		return ".rar"
	case "text/plain":
		return ".txt"
	default:
		return ".bin"
	}
}

// downloadMedia downloads media from WhatsApp and returns MediaData struct
// This method handles downloading, size validation, mimetype detection, and SHA256 calculation
// Parameters:
//   - ctx: Context for cancellation
//   - deviceID: Device identifier for logging
//   - client: WhatsApp client for downloading
//   - msg: WhatsApp message containing downloadable media
//   - baseFilename: Base filename (without extension) to use for the media
//
// Returns MediaData struct with downloaded bytes and metadata, or error if download fails
func (m *Manager) downloadMedia(ctx context.Context, deviceID string, client *whatsmeow.Client, msg whatsmeow.DownloadableMessage, baseFilename string) (*MediaData, error) {
	startTime := time.Now()

	// Download media from WhatsApp
	m.logger.Debugf("Downloading media for device %s (filename: %s)", deviceID, baseFilename)
	data, err := client.Download(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to download media: %w", err)
	}

	downloadDuration := time.Since(startTime)
	m.logger.Debugf("Media download completed in %v, size: %d bytes", downloadDuration, len(data))

	// Enforce size limit
	if int64(len(data)) > MaxMediaSize {
		return nil, fmt.Errorf("media size %d bytes exceeds maximum allowed size of %d bytes", len(data), MaxMediaSize)
	}

	// Detect MIME type from content
	detectedMimetype := detectMimetype(data)
	m.logger.Debugf("Detected MIME type: %s", detectedMimetype)

	// Get file extension
	extension := getExtensionFromMimetype(detectedMimetype)
	filename := baseFilename + extension

	// Calculate SHA256 hash
	hash := sha256.Sum256(data)
	sha256Hex := hex.EncodeToString(hash[:])

	// Create MediaData struct
	mediaData := &MediaData{
		Data:     data,
		Mimetype: detectedMimetype,
		Filename: filename,
		Size:     int64(len(data)),
		SHA256:   sha256Hex,
	}

	m.logger.Debugf("Media processed successfully: filename=%s, size=%d, sha256=%s", filename, mediaData.Size, sha256Hex[:16]+"...")

	return mediaData, nil
}
