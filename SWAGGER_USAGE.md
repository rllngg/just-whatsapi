# Swagger Documentation Usage

## Overview
Swagger/OpenAPI documentation has been added to the WhatsApp Gateway HTTP server.

## Accessing Swagger UI

Once your server is running, you can access the Swagger UI at:

```
http://localhost:8080/swagger/index.html
```

(Replace `localhost:8080` with your actual server address if different)

## Available Endpoints

### Device Management
- **POST /device/new** - Create a new WhatsApp device and get QR code
- **GET /device** - List all registered devices

### Messaging
- **POST /message/text** - Send a text message
- **POST /message/image** - Send an image message
- **POST /message/file** - Send a file/document message
- **POST /message/presence** - Send typing indicator
- **POST /message/emoji** - Send emoji reaction to a message

## Regenerating Documentation

If you make changes to the API endpoints or annotations, regenerate the Swagger docs:

```bash
swag init -g src/core/gateway/http/server.go -o docs
```

Or use the full path:

```bash
~/go/bin/swag init -g src/core/gateway/http/server.go -o docs
```

## Swagger Files

The following files are auto-generated in the `docs/` directory:
- `docs.go` - Go code for Swagger spec
- `swagger.json` - JSON specification
- `swagger.yaml` - YAML specification

## Example Request Bodies

### Create Device
```bash
curl -X POST http://localhost:8080/device/new
```

### Send Text Message
```bash
curl -X POST http://localhost:8080/message/text \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "1234567890@s.whatsapp.net",
    "message": "Hello World"
  }'
```

### Send Image Message
```bash
curl -X POST http://localhost:8080/message/image \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "device_1",
    "chat_id": "1234567890@s.whatsapp.net",
    "file_url": "https://example.com/image.jpg",
    "caption": "Check this out!"
  }'
```

## Customization

You can customize the Swagger documentation by editing the annotations in `src/core/gateway/http/server.go`. Key annotations include:

- `@title` - API title
- `@version` - API version
- `@description` - API description
- `@host` - Default host
- `@BasePath` - Base path for all endpoints
- `@Summary` - Endpoint summary
- `@Description` - Detailed endpoint description
- `@Tags` - Group endpoints by tags
- `@Param` - Define parameters
- `@Success` / `@Failure` - Define responses
