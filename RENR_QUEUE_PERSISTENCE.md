# Renr Queue Persistence Implementation

## Overview

This document describes the SQL persistence and retry logic implementation for the Renr queue plugin, preventing message loss when the Renr Queue API is unavailable.

## Problem Statement

Previously, incoming WhatsApp messages were sent directly to the Renr Queue API with no persistence layer. If the API was unavailable, messages would be lost permanently.

## Solution

Added a SQLite-based persistence layer with background worker and exponential backoff retry logic, following the same pattern as the webhook plugin.

## Architecture

### Components

1. **Database Layer** (`src/plugins/renr/database.go`)
   - Manages SQLite persistence for queued messages
   - Provides CRUD operations for queue management
   - Handles duplicate detection via UNIQUE constraint

2. **Delivery Service** (`src/plugins/renr/delivery.go`)
   - Background worker that processes pending messages
   - Implements exponential backoff retry strategy
   - Handles per-device message ordering (FIFO)
   - Classifies errors for retry decisions

3. **Plugin Integration** (`src/plugins/renr/plugin.go`)
   - Initializes database and delivery service
   - Manages lifecycle (start/stop)
   - Provides configuration via environment variables

4. **Message Handlers** (`src/plugins/renr/incoming.go`)
   - Updated to use persistence layer
   - Fallback to direct enqueue if persistence fails

## Database Schema

```sql
CREATE TABLE IF NOT EXISTS renr_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    queue_topic TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_attempt DATETIME,
    next_retry DATETIME,

    UNIQUE(device_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_renr_queue_status ON renr_queue(status);
CREATE INDEX IF NOT EXISTS idx_renr_queue_next_retry ON renr_queue(next_retry);
CREATE INDEX IF NOT EXISTS idx_renr_queue_device_status ON renr_queue(device_id, status);
```

### Table Details

- **id**: Auto-incrementing primary key
- **device_id**: Device identifier (used for FIFO ordering and filtering)
- **message_id**: WhatsApp message ID
- **queue_topic**: Target queue topic (e.g., "channel-chat-message-incoming")
- **payload**: JSON payload to enqueue
- **status**: Current status (pending, processing, failed)
- **attempts**: Number of delivery attempts
- **last_error**: Last error message
- **created_at**: Timestamp when message was persisted
- **last_attempt**: Timestamp of last delivery attempt
- **next_retry**: Scheduled time for next retry attempt

### Indexes

- **idx_renr_queue_status**: Fast lookup by status
- **idx_renr_queue_next_retry**: Fast lookup for pending retries
- **idx_renr_queue_device_status**: Fast lookup by device and status

### Unique Constraint

`UNIQUE(device_id, message_id)` prevents duplicate message persistence.

## Message Flow

### State Transitions

```
New Message → pending → processing → DELETE (success)
                     ↓
                   failed → pending (retry) → ...
                     ↓
                permanent failure (>24h old)
```

### Processing Flow

1. **Incoming Message**:
   - WhatsApp message received → converted to QueueChatMessage
   - `deliveryService.Enqueue()` persists to database
   - Status: `pending`

2. **Background Worker** (every 5 seconds):
   - Queries all devices with pending messages
   - For each device:
     - Acquires per-device lock (ensures FIFO)
     - Waits for rate limiter token (respects RENR_QUEUE_MAX_REQUESTS_PER_SEC)
     - Gets oldest pending message for device
     - Marks as `processing`
     - Attempts delivery to Renr Queue API
     - On success: DELETE from database
     - On failure: Mark as `failed`, schedule retry

3. **Retry Strategy**:
   - Exponential backoff: 1min, 2min, 4min, 8min, 16min, 32min, ...
   - Max backoff: 24 hours (1440 minutes)
   - Formula: `backoff = 2^(attempts-1)` minutes, capped at 1440

4. **Cleanup** (every 5 minutes):
   - Messages older than 24 hours with attempts > 0 → marked as permanent failure
   - Statistics logged to console

## Configuration

### Environment Variables

```bash
# Enable/disable persistence (default: true)
RENR_QUEUE_PERSISTENCE_ENABLED=true

# Worker interval for processing pending messages (default: 5s)
RENR_QUEUE_WORKER_INTERVAL=5s

# Maximum retry delay for failed messages (default: 24h)
RENR_QUEUE_MAX_RETRY_DELAY=24h

# Maximum requests per second for queue processing (default: 25)
# Controls the rate limit for processing pending messages from the queue
RENR_QUEUE_MAX_REQUESTS_PER_SEC=25

# Database path (shared with whatsmeow, default: file:./whatsapp.db?_foreign_keys=on&mode=rwc)
DB_PATH=file:./whatsapp.db?_foreign_keys=on&mode=rwc
```

### Disabling Persistence

Set `RENR_QUEUE_PERSISTENCE_ENABLED=false` to disable persistence. This will:
- Skip database initialization
- Use direct enqueue (no retry on failure)
- Log warning about potential message loss

## Error Handling

### Error Classification

The `isRetryableError()` function classifies errors:

**Retryable Errors**:
- Network errors (timeout, connection refused, connection reset)
- HTTP 5xx errors (server errors)
- HTTP 429 (Too Many Requests)
- Unknown errors (safer to retry)

**Non-Retryable Errors**:
- HTTP 4xx errors (except 429)
  - 400 Bad Request
  - 401 Unauthorized
  - 404 Not Found
  - etc.

### Fallback Behavior

If persistence fails (database error), the system falls back to direct enqueue:
1. Log error about persistence failure
2. Attempt direct enqueue to Renr Queue API
3. If direct enqueue also fails, log error

This ensures best-effort delivery even if the database has issues.

## Message Ordering (FIFO)

### Per-Device Locks

The delivery service uses `sync.Map` to maintain per-device processing locks:

```go
type DeliveryService struct {
    processingDevices sync.Map // deviceID -> bool
    // ...
}
```

### FIFO Guarantee

1. Only ONE message per device is processed at a time
2. Messages are processed in `created_at` order (oldest first)
3. Concurrent processing across different devices is allowed
4. Lock is acquired before processing, released after completion

This ensures:
- Messages from the same device are delivered in order
- No race conditions within a device's message queue
- Efficient parallel processing across devices

## Logging

### Log Levels

**INFO**:
- Persistence initialization
- Successful message enqueue
- Delivery worker started/stopped
- Queue statistics (every 5 minutes if queue not empty)

**WARN**:
- Persistence disabled warning
- Failed delivery attempts with retry schedule
- Non-retryable errors
- Fallback to direct enqueue

**ERROR**:
- Database errors
- Permanent failures (messages older than 24h)
- Direct enqueue failures

**DEBUG**:
- Worker processing details
- Device already being processed (skip)
- Unknown error types being retried

### Example Log Output

```
[INFO] Renr queue persistence enabled: db=file:./whatsapp.db?_foreign_keys=on&mode=rwc
[INFO] Delivery service initialized: worker_interval=5s, max_retry_delay=24h, max_requests_per_sec=25
[INFO] Persisted incoming message: device=123, msg_id=ABC, topic=channel-chat-message-incoming
[INFO] Processing pending message: device=123, msg_id=ABC, attempt=1
[INFO] Successfully enqueued message: device=123, msg_id=ABC, attempt=1
[WARN] Failed to enqueue message: device=123, msg_id=DEF, attempt=2, next_retry=2024-11-13T10:32:00Z, error=...
[INFO] Renr queue stats: total=5, pending=3, processing=2, failed=0
[ERROR] Permanent failure: device=123, msg_id=XYZ, attempts=15, age=24h, error=...
```

## Testing

### Manual Testing Scenarios

1. **Normal Operation**:
   - Send WhatsApp message
   - Verify message persisted to database
   - Verify message delivered to Renr Queue
   - Verify message deleted from database

2. **Renr Queue Unavailable**:
   - Stop Renr Queue API
   - Send WhatsApp messages
   - Verify messages persist to database with status=pending
   - Verify retry attempts with exponential backoff
   - Start Renr Queue API
   - Verify messages delivered and deleted

3. **Message Ordering**:
   - Send multiple messages from same device rapidly
   - Verify messages delivered in FIFO order
   - Check database shows processing lock behavior

4. **Duplicate Messages**:
   - Attempt to persist same message_id twice
   - Verify UNIQUE constraint prevents duplicates

5. **Cleanup**:
   - Create messages older than 24 hours (manually in DB)
   - Wait for cleanup cycle
   - Verify old messages marked as permanent failure

### Database Inspection

```bash
# Open SQLite database
sqlite3 whatsapp.db

# View pending messages
SELECT device_id, message_id, status, attempts, created_at, next_retry
FROM renr_queue
WHERE status = 'pending'
ORDER BY device_id, created_at;

# View queue statistics
SELECT status, COUNT(*) as count
FROM renr_queue
GROUP BY status;

# View failed messages
SELECT device_id, message_id, attempts, last_error, created_at
FROM renr_queue
WHERE status = 'failed'
ORDER BY created_at DESC;
```

## Performance Considerations

### Database Performance

- Uses same SQLite database as whatsmeow (`whatsapp.db`)
- Connection string includes `mode=rwc` for better concurrency
- Indexes on frequently queried columns (status, next_retry, device_id)
- DELETE on success (no accumulation of successful messages)

### Worker Performance

- 5-second interval (configurable via `RENR_QUEUE_WORKER_INTERVAL`)
- Per-device parallelism (processes multiple devices concurrently)
- Per-device FIFO (only one message per device at a time)
- Lock-based synchronization (minimal contention)
- Rate limiting (configurable via `RENR_QUEUE_MAX_REQUESTS_PER_SEC`, default 25 req/s)
  - Uses token bucket algorithm for smooth request distribution
  - Prevents overwhelming the Renr Queue API
  - Burst capacity equals rate limit (allows immediate processing when idle)

### Memory Considerations

- Messages stored in SQLite (not in-memory)
- Background worker uses minimal memory
- Goroutines spawned per device (bounded by device count)

## Migration from Old Version

No migration required. The implementation:
- Creates table on first run if not exists
- Backward compatible (no breaking changes to existing functionality)
- Falls back to direct enqueue if persistence disabled

## Future Enhancements

Potential improvements (not yet implemented):

1. **Dead Letter Queue**:
   - Move permanent failures to separate table
   - Admin API to view/retry DLQ messages

2. **Metrics and Monitoring**:
   - Expose Prometheus metrics
   - Track enqueue rate, success rate, retry count

3. **Priority Queues**:
   - Add priority field
   - Process high-priority messages first

4. **Batch Processing**:
   - Send multiple messages in single API call
   - Reduce API overhead

5. **Configurable Backoff Strategy**:
   - Linear, exponential, or custom backoff
   - Per-message timeout configuration

6. **Message TTL**:
   - Configurable expiration time
   - Auto-delete expired messages

## Troubleshooting

### Messages Not Being Delivered

1. Check database for pending messages:
   ```sql
   SELECT * FROM renr_queue WHERE status = 'pending';
   ```

2. Check last_error column:
   ```sql
   SELECT device_id, message_id, last_error, attempts FROM renr_queue WHERE status = 'failed';
   ```

3. Check worker is running (logs should show "Delivery service started")

4. Check Renr Queue API is accessible

5. Check for 4xx errors (non-retryable, need to fix request format)

### High Database Size

1. Check for failed messages not being cleaned up:
   ```sql
   SELECT COUNT(*), status FROM renr_queue GROUP BY status;
   ```

2. Manually delete old failed messages:
   ```sql
   DELETE FROM renr_queue WHERE status = 'failed' AND created_at < datetime('now', '-7 days');
   ```

### Duplicate Messages

1. Check UNIQUE constraint is working:
   ```sql
   SELECT device_id, message_id, COUNT(*) FROM renr_queue GROUP BY device_id, message_id HAVING COUNT(*) > 1;
   ```

2. If duplicates exist, clean up manually:
   ```sql
   DELETE FROM renr_queue WHERE id NOT IN (
       SELECT MIN(id) FROM renr_queue GROUP BY device_id, message_id
   );
   ```

## Files Modified/Created

### Created Files

1. `/Users/erlangga/IdeaProjects/whatsapp/src/plugins/renr/database.go`
   - Database layer with schema and CRUD operations
   - ~270 lines

2. `/Users/erlangga/IdeaProjects/whatsapp/src/plugins/renr/delivery.go`
   - Delivery service with background worker
   - ~300 lines

3. `/Users/erlangga/IdeaProjects/whatsapp/RENR_QUEUE_PERSISTENCE.md`
   - This documentation file

### Modified Files

1. `/Users/erlangga/IdeaProjects/whatsapp/src/plugins/renr/plugin.go`
   - Added database and delivery service initialization
   - Added lifecycle management (Start/Stop)
   - Added configuration parsing

2. `/Users/erlangga/IdeaProjects/whatsapp/src/plugins/renr/incoming.go`
   - Updated `handleIncomingMessage()` to use persistence
   - Updated `handleHistorySynced()` to use persistence
   - Added fallback to direct enqueue

3. `/Users/erlangga/IdeaProjects/whatsapp/.env.example`
   - Added persistence configuration options

## Summary

The implementation provides:

✅ **Message Persistence**: SQLite-based queue for reliable message delivery
✅ **Automatic Retry**: Exponential backoff with 24-hour max delay
✅ **Message Ordering**: Strict FIFO per device
✅ **Duplicate Detection**: UNIQUE constraint prevents duplicates
✅ **Error Classification**: Smart retry decisions based on error type
✅ **Graceful Degradation**: Fallback to direct enqueue if persistence fails
✅ **Configuration**: Environment variables for customization
✅ **Monitoring**: Comprehensive logging and statistics
✅ **Performance**: Efficient per-device parallelism
✅ **Cleanup**: Automatic removal of old failures

The solution follows the established patterns in the codebase (similar to webhook plugin) and integrates seamlessly with the existing architecture.
