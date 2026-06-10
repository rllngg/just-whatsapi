# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a production-ready WhatsApp Gateway service built with Go that provides RESTful API endpoints for programmatic WhatsApp messaging. It uses an event-driven plugin architecture to decouple core WhatsApp functionality from integration layers (webhooks, queues).

## Build and Run Commands

```bash
# Install dependencies
go mod download

# Run locally (requires CGO for SQLite)
go run main.go

# Build binary (CGO required for SQLite)
CGO_ENABLED=1 go build -o whatsapp-gateway .

# Docker build (multi-stage with CGO)
docker build -t whatsapp-gateway .

# Run test environment (MinIO + webhook receiver)
cd test && docker-compose up -d

# Check test webhook receiver
curl http://localhost:3000/webhooks
```

## Architecture Overview

### Event-Driven Plugin System

The core architectural pattern is **event-driven decoupling**:

```
Manager (Core) --publishes events--> EventBus --routes to--> Plugins
Manager (Core) <--direct calls------- Plugins
```

**Key principle**: The Manager has ZERO knowledge of plugins. All Manager→Plugin communication flows through the EventBus (Watermill). Plugins can directly call Manager methods.

### Three-Layer Architecture

1. **Core Layer** (`src/core/`):
   - `whatsapp/manager.go` - Device lifecycle, message operations, event publishing
   - `whatsapp/device_db.go` - Persistent device_id ↔ WhatsApp JID mapping
   - `whatsapp/events.go` - Strongly-typed event structs
   - `event/event.go` - Watermill-based pub/sub EventBus
   - `gateway/http/server.go` - Fiber HTTP API endpoints

2. **Plugin Layer** (`src/plugins/`):
   - `webhook/` - Delivers WhatsApp events to HTTP webhooks (SQLite queue, retry logic, S3 uploads)
   - `renr/` - Bidirectional queue integration (WhatsApp ↔ Renr Queue System)

3. **Library Layer** (`src/renr-queue/`):
   - Standalone, reusable Go client for Renr Queue API
   - No WhatsApp dependencies, could be extracted as separate module

### Device Management Critical Details

**Two Separate Databases**:
- `whatsapp.db` (whatsmeow store): WhatsApp protocol state, encryption keys
- `device_mapping` table (in same db): User's device_id → WhatsApp's JID mapping

This enables:
- Custom device IDs (users don't need to know WhatsApp's JID format)
- Failover recovery (app restart automatically reconnects devices)

**Device Lifecycle States**:
1. `pending` - Device created, waiting for QR generation
2. `qr_ready` - QR code ready for scanning
3. `connected` - Device authenticated and connected

**Device Restoration**: On startup, `RestoreDevices()` automatically reconnects all previously connected devices from the database.

### Event System Architecture

**Event Flow**:
```
WhatsApp event occurs → Manager publishes to EventBus → All subscribers receive → Process independently
```

**Key Events**:
- `device.created`, `device.qr_ready`, `device.connected`, `device.disconnected`
- `message.received` - All incoming WhatsApp messages
- `message.text.sent`, `message.image.sent`, etc. - Outgoing message confirmations
- `message.*.failed` - Failure events for all operations

**Type Safety**: Events use strongly-typed structs (not `map[string]interface{}`), defined in `src/core/whatsapp/events.go`.

### Plugin System Details

**Plugin Interface Pattern**:
```go
type Plugin struct {
    eventBus EventSubscriber  // Subscribe to Manager events
    manager  *Manager         // Call Manager methods directly
    ctx      context.Context  // Lifecycle management
    cancel   context.CancelFunc
}
```

**Lifecycle**: `Initialize()` → `Start()` → `Stop()`

**Webhook Plugin** (`src/plugins/webhook/`):
- Subscribes to ALL WhatsApp events
- Queues events in SQLite (`webhook_queue` table)
- Background worker delivers via HTTP POST
- Exponential backoff retry (1min, 2min, 4min, ..., max 24h)
- S3/MinIO integration for media uploads
- Auto-cleanup after 7 days

**Renr Queue Plugin** (`src/plugins/renr/`):
- **Per-device subscribers**: When a device connects, creates dedicated queue subscriber for that device
- **Incoming** (`incoming.go`): WhatsApp messages → persisted to SQLite → background worker delivers to `channel-chat-message-incoming`
- **Outgoing** (`outgoing.go`): Subscribes to `channel-chat-message-outgoing` (filtered by device) → sends via WhatsApp
- **QR Handler** (`qr_handler.go`): Subscribes to `whatsapp-request-qr` → creates device → returns QR code
- **Media Handling** (`media.go`): Downloads from WhatsApp, uploads to S3, returns public URLs
- **Persistence Layer** (`database.go`, `delivery.go`): SQLite queue with retry logic to prevent message loss when Renr Queue is unavailable
- **Device Lifecycle**: Auto-start/stop subscribers on connect/disconnect events

### Message Flow Examples

**Sending Message (HTTP → WhatsApp)**:
```
1. POST /message/text with device_id and chat_id
2. server.go handler → manager.SendTextMessage()
3. Manager validates, sends to WhatsApp
4. Manager publishes message.text.sent or message.text.failed event
5. Plugins receive event (webhook delivers to URL, Renr enqueues, etc.)
```

**Receiving Message (WhatsApp → Queue)**:
```
1. WhatsApp sends message to device
2. whatsmeow event handler → Manager.handleMessage()
3. Manager publishes message.received event
4. Renr plugin receives event
5. Plugin downloads media from WhatsApp URL
6. Plugin uploads media to S3/MinIO
7. Plugin persists to SQLite (renr_queue table, status: pending)
8. Background worker processes pending messages (every 5s)
9. Worker enqueues to channel-chat-message-incoming with S3 URL
10. On success: DELETE from SQLite; On failure: retry with exponential backoff
```

**Queue → WhatsApp (Outgoing)**:
```
1. External system enqueues to channel-chat-message-outgoing with ref=device_id
2. Renr plugin's per-device subscriber pulls message
3. Plugin calls manager.SendTextMessage() or SendImageMessage()
4. Manager sends to WhatsApp
5. Manager publishes message.*.sent event
6. Plugin marks queue message as Complete
```

## Configuration

### Environment Variables

**Required**:
- `HTTP_PORT` - Server port (default: 8081)
- `DB_PATH` - SQLite path with connection params (default: `file:./whatsapp.db?_foreign_keys=on&mode=rwc`)

**Webhook Plugin** (optional, plugin disabled if missing):
- `PLUGINS_WEBHOOK_URL` - Webhook endpoint (required for plugin)
- `PLUGINS_WEBHOOK_S3_ENABLED` - Enable S3 uploads (true/false)
- `PLUGINS_WEBHOOK_S3_BUCKET`, `PLUGINS_WEBHOOK_S3_REGION`, `PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID`, etc.

**Renr Queue Plugin** (optional, plugin disabled if missing):
- `RENR_QUEUE_URL` - Queue API URL (default: http://localhost:8080)
- `RENR_QUEUE_USERNAME` - Basic auth username (required)
- `RENR_QUEUE_PASSWORD` - Basic auth password (required)
- `RENR_QUEUE_PERSISTENCE_ENABLED` - Enable persistence (default: true)
- `RENR_QUEUE_WORKER_INTERVAL` - Background worker poll interval (default: 5s)
- `RENR_QUEUE_MAX_RETRY_DELAY` - Maximum retry backoff delay (default: 24h)
- `RENR_QUEUE_MAX_REQUESTS_PER_SEC` - Rate limit for queue processing (default: 25)

**Logging**:
- `WHATSAPP_LOG_LEVEL` - INFO/DEBUG/ERROR
- `WATERMILL_DEBUG` - Enable event bus debug logging
- `WATERMILL_TRACE` - Enable event bus trace logging

### Graceful Degradation

If plugin configuration is missing, the plugin initialization fails but **the application continues** without that plugin. This allows:
- Running as HTTP-only gateway without webhooks
- Running without queue integration
- Partial functionality in degraded environments

## API Endpoints

All endpoints use JSON. Full Swagger docs available at `/swagger/index.html` when running.

**Device Management**:
- `POST /device/new` - Create device, returns QR code (optional `device_id` in body)
- `GET /device` - List all devices with status

**Messaging**:
- `POST /message/text` - Send text message (supports `reply_message_id`)
- `POST /message/image` - Send image with caption (downloads from `file_url`, uploads to WhatsApp)
- `POST /message/file` - Send document
- `POST /message/presence` - Send typing indicator
- `POST /message/emoji` - Send reaction
- `POST /message/history` - Fetch message history (asynchronous, requires primary device online)

All message endpoints require:
- `device_id` - Your device identifier
- `chat_id` - WhatsApp JID format (e.g., `6281234567890@s.whatsapp.net` for DM, `120363...@g.us` for group)

**Message History Endpoint Details**:
- **Asynchronous**: Returns `request_id` immediately, messages arrive later via events/webhooks
- **Requirements**: Primary WhatsApp device (phone) must be online and connected
- **Pagination**: Use `before_message_id` to fetch messages before a specific message
- **Batch size**: Default 50, max 100 messages per request
- **Response time**: 2-10 seconds depending on network and device availability
- **Events**: Publishes `message.history.requested`, `message.history.synced`, `message.history.failed`

## Database Schema

**device_mapping** table:
```sql
CREATE TABLE device_mapping (
    device_id TEXT PRIMARY KEY,  -- User's identifier
    device_jid TEXT NOT NULL     -- WhatsApp's JID
);
```

**webhook_queue** table (webhook plugin):
```sql
CREATE TABLE webhook_queue (
    id INTEGER PRIMARY KEY,
    event TEXT NOT NULL,
    payload TEXT NOT NULL,
    webhook_url TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_error TEXT,
    created_at DATETIME,
    next_retry DATETIME
);
```

**renr_queue** table (Renr plugin persistence):
```sql
CREATE TABLE renr_queue (
    id INTEGER PRIMARY KEY,
    device_id TEXT NOT NULL,
    message_id TEXT NOT NULL,
    queue_topic TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_error TEXT,
    created_at DATETIME,
    last_attempt DATETIME,
    next_retry DATETIME,
    UNIQUE(device_id, message_id)
);
```

## Code Patterns and Conventions

### Adding New Event Types

1. Define struct in `src/core/whatsapp/events.go`:
```go
type MyNewEvent struct {
    DeviceID string
    Data     string
}
```

2. Publish from Manager:
```go
manager.eventBus.Publish("my.new.event", MyNewEvent{...})
```

3. Subscribe in plugin:
```go
manager.eventBus.Subscribe("my.new.event", func(payload any) error {
    event := payload.(MyNewEvent)
    // Handle event
    return nil
})
```

### Creating New Plugins

1. Create `src/plugins/myplugin/plugin.go`:
```go
type Plugin struct {
    eventBus EventSubscriber
    manager  *whatsapp.Manager
    ctx      context.Context
    cancel   context.CancelFunc
}

func NewPlugin(eventBus EventSubscriber) (*Plugin, error) {
    // Load configuration, validate
    return &Plugin{eventBus: eventBus}, nil
}

func (p *Plugin) Initialize(manager *whatsapp.Manager) error {
    p.manager = manager
    p.ctx, p.cancel = context.WithCancel(context.Background())
    return nil
}

func (p *Plugin) Start() error {
    // Subscribe to events
    p.eventBus.Subscribe("message.received", p.handleMessage)
    return nil
}

func (p *Plugin) Stop() error {
    p.cancel()
    return nil
}
```

2. Register in `main.go`:
```go
myPlugin, err := myplugin.NewPlugin(eventBus)
if err == nil {
    myPlugin.Initialize(manager)
    myPlugin.Start()
    defer myPlugin.Stop()
}
```

### Media Handling Pattern

For operations involving media (images, files, audio, video):

1. **Downloading**: Use `http.Get()` to fetch from URL
2. **Uploading to WhatsApp**: Use whatsmeow's `Client.Upload()` method
3. **Uploading to S3**: Use AWS SDK v2 `s3.PutObject()` with public ACL
4. **URL Handling**: Always provide fallback URLs if S3 upload fails

Example in `src/plugins/renr/media.go`.

## Testing

### Test Environment

Located in `test/docker-compose.yml`:
- **MinIO** (port 9000): S3-compatible storage for media
  - Console: http://localhost:9001 (minioadmin/minioadmin)
- **Webhook Receiver** (port 3000): Go server that logs webhooks
  - View webhooks: `GET http://localhost:3000/webhooks`
  - Clear history: `DELETE http://localhost:3000/webhooks/clear`

Start test environment:
```bash
cd test
docker-compose up -d
```

### Manual Testing Flow

1. Start test environment and gateway
2. Create device: `curl -X POST http://localhost:8081/device/new`
3. Scan QR code with WhatsApp mobile app
4. Wait for `device.connected` webhook
5. Send test message: `curl -X POST http://localhost:8081/message/text -d '{"device_id":"...","chat_id":"...","message":"test"}'`
6. Check webhook receiver: `curl http://localhost:3000/webhooks`

## Important Implementation Details

### CGO Requirement

This project **requires CGO** because of SQLite (mattn/go-sqlite3). Always build with:
```bash
CGO_ENABLED=1 go build
```

The Dockerfile handles this automatically with `gcc`, `musl-dev`, and `sqlite-dev` in the builder stage.

### Concurrency Considerations

- **Device Map**: `manager.devices` uses `sync.RWMutex` for concurrent access
- **Event Bus**: Watermill handles concurrent publishing/subscribing
- **SQLite**: Connection string includes `mode=rwc` for better concurrency
- **Plugin Subscribers**: Each runs in separate goroutine with context cancellation

### Error Handling Philosophy

- **Fail gracefully**: Missing plugin config → disable plugin, not crash
- **Publish failure events**: All operations publish both success and failure events
- **Retry with backoff**: Webhook and Renr Queue delivery retry with exponential backoff (1min → 24h max)
- **Log, don't crash**: Plugins log errors but don't panic
- **Persistence for reliability**: Renr plugin persists messages to SQLite before delivery to prevent loss

### Renr Queue Persistence Pattern

The Renr plugin implements a **persistence-first pattern** to prevent message loss:

1. **Incoming Message Flow**:
   ```
   WhatsApp → Manager → Event → Plugin → SQLite (pending) → Background Worker → Renr Queue API
   ```

2. **Background Worker**:
   - Runs every 5 seconds (configurable via `RENR_QUEUE_WORKER_INTERVAL`)
   - Processes pending messages with FIFO guarantee per device
   - Per-device locks ensure message ordering (only one message per device processed at a time)
   - On success: DELETE from SQLite immediately
   - On failure: Mark as failed, schedule retry with exponential backoff

3. **Retry Strategy**:
   - Exponential backoff: 1min, 2min, 4min, 8min, 16min, 32min, ...
   - Max backoff: 24 hours (1440 minutes)
   - Error classification: 4xx errors (except 429) are non-retryable, 5xx and network errors are retryable
   - Permanent failure: Messages older than 24 hours marked as permanent failure (kept for debugging)

4. **Implementation Files**:
   - `src/plugins/renr/database.go` - SQLite persistence layer with `renr_queue` table
   - `src/plugins/renr/delivery.go` - Background worker with retry logic and error classification
   - `src/plugins/renr/incoming.go` - Updated to use `deliveryService.Enqueue()` instead of direct enqueue

5. **Key Features**:
   - **FIFO ordering**: Per-device processing locks ensure messages delivered in order
   - **Duplicate prevention**: UNIQUE constraint on (device_id, message_id)
   - **Graceful degradation**: Falls back to direct enqueue if persistence fails
   - **Cleanup**: Automatic marking of old failures (>24h) as permanent

See `RENR_QUEUE_PERSISTENCE.md` for detailed documentation.

### Queue Message Format

The Renr plugin uses `QueueChatMessage` struct (defined in `src/plugins/renr/types.go`):
```go
type QueueChatMessage struct {
    ChannelID int64           // Numeric device identifier
    MessageID string          // WhatsApp message ID
    Ref       string          // Device reference (device_id)
    To        QueueChatPerson // Recipient
    From      QueueChatPerson // Sender
    Body      QueueChatBody   // Content (text, files, etc.)
    Timestamp int64           // Unix timestamp
}
```

**Important**: When enqueuing to outgoing queue, always set:
- `ref` field to `device_id` (used for subscriber filtering)
- `channelId` to numeric channel identifier
- `timestamp` to current Unix timestamp

### Timestamp Parsing

The Renr queue client handles multiple timestamp formats:
- Unix timestamp (int64): `1699999999`
- ISO 8601: `"2024-11-14T10:30:00Z"`
- RFC3339: `"2024-11-14T10:30:00+07:00"`

Implementation in `src/renr-queue/renr.go` (`parseFlexibleTime()`).

### Message History Implementation

The message history feature allows fetching older messages from WhatsApp by requesting them from the user's primary device (phone).

**Implementation Location**: `src/core/whatsapp/manager.go:802` (`FetchMessageHistory`)

**How It Works**:
1. Client calls `POST /message/history` with `device_id`, `chat_id`, optional `before_message_id`, and `count`
2. Manager builds a history sync request using whatsmeow's `BuildHistorySyncRequest(messageInfo, count)`
3. Request is sent to the user's own JID with `SendRequestExtra{Peer: true}` to route to primary device
4. Manager publishes `message.history.requested` event
5. WhatsApp primary device receives request and sends back history
6. History arrives as `events.HistorySync` events (type: `ON_DEMAND`)
7. Manager publishes `message.history.synced` event with messages array
8. Webhook plugin delivers to configured webhook URL (if enabled)

**Key Implementation Details**:

1. **MessageInfo Construction**:
   - If `before_message_id` provided: Creates MessageInfo with that ID for pagination
   - If omitted: Creates MessageInfo with current timestamp to fetch most recent messages
   - MessageInfo includes the chat JID for filtering

2. **Request Validation**:
   - Device must exist and be in "connected" status
   - Count defaults to 50 if not provided or ≤ 0
   - Count capped at 100 to prevent overload
   - Chat ID must be valid WhatsApp JID format

3. **Asynchronous Nature**:
   - The HTTP endpoint returns immediately with `request_id`
   - Actual messages arrive later via `events.HistorySync` events
   - Response includes empty `messages` array initially
   - For synchronous operation, would need to implement request tracking with channels/timeouts

4. **Event Publishing**:
   - `message.history.requested` - Contains request details and `request_id`
   - `message.history.synced` - Contains actual messages (from HistorySync event handler)
   - `message.history.failed` - If request sending fails

**Limitations**:
- Requires primary WhatsApp device (phone) to be online and connected
- Primary device must have the requested message history stored
- Response time varies (2-10s typically) based on network and device
- WhatsApp may rate-limit excessive history requests
- History is only available for chats that exist on the primary device

**Future Enhancements** (Not Yet Implemented):
- Handle `events.HistorySync` events in manager to extract messages
- Store pending requests in a map with channels for synchronous responses
- Implement timeout for history requests (e.g., 30 seconds)
- Parse HistorySync protobuf messages into MessagePayload structs
- Correlate HistorySync responses with requests using `request_id`
- Support different HistorySync types (INITIAL_BOOTSTRAP, PUSH_NAME, etc.)

**Usage Example**:
```go
// In manager.go event handler (to be implemented):
func (m *Manager) setupEventHandlers(deviceID string, client *whatsmeow.Client) {
    client.AddEventHandler(func(evt interface{}) {
        switch v := evt.(type) {
        case *events.HistorySync:
            // Extract messages from v.Data
            // Publish message.history.synced event with messages
        }
    })
}
```

**Testing Considerations**:
- Requires actual WhatsApp phone connection for testing
- Cannot be easily mocked due to dependency on WhatsApp infrastructure
- Best tested with integration test using real QR code connection
- Consider timeout handling in tests

## Common Pitfalls

1. **Forgetting CGO**: If you get "undefined symbol" errors, ensure `CGO_ENABLED=1`
2. **Wrong JID format**: WhatsApp JIDs must end with `@s.whatsapp.net` (DM) or `@g.us` (group)
3. **Missing device connection**: Always check device is `connected` before sending messages
4. **Message history requires primary device**: The `/message/history` endpoint requires your primary WhatsApp phone to be online. If it's offline, the request will timeout without returning history.
4. **Event type assertion**: When handling events, always type-assert: `event := payload.(MessageReceivedEvent)`
5. **Queue ref field**: For Renr outgoing messages, `ref` MUST match device's `device_id`
6. **Media URLs**: Must be publicly accessible URLs; the gateway downloads them to upload to WhatsApp
7. **Database path**: Include `mode=rwc` in SQLite connection string for better concurrency

## Documentation References

- `docs/USAGE.md` - API usage guide with curl examples
- `docs/SPEC.md` - Complete API specification
- `docs/EVENT.md` - Event system documentation
- `docs/PLUGINS_WEBHOOK.md` - Webhook plugin guide
- `QUEUE_INTEGRATION.md` - Queue integration architecture
- `RENR_PLUGIN_IMPLEMENTATION.md` - Renr plugin details
- `RENR_QUEUE_PERSISTENCE.md` - Renr queue persistence and retry logic
- `src/renr-queue/README.md` - Queue client library docs
