# Test Environment

This directory contains the testing infrastructure for the WhatsApp service.

## Directory Structure

```
test/
├── docker-compose.yml       # Docker Compose configuration
├── webhook/                 # Webhook receiver service
│   ├── main.go             # Go webhook receiver implementation
│   ├── go.mod              # Go module file
│   └── Dockerfile          # Dockerfile for webhook receiver
└── data/                   # Persistent data directory
```

## Components

### 1. Webhook Receiver (Go)
A lightweight Go HTTP server that receives and logs webhooks for testing purposes.

**Endpoints:**
- `POST /webhook` - Receives webhooks
- `GET /webhooks` - Lists all received webhooks
- `DELETE|POST /webhooks/clear` - Clears all received webhooks
- `GET /health` - Health check

**Features:**
- Thread-safe in-memory storage
- Request headers and body logging
- JSON response format
- Lightweight Alpine-based Docker image

### 2. MinIO
S3-compatible object storage for testing media uploads.

**Access:**
- API: http://localhost:9000
- Console: http://localhost:9001
- Credentials: minioadmin / minioadmin
- Bucket: whatsapp-media (auto-created)

### 3. WhatsApp Service
The main WhatsApp integration service built from the parent directory.

## Running the Test Environment

**Prerequisites:**
- Docker and Docker Compose installed

**Start all services:**
```bash
cd test
docker-compose up -d
```

**View logs:**
```bash
docker-compose logs -f
```

**View specific service logs:**
```bash
docker-compose logs -f webhook-receiver
docker-compose logs -f whatsapp
docker-compose logs -f minio
```

**Stop all services:**
```bash
docker-compose down
```

**Stop and remove volumes:**
```bash
docker-compose down -v
```

## Testing

### Test Webhook Receiver

1. Send a test webhook:
```bash
curl http://localhost:3000/webhook \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"event": "test", "message": "Hello from test"}'
```

2. View all received webhooks:
```bash
curl http://localhost:3000/webhooks
```

3. Clear all webhooks:
```bash
curl http://localhost:3000/webhooks/clear -X DELETE
```

### Test MinIO

1. Access MinIO console:
   - Open http://localhost:9001 in your browser
   - Login with: minioadmin / minioadmin

2. Test S3 API:
```bash
# Configure AWS CLI or use MinIO client
mc alias set local http://localhost:9000 minioadmin minioadmin
mc ls local/whatsapp-media
```

### Test WhatsApp Service

The WhatsApp service is available at http://localhost:8080

Example API calls (based on your implementation):
```bash
# Check service health
curl http://localhost:8080/health

# Your WhatsApp API endpoints here
```

## Environment Variables

The docker-compose.yml pre-configures the following environment variables for the WhatsApp service:

- `PORT=8080` - HTTP server port
- `WEBHOOK_URL=http://webhook-receiver:3000/webhook` - Webhook delivery URL
- `WEBHOOK_SECRET=your-secret-key` - Webhook signing secret
- `S3_ENDPOINT=http://minio:9000` - MinIO S3 endpoint
- `S3_ACCESS_KEY=minioadmin` - S3 access key
- `S3_SECRET_KEY=minioadmin` - S3 secret key
- `S3_BUCKET=whatsapp-media` - S3 bucket name
- `S3_REGION=us-east-1` - S3 region
- `S3_USE_SSL=false` - Disable SSL for local MinIO
- `DATABASE_PATH=/data/whatsapp.db` - SQLite database path

## Network

All services run on the same Docker network (`whatsapp-network`) and can communicate using service names:
- `minio` - MinIO service
- `webhook-receiver` - Webhook receiver
- `whatsapp` - WhatsApp service

## Volumes

- `minio_data` - Persistent MinIO storage
- `whatsapp_data` - Persistent WhatsApp database and session data

## Building Individual Services

**Build webhook receiver:**
```bash
cd webhook
docker build -t webhook-receiver .
```

**Build WhatsApp service:**
```bash
cd ..
docker build -t whatsapp-service .
```

## Development

To rebuild services after code changes:

```bash
# Rebuild all services
docker-compose up -d --build

# Rebuild specific service
docker-compose up -d --build whatsapp
docker-compose up -d --build webhook-receiver
```

## Troubleshooting

**Container won't start:**
```bash
docker-compose ps
docker-compose logs [service-name]
```

**Reset everything:**
```bash
docker-compose down -v
docker-compose up -d
```

**Check network connectivity:**
```bash
docker-compose exec whatsapp ping webhook-receiver
docker-compose exec whatsapp ping minio
```
