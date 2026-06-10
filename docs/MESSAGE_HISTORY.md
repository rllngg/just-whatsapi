# Message History Feature Documentation

## Overview

The message history feature provides two ways to access older WhatsApp messages:

1. **Automatic History Sync** - When a device first connects via QR code, WhatsApp automatically sends recent message history
2. **Manual History Request** - Explicitly request older messages from a specific chat via API

Both methods use WhatsApp's history sync protocol and work with your primary WhatsApp device (phone).

## How It Works

### Architecture

```
HTTP Client → Gateway API → whatsmeow Library → WhatsApp Server → Primary Device
                ↓                                                        ↓
            Event Bus ← HistorySync Event ← WhatsApp Server ← Primary Device
                ↓
           Webhooks/Plugins
```

### Flow Diagrams

#### Automatic History Sync (First Connection)

```
1. User scans QR code
   ↓
2. Device connects to WhatsApp
   ↓
3. WhatsApp Server automatically sends history sync
   ↓
4. Gateway receives events.HistorySync
   ↓
5. Manager extracts messages from history sync
   ↓
6. Manager publishes message.history.synced event
   ↓
7. Plugins receive event (Renr enqueues, Webhook sends)
```

#### Manual History Request

1. **Request Phase**:
   - Client sends `POST /message/history` with device_id, chat_id, and optional pagination parameters
   - Gateway validates device is connected
   - Gateway builds history sync request using `BuildHistorySyncRequest`
   - Request is sent to user's own JID with `SendRequestExtra{Peer: true}`
   - Publishes `message.history.requested` event
   - Returns `request_id` to client immediately

2. **Response Phase** (Asynchronous):
   - Primary WhatsApp device receives the request
   - Primary device sends history back to gateway
   - Gateway receives `events.HistorySync` event with messages
   - Gateway publishes `message.history.synced` event
   - Webhook plugin delivers messages to configured webhook

## Automatic History Sync

### When It Happens

Automatic history sync occurs when:
- A device **first connects** via QR code (not on device restoration from database)
- WhatsApp server automatically initiates the sync
- No API call required

### What Gets Synced

WhatsApp automatically sends:
- Recent messages from recent conversations
- The exact scope is determined by WhatsApp's algorithm
- Typically includes messages from the last few days/weeks

### How to Receive

Messages are delivered via the same `message.history.synced` event as manual requests:

1. **Webhook Plugin**: Messages sent to configured webhook URL
2. **Renr Queue Plugin**: Messages enqueued to `channel-chat-message-incoming`
3. **Custom Plugins**: Subscribe to `message.history.synced` event

### Implementation Details

Location: `src/core/whatsapp/manager.go:148-204` (`handleHistorySync`)

The handler:
1. Receives `events.HistorySync` from whatsmeow
2. Extracts conversations and messages
3. Converts to `MessagePayload` format
4. Publishes `message.history.synced` event with all messages

### Differences from Manual Request

| Aspect | Automatic Sync | Manual Request |
|--------|---------------|----------------|
| **Trigger** | QR code first connection | HTTP API call |
| **Scope** | All recent chats | Specific chat |
| **Control** | WhatsApp decides | User controls (count, pagination) |
| **RequestID** | Empty | Generated UUID |
| **Frequency** | Once per new device | On demand |

## Manual History Request API

### Endpoint

```
POST /message/history
```

### Request Body

```json
{
  "device_id": "device_1",
  "chat_id": "6289685028129@s.whatsapp.net",
  "before_message_id": "3EB089B9D6ADD58153C561",
  "count": 50
}
```

**Fields**:
- `device_id` (required): Your device identifier
- `chat_id` (required): WhatsApp JID (e.g., `6281234567890@s.whatsapp.net` for DM, `120363...@g.us` for group)
- `before_message_id` (optional): Message ID to fetch messages before (for pagination)
- `count` (optional): Number of messages to fetch (default: 50, max: 100)

### Response

```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "messages": [],
  "count": 0
}
```

**Fields**:
- `request_id`: Unique identifier to track this request
- `messages`: Empty array (messages arrive via events/webhooks)
- `count`: 0 initially

### Success Response (200 OK)

The endpoint returns immediately with a tracking ID. Actual messages arrive asynchronously.

### Error Responses

**400 Bad Request**:
```json
{
  "error": "Invalid request body"
}
```

**500 Internal Server Error**:
```json
{
  "error": "device not found: device_1"
}
```

Common error scenarios:
- Device not found
- Device not connected
- Invalid chat ID format
- Failed to send history sync request

## Requirements

### Technical Requirements

1. **Primary Device Connection**:
   - Your primary WhatsApp device (phone) must be online
   - Phone must be connected to the internet
   - Phone must have the requested chat history stored

2. **Gateway Device Status**:
   - Device must be in "connected" status
   - Device must have valid WhatsApp session

3. **Network**:
   - Stable internet connection on both gateway and phone
   - No restrictive firewalls blocking WhatsApp protocol

### Limitations

1. **Asynchronous Operation**:
   - No immediate response with messages
   - Requires webhook or event listener to receive messages
   - Response time varies (typically 2-10 seconds)

2. **Primary Device Dependency**:
   - If phone is offline, request will timeout
   - If phone doesn't have history, nothing will be returned
   - Phone battery/data usage for large requests

3. **WhatsApp Restrictions**:
   - Rate limiting may apply for excessive requests
   - Maximum batch size: 100 messages
   - History only available for existing chats

4. **Pagination Complexity**:
   - Requires tracking message IDs for pagination
   - Must handle asynchronous responses correctly

## Usage Examples

### Basic Request (Most Recent Messages)

```bash
curl -X POST http://localhost:8081/message/history \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "6289685028129@s.whatsapp.net",
    "count": 50
  }'
```

### Pagination (Fetch Older Messages)

```bash
# Step 1: Get most recent 50 messages
curl -X POST http://localhost:8081/message/history \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "6289685028129@s.whatsapp.net",
    "count": 50
  }'

# Response includes request_id: "uuid-1234"

# Step 2: Wait for webhook with messages
# Find oldest message_id from webhook payload

# Step 3: Fetch next batch
curl -X POST http://localhost:8081/message/history \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "6289685028129@s.whatsapp.net",
    "before_message_id": "OLDEST_MESSAGE_ID_FROM_STEP_1",
    "count": 50
  }'

# Step 4: Repeat until no more messages
```

### Go Client Example

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type HistoryRequest struct {
    DeviceID        string `json:"device_id"`
    ChatID          string `json:"chat_id"`
    BeforeMessageID string `json:"before_message_id,omitempty"`
    Count           int    `json:"count"`
}

type HistoryResponse struct {
    RequestID string `json:"request_id"`
    Messages  []interface{} `json:"messages"`
    Count     int `json:"count"`
}

func fetchHistory(deviceID, chatID string, beforeID string, count int) (*HistoryResponse, error) {
    req := HistoryRequest{
        DeviceID:        deviceID,
        ChatID:          chatID,
        BeforeMessageID: beforeID,
        Count:           count,
    }

    body, _ := json.Marshal(req)
    resp, err := http.Post(
        "http://localhost:8081/message/history",
        "application/json",
        bytes.NewBuffer(body),
    )
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result HistoryResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

func main() {
    // Fetch most recent 50 messages
    result, err := fetchHistory("device_1", "6289685028129@s.whatsapp.net", "", 50)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Request ID: %s\n", result.RequestID)
    fmt.Println("Messages will arrive via webhook...")
}
```

## Events

### Published Events

1. **message.history.requested**

Triggered when a history sync request is sent.

```json
{
  "device_id": "device_1",
  "chat_id": "6289685028129@s.whatsapp.net",
  "before_message_id": "3EB089B9D6ADD58153C561",
  "count": 50,
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

2. **message.history.synced**

Triggered when messages are successfully retrieved (future implementation).

```json
{
  "device_id": "device_1",
  "chat_id": "6289685028129@s.whatsapp.net",
  "messages": [
    {
      "message_id": "ABC123",
      "chat_id": "6289685028129@s.whatsapp.net",
      "sender": "6289685028129@s.whatsapp.net",
      "timestamp": 1699999999,
      "from_me": false,
      "type": "text",
      "text": {
        "content": "Hello world"
      }
    }
  ],
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "count": 1
}
```

3. **message.history.failed**

Triggered when a history sync request fails.

```json
{
  "device_id": "device_1",
  "chat_id": "6289685028129@s.whatsapp.net",
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "error": "failed to send history sync request: connection timeout"
}
```

## Implementation Details

### Code Location

- **Manager Method**: `src/core/whatsapp/manager.go:802` (`FetchMessageHistory`)
- **HTTP Handler**: `src/core/gateway/http/server.go:304` (`handleFetchMessageHistory`)
- **Event Types**: `src/core/whatsapp/events.go:217-241`
- **Request/Response Types**: `src/core/whatsapp/manager.go:510-528`

### Key Implementation Points

1. **Request Validation**:
   ```go
   // Device must exist and be connected
   device, ok := m.GetDevice(req.DeviceID)
   if device.Status != "connected" {
       return nil, fmt.Errorf("device not connected")
   }

   // Count validation
   if req.Count <= 0 {
       req.Count = 50  // Default
   } else if req.Count > 100 {
       req.Count = 100  // Max cap
   }
   ```

2. **MessageInfo Construction**:
   ```go
   messageInfo := &types.MessageInfo{
       MessageSource: types.MessageSource{
           Chat: chatJID,
       },
       ID:        req.BeforeMessageID,
       Timestamp: time.Now(),
   }
   ```

3. **Sending History Sync Request**:
   ```go
   historySyncMsg := device.Client.BuildHistorySyncRequest(messageInfo, req.Count)
   ownJID := device.Client.Store.ID.ToNonAD()

   _, err = device.Client.SendMessage(
       ctx,
       ownJID,
       historySyncMsg,
       whatsmeow.SendRequestExtra{Peer: true},
   )
   ```

### Future Enhancements

The following features are documented but not yet implemented:

1. **HistorySync Event Handler**:
   - Parse `events.HistorySync` events
   - Extract messages from protobuf
   - Convert to `MessagePayload` structs
   - Publish `message.history.synced` event

2. **Synchronous Response**:
   - Track pending requests in a map
   - Use channels to wait for responses
   - Implement timeout (e.g., 30 seconds)
   - Return actual messages in HTTP response

3. **Request Correlation**:
   - Match HistorySync events to requests using `request_id`
   - Handle multiple concurrent requests
   - Proper error handling for timeouts

## Testing

### Manual Testing

1. **Start the gateway**:
   ```bash
   go run main.go
   ```

2. **Create and connect a device**:
   ```bash
   curl -X POST http://localhost:8081/device/new
   # Scan QR code with WhatsApp mobile app
   ```

3. **Send history request**:
   ```bash
   curl -X POST http://localhost:8081/message/history \
     -H "Content-Type: application/json" \
     -d '{
       "device_id": "device_1",
       "chat_id": "6289685028129@s.whatsapp.net",
       "count": 10
     }'
   ```

4. **Check webhook receiver** (if webhook plugin enabled):
   ```bash
   curl http://localhost:3000/webhooks
   ```

### Integration Testing Considerations

- Requires real WhatsApp connection (cannot be fully mocked)
- Primary device must be available for testing
- Consider using a test WhatsApp account
- Implement timeout handling in tests
- Test pagination logic with known message IDs

## Troubleshooting

### Common Issues

1. **"device not found" Error**:
   - Verify device_id exists: `GET /device`
   - Check device was created successfully

2. **"device not connected" Error**:
   - Ensure device status is "connected"
   - Reconnect device if necessary
   - Check WhatsApp session is valid

3. **No Messages Received**:
   - Verify primary phone is online and connected
   - Check phone has the requested chat history
   - Ensure webhook is configured correctly (if using webhooks)
   - Check event logs for `message.history.synced` events

4. **Invalid chat_id Format**:
   - Use correct JID format: `[phone_number]@s.whatsapp.net`
   - For groups: `[group_id]@g.us`
   - Don't include spaces or special characters

5. **Timeout Issues**:
   - Check network connectivity on both gateway and phone
   - Reduce batch size (use smaller `count`)
   - Verify no firewall blocking WhatsApp protocol

### Debug Tips

1. **Enable Debug Logging**:
   ```bash
   export WHATSAPP_LOG_LEVEL=DEBUG
   export WATERMILL_DEBUG=true
   go run main.go
   ```

2. **Monitor Events**:
   - Check logs for `message.history.requested` events
   - Look for `events.HistorySync` in whatsmeow logs
   - Verify webhook delivery attempts

3. **Test with Small Batch**:
   ```bash
   # Start with just 5 messages
   curl -X POST http://localhost:8081/message/history \
     -d '{"device_id":"...","chat_id":"...","count":5}'
   ```

## Best Practices

1. **Pagination Strategy**:
   - Always start without `before_message_id` for most recent messages
   - Use oldest message ID from previous batch for next request
   - Implement exponential backoff between requests
   - Stop pagination when response contains fewer messages than requested

2. **Error Handling**:
   - Always check HTTP status code
   - Handle async response delivery
   - Implement retry logic for failed requests
   - Set reasonable timeouts (30-60 seconds)

3. **Rate Limiting**:
   - Don't request history too frequently
   - Batch size of 50 is recommended
   - Space out requests by at least 1-2 seconds
   - Consider WhatsApp's rate limits

4. **Webhook Integration**:
   - Configure webhook URL before requesting history
   - Implement idempotency in webhook handler
   - Store received messages to avoid duplicates
   - Handle out-of-order delivery

5. **Production Considerations**:
   - Monitor primary device connectivity
   - Implement request queueing for reliability
   - Log all requests with `request_id` for troubleshooting
   - Set up alerts for failed history requests

## References

- Main Documentation: `CLAUDE.md`
- API Specification: `docs/SPEC.md`
- Usage Guide: `docs/USAGE.md`
- Event System: `docs/EVENT.md`
- Webhook Plugin: `docs/PLUGINS_WEBHOOK.md`
- whatsmeow Library: https://pkg.go.dev/go.mau.fi/whatsmeow
