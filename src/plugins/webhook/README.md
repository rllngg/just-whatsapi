# Webhook Plugin

This plugin enables webhook notifications for WhatsApp events with AWS S3 integration for media files.

## Features

- ✅ Webhook notifications for all WhatsApp events
- ✅ AWS S3 integration for automatic media file uploads
- ✅ SQLite-based retry queue with exponential backoff
- ✅ Automatic retry for failed webhooks (up to 24 hours)
- ✅ Supports all message types (text, image, video, audio, document)
- ✅ Event-driven architecture using Watermill

## Configuration

Configure the webhook plugin using environment variables:

### Required
- `PLUGINS_WEBHOOK_URL` - Your webhook endpoint URL

### Optional (S3 Integration)
- `PLUGINS_WEBHOOK_S3_ENABLED` - Enable S3 uploads (true/false)
- `PLUGINS_WEBHOOK_S3_REGION` - AWS region (e.g., us-east-1)
- `PLUGINS_WEBHOOK_S3_BUCKET` - S3 bucket name
- `PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID` - AWS access key ID
- `PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY` - AWS secret access key
- `PLUGINS_WEBHOOK_S3_ENDPOINT` - Optional: Custom S3 endpoint (for MinIO, etc.)

## Events

The plugin listens to and sends webhooks for the following events:

### Device Events
- ✅ `device.created` - When a new device is created (manager.go:141)
- ✅ `device.connected` - When device successfully connects
- ✅ `device.disconnected` - When device loses connection (manager.go:151)
- ✅ `device.logout` - When device is logged out (manager.go:162)
- ✅ `device.qr_ready` - When QR code is ready for scanning

### Message Events (Incoming from WhatsApp)
- ✅ `message.received` - New message received (manager.go:143)
  - Supports text, image, video, audio, and document messages
  - Media files are automatically uploaded to S3 if enabled

### Message Events (Outgoing from HTTP API)
- ✅ `message.text.sent` - Text message sent successfully (manager.go:386)
- ✅ `message.text.failed` - Text message failed to send
- ✅ `message.image.sent` - Image message sent successfully (manager.go:453)
- ✅ `message.image.failed` - Image message failed to send
- ✅ `message.file.sent` - File message sent successfully (manager.go:519)
- ✅ `message.file.failed` - File message failed to send
- ✅ `message.presence.sent` - Presence update sent
- ✅ `message.presence.failed` - Presence update failed
- ✅ `message.reaction.sent` - Reaction sent
- ✅ `message.reaction.failed` - Reaction failed

### Read Receipts
- ✅ `message.read` - Message read by recipient (manager.go:176)

## Webhook Payload Format

All webhooks are sent with the following structure:

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "message": {
      "message_id": "ABC123",
      "chat_id": "1234567890@s.whatsapp.net",
      "sender": "1234567890@s.whatsapp.net",
      "timestamp": 1234567890,
      "from_me": false,
      "type": "text",
      "text": {
        "content": "Hello, world!"
      }
    }
  },
  "timestamp": 1234567890
}
```

### Image Message Example

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "message": {
      "message_id": "ABC123",
      "chat_id": "1234567890@s.whatsapp.net",
      "sender": "1234567890@s.whatsapp.net",
      "timestamp": 1234567890,
      "from_me": false,
      "type": "image",
      "has_media": true,
      "image": {
        "mimetype": "image/jpeg",
        "caption": "Check this out!",
        "url": "https://whatsapp-url...",
        "media_url": "https://your-bucket.s3.amazonaws.com/whatsapp-media/1234567890-image.jpg"
      }
    }
  },
  "timestamp": 1234567890
}
```

## Retry Logic

The plugin implements automatic retry with exponential backoff:

- Failed webhooks are retried automatically
- Backoff schedule: 1min, 2min, 4min, 8min, 16min, 32min, 64min, etc.
- Maximum backoff: 24 hours
- Webhooks older than 24 hours are discarded
- Successfully delivered webhooks are kept for 7 days then cleaned up

## Database

The plugin uses SQLite to persist the webhook queue:

- Database file: `webhook_queue.db`
- Table: `webhook_queue`
- Tracks delivery status, attempts, and retry schedule
- Automatic cleanup of old webhooks

## Architecture

```
EventBus (Watermill)
    ↓
Subscriber (webhook/subscriber.go)
    ↓
DeliveryService (webhook/delivery.go)
    ↓
Database Queue (webhook/database.go)
    ↓
HTTP POST to Webhook URL
```

### S3 Upload Flow (for media messages)

```
Message with Media
    ↓
Extract Media Data
    ↓
Upload to S3 (webhook/s3.go)
    ↓
Add S3 URL to Payload
    ↓
Enqueue Webhook
```

## Usage Example

```go
import (
    "whatsapp/src/core/event"
    "whatsapp/src/plugins/webhook"
)

// Create event bus
eventBus := event.NewEventBus(logger)

// Initialize webhook plugin
webhookPlugin, err := webhook.NewPlugin(eventBus)
if err != nil {
    log.Fatal(err)
}

// Start the plugin
if err := webhookPlugin.Start(); err != nil {
    log.Fatal(err)
}

// Stop the plugin on shutdown
defer webhookPlugin.Stop()
```

## Dependencies

To use this plugin, add the following dependencies to your `go.mod`:

```bash
go get github.com/aws/aws-sdk-go-v2/aws
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/mattn/go-sqlite3
```

## Testing Your Webhook

You can test your webhook endpoint using tools like:
- [webhook.site](https://webhook.site) - Free webhook testing
- [ngrok](https://ngrok.com) - Expose local server to internet
- [RequestBin](https://requestbin.com) - Inspect webhook requests

## Security Considerations

1. **Use HTTPS** - Always use HTTPS for your webhook URL
2. **Validate Requests** - Implement signature verification on your webhook endpoint
3. **Rate Limiting** - Implement rate limiting on your webhook endpoint
4. **Secure S3** - Use private S3 buckets and signed URLs if needed
5. **Environment Variables** - Never commit `.env` file with credentials

## Troubleshooting

### Webhooks not being delivered
- Check `webhook_queue.db` for pending/failed webhooks
- Verify webhook URL is accessible
- Check logs for delivery errors

### S3 uploads failing
- Verify AWS credentials are correct
- Check S3 bucket permissions
- Ensure bucket exists in the specified region

### Database locked errors
- Ensure only one instance of the application is running
- Check file permissions on `webhook_queue.db`
