# WhatsApp Gateway API

A production-ready WhatsApp webhook integration service built with Go, providing RESTful API endpoints for sending and receiving WhatsApp messages programmatically.

## Features

- 🔐 QR Code Authentication
- 📨 Send & Receive Messages
- 📎 Media Support (images, documents, audio, video)
- 🔔 Webhook Integration
- 💾 SQLite Database Storage
- 🐳 Docker Support with MinIO
- 📝 Comprehensive API Documentation (Swagger/OpenAPI)
- 🔌 Plugin System for Custom Integrations

## Quick Start

```bash
# Clone the repository
git clone <repository-url>
cd whatsapp

# Run with Docker
docker-compose up -d

# Or run locally
go mod download
go run main.go
```

## Documentation

Comprehensive documentation is available in the [`docs/`](./docs) directory:

- **[Usage Guide](./docs/USAGE.md)** - API endpoints and usage examples
- **[Swagger Documentation](./docs/SWAGGER_USAGE.md)** - OpenAPI/Swagger interface
- **[API Specification](./docs/SPEC.md)** - Detailed API reference
- **[Implementation Summary](./docs/IMPLEMENTATION_SUMMARY.md)** - Technical implementation details
- **[Event Handling](./docs/EVENT.md)** - Event system documentation
- **[Webhook Plugins](./docs/PLUGINS_WEBHOOK.md)** - Plugin development guide
- **[Web Plugins](./docs/PLUGINS_WEB.md)** - Web integration plugins

## API Endpoints

The service provides RESTful endpoints for:

- `/connect` - Initialize WhatsApp connection
- `/send` - Send messages and media
- `/webhook` - Configure webhook endpoints
- `/history` - Message history retrieval
- `/contacts` - Contact management

See the [API Documentation](./docs/USAGE.md) for complete endpoint details.

## Architecture

- **Backend**: Go with whatsmeow library
- **Database**: SQLite
- **Storage**: MinIO (S3-compatible)
- **API Docs**: Swagger/OpenAPI
- **Testing**: Go webhook receiver included

## Contributing

Contributions are welcome! Please see the documentation in the `docs/` directory for implementation details.

## License

[Add your license here]

## Support

For issues and questions, please open an issue on the GitHub repository.
