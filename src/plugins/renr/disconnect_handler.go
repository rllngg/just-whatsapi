package renr

import (
	"context"
	"encoding/json"
	"fmt"

	renrqueue "whatsapp/src/renr-queue"
)

// startDisconnectSubscriber starts a global subscriber for channel-disconnect requests
func (p *Plugin) startDisconnectSubscriber() error {
	p.logger.Infof("Starting channel-disconnect subscriber...")

	logger := &queueLoggerAdapter{waLogger: p.logger}

	subscriber, err := p.queueClient.Subscribe(
		"channel-disconnect",
		"1s", // Poll interval
		p.handleDisconnectRequest,
		renrqueue.WithLogger(logger),
	)

	if err != nil {
		return fmt.Errorf("failed to start disconnect subscriber: %w", err)
	}

	p.disconnectSubscriber = subscriber
	p.logger.Infof("Started channel-disconnect subscriber")
	return nil
}

// handleDisconnectRequest processes disconnect requests from the queue
func (p *Plugin) handleDisconnectRequest(msg *renrqueue.QueueItemResponse) error {
	// Check payload is not nil
	if msg.Payload == nil {
		p.logger.Errorf("Disconnect request payload is nil")
		return fmt.Errorf("message payload is nil")
	}

	// Parse the disconnect request
	var request ChannelDisconnectRequest
	if err := json.Unmarshal([]byte(*msg.Payload), &request); err != nil {
		p.logger.Errorf("Failed to parse disconnect request: %v", err)
		// Return error to retry - malformed JSON might be temporary
		return fmt.Errorf("failed to parse disconnect request: %w", err)
	}

	// Validate required fields
	if request.ChannelID <= 0 {
		p.logger.Errorf("Invalid disconnect request: channel_id must be > 0 (got %d)", request.ChannelID)
		// Non-retryable validation error - send ERROR update and complete
		p.sendChannelUpdate(
			request.ChannelID,
			"",
			"ERROR",
			request.Reference,
			nil,
			strPtr("Invalid channel_id"),
		)
		return nil // Complete message (non-retryable error)
	}

	if request.Reference == "" {
		p.logger.Errorf("Invalid disconnect request: reference (device_id) is required")
		p.sendChannelUpdate(
			request.ChannelID,
			"",
			"ERROR",
			"",
			nil,
			strPtr("Missing device_id in reference field"),
		)
		return nil // Complete message (non-retryable error)
	}

	deviceID := request.Reference
	reason := request.ErrorReason
	if reason == "" {
		reason = "Requested by system"
	}

	// Log received request
	p.logger.Infof("Processing disconnect request: channel_id=%d, device_id=%s, reason=%s",
		request.ChannelID, deviceID, reason)

	// Get device's WhatsApp JID before logout
	device, exists := p.manager.GetDevice(deviceID)
	if !exists {
		errorMsg := fmt.Sprintf("Device not found: %s", deviceID)
		p.logger.Errorf("%s", errorMsg)
		p.sendChannelUpdate(
			request.ChannelID,
			"",
			"ERROR",
			deviceID,
			nil,
			strPtr(errorMsg),
		)
		return nil // Complete message (device doesn't exist)
	}

	// Extract WhatsApp JID
	deviceJID := ""
	if device.Client != nil && device.Client.Store.ID != nil {
		deviceJID = device.Client.Store.ID.User
	}
	if deviceJID == "" {
		// Fallback to deviceID if JID not available (device not connected)
		deviceJID = deviceID
		p.logger.Warnf("Device %s has no JID, using device_id as reference", deviceID)
	}

	// Execute logout via Manager
	ctx := context.Background()
	err := p.manager.LogoutDevice(ctx, deviceID, reason)

	if err != nil {
		// Logout failed - send ERROR update
		p.logger.Errorf("Failed to logout device %s: %v", deviceID, err)

		errorMsg := fmt.Sprintf("Logout failed: %v", err)
		p.sendChannelUpdate(
			request.ChannelID,
			"",
			"ERROR",
			deviceJID, // Use WhatsApp JID in error response
			nil,
			strPtr(errorMsg),
		)

		// Return error to retry
		return fmt.Errorf("failed to logout device %s: %w", deviceID, err)
	}

	// Success - send DISCONNECTED update with WhatsApp JID
	p.logger.Infof("Device logged out successfully: device_id=%s, jid=%s", deviceID, deviceJID)

	p.sendChannelUpdate(
		request.ChannelID,
		"",
		"DISCONNECTED",
		deviceJID, // Use WhatsApp JID as reference
		nil,
		&reason, // Include reason in successful disconnect
	)

	p.logger.Infof("Channel update sent: channel_id=%d, status=DISCONNECTED", request.ChannelID)

	return nil // Success - complete queue message
}

// strPtr is a helper to create string pointers
func strPtr(s string) *string {
	return &s
}
