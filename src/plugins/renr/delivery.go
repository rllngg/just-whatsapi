package renr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"sync"
	"time"

	renrqueue "whatsapp/src/renr-queue"

	waLog "go.mau.fi/whatsmeow/util/log"
	"golang.org/x/time/rate"
)

// DeliveryService handles Renr queue message delivery with retry logic
type DeliveryService struct {
	db                *Database
	queueClient       *renrqueue.Client
	logger            waLog.Logger
	workerInterval    time.Duration
	maxRetryDelay     time.Duration
	rateLimiter       *rate.Limiter // Rate limiter for queue processing
	processingDevices sync.Map      // deviceID -> bool (lock to ensure FIFO per device)
	stopChan          chan struct{}
	ctx               context.Context
	cancel            context.CancelFunc
}

// DeliveryConfig contains configuration for the delivery service
type DeliveryConfig struct {
	WorkerInterval    time.Duration
	MaxRetryDelay     time.Duration
	MaxRequestsPerSec int // Maximum requests per second for queue processing (default: 25)
}

// NewDeliveryService creates a new delivery service
func NewDeliveryService(db *Database, queueClient *renrqueue.Client, logger waLog.Logger, config DeliveryConfig) *DeliveryService {
	// Apply defaults
	if config.WorkerInterval == 0 {
		config.WorkerInterval = 5 * time.Second
	}
	if config.MaxRetryDelay == 0 {
		config.MaxRetryDelay = 24 * time.Hour
	}
	if config.MaxRequestsPerSec == 0 {
		config.MaxRequestsPerSec = 25 // Default: 25 requests per second
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create rate limiter with burst capacity equal to rate
	// This allows smooth distribution of requests over time
	rateLimiter := rate.NewLimiter(rate.Limit(config.MaxRequestsPerSec), config.MaxRequestsPerSec)

	logger.Infof("Delivery service initialized: worker_interval=%s, max_retry_delay=%s, max_requests_per_sec=%d",
		config.WorkerInterval, config.MaxRetryDelay, config.MaxRequestsPerSec)

	return &DeliveryService{
		db:             db,
		queueClient:    queueClient,
		logger:         logger,
		workerInterval: config.WorkerInterval,
		maxRetryDelay:  config.MaxRetryDelay,
		rateLimiter:    rateLimiter,
		stopChan:       make(chan struct{}),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start starts the background worker for processing pending messages
func (ds *DeliveryService) Start() {
	ds.logger.Infof("Starting Renr queue delivery worker...")

	go func() {
		ticker := time.NewTicker(ds.workerInterval)
		defer ticker.Stop()

		// Initial processing
		ds.processPending()

		// Periodic cleanup ticker (every 5 minutes)
		cleanupTicker := time.NewTicker(5 * time.Minute)
		defer cleanupTicker.Stop()

		for {
			select {
			case <-ds.ctx.Done():
				ds.logger.Infof("Delivery worker stopped (context done)")
				return
			case <-ds.stopChan:
				ds.logger.Infof("Delivery worker stopped (stop signal)")
				return
			case <-ticker.C:
				ds.processPending()
			case <-cleanupTicker.C:
				ds.cleanup()
			}
		}
	}()

	ds.logger.Infof("Renr queue delivery worker started")
}

// Stop gracefully stops the delivery service
func (ds *DeliveryService) Stop() {
	ds.logger.Infof("Stopping Renr queue delivery service...")
	ds.cancel()
	close(ds.stopChan)
	ds.logger.Infof("Renr queue delivery service stopped")
}

// Enqueue adds a message to the persistence queue
// This is the public API used by incoming message handlers
func (ds *DeliveryService) Enqueue(deviceID, messageID, topic, payload string) error {
	err := ds.db.Enqueue(deviceID, messageID, topic, payload)
	if err != nil {
		ds.logger.Errorf("Failed to persist message: device=%s, msg_id=%s, error=%v",
			deviceID, messageID, err)
		return fmt.Errorf("failed to persist message: %w", err)
	}

	ds.logger.Infof("Persisted incoming message: device=%s, msg_id=%s, topic=%s",
		deviceID, messageID, topic)

	return nil
}

// processPending processes pending messages for all devices
func (ds *DeliveryService) processPending() {
	// Get all devices with pending messages
	devices, err := ds.db.GetAllPendingDevices()
	if err != nil {
		ds.logger.Errorf("Failed to get pending devices: %v", err)
		return
	}

	if len(devices) == 0 {
		// No pending messages
		return
	}

	ds.logger.Debugf("Processing pending messages for %d devices", len(devices))

	// Process one message per device (FIFO within each device)
	for _, deviceID := range devices {
		// Process in a separate goroutine to allow parallel processing across devices
		// but maintain FIFO within each device
		go ds.processDevice(deviceID)
	}
}

// processDevice processes all pending messages for a specific device
// Uses a lock to ensure only one goroutine per device processes messages (FIFO)
// Continuously processes messages until the queue is empty or context is cancelled
func (ds *DeliveryService) processDevice(deviceID string) {
	// Try to acquire lock for this device
	_, loaded := ds.processingDevices.LoadOrStore(deviceID, true)
	if loaded {
		// Another goroutine is already processing this device
		ds.logger.Debugf("Device already being processed, skipping: device=%s", deviceID)
		return
	}

	// Release lock when done
	defer ds.processingDevices.Delete(deviceID)

	// Track total processing session metrics
	sessionStartTime := time.Now()
	messagesProcessed := 0

	// Continuously process messages until queue is empty or context cancelled
	for {
		// Check context cancellation
		select {
		case <-ds.ctx.Done():
			ds.logger.Debugf("Context cancelled, stopping device processing: device=%s, processed=%d",
				deviceID, messagesProcessed)
			return
		default:
			// Continue processing
		}

		// Apply rate limiting before processing
		// Wait for rate limiter to allow this request
		if err := ds.rateLimiter.Wait(ds.ctx); err != nil {
			// Context cancelled or other error
			ds.logger.Debugf("Rate limiter wait cancelled: device=%s, processed=%d, error=%v",
				deviceID, messagesProcessed, err)
			return
		}

		// Get next pending message for this device
		msg, err := ds.db.GetNextPendingForDevice(deviceID)
		if err != nil {
			ds.logger.Errorf("Failed to get next pending message: device=%s, error=%v", deviceID, err)
			return
		}

		if msg == nil {
			// No more pending messages for this device
			if messagesProcessed > 0 {
				sessionDuration := time.Since(sessionStartTime).Milliseconds()
				ds.logger.Infof("Queue emptied for device: device=%s, messages_processed=%d, session_duration_ms=%d",
					deviceID, messagesProcessed, sessionDuration)
			}
			return
		}

		// Mark as processing
		if err := ds.db.MarkProcessing(msg.ID); err != nil {
			ds.logger.Errorf("Failed to mark message as processing: device=%s, msg_id=%s, error=%v",
				deviceID, msg.MessageID, err)
			return
		}

		ds.logger.Infof("Processing pending message: device=%s, msg_id=%s, attempt=%d",
			deviceID, msg.MessageID, msg.Attempts+1)

		// Attempt delivery
		deliveryStartTime := time.Now()
		if err := ds.deliver(msg); err != nil {
			deliveryDuration := time.Since(deliveryStartTime).Milliseconds()

			// Delivery failed, mark as failed with retry
			if markErr := ds.db.MarkFailed(msg.ID, err.Error()); markErr != nil {
				ds.logger.Errorf("Failed to mark message as failed: device=%s, msg_id=%s, error=%v",
					deviceID, msg.MessageID, markErr)
				return
			}

			// Calculate next retry time for logging
			backoffMinutes := 1 << msg.Attempts
			if backoffMinutes > 1440 {
				backoffMinutes = 1440
			}
			nextRetry := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

			ds.logger.Warnf("Failed to enqueue message: device=%s, msg_id=%s, attempt=%d, next_retry=%s, delivery_time_ms=%d, error=%v",
				deviceID, msg.MessageID, msg.Attempts+1, nextRetry.Format(time.RFC3339), deliveryDuration, err)
		} else {
			deliveryDuration := time.Since(deliveryStartTime).Milliseconds()

			// Success, delete from queue
			if err := ds.db.MarkSuccess(msg.ID); err != nil {
				ds.logger.Errorf("Failed to mark message as success: device=%s, msg_id=%s, error=%v",
					deviceID, msg.MessageID, err)
				return
			}

			ds.logger.Infof("Successfully enqueued message: device=%s, msg_id=%s, attempt=%d, delivery_time_ms=%d",
				deviceID, msg.MessageID, msg.Attempts+1, deliveryDuration)
		}

		// Increment counter
		messagesProcessed++
	}
}

// deliver attempts to enqueue the message to the Renr Queue API
func (ds *DeliveryService) deliver(msg *RenrQueueMessage) error {
	// Create enqueue request
	ref := msg.DeviceID
	_, err := ds.queueClient.Enqueue(msg.QueueTopic, renrqueue.EnqueueRequest{
		Ref:     &ref,
		Payload: msg.Payload,
	})

	if err != nil {
		// Classify error for retry decision
		if !ds.isRetryableError(err) {
			ds.logger.Errorf("Non-retryable error, marking as permanent failure: device=%s, msg_id=%s, error=%v",
				msg.DeviceID, msg.MessageID, err)
			// For non-retryable errors, we still return the error but it will be logged
			// The caller will mark it as failed, and cleanup will eventually mark it as permanent
		}
		return err
	}

	return nil
}

// isRetryableError determines if an error is worth retrying
func (ds *DeliveryService) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are retryable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Check for connection errors
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for syscall errors
	if errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ECONNRESET) {
		return true
	}

	// Check error string for API status codes
	// The renrqueue package returns errors like "API error (status 400): ..."
	errStr := err.Error()

	// Check for 4xx errors (client errors - not retryable except 429)
	if strings.Contains(errStr, "API error (status 4") {
		// 429 Too Many Requests is retryable
		if strings.Contains(errStr, "API error (status 429)") {
			return true
		}
		// Other 4xx errors are not retryable
		ds.logger.Warnf("Client error (4xx), not retryable: %v", err)
		return false
	}

	// Check for 5xx errors (server errors - retryable)
	if strings.Contains(errStr, "API error (status 5") {
		return true
	}

	// Default: retry unknown errors (safer to retry than to lose messages)
	ds.logger.Debugf("Unknown error type, will retry: %v", err)
	return true
}

// cleanup runs periodic cleanup tasks
func (ds *DeliveryService) cleanup() {
	ds.logger.Debugf("Running periodic cleanup...")

	// Mark old failed messages as permanent failures
	if err := ds.db.CleanupOldFailed(); err != nil {
		ds.logger.Errorf("Failed to cleanup old failed messages: %v", err)
	}

	// Log queue statistics
	stats, err := ds.db.GetStats()
	if err != nil {
		ds.logger.Errorf("Failed to get queue stats: %v", err)
		return
	}

	if stats["total"] > 0 {
		ds.logger.Infof("Renr queue stats: total=%d, pending=%d, processing=%d, failed=%d",
			stats["total"], stats["pending"], stats["processing"], stats["failed"])
	}
}
