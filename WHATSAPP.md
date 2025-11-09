# WhatsApp Web Multi-Device API Client Documentation

## Overview

whatsmeow is a Go library that implements the WhatsApp Web multidevice protocol. It allows you to connect to WhatsApp as a Web client and send/receive messages.

## Installation

```bash
go get go.mau.fi/whatsmeow
```

## Basic Usage

### Creating a Client

```go
import (
    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
)

// Create a container for the device store
container, err := sqlstore.New("sqlite3", "file:whatsapp.db?_foreign_keys=on", nil)
if err != nil {
    panic(err)
}

// Get the first device (or create if none exist)
deviceStore, err := container.GetFirstDevice()
if err != nil {
    panic(err)
}

// Create the client
client := whatsmeow.NewClient(deviceStore, nil)
```

### Connecting to WhatsApp

```go
if client.Store.ID == nil {
    // No ID stored, need to login
    qrChan, err := client.GetQRChannel(context.Background())
    if err != nil {
        panic(err)
    }
    
    err = client.Connect()
    if err != nil {
        panic(err)
    }
    
    for evt := range qrChan {
        if evt.Event == "code" {
            // Display QR code to user
            fmt.Println("QR code:", evt.Code)
        } else {
            fmt.Println("Login event:", evt.Event)
        }
    }
} else {
    // Already logged in, just connect
    err = client.Connect()
    if err != nil {
        panic(err)
    }
}
```

### Sending Messages

```go
// Send a text message
jid, _ := types.ParseJID("1234567890@s.whatsapp.net")
message := &waProto.Message{
    Conversation: proto.String("Hello, World!"),
}
resp, err := client.SendMessage(context.Background(), jid, message)
```

### Receiving Messages

```go
client.AddEventHandler(func(evt interface{}) {
    switch v := evt.(type) {
    case *events.Message:
        fmt.Printf("Received message: %+v\n", v.Message)
    }
})
```

## Key Types and Functions

### Client
- `NewClient(deviceStore, log waLog.Logger) *Client` - Create a new client
- `Connect() error` - Connect to WhatsApp
- `Disconnect()` - Disconnect from WhatsApp
- `SendMessage(ctx, jid, message) (SendResponse, error)` - Send a message
- `AddEventHandler(handler)` - Add an event handler

### Message Types
- `*waProto.Message` - The main message type
- `*events.Message` - Event for received messages
- `types.JID` - Jabber ID for contacts/groups

### Store
- `sqlstore.New(driver, dsn, log) (Container, error)` - Create SQL store
- `container.GetFirstDevice() (*store.Device, error)` - Get first device

## Important Notes

- The library uses protocol buffers for message structures
- SQLite is used for storing session data and messages
- QR code authentication is required for initial login
- The client must be connected to send/receive messages
- Event handlers are used to process incoming events

## Dependencies

- `go.mau.fi/whatsmeow`
- `github.com/mattn/go-sqlite3` (for SQLite storage)
- `google.golang.org/protobuf` (for protocol buffers)