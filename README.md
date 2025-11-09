# WhatsApp CLI Tool

A Go-based CLI tool for interacting with WhatsApp using the whatsmeow library.

## Features

- Connect to WhatsApp using QR code authentication
- Store messages in SQLite database
- Basic CRUD operations for messages
- Event-driven message handling

## Installation

```bash
go mod download
go build -o whatsapp
```

## Usage

### Running the Application

```bash
./whatsapp
```

On first run, the application will:
1. Display a QR code in the terminal
2. Scan the QR code with your WhatsApp mobile app
3. Store the session for future use

### Database Schema

The application creates three tables:

- **contacts**: Stores contact information
- **messages**: Stores received and sent messages
- **message_status**: Tracks message delivery status

### API Usage

The main application structure provides these methods:

- `GetMessages(limit int)`: Retrieve recent messages
- `SendMessage(toJID, content string)`: Send a text message

## Dependencies

- `go.mau.fi/whatsmeow` - WhatsApp Web multidevice API
- `github.com/mattn/go-sqlite3` - SQLite driver
- `google.golang.org/protobuf` - Protocol buffers

## Project Structure

```
whatsapp/
├── go.mod          # Go module definition
├── go.sum          # Dependency checksums
├── main.go         # Main application code
├── WHATSAPP.md     # WhatsMeow library documentation
└── whatsapp.db     # SQLite database (created on first run)
```

## Notes

- The application stores session data in the SQLite database
- Messages are automatically saved when received
- Use Ctrl+C to gracefully disconnect and shutdown