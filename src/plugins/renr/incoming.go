package renr

import (
	"encoding/json"
	"fmt"
	"strconv"

	"whatsapp/src/core/whatsapp"
	renrqueue "whatsapp/src/renr-queue"

	"github.com/ThreeDotsLabs/watermill/message"
)

// handleIncomingMessage processes incoming WhatsApp messages and enqueues them
func (p *Plugin) handleIncomingMessage(msg *message.Message) error {
	var event whatsapp.MessageReceivedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		p.logger.Errorf("Failed to unmarshal message event: %v", err)
		return nil // Don't retry, just log
	}

	p.logger.Debugf("Received message event: device=%s, from=%s (%s), chat=%s, type=%s, is_group=%v",
		event.DeviceID, event.Message.SenderName, event.Message.Sender, event.Message.ChatName, event.Message.Type, event.Message.IsGroup)

	// Convert to QueueChatMessage
	queueMsg, err := p.convertToQueueChatMessage(event)
	if err != nil {
		p.logger.Errorf("Failed to convert message: device=%s, error=%v", event.DeviceID, err)
		return nil // Don't retry, just log
	}

	// Marshal to JSON
	payload, err := json.Marshal(queueMsg)
	if err != nil {
		p.logger.Errorf("Failed to marshal message: device=%s, error=%v", event.DeviceID, err)
		return nil // Don't retry
	}

	// Enqueue using persistence layer if available
	if p.deliveryService != nil {
		err = p.deliveryService.Enqueue(event.DeviceID, event.Message.MessageID, "channel-chat-message-incoming", string(payload))
		if err != nil {
			p.logger.Errorf("Failed to persist incoming message: device=%s, error=%v", event.DeviceID, err)
			// Fallback to direct enqueue
			p.logger.Warnf("Falling back to direct enqueue (no persistence): device=%s", event.DeviceID)
			ref := event.DeviceID
			_, err = p.queueClient.Enqueue("channel-chat-message-incoming", renrqueue.EnqueueRequest{
				Ref:     &ref,
				Payload: string(payload),
			})
			if err != nil {
				p.logger.Errorf("Direct enqueue also failed: device=%s, error=%v", event.DeviceID, err)
			}
		}
	} else {
		// No persistence, direct enqueue
		ref := event.DeviceID
		_, err = p.queueClient.Enqueue("channel-chat-message-incoming", renrqueue.EnqueueRequest{
			Ref:     &ref,
			Payload: string(payload),
		})
		if err != nil {
			p.logger.Errorf("Failed to enqueue incoming message (no persistence): device=%s, error=%v", event.DeviceID, err)
			return nil // Don't retry, just log
		}
		p.logger.Infof("Enqueued incoming message (no persistence): device=%s, from=%s (%s), chat=%s, type=%s, msg_id=%s",
			event.DeviceID, queueMsg.From.Name, queueMsg.From.Phone, queueMsg.To.Name, queueMsg.Body.Type, queueMsg.MessageID)
	}

	return nil
}

// convertToQueueChatMessage converts MessageReceivedEvent to QueueChatMessage
// All names are pre-resolved by the Manager's extractMessagePayload method which:
// - For SenderName: Uses contact's FullName, falling back to PushName
// - For ChatName in groups: Uses group name from WhatsApp
// - For ChatName in DMs: Uses contact's FullName, falling back to PushName or SenderName
func (p *Plugin) convertToQueueChatMessage(event whatsapp.MessageReceivedEvent) (*QueueChatMessage, error) {
	// Get channel ID from device ID
	channelID, err := strconv.ParseInt(event.DeviceID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid device ID: %w", err)
	}

	// Extract from (sender) - name already resolved by Manager
	from := QueueChatPerson{
		ID:    formatPhoneToJID(extractPhoneFromJID(event.Message.Sender)),
		Name:  event.Message.SenderName, // Pre-resolved: FullName > PushName
		Phone: extractPhoneFromJID(event.Message.Sender),
		Email: "",
	}

	// Extract to (recipient - the chat/group) - name already resolved by Manager
	var to QueueChatPerson
	if event.Message.IsGroup {
		// For group messages, 'to' is the group with its resolved name
		to = QueueChatPerson{
			ID:    event.Message.ChatID,
			Name:  event.Message.ChatName, // Pre-resolved group name from WhatsApp
			Phone: "",                     // Groups don't have phone numbers
			Email: "",
		}
	} else {
		// For direct messages, 'to' is the chat recipient with resolved contact name
		to = QueueChatPerson{
			ID:    event.Message.ChatID,
			Name:  event.Message.ChatName, // Pre-resolved: FullName > PushName > SenderName
			Phone: extractPhoneFromJID(event.Message.ChatID),
			Email: "",
		}
	}

	// Build body based on message type
	var body QueueChatBody
	var replyTo *string

	switch event.Message.Type {
	case "text":
		if event.Message.Text != nil {
			body = QueueChatBody{
				Type:    "text",
				Content: event.Message.Text.Content,
			}
		}

	case "image":
		if event.Message.Image != nil {
			var mediaURL string

			// NEW: Prefer Media.Data if available (Phase 1 implementation)
			if event.Message.Image.Media != nil && len(event.Message.Image.Media.Data) > 0 {
				p.logger.Debugf("📥 Using Media.Data for image (size: %d bytes)",
					len(event.Message.Image.Media.Data))
				mediaURL = p.uploadMediaToS3(
					event.Message.Image.Media.Data,
					event.Message.Image.Media.Filename,
					event.Message.Image.Media.Mimetype,
					event.DeviceID,
				)
			} else if event.Message.Image.URL != "" {
				// FALLBACK: Download from URL (backward compatibility)
				p.logger.Warnf("⚠️  Media.Data not available, using legacy URL download")
				mediaURL = p.handleMediaLegacy(event.Message.Image.URL)
			} else {
				p.logger.Errorf("❌ No media data or URL available for image")
			}

			body = QueueChatBody{
				Type:     "image",
				Content:  event.Message.Image.Caption,
				FilesURL: []string{mediaURL},
			}
		}

	case "document":
		if event.Message.Document != nil {
			var mediaURL string

			if event.Message.Document.Media != nil && len(event.Message.Document.Media.Data) > 0 {
				p.logger.Debugf("📥 Using Media.Data for document (size: %d bytes)",
					len(event.Message.Document.Media.Data))
				mediaURL = p.uploadMediaToS3(
					event.Message.Document.Media.Data,
					event.Message.Document.Media.Filename,
					event.Message.Document.Media.Mimetype,
					event.DeviceID,
				)
			} else if event.Message.Document.URL != "" {
				p.logger.Warnf("⚠️  Media.Data not available, using legacy URL download")
				mediaURL = p.handleMediaLegacy(event.Message.Document.URL)
			}

			body = QueueChatBody{
				Type:     "document",
				Content:  event.Message.Document.Caption,
				FilesURL: []string{mediaURL},
			}
		}

	case "video":
		if event.Message.Video != nil {
			var mediaURL string

			if event.Message.Video.Media != nil && len(event.Message.Video.Media.Data) > 0 {
				p.logger.Debugf("📥 Using Media.Data for video (size: %d bytes)",
					len(event.Message.Video.Media.Data))
				mediaURL = p.uploadMediaToS3(
					event.Message.Video.Media.Data,
					event.Message.Video.Media.Filename,
					event.Message.Video.Media.Mimetype,
					event.DeviceID,
				)
			} else if event.Message.Video.URL != "" {
				p.logger.Warnf("⚠️  Media.Data not available, using legacy URL download")
				mediaURL = p.handleMediaLegacy(event.Message.Video.URL)
			}

			body = QueueChatBody{
				Type:     "video",
				Content:  event.Message.Video.Caption,
				FilesURL: []string{mediaURL},
			}
		}

	case "audio":
		if event.Message.Audio != nil {
			var mediaURL string

			if event.Message.Audio.Media != nil && len(event.Message.Audio.Media.Data) > 0 {
				p.logger.Debugf("📥 Using Media.Data for audio (size: %d bytes)",
					len(event.Message.Audio.Media.Data))
				mediaURL = p.uploadMediaToS3(
					event.Message.Audio.Media.Data,
					event.Message.Audio.Media.Filename,
					event.Message.Audio.Media.Mimetype,
					event.DeviceID,
				)
			} else if event.Message.Audio.URL != "" {
				p.logger.Warnf("⚠️  Media.Data not available, using legacy URL download")
				mediaURL = p.handleMediaLegacy(event.Message.Audio.URL)
			}

			body = QueueChatBody{
				Type:     "audio",
				Content:  "",
				FilesURL: []string{mediaURL},
			}
		}

	case "sticker":
		if event.Message.Sticker != nil {
			var mediaURL string

			if event.Message.Sticker.Media != nil && len(event.Message.Sticker.Media.Data) > 0 {
				p.logger.Debugf("📥 Using Media.Data for sticker (size: %d bytes)",
					len(event.Message.Sticker.Media.Data))

				// Use intelligent filename detection for stickers
				// Some stickers are zip files but come as .bin, this fixes the extension
				stickerFilename := getStickerFilename(
					event.Message.Sticker.Media.Filename,
					event.Message.Sticker.Media.Mimetype,
				)

				p.logger.Debugf("🎨 Sticker filename: %s (mimetype: %s, original: %s)",
					stickerFilename,
					event.Message.Sticker.Media.Mimetype,
					event.Message.Sticker.Media.Filename)

				mediaURL = p.uploadMediaToS3(
					event.Message.Sticker.Media.Data,
					stickerFilename,
					event.Message.Sticker.Media.Mimetype,
					event.DeviceID,
				)
			} else if event.Message.Sticker.URL != "" {
				p.logger.Warnf("⚠️  Media.Data not available, using legacy URL download")
				mediaURL = p.handleMediaLegacy(event.Message.Sticker.URL)
			}

			body = QueueChatBody{
				Type:     "sticker",
				Content:  "",
				FilesURL: []string{mediaURL},
			}
		}

	case "contact":
		if event.Message.Contact != nil {
			body = QueueChatBody{
				Type:    "contact",
				Content: event.Message.Contact.DisplayName,
				Contact: &ContactData{
					DisplayName: event.Message.Contact.DisplayName,
					VCard:       event.Message.Contact.VCard,
				},
			}
		}

	case "contacts":
		if event.Message.Contacts != nil {
			body = QueueChatBody{
				Type:    "contacts",
				Content: event.Message.Contacts.DisplayName,
				Contacts: &ContactsData{
					DisplayName: event.Message.Contacts.DisplayName,
					Contacts:    event.Message.Contacts.Contacts,
				},
			}
		}

	case "location":
		if event.Message.Location != nil {
			body = QueueChatBody{
				Type:    "location",
				Content: event.Message.Location.Name,
				Location: &LocationData{
					Latitude:  event.Message.Location.Latitude,
					Longitude: event.Message.Location.Longitude,
					Name:      event.Message.Location.Name,
					Address:   event.Message.Location.Address,
					URL:       event.Message.Location.URL,
					IsLive:    false,
				},
			}
		}

	case "live_location":
		if event.Message.Location != nil {
			body = QueueChatBody{
				Type:    "live_location",
				Content: event.Message.Location.Name,
				Location: &LocationData{
					Latitude:  event.Message.Location.Latitude,
					Longitude: event.Message.Location.Longitude,
					Name:      event.Message.Location.Name,
					IsLive:    true,
				},
			}
		}

	case "reaction":
		if event.Message.Reaction != nil {
			emoji := event.Message.Reaction.Emoji
			action := "added"
			if emoji == "" {
				action = "removed"
			}
			body = QueueChatBody{
				Type:    "reaction",
				Content: fmt.Sprintf("Reaction %s: %s", action, emoji),
				Reaction: &ReactionData{
					Emoji:           emoji,
					TargetMessageID: event.Message.Reaction.TargetMessageID,
					TargetSender:    event.Message.Reaction.TargetSender,
				},
			}
		}

	case "protocol":
		if event.Message.Protocol != nil {
			// Only forward revoke (delete) messages, skip other protocol noise
			if event.Message.Protocol.Type == "revoke" {
				body = QueueChatBody{
					Type:    "protocol",
					Content: fmt.Sprintf("Message deleted: %s", event.Message.Protocol.TargetMessageID),
					Protocol: &ProtocolData{
						ProtocolType:    event.Message.Protocol.Type,
						TargetMessageID: event.Message.Protocol.TargetMessageID,
					},
				}
			} else {
				// Skip non-revoke protocol messages
				p.logger.Debugf("Skipping protocol message type: %s", event.Message.Protocol.Type)
				return nil, fmt.Errorf("skipping protocol message type: %s", event.Message.Protocol.Type)
			}
		}

	case "poll":
		if event.Message.Poll != nil {
			options := make([]PollOption, len(event.Message.Poll.Options))
			for i, opt := range event.Message.Poll.Options {
				options[i] = PollOption{Name: opt.Name}
			}
			body = QueueChatBody{
				Type:    "poll",
				Content: event.Message.Poll.Name,
				Poll: &PollData{
					Name:            event.Message.Poll.Name,
					Options:         options,
					SelectableCount: event.Message.Poll.SelectableCount,
				},
			}
		}

	case "poll_vote":
		if event.Message.PollVote != nil {
			body = QueueChatBody{
				Type:    "poll_vote",
				Content: fmt.Sprintf("Vote on poll: %s", event.Message.PollVote.PollMessageID),
				PollVote: &PollVoteData{
					PollMessageID:   event.Message.PollVote.PollMessageID,
					SelectedOptions: event.Message.PollVote.SelectedOptions,
				},
			}
		}

	default:
		body = QueueChatBody{
			Type:    "unknown",
			Content: fmt.Sprintf("Unsupported message type: %s", event.Message.Type),
		}
	}

	// TODO: Extract reply_to if available from event
	// For now, leave it nil

	queueMsg := &QueueChatMessage{
		ChannelID: channelID,
		MessageID: event.Message.MessageID,
		Ref:       event.DeviceID,
		To:        to,
		From:      from,
		ReplyTo:   replyTo,
		Body:      body,
		Timestamp: event.Message.Timestamp,
	}

	return queueMsg, nil
}

// handleHistorySynced processes message history synced events and enqueues historical messages
func (p *Plugin) handleHistorySynced(msg *message.Message) error {
	var event whatsapp.MessageHistorySyncedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		p.logger.Errorf("Failed to unmarshal history synced event: %v", err)
		return nil // Don't retry, just log
	}

	p.logger.Infof("Received history synced event: device=%s, count=%d, request_id=%s",
		event.DeviceID, event.Count, event.RequestID)

	// Process each historical message
	successCount := 0
	for _, historyMsg := range event.Messages {
		// Convert MessagePayload to MessageReceivedEvent format
		receivedEvent := whatsapp.MessageReceivedEvent{
			DeviceID: event.DeviceID,
			Message:  historyMsg,
		}

		// Use existing conversion logic
		queueMsg, err := p.convertToQueueChatMessage(receivedEvent)
		if err != nil {
			p.logger.Errorf("Failed to convert history message: device=%s, msg_id=%s, error=%v",
				event.DeviceID, historyMsg.MessageID, err)
			continue // Skip this message, continue with others
		}

		// Marshal to JSON
		payload, err := json.Marshal(queueMsg)
		if err != nil {
			p.logger.Errorf("Failed to marshal history message: device=%s, msg_id=%s, error=%v",
				event.DeviceID, historyMsg.MessageID, err)
			continue
		}

		// Enqueue using persistence layer if available
		if p.deliveryService != nil {
			err = p.deliveryService.Enqueue(event.DeviceID, historyMsg.MessageID, "channel-chat-message-incoming", string(payload))
			if err != nil {
				p.logger.Errorf("Failed to persist history message: device=%s, msg_id=%s, error=%v",
					event.DeviceID, historyMsg.MessageID, err)
				// Fallback to direct enqueue
				ref := event.DeviceID
				_, err = p.queueClient.Enqueue("channel-chat-message-incoming", renrqueue.EnqueueRequest{
					Ref:     &ref,
					Payload: string(payload),
				})
				if err != nil {
					p.logger.Errorf("Direct enqueue also failed for history message: device=%s, msg_id=%s, error=%v",
						event.DeviceID, historyMsg.MessageID, err)
					continue
				}
			}
		} else {
			// No persistence, direct enqueue
			ref := event.DeviceID
			_, err = p.queueClient.Enqueue("channel-chat-message-incoming", renrqueue.EnqueueRequest{
				Ref:     &ref,
				Payload: string(payload),
			})
			if err != nil {
				p.logger.Errorf("Failed to enqueue history message (no persistence): device=%s, msg_id=%s, error=%v",
					event.DeviceID, historyMsg.MessageID, err)
				continue
			}
		}

		successCount++
	}

	p.logger.Infof("Enqueued %d/%d history messages: device=%s, request_id=%s",
		successCount, event.Count, event.DeviceID, event.RequestID)

	return nil
}
