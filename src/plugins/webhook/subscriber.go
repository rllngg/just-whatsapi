package webhook

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ThreeDotsLabs/watermill/message"
)

// EventSubscriber interface for event bus
type EventSubscriber interface {
	Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error)
}

// Subscriber handles webhook events from the event bus
type Subscriber struct {
	eventBus        EventSubscriber
	deliveryService *DeliveryService
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewSubscriber creates a new webhook subscriber
func NewSubscriber(eventBus EventSubscriber, deliveryService *DeliveryService) *Subscriber {
	ctx, cancel := context.WithCancel(context.Background())
	return &Subscriber{
		eventBus:        eventBus,
		deliveryService: deliveryService,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// Start starts listening to events and enqueueing webhooks
func (s *Subscriber) Start() error {
	// Subscribe to all relevant events
	events := []string{
		"device.created",
		"device.connected",
		"device.disconnected",
		"device.logout",
		"device.qr_ready",
		"message.received",
		"message.read",
		"message.text.sent",
		"message.text.failed",
		"message.image.sent",
		"message.image.failed",
		"message.file.sent",
		"message.file.failed",
		"message.presence.sent",
		"message.presence.failed",
		"message.reaction.sent",
		"message.reaction.failed",
	}

	for _, event := range events {
		go s.subscribeToEvent(event)
	}

	return nil
}

// Stop stops the subscriber
func (s *Subscriber) Stop() {
	s.cancel()
}

// subscribeToEvent subscribes to a specific event and processes messages
func (s *Subscriber) subscribeToEvent(event string) {
	messages, err := s.eventBus.Subscribe(s.ctx, event)
	if err != nil {
		fmt.Printf("Failed to subscribe to event %s: %v\n", event, err)
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg := <-messages:
			s.handleMessage(event, msg)
			msg.Ack()
		}
	}
}

// handleMessage processes an event message and enqueues webhook
func (s *Subscriber) handleMessage(event string, msg *message.Message) {
	// Parse the message payload
	var payload map[string]interface{}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		fmt.Printf("Failed to unmarshal message payload for event %s: %v\n", event, err)
		return
	}

	// Extract device_id for webhook payload
	deviceID, _ := payload["device_id"].(string)

	// Build webhook payload according to spec
	webhookPayload := map[string]interface{}{
		"device_id": deviceID,
		"payload":   payload,
	}

	// Enqueue the webhook
	if err := s.deliveryService.Enqueue(event, webhookPayload); err != nil {
		fmt.Printf("Failed to enqueue webhook for event %s: %v\n", event, err)
	}
}
