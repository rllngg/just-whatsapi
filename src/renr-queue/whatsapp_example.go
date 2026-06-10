package renrqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// This file demonstrates how to use the generic subscriber for WhatsApp QR code requests
// Note: This is a reference implementation that requires the whatsapp package

// WhatsAppRequestQRPayload represents the incoming request payload
type WhatsAppRequestQRPayload struct {
	Channel ChannelInfo `json:"channel"`
}

// ChannelInfo contains channel information
type ChannelInfo struct {
	ID       int64  `json:"id"`
	Metadata string `json:"metadata"`
	Status   string `json:"status"`
}

// QueueChannelUpdate represents the outgoing channel update payload
type QueueChannelUpdate struct {
	ChannelID   int64   `json:"channel_id"`
	Status      string  `json:"status"`
	Reference   string  `json:"reference"`
	Data        *string `json:"data,omitempty"`
	ErrorReason *string `json:"errorReason,omitempty"`
}

// WhatsAppManager interface abstracts the WhatsApp device management
// Implement this interface in your whatsapp package
type WhatsAppManager interface {
	CreateDevice(ctx context.Context, deviceID string) (WhatsAppDevice, error)
}

// WhatsAppDevice interface represents a WhatsApp device
type WhatsAppDevice interface {
	GetQRCode() string
	GetStatus() string
}

// ExampleWhatsAppQRHandler demonstrates how to create a handler for WhatsApp QR requests
// This can be used as a reference for implementing your own handlers
func ExampleWhatsAppQRHandler(client *Client, manager WhatsAppManager) (*Subscriber, error) {
	// Create the message handler
	handler := func(msg *QueueItemResponse) error {
		// Parse the payload
		if msg.Payload == nil {
			return fmt.Errorf("message payload is nil")
		}

		var payload WhatsAppRequestQRPayload
		if err := json.Unmarshal([]byte(*msg.Payload), &payload); err != nil {
			return fmt.Errorf("invalid JSON payload: %w", err)
		}

		// Validate channel ID
		if payload.Channel.ID == 0 {
			return fmt.Errorf("channel ID is required and must be non-zero")
		}

		// Use channel ID as device ID
		deviceID := strconv.FormatInt(payload.Channel.ID, 10)

		// Create device and get QR code
		device, err := manager.CreateDevice(context.Background(), deviceID)
		if err != nil {
			// Send failure update to channel-update queue
			errorMsg := fmt.Sprintf("Failed to create WhatsApp device: %v", err)
			sendChannelUpdate(client, payload.Channel.ID, "ERROR", deviceID, nil, &errorMsg)
			return err
		}

		// Wait for QR code to be ready (with timeout)
		qrCode := waitForQRCode(device, 30*time.Second)
		if qrCode == "" {
			errorMsg := "QR code generation timeout"
			sendChannelUpdate(client, payload.Channel.ID, "ERROR", deviceID, nil, &errorMsg)
			return fmt.Errorf("%s", errorMsg)
		}

		// Send success update to channel-update queue
		sendChannelUpdate(client, payload.Channel.ID, "QR_READY", deviceID, &qrCode, nil)

		return nil // Message will be marked as completed
	}

	// Subscribe to the queue with 1 second polling interval
	return client.Subscribe("whatsapp-request-qr", "1s", handler)
}

// waitForQRCode waits for the QR code to be ready with a timeout
func waitForQRCode(device WhatsAppDevice, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if device.GetQRCode() != "" && device.GetStatus() == "qr_ready" {
			return device.GetQRCode()
		}

		// Check every 100ms
		time.Sleep(100 * time.Millisecond)
	}

	return ""
}

// sendChannelUpdate sends a channel update to the queue
func sendChannelUpdate(client *Client, channelID int64, status, reference string, data, errorReason *string) {
	update := QueueChannelUpdate{
		ChannelID:   channelID,
		Status:      status,
		Reference:   reference,
		Data:        data,
		ErrorReason: errorReason,
	}

	payloadJSON, err := json.Marshal(update)
	if err != nil {
		return
	}

	// Enqueue the channel update
	ref := fmt.Sprintf("channel-%d", channelID)
	_, _ = client.Enqueue("channel-update", EnqueueRequest{
		Ref:     &ref,
		Payload: string(payloadJSON),
	})
}
