package webhook

import (
	"context"
	"fmt"
)

// Plugin is the main webhook plugin that integrates all components
type Plugin struct {
	config          *Config
	db              *Database
	s3Client        *S3Client
	deliveryService *DeliveryService
	subscriber      *Subscriber
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewPlugin creates a new webhook plugin instance
func NewPlugin(eventBus EventSubscriber) (*Plugin, error) {
	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Initialize database
	db, err := NewDatabase("webhook_queue.db")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize S3 client (optional)
	s3Client, err := NewS3Client(config.S3Config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 client: %w", err)
	}

	// Initialize delivery service
	deliveryService := NewDeliveryService(db, s3Client, config.WebhookURL)

	// Initialize subscriber
	subscriber := NewSubscriber(eventBus, deliveryService)

	ctx, cancel := context.WithCancel(context.Background())

	return &Plugin{
		config:          config,
		db:              db,
		s3Client:        s3Client,
		deliveryService: deliveryService,
		subscriber:      subscriber,
		ctx:             ctx,
		cancel:          cancel,
	}, nil
}

// Start starts the webhook plugin
func (p *Plugin) Start() error {
	// Start the delivery service worker
	go p.deliveryService.Start(p.ctx)

	// Start the event subscriber
	if err := p.subscriber.Start(); err != nil {
		return fmt.Errorf("failed to start subscriber: %w", err)
	}

	fmt.Println("Webhook plugin started successfully")
	return nil
}

// Stop stops the webhook plugin
func (p *Plugin) Stop() error {
	fmt.Println("Stopping webhook plugin...")

	// Stop subscriber
	p.subscriber.Stop()

	// Stop delivery service
	p.deliveryService.Stop()

	// Cancel context
	p.cancel()

	// Close database
	if err := p.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	fmt.Println("Webhook plugin stopped")
	return nil
}

// GetDeliveryService returns the delivery service for direct webhook enqueuing
func (p *Plugin) GetDeliveryService() *DeliveryService {
	return p.deliveryService
}
