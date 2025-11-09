package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DeliveryService handles webhook delivery with retry logic
type DeliveryService struct {
	db         *Database
	s3Client   *S3Client
	webhookURL string
	httpClient *http.Client
	stopChan   chan struct{}
}

// NewDeliveryService creates a new webhook delivery service
func NewDeliveryService(db *Database, s3Client *S3Client, webhookURL string) *DeliveryService {
	return &DeliveryService{
		db:         db,
		s3Client:   s3Client,
		webhookURL: webhookURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		stopChan: make(chan struct{}),
	}
}

// Start starts the webhook delivery worker
func (ds *DeliveryService) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	// Initial processing
	ds.processPending(ctx)

	// Periodic processing
	for {
		select {
		case <-ctx.Done():
			return
		case <-ds.stopChan:
			return
		case <-ticker.C:
			ds.processPending(ctx)
		}
	}
}

// Stop stops the delivery service
func (ds *DeliveryService) Stop() {
	close(ds.stopChan)
}

// processPending processes pending webhooks from the queue
func (ds *DeliveryService) processPending(ctx context.Context) {
	webhooks, err := ds.db.GetPending(10)
	if err != nil {
		fmt.Printf("Failed to get pending webhooks: %v\n", err)
		return
	}

	for _, wh := range webhooks {
		// Mark as processing
		if err := ds.db.MarkProcessing(wh.ID); err != nil {
			fmt.Printf("Failed to mark webhook %d as processing: %v\n", wh.ID, err)
			continue
		}

		// Deliver webhook
		if err := ds.deliver(ctx, wh); err != nil {
			// Mark as failed with error
			if err := ds.db.MarkFailed(wh.ID, err.Error()); err != nil {
				fmt.Printf("Failed to mark webhook %d as failed: %v\n", wh.ID, err)
			}
		} else {
			// Mark as success
			if err := ds.db.MarkSuccess(wh.ID); err != nil {
				fmt.Printf("Failed to mark webhook %d as success: %v\n", wh.ID, err)
			}
		}
	}

	// Cleanup old webhooks (older than 7 days)
	if err := ds.db.CleanupOld(7 * 24 * time.Hour); err != nil {
		fmt.Printf("Failed to cleanup old webhooks: %v\n", err)
	}
}

// deliver sends the webhook HTTP request
func (ds *DeliveryService) deliver(ctx context.Context, wh *WebhookQueue) error {
	// Parse payload
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(wh.Payload), &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// Create webhook payload with timestamp
	webhookPayload := map[string]interface{}{
		"event":     wh.Event,
		"payload":   payload,
		"timestamp": time.Now().Unix(),
	}

	// Marshal to JSON
	body, err := json.Marshal(webhookPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", wh.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "WhatsApp-Webhook/1.0")

	// Send request
	resp, err := ds.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

// Enqueue adds a webhook to the delivery queue
func (ds *DeliveryService) Enqueue(event string, payload map[string]interface{}) error {
	return ds.db.Enqueue(event, payload, ds.webhookURL)
}

// EnqueueWithS3Upload adds a webhook to the queue after uploading media to S3
func (ds *DeliveryService) EnqueueWithS3Upload(ctx context.Context, event string, payload map[string]interface{}, mediaData []byte, mediaFilename string, mediaContentType string) error {
	// Upload to S3 if client is available and media data is provided
	if ds.s3Client != nil && len(mediaData) > 0 {
		s3URL, err := ds.s3Client.UploadFile(ctx, mediaData, mediaFilename, mediaContentType)
		if err != nil {
			return fmt.Errorf("failed to upload media to S3: %w", err)
		}

		// Add S3 URL to payload
		if payload["payload"] != nil {
			if payloadMap, ok := payload["payload"].(map[string]interface{}); ok {
				if payloadMap["message"] != nil {
					if messageMap, ok := payloadMap["message"].(map[string]interface{}); ok {
						messageMap["media_url"] = s3URL
					}
				}
			}
		}
	}

	return ds.db.Enqueue(event, payload, ds.webhookURL)
}
