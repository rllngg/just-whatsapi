# Test Environment

This directory contains the testing infrastructure for the WhatsApp service.

## Components

### 1. Webhook Receiver
A simple Node.js Express server that receives and logs webhooks for testing purposes.

**Endpoints:**
- `POST /webhook` - Receives webhooks
- `GET /webhooks` - Lists all received webhooks
- `DELETE /webhooks` - Clears all received webhooks
- `GET /health` - Health check

### 2. MinIO
S3-compatible object storage for testing media uploads.

**Access:**
- API: http://localhost:9000
- Console: http://localhost:9001
- Credentials: minioadmin / minioadmin

## Running the Test Environment

Start all services:
```bash
docker-compose up -d
```

Stop all services:
```bash
docker-compose down
```

View logs:
```bash
docker-compose logs -f
```

View webhook receiver logs:
```bash
docker-compose logs -f webhook-receiver
```

## Testing

1. Send a test webhook:
```bash
curl http://localhost:3000/webhook -X POST -H "Content-Type: application/json" -d '{"test": "data"}'
```

2. View received webhooks:
```bash
curl http://localhost:3000/webhooks
```

3. Access MinIO console:
Open http://localhost:9001 in your browser and login with minioadmin/minioadmin

## Environment Variables

The WhatsApp service is pre-configured to use:
- `WEBHOOK_URL=http://webhook-receiver:3000/webhook`
- `S3_ENDPOINT=http://minio:9000`
- MinIO credentials and bucket settings
