package renr

import (
	"context"
	"fmt"
	"sync"

	"whatsapp/src/core/whatsapp"
	renrqueue "whatsapp/src/renr-queue"

	"github.com/ThreeDotsLabs/watermill/message"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// EventSubscriber interface for event bus
type EventSubscriber interface {
	Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error)
}

// Plugin handles Renr queue integration via events
type Plugin struct {
	manager     *whatsapp.Manager
	eventBus    EventSubscriber
	queueClient *renrqueue.Client
	logger      waLog.Logger

	// Subscriber management
	messageSubscribers map[string]*renrqueue.Subscriber // deviceID -> subscriber
	qrSubscriber       *renrqueue.Subscriber            // QR request subscriber
	subscribersMu      sync.RWMutex

	// Event subscribers
	ctx    context.Context
	cancel context.CancelFunc
}

// NewPlugin creates a new Renr queue plugin
func NewPlugin(manager *whatsapp.Manager, eventBus EventSubscriber, logger waLog.Logger) (*Plugin, error) {
	// Create queue client from environment
	queueClient, err := renrqueue.NewClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to create queue client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Plugin{
		manager:            manager,
		eventBus:           eventBus,
		queueClient:        queueClient,
		logger:             logger,
		messageSubscribers: make(map[string]*renrqueue.Subscriber),
		ctx:                ctx,
		cancel:             cancel,
	}, nil
}

// Start starts the Renr plugin
func (p *Plugin) Start() error {
	p.logger.Infof("Starting Renr queue plugin...")

	// Start QR request subscriber
	if err := p.startQRRequestSubscriber(); err != nil {
		p.logger.Errorf("Failed to start QR subscriber: %v", err)
		// Don't fail the whole plugin, continue
	}

	// Subscribe to device lifecycle events
	go p.subscribeToEvent("device.connected", p.handleDeviceConnected)
	go p.subscribeToEvent("device.connected", p.handleDeviceConnectedForQR) // Also send QR connected status
	go p.subscribeToEvent("device.disconnected", p.handleDeviceDisconnected)
	go p.subscribeToEvent("device.logout", p.handleDeviceLogout)
	go p.subscribeToEvent("device.restored", p.handleDeviceRestored)

	// Subscribe to incoming message events
	go p.subscribeToEvent("message.received", p.handleIncomingMessage)

	p.logger.Infof("Renr queue plugin started successfully")
	return nil
}

// Stop stops the Renr plugin
func (p *Plugin) Stop() error {
	p.logger.Infof("Stopping Renr queue plugin...")

	// Stop QR subscriber
	if p.qrSubscriber != nil {
		p.qrSubscriber.Stop()
		p.logger.Infof("Stopped QR request subscriber")
	}

	// Stop all message subscribers
	p.subscribersMu.Lock()
	for deviceID, sub := range p.messageSubscribers {
		sub.Stop()
		p.logger.Infof("Stopped message subscriber for device: %s", deviceID)
	}
	p.messageSubscribers = make(map[string]*renrqueue.Subscriber)
	p.subscribersMu.Unlock()

	// Stop context
	p.cancel()

	p.logger.Infof("Renr queue plugin stopped")
	return nil
}

// subscribeToEvent subscribes to a specific event and calls handler
func (p *Plugin) subscribeToEvent(topic string, handler func(*message.Message) error) {
	messages, err := p.eventBus.Subscribe(p.ctx, topic)
	if err != nil {
		p.logger.Errorf("Failed to subscribe to %s: %v", topic, err)
		return
	}

	p.logger.Infof("Subscribed to event: %s", topic)

	for {
		select {
		case <-p.ctx.Done():
			p.logger.Debugf("Stopped listening to %s", topic)
			return
		case msg, ok := <-messages:
			if !ok {
				p.logger.Warnf("Event channel closed for %s", topic)
				return
			}

			if err := handler(msg); err != nil {
				p.logger.Errorf("Error handling event %s: %v", topic, err)
			}

			msg.Ack()
		}
	}
}
