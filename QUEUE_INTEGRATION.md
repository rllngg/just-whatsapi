# Queue Integration Summary

## Overview
Successfully integrated the Renr Queue subscriber system into the WhatsApp Gateway application with a clean, generic API.

## What Was Done

### 1. Fixed Timestamp Parsing Issue
**File:** `src/renr-queue/renr.go`

Added `FlexibleTime` custom type to handle timestamps without timezone information:
- Supports multiple timestamp formats (RFC3339, without timezone, various precision levels)
- Automatically tries multiple formats during JSON unmarshaling
- Fixed the original error: `parsing time "2025-11-12T15:45:46.871959" as "2006-01-02T15:04:05Z07:00"`

### 2. Created Generic Subscriber API
**File:** `src/renr-queue/subscriber.go`

A simple, reusable queue subscriber with callback-based API:

```go
subscriber, err := client.Subscribe("my-topic", "1s", func(msg *QueueItemResponse) error {
    // Process message
    return nil // nil = success, error = failed
})
```

**Features:**
- Simple callback API
- Automatic message completion/failure handling
- Built-in retry on error
- Optional ref filtering: `WithRef("user-123")`
- Custom logger support: `WithLogger(myLogger)`
- Runs in background automatically
- Graceful shutdown: `subscriber.Stop()`

### 3. Moved WhatsApp Examples to renr-queue Package
**Files moved:**
- `src/core/whatsapp/subscribe.go` → `src/renr-queue/whatsapp_example.go`
- `src/core/whatsapp/subscribe_example.go` → `src/renr-queue/whatsapp_integration.go`
- Deleted: `src/core/whatsapp/SUBSCRIBE_README.md`

**New example files:**
- `whatsapp_example.go` - WhatsApp-specific implementation example
- `whatsapp_integration.go` - 5 simple integration examples

### 4. Integrated Queue Subscriber into Manager
**File:** `src/core/whatsapp/manager.go`

Added queue integration directly into the WhatsApp Manager:

**New Fields:**
- `queueClient *renrqueue.Client` - Queue client instance
- `subscriber *renrqueue.Subscriber` - Active subscriber instance

**New Methods:**
- `SetQueueClient(client *renrqueue.Client)` - Set queue client
- `StartQueueSubscriber() error` - Start subscriber for WhatsApp QR requests
- `StopQueueSubscriber()` - Stop subscriber
- `handleQRRequest(msg *QueueItemResponse) error` - Process QR request messages
- `waitForQRCode(device *Device, timeout time.Duration) string` - Wait for QR code
- `sendChannelUpdate(...)` - Send channel updates to queue

**Features:**
- Automatically processes `whatsapp-request-qr` topic
- Creates WhatsApp devices on demand
- Waits for QR code generation (30s timeout)
- Sends status updates to `channel-update` queue
- Thread-safe with mutex protection
- Graceful shutdown in `Close()` method

### 5. Updated Main Application
**File:** `main.go`

Integrated queue subscriber into application startup:

```go
// Initialize queue client
queueClient, err := renrqueue.NewClientFromEnv()
if err != nil {
    log.Printf("Warning: Failed to create queue client: %v", err)
} else {
    // Set queue client on manager
    manager.SetQueueClient(queueClient)

    // Start queue subscriber
    if err := manager.StartQueueSubscriber(); err != nil {
        log.Printf("Warning: Failed to start queue subscriber: %v", err)
    }
}
```

**Features:**
- Graceful degradation (app continues if queue unavailable)
- Automatic cleanup on shutdown via `manager.Close()`
- Configuration via environment variables

## Configuration

Add to `.env` file:

```env
RENR_QUEUE_URL=https://api.renr.biz.id
RENR_QUEUE_USERNAME=your-username
RENR_QUEUE_PASSWORD=your-password
```

## How It Works

### Message Flow

1. **Incoming QR Request**
   - Message arrives in `whatsapp-request-qr` queue
   - Subscriber automatically pulls the message
   - `handleQRRequest()` processes it

2. **Device Creation**
   - Extracts channel ID from payload
   - Creates WhatsApp device with channel ID as device ID
   - Waits for QR code generation (max 30 seconds)

3. **Status Updates**
   - On success: Sends `QR_READY` to `channel-update` queue with QR code
   - On error: Sends `ERROR` to `channel-update` queue with error message
   - Message automatically marked as completed/failed

### Queue Message Format

**Request Payload (whatsapp-request-qr):**
```json
{
  "channel": {
    "id": 123,
    "metadata": "optional metadata",
    "status": "pending"
  }
}
```

**Response Payload (channel-update):**
```json
{
  "channel_id": 123,
  "status": "QR_READY",
  "reference": "123",
  "data": "base64_encoded_qr_code",
  "errorReason": null
}
```

## Usage

### Start the Application

```bash
go run main.go
```

Expected output:
```
Restoring previously connected devices...
Initializing queue subscriber for WhatsApp QR requests...
INFO: Subscriber started for topic: whatsapp-request-qr (interval: 1s)
Queue subscriber started successfully
Starting Fiber HTTP server on :8081
```

### Enqueue a QR Request

Using the queue client:

```go
client, _ := renrqueue.NewClientFromEnv()

payload := `{
  "channel": {
    "id": 123,
    "metadata": "test",
    "status": "pending"
  }
}`

item, err := client.Enqueue("whatsapp-request-qr", renrqueue.EnqueueRequest{
    Payload: payload,
})
```

### Monitor Queue Status

```go
stats, err := client.GetStats("whatsapp-request-qr")
fmt.Printf("Pending: %d, Processing: %d, Completed: %d\n",
    stats.Pending, stats.Processing, stats.Completed)
```

## Error Handling

The subscriber automatically handles errors:

1. **Payload validation errors** → Message marked as failed (will retry)
2. **Device creation errors** → Error sent to channel-update queue
3. **QR timeout** → Error sent to channel-update queue
4. **Queue unavailable** → Application logs warning but continues

## Benefits

1. ✅ **Simple API** - One-line subscription with callbacks
2. ✅ **Automatic Retry** - Failed messages retry automatically
3. ✅ **Graceful Degradation** - App works without queue
4. ✅ **Clean Architecture** - Queue logic separated from business logic
5. ✅ **Thread-Safe** - Concurrent processing with mutex protection
6. ✅ **Easy Testing** - Mock the queue client for testing
7. ✅ **Logging** - Integrated with WhatsApp logger

## Files Modified/Created

### Created:
- `src/renr-queue/subscriber.go` - Generic subscriber implementation
- `src/renr-queue/whatsapp_example.go` - WhatsApp example
- `src/renr-queue/whatsapp_integration.go` - Integration examples
- `QUEUE_INTEGRATION.md` - This documentation

### Modified:
- `src/renr-queue/renr.go` - Added FlexibleTime for timestamp parsing
- `src/renr-queue/README.md` - Added subscriber documentation
- `src/core/whatsapp/manager.go` - Integrated queue subscriber
- `main.go` - Added queue initialization

### Deleted:
- `src/core/whatsapp/SUBSCRIBE_README.md`
- `src/core/whatsapp/subscribe.go`
- `src/core/whatsapp/subscribe_example.go`

## Next Steps

1. Deploy application with queue credentials
2. Test with real queue messages
3. Monitor logs for errors
4. Consider adding metrics/monitoring
5. Add more queue topics for other operations

## Troubleshooting

**Queue subscriber not starting:**
- Check `.env` has correct `RENR_QUEUE_*` values
- Verify queue API is accessible
- Check application logs for detailed errors

**Messages not processing:**
- Check queue stats: `client.GetStats("whatsapp-request-qr")`
- Verify message format matches expected payload
- Check application logs for processing errors

**QR code timeout:**
- Increase timeout in `manager.go` if needed
- Check WhatsApp connection status
- Verify device creation is working
