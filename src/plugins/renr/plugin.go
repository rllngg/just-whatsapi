package renr

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"whatsapp/src/core/whatsapp"
	renrqueue "whatsapp/src/renr-queue"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// EventSubscriber interface for event bus
type EventSubscriber interface {
	Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error)
}

// avatarCacheEntry represents a cached avatar URL with timestamp
type avatarCacheEntry struct {
	URL       string
	FetchedAt time.Time
}

// Plugin handles Renr queue integration via events
type Plugin struct {
	manager     *whatsapp.Manager
	eventBus    EventSubscriber
	queueClient *renrqueue.Client
	logger      waLog.Logger
	httpClient  *http.Client // HTTP client with timeout for media downloads

	// Persistence and delivery
	db              *Database
	deliveryService *DeliveryService

	// Subscriber management
	messageSubscribers   map[string]*renrqueue.Subscriber // deviceID -> subscriber
	qrSubscriber         *renrqueue.Subscriber            // QR request subscriber
	disconnectSubscriber *renrqueue.Subscriber            // Disconnect request subscriber
	subscribersMu        sync.RWMutex

	// Avatar cache (key: "deviceID:jid")
	avatarCache   map[string]avatarCacheEntry
	avatarCacheMu sync.RWMutex

	// S3 configuration (separate from webhook plugin)
	s3Enabled         bool
	s3Bucket          string
	s3Region          string
	s3AccessKeyID     string
	s3SecretAccessKey string
	s3Endpoint        string
	s3URLStyle        string
	s3Client          *s3.Client // AWS SDK v2 client

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

	// Get database path from environment (same as whatsmeow)
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "file:./whatsapp.db?_foreign_keys=on&mode=rwc"
	}

	// Initialize persistence database
	db, err := NewDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize persistence database: %w", err)
	}

	// Parse delivery service configuration
	config := DeliveryConfig{
		WorkerInterval:    5 * time.Second,
		MaxRetryDelay:     24 * time.Hour,
		MaxRequestsPerSec: 25, // Default: 25 requests per second
	}

	// Allow override via environment variables
	if intervalStr := os.Getenv("RENR_QUEUE_WORKER_INTERVAL"); intervalStr != "" {
		if interval, err := time.ParseDuration(intervalStr); err == nil {
			config.WorkerInterval = interval
		}
	}
	if delayStr := os.Getenv("RENR_QUEUE_MAX_RETRY_DELAY"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			config.MaxRetryDelay = delay
		}
	}
	if rateStr := os.Getenv("RENR_QUEUE_MAX_REQUESTS_PER_SEC"); rateStr != "" {
		if rateLimit, err := strconv.Atoi(rateStr); err == nil && rateLimit > 0 {
			config.MaxRequestsPerSec = rateLimit
		}
	}

	// Check if persistence is disabled
	persistenceEnabled := true
	if disabledStr := os.Getenv("RENR_QUEUE_PERSISTENCE_ENABLED"); disabledStr != "" {
		if disabled, err := strconv.ParseBool(disabledStr); err == nil {
			persistenceEnabled = disabled
		}
	}

	if !persistenceEnabled {
		logger.Warnf("Renr queue persistence is DISABLED - messages may be lost if queue is unavailable")
		db.Close()
		db = nil
	} else {
		logger.Infof("Renr queue persistence enabled: db=%s", dbPath)
	}

	// Initialize delivery service
	var deliveryService *DeliveryService
	if db != nil {
		deliveryService = NewDeliveryService(db, queueClient, logger, config)
	}

	// Create HTTP client with timeout for media downloads
	httpClient := &http.Client{
		Timeout: 60 * time.Second, // 60 second timeout for downloads
		Transport: &http.Transport{
			MaxIdleConns:       100,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: true, // Prevent double compression
		},
	}

	// Load Renr plugin's own S3 configuration
	s3Enabled := os.Getenv("RENR_PLUGIN_S3_ENABLED") == "true"

	ctx, cancel := context.WithCancel(context.Background())

	plugin := &Plugin{
		manager:            manager,
		eventBus:           eventBus,
		queueClient:        queueClient,
		logger:             logger,
		httpClient:         httpClient,
		db:                 db,
		deliveryService:    deliveryService,
		messageSubscribers: make(map[string]*renrqueue.Subscriber),
		avatarCache:        make(map[string]avatarCacheEntry),
		ctx:                ctx,
		cancel:             cancel,
		// S3 configuration
		s3Enabled:         s3Enabled,
		s3Bucket:          os.Getenv("RENR_PLUGIN_S3_BUCKET"),
		s3Region:          os.Getenv("RENR_PLUGIN_S3_REGION"),
		s3AccessKeyID:     os.Getenv("RENR_PLUGIN_S3_ACCESS_KEY_ID"),
		s3SecretAccessKey: os.Getenv("RENR_PLUGIN_S3_SECRET_ACCESS_KEY"),
		s3Endpoint:        os.Getenv("RENR_PLUGIN_S3_ENDPOINT"),
		s3URLStyle:        os.Getenv("RENR_PLUGIN_S3_URL_STYLE"),
	}

	// Initialize S3 client if enabled
	if s3Enabled {
		if err := plugin.initializeS3Client(); err != nil {
			logger.Warnf("⚠️  Failed to initialize S3 client: %v (S3 uploads disabled)", err)
			plugin.s3Enabled = false
		} else {
			logger.Infof("✅ Renr Plugin S3 enabled: bucket=%s, region=%s", plugin.s3Bucket, plugin.s3Region)
		}
	} else {
		logger.Warnf("⚠️  Renr Plugin S3 DISABLED (set RENR_PLUGIN_S3_ENABLED=true to enable)")
	}

	return plugin, nil
}

// Start starts the Renr plugin
func (p *Plugin) Start() error {
	p.logger.Infof("Starting Renr queue plugin...")

	// Start delivery service if enabled
	if p.deliveryService != nil {
		p.deliveryService.Start()
		p.logger.Infof("Delivery service started")
	}

	// Start QR request subscriber
	if err := p.startQRRequestSubscriber(); err != nil {
		p.logger.Errorf("Failed to start QR subscriber: %v", err)
		// Don't fail the whole plugin, continue
	}

	// Start disconnect request subscriber
	if err := p.startDisconnectSubscriber(); err != nil {
		p.logger.Errorf("Failed to start disconnect subscriber: %v", err)
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

	// Subscribe to message history synced events
	go p.subscribeToEvent("message.history.synced", p.handleHistorySynced)

	// Subscribe to message read receipt events
	go p.subscribeToEvent("message.read", p.handleMessageRead)

	p.logger.Infof("Renr queue plugin started successfully")
	return nil
}

// Stop stops the Renr plugin
func (p *Plugin) Stop() error {
	p.logger.Infof("Stopping Renr queue plugin...")

	// Stop delivery service
	if p.deliveryService != nil {
		p.deliveryService.Stop()
		p.logger.Infof("Delivery service stopped")
	}

	// Stop QR subscriber
	if p.qrSubscriber != nil {
		p.qrSubscriber.Stop()
		p.logger.Infof("Stopped QR request subscriber")
	}

	// Stop disconnect subscriber
	if p.disconnectSubscriber != nil {
		p.disconnectSubscriber.Stop()
		p.logger.Infof("Stopped disconnect request subscriber")
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

	// Close database
	if p.db != nil {
		if err := p.db.Close(); err != nil {
			p.logger.Errorf("Failed to close database: %v", err)
		}
		p.logger.Infof("Database closed")
	}

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

// initializeS3Client initializes the S3 client with Renr plugin configuration
func (p *Plugin) initializeS3Client() error {
	// Validate required config
	if p.s3Bucket == "" || p.s3Region == "" {
		return fmt.Errorf("S3 bucket and region are required")
	}

	if p.s3AccessKeyID == "" || p.s3SecretAccessKey == "" {
		return fmt.Errorf("S3 credentials are required")
	}

	// Create AWS config
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(p.s3Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			p.s3AccessKeyID,
			p.s3SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client with optional custom endpoint
	var opts []func(*s3.Options)
	if p.s3Endpoint != "" {
		opts = append(opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(p.s3Endpoint)
			o.UsePathStyle = (p.s3URLStyle == "path")
		})
	}

	p.s3Client = s3.NewFromConfig(cfg, opts...)
	return nil
}
