package renr

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"whatsapp/src/core/whatsapp"
	renrqueue "whatsapp/src/renr-queue"

	"github.com/ThreeDotsLabs/watermill/message"
)

// startQRRequestSubscriber starts subscriber for QR requests
func (p *Plugin) startQRRequestSubscriber() error {
	logger := &queueLoggerAdapter{waLogger: p.logger}

	subscriber, err := p.queueClient.Subscribe(
		"whatsapp-request-qr",
		"1s",
		p.handleQRRequest,
		renrqueue.WithLogger(logger),
	)

	if err != nil {
		return fmt.Errorf("failed to start QR subscriber: %w", err)
	}

	p.qrSubscriber = subscriber
	p.logger.Infof("Started QR request subscriber")
	return nil
}

// handleQRRequest processes QR code requests from queue
func (p *Plugin) handleQRRequest(msg *renrqueue.QueueItemResponse) error {
	if msg.Payload == nil {
		return fmt.Errorf("message payload is nil")
	}

	var request WhatsAppQRRequest
	if err := json.Unmarshal([]byte(*msg.Payload), &request); err != nil {
		return fmt.Errorf("invalid JSON payload: %w", err)
	}

	// Validate
	if request.Channel.ID == 0 {
		return fmt.Errorf("channel_id is required")
	}

	deviceID := strconv.FormatInt(request.Channel.ID, 10)
	p.logger.Infof("Processing QR request: channel_id=%d, device_id=%s",
		request.Channel.ID, deviceID)

	// Create device via manager
	device, err := p.manager.CreateDevice(context.Background(), deviceID)
	if err != nil {
		p.logger.Errorf("Failed to create device: %v", err)

		// Send error update
		errorMsg := fmt.Sprintf("Failed to create device: %v", err)
		p.sendChannelUpdate(request.Channel.ID, "", "ERROR", deviceID, nil, &errorMsg)
		return err
	}

	// Wait for QR code (with timeout)
	qrCode := p.waitForQRCode(device, 30*time.Second)
	if qrCode == "" {
		errorMsg := "QR code generation timeout"
		p.sendChannelUpdate(request.Channel.ID, "", "ERROR", deviceID, nil, &errorMsg)
		return fmt.Errorf("%s", errorMsg)
	}

	p.logger.Infof("QR code generated: channel_id=%d", request.Channel.ID)

	// Send QR ready update
	p.sendChannelUpdate(request.Channel.ID, "", "WAITING_INTEGRATION", deviceID, &qrCode, nil)

	return nil
}

// waitForQRCode waits for QR code to be ready
func (p *Plugin) waitForQRCode(device *whatsapp.Device, timeout time.Duration) string {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if device.QRCode != "" && device.Status == "qr_ready" {
			return device.QRCode
		}
		time.Sleep(100 * time.Millisecond)
	}

	return ""
}

// sendChannelUpdate sends channel update to queue
func (p *Plugin) sendChannelUpdate(channelID int64, name string, status, reference string, data, errorReason *string) {
	update := QueueChannelUpdate{
		Name:        name,
		ChannelID:   channelID,
		Status:      status,
		Reference:   reference,
		Data:        data,
		ErrorReason: errorReason,
	}

	payload, err := json.Marshal(update)
	if err != nil {
		p.logger.Errorf("Failed to marshal channel update: %v", err)
		return
	}
	channelIDStr := strconv.FormatInt(channelID, 10)
	res, err := p.queueClient.Enqueue("channel-update", renrqueue.EnqueueRequest{
		Ref:     &channelIDStr,
		Payload: string(payload),
	})

	if err != nil {
		p.logger.Errorf("Failed to enqueue channel update: %v", err)
		return
	}

	p.logger.Infof("Channel update sent: channel_id=%d, status=%s, id=%s", channelID, status, res.ID)
}

// handleDeviceConnectedForQR sends CONNECTED status to queue
func (p *Plugin) handleDeviceConnectedForQR(msg *message.Message) error {
	var event whatsapp.DeviceConnectedEvent
	if err := json.Unmarshal(msg.Payload, &event); err != nil {
		return err
	}

	// Convert device ID to channel ID
	channelID, err := strconv.ParseInt(event.DeviceID, 10, 64)
	if err != nil {
		p.logger.Warnf("Invalid device ID for channel update: %s", event.DeviceID)
		return nil // Don't fail, just skip
	}

	// Send CONNECTED update with user's WhatsApp JID as reference
	p.sendChannelUpdate(channelID, event.PhoneNumber, "CONNECTED", fmt.Sprintf("%s@s.whatsapp.net", event.PhoneNumber), nil, nil)
	p.logger.Infof("Device connected, sent CONNECTED status: channel_id=%d, jid=%s, lid=%s, phoneNumber", channelID, event.DeviceJID, event.DeviceLID)

	return nil
}
