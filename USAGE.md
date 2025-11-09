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
