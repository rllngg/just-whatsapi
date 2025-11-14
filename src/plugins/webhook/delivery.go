package webhook

import (
	"bytes"
	"context"
	"encoding/base64"
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
// If webhookURL is empty, events are stored in SQLite but not delivered (store-only mode)
func NewDeliveryService(db *Database, s3Client *S3Client, webhookURL string) *DeliveryService {
	if webhookURL == "" {
		fmt.Println("[WEBHOOK] Starting in STORE-ONLY mode (no webhook URL configured)")
		fmt.Println("[WEBHOOK] Events will be stored in SQLite but not delivered")
	} else {
		fmt.Printf("[WEBHOOK] Starting with webhook URL: %s\n", webhookURL)
	}

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
	// Skip processing if no webhook URL configured (store-only mode)
	if ds.webhookURL == "" {
		// Just cleanup old webhooks in store-only mode
		if err := ds.db.CleanupOld(7 * 24 * time.Hour); err != nil {
			fmt.Printf("Failed to cleanup old webhooks: %v\n", err)
		}
		return
	}

	webhooks, err := ds.db.GetPending(10)
	if err != nil {
		fmt.Printf("Failed to get pending webhooks: %v\n", err)
		return
	}

	for _, wh := range webhooks {
		// Skip if webhook has no URL (stored for logging only)
		if wh.WebhookURL == "" {
			fmt.Printf("[WEBHOOK] Skipping webhook %d (no URL, store-only mode)\n", wh.ID)
			continue
		}

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

	// Create webhook payload with timestamp using DTO
	webhookPayload := WebhookPayload{
		Event:     wh.Event,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}

	// Marshal to JSON
	body, err := json.Marshal(webhookPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	// Log webhook details
	fmt.Printf("[WEBHOOK] Sending webhook to URL: %s\n", wh.WebhookURL)
	fmt.Printf("[WEBHOOK] Event: %s, Payload: %s\n", wh.Event, string(body))

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
		fmt.Printf("[WEBHOOK] Failed to send webhook: %v\n", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Printf("[WEBHOOK] Webhook failed with status: %d\n", resp.StatusCode)
		return fmt.Errorf("webhook returned non-2xx status: %d", resp.StatusCode)
	}

	fmt.Printf("[WEBHOOK] Webhook delivered successfully, status: %d\n", resp.StatusCode)
	return nil
}

// Enqueue intelligently handles media if present in payload
// If webhook URL is empty, events are stored in SQLite but not delivered
func (ds *DeliveryService) Enqueue(event string, payload map[string]interface{}) error {
	// Check if event contains media data
	if mediaData := ds.extractMediaFromPayload(payload); mediaData != nil && ds.s3Client != nil {
		fmt.Printf("[WEBHOOK] Media detected in webhook payload (size: %d bytes)\n", len(mediaData.Data))

		// Upload media to S3
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		s3URL, err := ds.s3Client.UploadFile(ctx,
			mediaData.Data,
			mediaData.Filename,
			mediaData.Mimetype)

		if err != nil {
			// Log error but don't fail - keep base64 data in payload
			fmt.Printf("[WEBHOOK] Failed to upload media to S3: %v (keeping base64 data)\n", err)
		} else {
			// Replace base64 data with S3 URL in payload to reduce webhook size
			ds.replaceMediaWithURL(payload, s3URL)
			fmt.Printf("[WEBHOOK] Uploaded media to S3: %s\n", s3URL)
		}
	}

	if ds.webhookURL == "" {
		fmt.Printf("[WEBHOOK] Storing event in SQLite (no webhook URL configured): event=%s\n", event)
	} else {
		fmt.Printf("[WEBHOOK] Enqueuing event for delivery: event=%s, url=%s\n", event, ds.webhookURL)
	}
	return ds.db.Enqueue(event, payload, ds.webhookURL)
}

// EnqueueWithS3Upload adds a webhook to the queue after uploading media to S3
// If webhook URL is empty, events are stored in SQLite but not delivered
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

	if ds.webhookURL == "" {
		fmt.Printf("[WEBHOOK] Storing event with media in SQLite (no webhook URL configured): event=%s\n", event)
	} else {
		fmt.Printf("[WEBHOOK] Enqueuing event with media for delivery: event=%s, url=%s\n", event, ds.webhookURL)
	}

	return ds.db.Enqueue(event, payload, ds.webhookURL)
}

// MediaData represents media file data (matching events.MediaData)
type MediaData struct {
	Data     []byte
	Mimetype string
	Filename string
	Size     int64
	SHA256   string
}

// extractMediaFromPayload extracts MediaData from event payload
func (ds *DeliveryService) extractMediaFromPayload(payload map[string]interface{}) *MediaData {
	// Navigate: payload -> message -> (image|video|audio|document|sticker) -> media
	messageMap, ok := payload["message"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Check each media type
	for _, mediaType := range []string{"image", "video", "audio", "document", "sticker"} {
		if mediaMap, ok := messageMap[mediaType].(map[string]interface{}); ok {
			if media, ok := mediaMap["media"].(map[string]interface{}); ok {
				// Extract MediaData fields
				dataStr, _ := media["data"].(string)
				if dataStr == "" {
					continue
				}

				// Decode base64 data
				data, err := base64.StdEncoding.DecodeString(dataStr)
				if err != nil {
					fmt.Printf("[WEBHOOK] Failed to decode media data: %v\n", err)
					continue
				}

				return &MediaData{
					Data:     data,
					Mimetype: getStringField(media, "mimetype"),
					Filename: getStringField(media, "filename"),
					Size:     int64(len(data)),
					SHA256:   getStringField(media, "sha256"),
				}
			}
		}
	}
	return nil
}

// replaceMediaWithURL replaces media.data with media_url in payload
func (ds *DeliveryService) replaceMediaWithURL(payload map[string]interface{}, s3URL string) {
	messageMap, ok := payload["message"].(map[string]interface{})
	if !ok {
		return
	}

	for _, mediaType := range []string{"image", "video", "audio", "document", "sticker"} {
		if mediaMap, ok := messageMap[mediaType].(map[string]interface{}); ok {
			// Remove large data field to reduce webhook payload size
			if media, ok := mediaMap["media"].(map[string]interface{}); ok {
				delete(media, "data")
			}
			// Add S3 URL at media type level
			mediaMap["media_url"] = s3URL
		}
	}
}

// getStringField safely returns string value from map
func getStringField(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}
