# Renr Queue Client

Go client library for the Renr Queue System API with retry and dead letter queue support.

## Features

- HTTP Basic Authentication
- Complete queue operations (enqueue, pull, complete, fail)
- **Simple subscriber API with callback handlers**
- Automatic retry with exponential backoff
- Dead letter queue management
- Queue statistics and monitoring
- Environment variable configuration
- Type-safe Go structs

## Installation

The client uses standard Go modules and requires:
- Go 1.25+
- `github.com/joho/godotenv` (already included in project dependencies)

## Configuration

### Environment Variables

Add the following variables to your `.env` file:

```env
RENR_QUEUE_URL=https://api.renr.biz.id
RENR_QUEUE_USERNAME=your-username
RENR_QUEUE_PASSWORD=your-password
```

**Environment Variables:**
- `RENR_QUEUE_URL` - Base URL for the queue API (default: `http://localhost:8080`)
- `RENR_QUEUE_USERNAME` - Username for basic authentication (required)
- `RENR_QUEUE_PASSWORD` - Password for basic authentication (required)

## Usage

### Option 1: Create Client from Environment Variables

```go
import "whatsapp/src/renr-queue"

// Load .env file
if err := renrqueue.LoadEnv(); err != nil {
    log.Printf("Warning: Could not load .env file: %v", err)
}

// Create client from environment variables
client, err := renrqueue.NewClientFromEnv()
if err != nil {
    log.Fatalf("Failed to create client: %v", err)
}
```

### Option 2: Create Client Manually

```go
client := renrqueue.NewClient(
    "https://api.renr.biz.id",
    "username",
    "password",
)
```

## Quick Start: Subscribe to Queue (Recommended)

The simplest way to process queue messages is using the **Subscribe** API:

```go
import renrqueue "whatsapp/src/renr-queue"

// Create client
client, err := renrqueue.NewClientFromEnv()
if err != nil {
    log.Fatal(err)
}

// Subscribe to a topic with a callback that runs every 1 second
subscriber, err := client.Subscribe("my-topic", "1s", func(msg *renrqueue.QueueItemResponse) error {
    fmt.Printf("Processing message ID=%d\n", msg.ID)

    // Parse and process your message
    if msg.Payload != nil {
        fmt.Printf("Payload: %s\n", *msg.Payload)
    }

    // Return nil to mark as completed
    // Return error to mark as failed (will auto-retry)
    return nil
})

if err != nil {
    log.Fatal(err)
}

// Subscriber runs in background automatically
// Stop when done:
defer subscriber.Stop()
```

### Subscriber Features

**Filter by Reference:**
```go
subscriber, err := client.Subscribe("notifications", "500ms", handler,
    renrqueue.WithRef("user-123"))
```

**Custom Logger:**
```go
subscriber, err := client.Subscribe("my-topic", "1s", handler,
    renrqueue.WithLogger(myLogger))
```

**Multiple Subscribers:**
```go
sub1, _ := client.Subscribe("emails", "1s", emailHandler)
sub2, _ := client.Subscribe("sms", "500ms", smsHandler)
sub3, _ := client.Subscribe("push", "2s", pushHandler)

defer func() {
    sub1.Stop()
    sub2.Stop()
    sub3.Stop()
}()
```

**Error Handling:**
```go
subscriber, err := client.Subscribe("tasks", "1s", func(msg *renrqueue.QueueItemResponse) error {
    var task MyTask
    if err := json.Unmarshal([]byte(*msg.Payload), &task); err != nil {
        return err // Message marked as failed and will retry
    }

    // Process task...
    if err := processTask(task); err != nil {
        return fmt.Errorf("processing failed: %w", err)
    }

    return nil // Message marked as completed
})
```

See `whatsapp_integration.go` for more examples.

## API Examples (Manual Mode)

### Enqueue a Message

```go
item, err := client.Enqueue("email", renrqueue.EnqueueRequest{
    Ref:     renrqueue.StringPtr("order-12345"),
    Payload: `{"to":"user@example.com","subject":"Hello"}`,
    TimeoutInSeconds: renrqueue.IntPtr(3600), // Optional: 1 hour timeout
})
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Enqueued: ID=%d, Status=%s\n", item.ID, item.Status)
```

### Pull and Process Message

```go
// Pull next message (returns nil if queue is empty)
msg, err := client.Pull("email", nil)
if err != nil {
    log.Fatal(err)
}

if msg == nil {
    fmt.Println("No messages available")
    return
}

fmt.Printf("Processing message ID=%d, Payload=%s\n", msg.ID, *msg.Payload)

// Mark as completed after successful processing
completed, err := client.Complete("email", msg.ID)
if err != nil {
    log.Fatal(err)
}
```

### Handle Failure with Automatic Retry

```go
// Mark message as failed (automatically retries if retries available)
result, err := client.MarkAsFailed("email", msg.ID, renrqueue.MarkFailedRequest{
    Reason: "Connection timeout after 30 seconds",
})
if err != nil {
    log.Fatal(err)
}

if result.WillRetry {
    fmt.Printf("Will retry at %v (New ID: %d)\n",
        result.NextRetryAt, *result.NewQueueItemID)
} else {
    fmt.Println("Max retries exceeded - moved to dead letter queue")
}
```

### Get Queue Statistics

```go
stats, err := client.GetStats("email")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Pending: %d, Processing: %d, Completed: %d, Failed: %d\n",
    stats.Pending, stats.Processing, stats.Completed, stats.Failed)
```

### Get Failed Messages (Dead Letter Queue)

```go
// Get first page of failed messages (page 0, size 20)
failedMsgs, err := client.GetFailedMessages("email", 0, 20)
if err != nil {
    log.Fatal(err)
}

for _, msg := range failedMsgs.Content {
    fmt.Printf("Failed: ID=%d, Reason=%s, RetryCount=%d\n",
        msg.ID, *msg.Response, msg.RetryCount)
}
```

### Manually Retry Failed Message

```go
// Retry a message from dead letter queue
retryResult, err := client.RetryFailed("email", failedMessageID)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Retried: Original ID=%d, New ID=%d\n",
    retryResult.OriginalID, retryResult.NewQueueItem.ID)
```

## Queue States

- `PENDING` - Message waiting to be processed
- `PROCESSING` - Message currently being processed (pulled)
- `COMPLETED` - Message successfully processed
- `FAILED` - Message failed after max retries (dead letter)

## Retry Strategy

The queue system uses exponential backoff for automatic retries:

1. **Retry 1**: 30 seconds delay
2. **Retry 2**: 150 seconds delay (2.5 minutes)
3. **Retry 3**: 750 seconds delay (12.5 minutes)
4. **After 3 retries**: Marked as FAILED (dead letter queue)

## Error Handling

All methods return detailed error information:

```go
item, err := client.Enqueue("email", req)
if err != nil {
    // Error includes status code and message from API
    log.Printf("API error: %v", err)
    return
}
```

## Helper Functions

```go
// Create string pointer
ref := renrqueue.StringPtr("order-123")

// Create int pointer
timeout := renrqueue.IntPtr(3600)
```

## Complete Examples

- **`whatsapp_integration.go`** - Simple subscriber examples and integration patterns
- **`whatsapp_example.go`** - WhatsApp-specific use case demonstrating complex message handling
- **`example_usage.go`** - Manual API usage examples (pull, complete, fail)

## API Documentation

For complete API documentation, see the Swagger specification at:
- `swagger.yaml` in this directory
- Production: https://api.renr.biz.id/swagger-ui.html (if available)
