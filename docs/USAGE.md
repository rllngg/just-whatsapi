# WhatsApp Device Manager - Usage Guide

## Starting the Server

```bash
./whatsapp-server
```

The server will start on port `8081` by default.

## API Endpoints

### 1. Create New Device

Create a new WhatsApp device and get a QR code for authentication.

**Request:**
```bash
curl -X POST http://localhost:8081/device/new
```

**Response:**
```json
{
  "qr_code": "MkBwRTF5d3ZWVlJCL01aREtuM3JKbFJ0ekZVRDEvanBIZ25HZ2EvVDRKQ3V5dlpkU09seDAwaDhQQUdYUkt5dEpnZjg2UDBWRkdqNUlLODZWam5yOFMzcm1HdUxDQjNRUTNtYkk9LHAwcmJxUHQ4b2JmT1NYcjNLUTdhWVJhTVZ4ZGduMzYzUEFwdHM0QldsaDQ9LGJjVDVGMzRoYzhwK1dGMkFZS291Nnc5a09FSkV3MVNEZTVjajcrK1pxaDQ9LDVhR0liWUh4RklkaGdPaGkvb0JMN1pSc0pUZGdCbHI0M1EzR1RrVHJublE9"
}
```

The `qr_code` is base64 encoded. To decode it:

```bash
curl -s -X POST http://localhost:8081/device/new | jq -r '.qr_code' | base64 -d
```

### 2. Get All Devices

List all connected devices.

**Request:**
```bash
curl http://localhost:8081/device
```

**Response:**
```json
[
  {
    "id": "device_1",
    "status": "connected"
  }
]
```

## Events

The system publishes events via Watermill when devices change state:

### Event Topics:

1. **`device.created`** - Published when a new device is created
   ```json
   {
     "device_id": "device_1",
     "status": "pending"
   }
   ```

2. **`device.qr_ready`** - Published when QR code is ready to scan
   ```json
   {
     "device_id": "device_1",
     "status": "qr_ready"
   }
   ```

3. **`device.connected`** - Published when device successfully connects
   ```json
   {
     "device_id": "device_1",
     "status": "connected"
   }
   ```

## Testing

Run the automated tests:

```bash
go test -v ./...
```

Run the test client:

```bash
./test_client.sh
```

## Example: Complete Flow

### Using cURL

```bash
# 1. Start the server
./whatsapp-server

# 2. Create a new device
QR_CODE=$(curl -s -X POST http://localhost:8081/device/new | jq -r '.qr_code')

# 3. Display the QR code data
echo "$QR_CODE" | base64 -d

# 4. Scan the QR code with WhatsApp mobile app
# (The QR code needs to be displayed visually for scanning)

# 5. Check connected devices
curl http://localhost:8081/device
```

### Using Go Code

```go
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type DeviceResponse struct {
    QRCode string `json:"qr_code"`
}

func main() {
    // Create new device
    resp, err := http.Post("http://localhost:8081/device/new", "", nil)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    var device DeviceResponse
    json.NewDecoder(resp.Body).Decode(&device)

    fmt.Printf("QR Code: %s\n", device.QRCode)
}
```

## Fetching Message History

### Request Message History

Fetch older messages from a chat. The request is asynchronous - messages arrive via webhooks.

**Request:**
```bash
curl -X POST http://localhost:8081/message/history \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "6289685028129@s.whatsapp.net",
    "count": 50
  }'
```

**Response:**
```json
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "messages": [],
  "count": 0
}
```

### Pagination (Fetch Older Messages)

To fetch older messages, use the `before_message_id` parameter with the oldest message ID from the previous response:

```bash
curl -X POST http://localhost:8081/message/history \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "6289685028129@s.whatsapp.net",
    "before_message_id": "3EB089B9D6ADD58153C561",
    "count": 50
  }'
```

### Important Notes About Message History

1. **Asynchronous Response**: The endpoint returns immediately with a `request_id`. Actual messages arrive later via:
   - `events.HistorySync` events (on the event bus)
   - Webhook delivery (topic: `message.history.synced`)

2. **Requirements**:
   - Your primary WhatsApp device (phone) must be online
   - The device must be in "connected" status
   - The primary device must have the requested message history

3. **Response Time**: Usually 2-10 seconds, depending on:
   - Network connectivity
   - Primary device availability
   - Number of messages requested

4. **Pagination Strategy**:
   ```bash
   # Step 1: Get most recent messages (omit before_message_id)
   curl -X POST .../message/history -d '{"device_id":"...", "chat_id":"...", "count":50}'

   # Step 2: From webhook/event, get oldest message_id from results
   # Step 3: Fetch next batch using that message_id
   curl -X POST .../message/history -d '{"device_id":"...", "chat_id":"...", "before_message_id":"OLDEST_MSG_ID", "count":50}'

   # Step 4: Repeat step 3 until no more messages
   ```

5. **Recommended Batch Size**: 50 messages (max: 100)

## Database

Device sessions are stored in `whatsapp.db` SQLite database. This allows devices to reconnect without re-authentication.

## Architecture

- **Fiber**: High-performance HTTP server
- **Watermill**: Event-driven messaging (GoChannel)
- **whatsmeow**: WhatsApp Web multidevice protocol
- **SQLite**: Session persistence

## Project Structure

```
src/
├── core/
│   ├── event/          # Watermill event bus
│   ├── gateway/http/   # Fiber HTTP server
│   └── whatsapp/       # WhatsApp device manager
```
