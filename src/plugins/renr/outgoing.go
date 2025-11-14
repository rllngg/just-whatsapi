package renr

import (
	"context"
	"encoding/json"
	"fmt"

	"whatsapp/src/core/whatsapp"
	renrqueue "whatsapp/src/renr-queue"

	"github.com/ThreeDotsLabs/watermill/message"
)

// handleDeviceConnected starts message subscriber when device connects
func (p *Plugin) handleDeviceConnected(msg *message.Message) error {
	var event whatsapp.DeviceConnectedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	p.logger.Infof("Device connected, starting message subscriber: %s", event.DeviceID)
	return p.startMessageSubscriber(event.DeviceID)
}

// handleDeviceRestored starts message subscriber when device is restored
func (p *Plugin) handleDeviceRestored(msg *message.Message) error {
	var event whatsapp.DeviceRestoredEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	p.logger.Infof("Device restored, starting message subscriber: %s", event.DeviceID)
	return p.startMessageSubscriber(event.DeviceID)
}

// handleDeviceDisconnected stops message subscriber when device disconnects
func (p *Plugin) handleDeviceDisconnected(msg *message.Message) error {
	var event whatsapp.DeviceDisconnectedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	p.logger.Infof("Device disconnected, stopping message subscriber: %s", event.DeviceID)
	p.stopMessageSubscriber(event.DeviceID)
	return nil
}

// handleDeviceLogout stops message subscriber when device logs out
func (p *Plugin) handleDeviceLogout(msg *message.Message) error {
	var event whatsapp.DeviceLogoutEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	p.logger.Infof("Device logged out, stopping message subscriber: %s", event.DeviceID)
	p.stopMessageSubscriber(event.DeviceID)
	return nil
}

// startMessageSubscriber starts a subscriber for outgoing messages
func (p *Plugin) startMessageSubscriber(deviceID string) error {
	p.subscribersMu.Lock()
	defer p.subscribersMu.Unlock()

	// Check if already exists
	if _, exists := p.messageSubscribers[deviceID]; exists {
		p.logger.Warnf("Message subscriber already exists for device: %s", deviceID)
		return nil
	}

	// Create logger adapter
	logger := &queueLoggerAdapter{waLogger: p.logger}

	// Start subscriber
	subscriber, err := p.queueClient.Subscribe(
		"channel-chat-message-outgoing",
		"1s",
		func(msg *renrqueue.QueueItemResponse) error {
			return p.handleOutgoingMessage(deviceID, msg)
		},
		renrqueue.WithRef(deviceID),
		renrqueue.WithLogger(logger),
	)

	if err != nil {
		return fmt.Errorf("failed to start subscriber: %w", err)
	}

	p.messageSubscribers[deviceID] = subscriber
	p.logger.Infof("Started message subscriber for device: %s", deviceID)
	return nil
}

// stopMessageSubscriber stops a subscriber for a device
func (p *Plugin) stopMessageSubscriber(deviceID string) {
	p.subscribersMu.Lock()
	defer p.subscribersMu.Unlock()

	if sub, exists := p.messageSubscribers[deviceID]; exists {
		sub.Stop()
		delete(p.messageSubscribers, deviceID)
		p.logger.Infof("Stopped message subscriber for device: %s", deviceID)
	}
}

// handleOutgoingMessage processes outgoing queue messages
func (p *Plugin) handleOutgoingMessage(deviceID string, msg *renrqueue.QueueItemResponse) error {
	// Parse payload
	if msg.Payload == nil {
		return fmt.Errorf("message payload is nil")
	}

	var chatMsg QueueChatMessage
	if err := json.Unmarshal([]byte(*msg.Payload), &chatMsg); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}
	if chatMsg.Body.Type == "" {
		return fmt.Errorf("body.type is required")
	}

	p.logger.Infof("Processing outgoing message: device=%s, type=%s, to=%s",
		deviceID, chatMsg.Body.Type, chatMsg.To.ID)

	// Route based on type
	switch chatMsg.Body.Type {
	case "text":
		return p.sendTextMessage(deviceID, chatMsg)
	case "image":
		return p.sendImageMessage(deviceID, chatMsg)
	case "document":
		return p.sendDocumentMessage(deviceID, chatMsg)
	case "video":
		return p.sendVideoMessage(deviceID, chatMsg)
	case "audio":
		return p.sendAudioMessage(deviceID, chatMsg)
	case "file":
		return p.sendFileMessage(deviceID, chatMsg)
	case "reaction":
		return p.sendReactionMessage(deviceID, chatMsg)
	default:
		return fmt.Errorf("unsupported message type: %s", chatMsg.Body.Type)
	}
}

// sendTextMessage sends a text message via manager
func (p *Plugin) sendTextMessage(deviceID string, msg QueueChatMessage) error {
	chatID := msg.To.ID

	req := whatsapp.SendTextMessageRequest{
		DeviceID:       deviceID,
		ChatID:         chatID,
		Message:        msg.Body.Content,
		ReplyMessageID: getStringValue(msg.ReplyTo),
	}

	_, err := p.manager.SendTextMessage(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send text message: device=%s, to=%s, error=%v", deviceID, chatID, err)
		return err
	}

	p.logger.Infof("Sent text message: device=%s, to=%s", deviceID, chatID)
	return nil
}

// sendImageMessage sends an image message via manager
func (p *Plugin) sendImageMessage(deviceID string, msg QueueChatMessage) error {
	if len(msg.Body.FilesURL) == 0 {
		return fmt.Errorf("filesURL is required for image messages")
	}

	chatID := msg.To.ID

	req := whatsapp.SendImageMessageRequest{
		DeviceID:       deviceID,
		ChatID:         chatID,
		FileURL:        msg.Body.FilesURL[0],
		Caption:        msg.Body.Content,
		ReplyMessageID: getStringValue(msg.ReplyTo),
	}

	_, err := p.manager.SendImageMessage(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send image: device=%s, to=%s, error=%v", deviceID, chatID, err)
		return err
	}

	p.logger.Infof("Sent image message: device=%s, to=%s", deviceID, chatID)
	return nil
}

// sendDocumentMessage sends a document message via manager
func (p *Plugin) sendDocumentMessage(deviceID string, msg QueueChatMessage) error {
	if len(msg.Body.FilesURL) == 0 {
		return fmt.Errorf("filesURL is required for document messages")
	}

	chatID := msg.To.ID

	req := whatsapp.SendFileMessageRequest{
		DeviceID:       deviceID,
		ChatID:         chatID,
		FileURL:        msg.Body.FilesURL[0],
		Caption:        msg.Body.Content,
		ReplyMessageID: getStringValue(msg.ReplyTo),
	}

	_, err := p.manager.SendFileMessage(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send document: device=%s, to=%s, error=%v", deviceID, chatID, err)
		return err
	}

	p.logger.Infof("Sent document message: device=%s, to=%s", deviceID, chatID)
	return nil
}

// sendVideoMessage sends a video message via manager
func (p *Plugin) sendVideoMessage(deviceID string, msg QueueChatMessage) error {
	if len(msg.Body.FilesURL) == 0 {
		return fmt.Errorf("filesURL is required for video messages")
	}

	chatID := msg.To.ID

	// Video uses the image endpoint in the manager
	req := whatsapp.SendImageMessageRequest{
		DeviceID:       deviceID,
		ChatID:         chatID,
		FileURL:        msg.Body.FilesURL[0],
		Caption:        msg.Body.Content,
		ReplyMessageID: getStringValue(msg.ReplyTo),
	}

	_, err := p.manager.SendImageMessage(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send video: device=%s, to=%s, error=%v", deviceID, chatID, err)
		return err
	}

	p.logger.Infof("Sent video message: device=%s, to=%s", deviceID, chatID)
	return nil
}

// sendAudioMessage sends an audio message via manager
func (p *Plugin) sendAudioMessage(deviceID string, msg QueueChatMessage) error {
	if len(msg.Body.FilesURL) == 0 {
		return fmt.Errorf("filesURL is required for audio messages")
	}

	chatID := msg.To.ID

	// Audio uses the file endpoint in the manager
	req := whatsapp.SendFileMessageRequest{
		DeviceID:       deviceID,
		ChatID:         chatID,
		FileURL:        msg.Body.FilesURL[0],
		Caption:        msg.Body.Content,
		ReplyMessageID: getStringValue(msg.ReplyTo),
	}

	_, err := p.manager.SendFileMessage(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send audio: device=%s, to=%s, error=%v", deviceID, chatID, err)
		return err
	}

	p.logger.Infof("Sent audio message: device=%s, to=%s", deviceID, chatID)
	return nil
}

// sendFileMessage sends a file message via manager
func (p *Plugin) sendFileMessage(deviceID string, msg QueueChatMessage) error {
	if len(msg.Body.FilesURL) == 0 {
		return fmt.Errorf("filesURL is required for file messages")
	}

	chatID := msg.To.ID

	req := whatsapp.SendFileMessageRequest{
		DeviceID:       deviceID,
		ChatID:         chatID,
		FileURL:        msg.Body.FilesURL[0],
		Caption:        msg.Body.Content,
		ReplyMessageID: getStringValue(msg.ReplyTo),
	}

	_, err := p.manager.SendFileMessage(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send file: device=%s, to=%s, error=%v", deviceID, chatID, err)
		return err
	}

	p.logger.Infof("Sent file message: device=%s, to=%s", deviceID, chatID)
	return nil
}

// sendReactionMessage sends a reaction via manager
func (p *Plugin) sendReactionMessage(deviceID string, msg QueueChatMessage) error {
	if msg.Body.Reaction == nil {
		return fmt.Errorf("reaction data is required for reaction messages")
	}

	chatID := msg.To.ID

	req := whatsapp.SendReactionRequest{
		DeviceID:  deviceID,
		ChatID:    chatID,
		MessageID: msg.Body.Reaction.TargetMessageID,
		Emoji:     msg.Body.Reaction.Emoji,
	}

	_, err := p.manager.SendReaction(context.Background(), req)
	if err != nil {
		p.logger.Errorf("Failed to send reaction: device=%s, to=%s, target=%s, error=%v",
			deviceID, chatID, req.MessageID, err)
		return err
	}

	p.logger.Infof("Sent reaction: device=%s, to=%s, target=%s, emoji=%s",
		deviceID, chatID, req.MessageID, req.Emoji)
	return nil
}
