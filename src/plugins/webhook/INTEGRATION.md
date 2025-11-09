# Webhook Plugin Integration Guide

## Step 1: Install Dependencies

Add the required Go modules:

```bash
go get github.com/aws/aws-sdk-go-v2/aws
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/mattn/go-sqlite3
```

## Step 2: Configure Environment Variables

Copy `.env.example` to `.env` and configure:

```env
PLUGINS_WEBHOOK_URL=https://your-webhook-endpoint.com/webhook

# Optional: Enable S3 for media uploads
PLUGINS_WEBHOOK_S3_ENABLED=true
PLUGINS_WEBHOOK_S3_REGION=us-east-1
PLUGINS_WEBHOOK_S3_BUCKET=your-bucket-name
PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID=your-access-key
PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY=your-secret
```

## Step 3: Initialize in Main Application

In your `main.go` or application initialization:

```go
package main

import (
    "log"
    "whatsapp/src/core/event"
    "whatsapp/src/plugins/webhook"
)

func main() {
    // Initialize event bus
    eventBus := event.NewEventBus(logger)
    
    // Initialize webhook plugin
    webhookPlugin, err := webhook.NewPlugin(eventBus)
    if err != nil {
        log.Fatalf("Failed to initialize webhook plugin: %v", err)
    }
    
    // Start webhook plugin
    if err := webhookPlugin.Start(); err != nil {
        log.Fatalf("Failed to start webhook plugin: %v", err)
    }
    
    // Ensure cleanup on shutdown
    defer webhookPlugin.Stop()
    
    // ... rest of your application
}
```

## Step 4: Set Event Publisher in WhatsApp Manager

Make sure your WhatsApp manager has the event publisher set:

```go
manager, err := whatsapp.NewManager(dbPath, logger)
if err != nil {
    log.Fatal(err)
}

// Set the event bus as publisher
manager.SetEventPublisher(eventBus)
```

## Step 5: Test the Integration

1. Start your application
2. Create a WhatsApp device
3. Send a message
4. Check your webhook endpoint for the payload

## Webhook Endpoint Implementation Example

Here's a simple webhook endpoint in Go:

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
)

type WebhookPayload struct {
    Event     string                 `json:"event"`
    DeviceID  string                 `json:"device_id"`
    Payload   map[string]interface{} `json:"payload"`
    Timestamp int64                  `json:"timestamp"`
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
    // Read body
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }
    defer r.Body.Close()
    
    // Parse payload
    var payload WebhookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }
    
    // Process webhook
    log.Printf("Received webhook: %s for device %s", payload.Event, payload.DeviceID)
    log.Printf("Payload: %+v", payload.Payload)
    
    // Respond with success
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func main() {
    http.HandleFunc("/webhook", webhookHandler)
    log.Println("Webhook server listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

## Monitoring

### Check Webhook Queue Status

Query the SQLite database to see pending webhooks:

```sql
sqlite3 webhook_queue.db

-- View pending webhooks
SELECT * FROM webhook_queue WHERE status = 'pending';

-- View failed webhooks
SELECT * FROM webhook_queue WHERE status = 'failed';

-- Count by status
SELECT status, COUNT(*) FROM webhook_queue GROUP BY status;
```

### Logs

The plugin logs important events:
- Webhook delivery attempts
- S3 upload status
- Database errors
- Retry schedules

## Troubleshooting

### Issue: Webhooks not being sent

**Check:**
1. Event bus is properly initialized
2. Manager has event publisher set
3. Webhook URL is accessible
4. Database permissions are correct

**Debug:**
```bash
# Check pending webhooks
sqlite3 webhook_queue.db "SELECT * FROM webhook_queue WHERE status != 'success' LIMIT 10;"
```

### Issue: S3 uploads failing

**Check:**
1. AWS credentials are correct
2. S3 bucket exists
3. IAM permissions allow `s3:PutObject`
4. Bucket region matches configuration

### Issue: Database locked

**Solution:**
- Ensure only one application instance is running
- Close any SQLite browser connections
- Check file permissions on `webhook_queue.db`

## Advanced Configuration

### Custom Retry Logic

Modify `delivery.go` to customize retry backoff:

```go
// In MarkFailed function
backoffMinutes := 1 << (attempts - 1) // Default: exponential
// Custom: linear backoff
// backoffMinutes := attempts * 5 // 5, 10, 15, 20...
```

### Webhook Signature Verification

Add HMAC signature to webhooks in `delivery.go`:

```go
import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
)

func (ds *DeliveryService) deliver(ctx context.Context, wh *WebhookQueue) error {
    // ... existing code ...
    
    // Add signature
    secret := os.Getenv("WEBHOOK_SECRET")
    h := hmac.New(sha256.New, []byte(secret))
    h.Write(body)
    signature := hex.EncodeToString(h.Sum(nil))
    
    req.Header.Set("X-Webhook-Signature", signature)
    
    // ... rest of delivery code ...
}
```

## Production Checklist

- [ ] Configure webhook URL with HTTPS
- [ ] Set up S3 bucket with proper permissions
- [ ] Configure AWS IAM user with minimal permissions
- [ ] Set up monitoring for webhook failures
- [ ] Configure database backups for `webhook_queue.db`
- [ ] Set up log rotation
- [ ] Implement webhook signature verification
- [ ] Test failover scenarios
- [ ] Document webhook payload structure for your team
- [ ] Set up alerts for high failure rates
