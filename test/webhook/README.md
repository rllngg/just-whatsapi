# Webhook Receiver

A lightweight HTTP server written in Go for receiving and inspecting webhooks during testing.

## Features

- Thread-safe in-memory webhook storage
- JSON request/response handling
- Request headers and body logging
- RESTful API for webhook management
- Health check endpoint
- Minimal Docker image (~10MB)

## API Endpoints

### POST /webhook
Receives and stores a webhook.

**Request:**
```bash
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{"event": "message.received", "data": {"from": "1234567890"}}'
```

**Response:**
```json
{
  "success": true,
  "message": "Webhook received",
  "timestamp": "2025-11-09T10:30:00Z"
}
```

### GET /webhooks
Retrieves all received webhooks.

**Request:**
```bash
curl http://localhost:3000/webhooks
```

**Response:**
```json
{
  "count": 2,
  "webhooks": [
    {
      "timestamp": "2025-11-09T10:30:00Z",
      "headers": {
        "Content-Type": ["application/json"]
      },
      "body": {
        "event": "message.received"
      }
    }
  ]
}
```

### DELETE|POST /webhooks/clear
Clears all stored webhooks.

**Request:**
```bash
curl -X DELETE http://localhost:3000/webhooks/clear
```

**Response:**
```json
{
  "success": true,
  "message": "Cleared 5 webhooks"
}
```

### GET /health
Health check endpoint.

**Request:**
```bash
curl http://localhost:3000/health
```

**Response:**
```json
{
  "status": "ok"
}
```

## Running Locally

**With Go:**
```bash
cd test/webhook
go run main.go
```

**With Docker:**
```bash
docker build -t webhook-receiver .
docker run -p 3000:3000 webhook-receiver
```

**With custom port:**
```bash
PORT=8000 go run main.go
# or
docker run -p 8000:3000 -e PORT=3000 webhook-receiver
```

## Configuration

Environment variables:
- `PORT` - HTTP server port (default: 3000)

## Development

**Run locally:**
```bash
go run main.go
```

**Test the server:**
```bash
# Send test webhook
curl -X POST http://localhost:3000/webhook \
  -H "Content-Type: application/json" \
  -d '{"test": "data"}'

# View webhooks
curl http://localhost:3000/webhooks

# Clear webhooks
curl -X DELETE http://localhost:3000/webhooks/clear
```

## Docker Build

The Dockerfile uses multi-stage builds:
1. **Builder stage**: Compiles the Go binary
2. **Runtime stage**: Creates minimal Alpine image with just the binary

**Build:**
```bash
docker build -t webhook-receiver .
```

**Image size:** ~10MB (Alpine + static Go binary)
