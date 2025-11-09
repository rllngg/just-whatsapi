package whatsapp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/ThreeDotsLabs/watermill/message"
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
func (m *Manager) extractMessagePayload(v *events.Message) map[string]interface{} {
	payload := map[string]interface{}{
		"message_id": v.Info.ID,
		"chat_id":    v.Info.Chat.String(),
		"sender":     v.Info.Sender.String(),
		"timestamp":  v.Info.Timestamp.Unix(),
		"from_me":    v.Info.IsFromMe,
	}

	// Extract message content based on type
	msg := v.Message
	if msg.Conversation != nil {
		payload["type"] = "text"
		payload["text"] = map[string]interface{}{
			"content": *msg.Conversation,
		}
	} else if msg.ExtendedTextMessage != nil && msg.ExtendedTextMessage.Text != nil {
		payload["type"] = "text"
		payload["text"] = map[string]interface{}{
			"content": *msg.ExtendedTextMessage.Text,
		}
	} else if msg.ImageMessage != nil {
		payload["type"] = "image"
		imageInfo := map[string]interface{}{
			"mimetype": msg.ImageMessage.GetMimetype(),
			"caption":  msg.ImageMessage.GetCaption(),
		}
		if msg.ImageMessage.URL != nil {
			imageInfo["url"] = *msg.ImageMessage.URL
		}
		payload["image"] = imageInfo
		payload["has_media"] = true
	} else if msg.DocumentMessage != nil {
		payload["type"] = "document"
		docInfo := map[string]interface{}{
			"mimetype": msg.DocumentMessage.GetMimetype(),
			"caption":  msg.DocumentMessage.GetCaption(),
			"filename": msg.DocumentMessage.GetFileName(),
		}
		if msg.DocumentMessage.URL != nil {
			docInfo["url"] = *msg.DocumentMessage.URL
		}
		payload["document"] = docInfo
		payload["has_media"] = true
	} else if msg.VideoMessage != nil {
		payload["type"] = "video"
		videoInfo := map[string]interface{}{
			"mimetype": msg.VideoMessage.GetMimetype(),
			"caption":  msg.VideoMessage.GetCaption(),
		}
		if msg.VideoMessage.URL != nil {
			videoInfo["url"] = *msg.VideoMessage.URL
		}
		payload["video"] = videoInfo
		payload["has_media"] = true
	} else if msg.AudioMessage != nil {
		payload["type"] = "audio"
		audioInfo := map[string]interface{}{
			"mimetype": msg.AudioMessage.GetMimetype(),
		}
		if msg.AudioMessage.URL != nil {
			audioInfo["url"] = *msg.AudioMessage.URL
		}
		payload["audio"] = audioInfo
		payload["has_media"] = true
	} else {
		payload["type"] = "unknown"
	}

	return payload
}

// setupEventHandlers sets up event handlers for a WhatsApp client
func (m *Manager) setupEventHandlers(deviceID string, client *whatsmeow.Client) {
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			// Handle incoming message from WhatsApp
			messagePayload := m.extractMessagePayload(v)
			m.publishEvent("message.received", map[string]interface{}{
				"device_id": deviceID,
				"message":   messagePayload,
				"source":    "whatsapp",
			})

		case *events.Disconnected:
			// Handle device disconnect (internet issue)
			m.publishEvent("device.disconnected", map[string]interface{}{
				"device_id": deviceID,
				"reason":    "connection_lost",
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

			m.publishEvent("device.logout", map[string]interface{}{
				"device_id": deviceID,
				"reason":    v.Reason.String(),
			})

		case *events.Connected:
			// Handle successful connection
			m.publishEvent("device.connected", map[string]interface{}{
				"device_id": deviceID,
				"status":    "connected",
			})

		case *events.Receipt:
			// Handle read receipts
			m.publishEvent("message.read", map[string]interface{}{
				"device_id":   deviceID,
				"chat_id":     v.Chat.String(),
				"sender":      v.Sender.String(),
				"message_ids": v.MessageIDs,
				"timestamp":   v.Timestamp.Unix(),
				"type":        v.Type.GoString(),
			})
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
	m.publishEvent("device.created", map[string]interface{}{
		"device_id": deviceID,
		"status":    "pending",
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
				m.publishEvent("device.qr_ready", map[string]interface{}{
					"device_id": deviceID,
					"status":    "qr_ready",
				})
			} else if evt.Event == "success" {
				device.Status = "connected"
				m.mu.Lock()
				m.devices[device.ID] = device
				m.mu.Unlock()

				// Save device mapping to database
				if device.Client.Store.ID != nil {
					deviceJID := device.Client.Store.ID.User
					if err := m.deviceDB.SaveMapping(deviceID, deviceJID); err != nil {
						m.logger.Errorf("Failed to save device mapping: %v", err)
					} else {
						m.logger.Infof("Saved device mapping: %s -> %s", deviceID, deviceJID)
					}
				}

				// Publish device connected event
				m.publishEvent("device.connected", map[string]interface{}{
					"device_id": deviceID,
					"status":    "connected",
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
func (m *Manager) publishEvent(topic string, payload map[string]interface{}) {
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
			m.publishEvent("device.restore_failed", map[string]interface{}{
				"device_id": mapping.DeviceID,
				"error":     err.Error(),
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
		m.publishEvent("device.restored", map[string]interface{}{
			"device_id":  mapping.DeviceID,
			"device_jid": mapping.DeviceJID,
			"status":     "connected",
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

// MessageResponse represents the response for message operations
type MessageResponse struct {
	MessageID string `json:"message_id"`
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
		m.publishEvent("message.text.failed", map[string]interface{}{
			"device_id": req.DeviceID,
			"chat_id":   req.ChatID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Publish event for message sent from HTTP handler
	m.publishEvent("message.text.sent", map[string]interface{}{
		"device_id":  req.DeviceID,
		"chat_id":    req.ChatID,
		"message_id": resp.ID,
		"source":     "http",
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
		m.publishEvent("message.image.failed", map[string]interface{}{
			"device_id": req.DeviceID,
			"chat_id":   req.ChatID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to send image: %w", err)
	}

	// Publish event for image message sent from HTTP handler
	m.publishEvent("message.image.sent", map[string]interface{}{
		"device_id":  req.DeviceID,
		"chat_id":    req.ChatID,
		"message_id": resp.ID,
		"source":     "http",
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
		m.publishEvent("message.file.failed", map[string]interface{}{
			"device_id": req.DeviceID,
			"chat_id":   req.ChatID,
			"error":     err.Error(),
		})
		return nil, fmt.Errorf("failed to send file: %w", err)
	}

	// Publish event for file message sent from HTTP handler
	m.publishEvent("message.file.sent", map[string]interface{}{
		"device_id":  req.DeviceID,
		"chat_id":    req.ChatID,
		"message_id": resp.ID,
		"source":     "http",
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
		m.publishEvent("message.presence.failed", map[string]interface{}{
			"device_id": req.DeviceID,
			"chat_id":   req.ChatID,
			"error":     err.Error(),
		})
		return fmt.Errorf("failed to send presence: %w", err)
	}

	err = device.Client.SendChatPresence(ctx, jid, types.ChatPresenceComposing, types.ChatPresenceMediaText)
	if err != nil {
		m.publishEvent("message.presence.failed", map[string]interface{}{
			"device_id": req.DeviceID,
			"chat_id":   req.ChatID,
			"error":     err.Error(),
		})
		return fmt.Errorf("failed to send chat presence: %w", err)
	}

	m.publishEvent("message.presence.sent", map[string]interface{}{
		"device_id": req.DeviceID,
		"chat_id":   req.ChatID,
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
		m.publishEvent("message.reaction.failed", map[string]interface{}{
			"device_id":  req.DeviceID,
			"chat_id":    req.ChatID,
			"message_id": req.MessageID,
			"error":      err.Error(),
		})
		return nil, fmt.Errorf("failed to send reaction: %w", err)
	}

	m.publishEvent("message.reaction.sent", map[string]interface{}{
		"device_id":        req.DeviceID,
		"chat_id":          req.ChatID,
		"target_message":   req.MessageID,
		"reaction_message": resp.ID,
	})

	return &MessageResponse{
		MessageID: resp.ID,
	}, nil
}
