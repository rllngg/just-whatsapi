package renr

import (
	"encoding/json"
	"fmt"
	"strconv"

	"whatsapp/src/core/whatsapp"

	"github.com/ThreeDotsLabs/watermill/message"
)

// handleMessageRead processes message read receipt events and enqueues them
func (p *Plugin) handleMessageRead(msg *message.Message) error {
	var event whatsapp.MessageReadEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		p.logger.Errorf("Failed to unmarshal message read event: %v", err)
		return nil // Don't retry, just log
	}

	// Filter: Only process "read" receipts, skip "delivered", "played", etc.
	if event.Type != "types.ReceiptTypeRead" {
		p.logger.Debugf("Skipping non-read receipt: device=%s, chat=%s, type=%s",
			event.DeviceID, event.ChatID, event.Type)
		return nil
	}

	p.logger.Debugf("Received read receipt: device=%s, chat=%s, sender=%s, msg_ids=%v",
		event.DeviceID, event.ChatID, event.Sender, event.MessageIDs)

	// Convert device ID to channel ID
	channelID, err := strconv.ParseInt(event.DeviceID, 10, 64)
	if err != nil {
		p.logger.Errorf("Invalid device ID for read receipt: %v", err)
		return nil // Don't retry
	}
	queueMsg := p.convertToQueueMessageRead(event, channelID)
	payload, err := json.Marshal(queueMsg)
	if err != nil {
		p.logger.Errorf("Failed to marshal read receipt: device=%s, error=%v, chat=%s",
			event.DeviceID, err, event.ChatID)
	}
	err = p.deliveryService.Enqueue(event.DeviceID, fmt.Sprintf("read_%d_chat_%s", event.Timestamp, event.ChatID), "channel-chat-message-read", string(payload))
	if err != nil {
		p.logger.Errorf("Failed to enqueue read receipt: device=%s, error=%v, chat=%s",
			event.DeviceID, err, event.ChatID)
	}
	return nil
}

// convertToQueueMessageRead converts MessageReadEvent to QueueMessageRead
// Following MessageReceivedEvent pattern:
// - from: the reader (who read the message - the Sender field in read receipts)
// - to: the chat (where the message was read - the ChatID field)
func (p *Plugin) convertToQueueMessageRead(event whatsapp.MessageReadEvent, channelID int64) *QueueMessageRead {
	// Extract reader (who read the message - the Sender field in read receipts)
	from := QueueChatPerson{
		ID:    extractPhoneFromJID(event.Sender),
		Name:  "Loading...", // Placeholder - matches reference implementation
		Phone: extractPhoneFromJID(event.Sender),
		Email: "",
	}

	// Extract chat (where the message was read - following MessageReceivedEvent pattern)
	to := QueueChatPerson{
		ID:    event.ChatID, // Use ChatID like in MessageReceivedEvent
		Name:  "Loading...", // Placeholder - could be enhanced with contact/group name resolution
		Phone: extractPhoneFromJID(event.ChatID),
		Email: "",
	}

	return &QueueMessageRead{
		ChannelID:   channelID,
		From:        from,
		To:          to,              // ChatID as the destination
		Timestamp:   event.Timestamp, // Already in Unix seconds
		ErrorReason: "",              // Always empty for read receipts
	}
}
