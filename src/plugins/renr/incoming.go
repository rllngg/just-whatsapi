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

	p.logger.Debugf("Received message event: device=%s, from=%s, type=%s",
		event.DeviceID, event.Message.Sender, event.Message.Type)

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

	// Enqueue
	ref := event.DeviceID
	_, err = p.queueClient.Enqueue("channel-chat-message-incoming", renrqueue.EnqueueRequest{
		Ref:     &ref,
		Payload: string(payload),
	})

	if err != nil {
		p.logger.Errorf("Failed to enqueue incoming message: device=%s, error=%v", event.DeviceID, err)
		return nil // Don't retry, just log
	}

	p.logger.Infof("Enqueued incoming message: device=%s, from=%s, type=%s, msg_id=%s",
		event.DeviceID, queueMsg.From.Phone, queueMsg.Body.Type, queueMsg.MessageID)

	return nil
}

// convertToQueueChatMessage converts MessageReceivedEvent to QueueChatMessage
func (p *Plugin) convertToQueueChatMessage(event whatsapp.MessageReceivedEvent) (*QueueChatMessage, error) {
	// Get channel ID from device ID
	channelID, err := strconv.ParseInt(event.DeviceID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid device ID: %w", err)
	}

	// Extract from (sender)
	from := QueueChatPerson{
		ID:    event.Message.Sender,
		Name:  extractNameFromSender(event.Message.Sender),
		Phone: extractPhoneFromJID(event.Message.Sender),
		Email: "",
	}

	// Extract to (recipient - the chat/group)
	to := QueueChatPerson{
		ID:    event.Message.ChatID,
		Name:  "Me",
		Phone: extractPhoneFromJID(event.Message.ChatID),
		Email: "",
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
			// Handle media - download and upload to S3
			mediaURL := p.handleMedia(event.Message.Image.URL, "image", event.Message.MessageID, event.DeviceID)
			body = QueueChatBody{
				Type:     "image",
				Content:  event.Message.Image.Caption,
				FilesURL: []string{mediaURL},
			}
		}

	case "document":
		if event.Message.Document != nil {
			mediaURL := p.handleMedia(event.Message.Document.URL, "document", event.Message.MessageID, event.DeviceID)
			body = QueueChatBody{
				Type:     "document",
				Content:  event.Message.Document.Caption,
				FilesURL: []string{mediaURL},
			}
		}

	case "video":
		if event.Message.Video != nil {
			mediaURL := p.handleMedia(event.Message.Video.URL, "video", event.Message.MessageID, event.DeviceID)
			body = QueueChatBody{
				Type:     "video",
				Content:  event.Message.Video.Caption,
				FilesURL: []string{mediaURL},
			}
		}

	case "audio":
		if event.Message.Audio != nil {
			mediaURL := p.handleMedia(event.Message.Audio.URL, "audio", event.Message.MessageID, event.DeviceID)
			body = QueueChatBody{
				Type:     "audio",
				Content:  "",
				FilesURL: []string{mediaURL},
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
