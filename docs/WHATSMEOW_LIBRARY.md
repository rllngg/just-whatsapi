# whatsmeow - WhatsApp Web Multi-Device API Client

## Overview

**whatsmeow** is a Go library that implements the WhatsApp Web multidevice protocol. It allows you to connect to WhatsApp as a Web client and send/receive messages programmatically.

**Repository**: [github.com/tulir/whatsmeow](https://github.com/tulir/whatsmeow)
**Documentation**: [pkg.go.dev/go.mau.fi/whatsmeow](https://pkg.go.dev/go.mau.fi/whatsmeow)
**Matrix Support Room**: #whatsmeow:maunium.net
**License**: MPL-2.0

## Supported Features

The library provides comprehensive WhatsApp functionality:

✅ **Messaging**:
- Direct messages (text and media)
- Group messages (text and media)
- Message reactions
- Message replies
- Message deletion
- Message editing
- Polls and poll voting

✅ **Group Management**:
- Create/modify groups
- Add/remove participants
- Group invite links
- Group settings management

✅ **Status & Presence**:
- Typing notifications
- Read receipts
- Delivery receipts
- Presence updates (online/offline)
- Status message transmission

✅ **Media Handling**:
- Image messages
- Document messages
- Video messages
- Audio messages
- Sticker messages
- Media uploads and downloads

✅ **Advanced Features**:
- App state synchronization (contacts, chat settings)
- Retry receipt handling for failed decryption
- Newsletter creation and management
- Device management
- Message history sync
- Contact syncing

❌ **Not Supported**:
- Broadcast list messages
- Voice/video calls

## Installation

```bash
go get go.mau.fi/whatsmeow
```

### Required Dependencies

You must import a SQL database driver for session storage:

```go
import (
    _ "github.com/mattn/go-sqlite3"  // For SQLite
    // OR
    _ "github.com/lib/pq"            // For PostgreSQL
)
```

Additional dependencies:
- `google.golang.org/protobuf` - Protocol buffers for message structures
- `go.mau.fi/libsignal` - Signal protocol encryption
- Noise Protocol implementation for end-to-end encryption

## Basic Usage

### 1. Import Required Packages

```go
import (
    "context"
    "fmt"

    "go.mau.fi/whatsmeow"
    "go.mau.fi/whatsmeow/store/sqlstore"
    "go.mau.fi/whatsmeow/types"
    "go.mau.fi/whatsmeow/types/events"
    waLog "go.mau.fi/whatsmeow/util/log"
    waProto "go.mau.fi/whatsmeow/binary/proto"

    "google.golang.org/protobuf/proto"
)
```

### 2. Create Database Container and Device Store

```go
// Create logger
dbLog := waLog.Stdout("Database", "INFO", true)

// Create SQL store container
container, err := sqlstore.New(context.Background(), "sqlite3",
    "file:whatsapp.db?_foreign_keys=on", dbLog)
if err != nil {
    panic(err)
}

// Get the first device (or create if none exist)
deviceStore, err := container.GetFirstDevice(context.Background())
if err != nil {
    panic(err)
}
```

### 3. Create WhatsApp Client

```go
// Create logger for client
clientLog := waLog.Stdout("Client", "INFO", true)

// Create the client
client := whatsmeow.NewClient(deviceStore, clientLog)
```

### 4. Add Event Handler

```go
client.AddEventHandler(func(evt interface{}) {
    switch v := evt.(type) {
    case *events.Message:
        fmt.Printf("Received message from %s: %s\n",
            v.Info.Sender, v.Message.GetConversation())

    case *events.Receipt:
        fmt.Printf("Message %s was delivered\n", v.MessageIDs[0])

    case *events.Presence:
        fmt.Printf("%s is %s\n", v.From, v.Unavailable)
    }
})
```

### 5. Connect to WhatsApp

#### First Time Login (QR Code Required)

```go
if client.Store.ID == nil {
    // No ID stored, need to login via QR code
    qrChan, err := client.GetQRChannel(context.Background())
    if err != nil {
        panic(err)
    }

    err = client.Connect()
    if err != nil {
        panic(err)
    }

    // Wait for QR code events
    for evt := range qrChan {
        if evt.Event == "code" {
            // Display QR code to user
            fmt.Println("QR code:", evt.Code)
            // You can render this as QR image using a QR library
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

#### Subsequent Connections

```go
// Session persists in device store, just connect
err := client.Connect()
if err != nil {
    panic(err)
}
```

## Sending Messages

### Text Message

```go
recipient, _ := types.ParseJID("1234567890@s.whatsapp.net")

message := &waProto.Message{
    Conversation: proto.String("Hello, World!"),
}

resp, err := client.SendMessage(context.Background(), recipient, message)
if err != nil {
    fmt.Println("Error sending:", err)
} else {
    fmt.Println("Message sent, ID:", resp.ID)
}
```

### Reply to Message

```go
message := &waProto.Message{
    ExtendedTextMessage: &waProto.ExtendedTextMessage{
        Text: proto.String("This is a reply"),
        ContextInfo: &waProto.ContextInfo{
            StanzaID:      proto.String(originalMessageID),
            Participant:   proto.String(originalSender),
            QuotedMessage: originalMessage,
        },
    },
}

client.SendMessage(ctx, recipient, message)
```

### Image Message

```go
// First, upload the image
imageData, _ := os.ReadFile("photo.jpg")
uploaded, err := client.Upload(context.Background(), imageData, whatsmeow.MediaImage)
if err != nil {
    panic(err)
}

message := &waProto.Message{
    ImageMessage: &waProto.ImageMessage{
        URL:           proto.String(uploaded.URL),
        DirectPath:    proto.String(uploaded.DirectPath),
        MediaKey:      uploaded.MediaKey,
        Mimetype:      proto.String("image/jpeg"),
        FileEncSHA256: uploaded.FileEncSHA256,
        FileSHA256:    uploaded.FileSHA256,
        FileLength:    proto.Uint64(uint64(len(imageData))),
        Caption:       proto.String("Check out this image!"),
    },
}

client.SendMessage(ctx, recipient, message)
```

### Document Message

```go
docData, _ := os.ReadFile("document.pdf")
uploaded, err := client.Upload(context.Background(), docData, whatsmeow.MediaDocument)

message := &waProto.Message{
    DocumentMessage: &waProto.DocumentMessage{
        URL:           proto.String(uploaded.URL),
        DirectPath:    proto.String(uploaded.DirectPath),
        MediaKey:      uploaded.MediaKey,
        Mimetype:      proto.String("application/pdf"),
        FileEncSHA256: uploaded.FileEncSHA256,
        FileSHA256:    uploaded.FileSHA256,
        FileLength:    proto.Uint64(uint64(len(docData))),
        FileName:      proto.String("document.pdf"),
    },
}

client.SendMessage(ctx, recipient, message)
```

### Reaction to Message

```go
reaction := &waProto.Message{
    ReactionMessage: &waProto.ReactionMessage{
        Key: &waProto.MessageKey{
            RemoteJID: proto.String(chatJID.String()),
            FromMe:    proto.Bool(false),
            ID:        proto.String(messageID),
        },
        Text:              proto.String("👍"),
        SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
    },
}

client.SendMessage(ctx, chatJID, reaction)
```

## Receiving Messages

### Event Types

The library uses event handlers to process incoming data. Key event types:

#### **events.Message**
Received WhatsApp message with content and metadata.

```go
case *events.Message:
    fmt.Println("From:", v.Info.Sender)
    fmt.Println("Chat:", v.Info.Chat)
    fmt.Println("Message ID:", v.Info.ID)
    fmt.Println("Timestamp:", v.Info.Timestamp)

    // Extract message content
    if v.Message.GetConversation() != "" {
        fmt.Println("Text:", v.Message.GetConversation())
    }

    if v.Message.GetImageMessage() != nil {
        fmt.Println("Image URL:", v.Message.GetImageMessage().GetURL())
    }
```

#### **events.Receipt**
Delivery/read receipt for sent messages.

```go
case *events.Receipt:
    fmt.Println("Receipt type:", v.Type)  // "delivered", "read", "sender"
    fmt.Println("Message IDs:", v.MessageIDs)
```

#### **events.HistorySync**
Historical messages from message history sync.

```go
case *events.HistorySync:
    fmt.Println("History sync type:", v.Data.SyncType)
    fmt.Println("Conversations:", len(v.Data.Conversations))
```

#### **events.Connected**
Client successfully connected to WhatsApp.

```go
case *events.Connected:
    fmt.Println("Connected to WhatsApp")
```

#### **events.Disconnected**
Client disconnected from WhatsApp.

```go
case *events.Disconnected:
    fmt.Println("Disconnected from WhatsApp")
```

### Multiple Event Handlers

```go
// Add first handler
handlerID1 := client.AddEventHandler(func(evt interface{}) {
    // Handle messages
})

// Add second handler
handlerID2 := client.AddEventHandler(func(evt interface{}) {
    // Log all events
})

// Remove handler when no longer needed
client.RemoveEventHandler(handlerID1)
```

## Advanced Features

### Message Decryption

The client automatically decrypts most messages, but some require manual decryption:

```go
// Decrypt reaction
case *events.Message:
    if v.Message.GetReactionMessage() != nil {
        decrypted, err := client.DecryptReaction(ctx, v)
        if err == nil {
            fmt.Println("Reaction:", decrypted.GetText())
        }
    }

    // Decrypt poll vote
    if v.Message.GetPollUpdateMessage() != nil {
        votes, err := client.DecryptPollVote(ctx, v)
        if err == nil {
            fmt.Println("Poll votes:", votes)
        }
    }
```

### Message History Sync

Request historical messages from primary device:

```go
messageInfo := &types.MessageInfo{
    MessageSource: types.MessageSource{
        Chat: chatJID,
    },
    ID:        "MESSAGE_ID_TO_START_FROM",  // Or empty for latest
    Timestamp: time.Now(),
}

historySyncMsg := client.BuildHistorySyncRequest(messageInfo, 50)
ownJID := client.Store.ID.ToNonAD()

_, err = client.SendMessage(
    ctx,
    ownJID,
    historySyncMsg,
    whatsmeow.SendRequestExtra{Peer: true},
)
```

### Upload Media

```go
// Upload image
imageData := []byte{...}
uploaded, err := client.Upload(ctx, imageData, whatsmeow.MediaImage)

// Upload document
docData := []byte{...}
uploaded, err := client.Upload(ctx, docData, whatsmeow.MediaDocument)

// Upload audio
audioData := []byte{...}
uploaded, err := client.Upload(ctx, audioData, whatsmeow.MediaAudio)

// Upload video
videoData := []byte{...}
uploaded, err := client.Upload(ctx, videoData, whatsmeow.MediaVideo)
```

### Download Media

```go
case *events.Message:
    if img := v.Message.GetImageMessage(); img != nil {
        data, err := client.Download(img)
        if err != nil {
            fmt.Println("Download failed:", err)
        } else {
            os.WriteFile("downloaded.jpg", data, 0644)
        }
    }
```

### Typing Notification

```go
// Start typing
client.SendChatPresence(chatJID, types.ChatPresenceComposing, types.ChatPresenceMediaText)

// Stop typing
client.SendChatPresence(chatJID, types.ChatPresencePaused, types.ChatPresenceMediaText)
```

### Presence Updates

```go
// Mark as online
client.SendPresence(types.PresenceAvailable)

// Mark as offline
client.SendPresence(types.PresenceUnavailable)
```

### Group Management

```go
// Create group
resp, err := client.CreateGroup(types.GroupParticipant{
    JID:         memberJID,
    IsAdmin:     false,
    IsSuperAdmin: false,
})

// Add participants
resp, err := client.UpdateGroupParticipants(groupJID, []types.JID{memberJID},
    whatsmeow.ParticipantChangeAdd)

// Remove participants
resp, err := client.UpdateGroupParticipants(groupJID, []types.JID{memberJID},
    whatsmeow.ParticipantChangeRemove)

// Promote to admin
resp, err := client.UpdateGroupParticipants(groupJID, []types.JID{memberJID},
    whatsmeow.ParticipantChangePromote)

// Get group info
info, err := client.GetGroupInfo(groupJID)
```

## Key Types and Structures

### Client

Main struct for WhatsApp operations:

- `NewClient(deviceStore, log waLog.Logger) *Client` - Create a new client
- `Connect() error` - Connect to WhatsApp
- `Disconnect()` - Disconnect from WhatsApp
- `SendMessage(ctx, jid, message) (SendResponse, error)` - Send a message
- `AddEventHandler(handler) string` - Add an event handler, returns handler ID
- `RemoveEventHandler(handlerID string)` - Remove event handler
- `Upload(ctx, data, mediaType) (UploadResponse, error)` - Upload media
- `Download(msg) ([]byte, error)` - Download media from message
- `GetGroupInfo(jid) (*GroupInfo, error)` - Get group information
- `UpdateGroupParticipants(jid, participants, action)` - Modify group participants

### MessageInfo

Metadata container for messages:

```go
type MessageInfo struct {
    MessageSource  // Chat, Sender, IsFromMe, IsGroup
    ID        string
    Type      string
    PushName  string
    Timestamp time.Time
    Category  string
    Multicast bool
    MediaCiphertextSHA256 []byte
}
```

### types.JID

Jabber ID for contacts/groups:

- Format: `1234567890@s.whatsapp.net` (individual)
- Format: `120363012345678901@g.us` (group)
- Parse with `types.ParseJID(string)`

### waProto.Message

Protocol buffer message structure:

```go
type Message struct {
    Conversation           *string              // Simple text
    ExtendedTextMessage    *ExtendedTextMessage // Text with formatting/reply
    ImageMessage           *ImageMessage
    DocumentMessage        *DocumentMessage
    AudioMessage           *AudioMessage
    VideoMessage           *VideoMessage
    StickerMessage         *StickerMessage
    ReactionMessage        *ReactionMessage
    PollCreationMessage    *PollCreationMessage
    PollUpdateMessage      *PollUpdateMessage
    // ... many more
}
```

### Store Types

- `sqlstore.New(ctx, driver, dsn, log) (Container, error)` - Create SQL store
- `container.GetFirstDevice(ctx) (*store.Device, error)` - Get first device
- `container.GetAllDevices(ctx) ([]*store.Device, error)` - Get all devices
- `container.NewDevice(ctx) (*store.Device, error)` - Create new device

## Important Implementation Notes

### Session Persistence

- Sessions persist in the SQL device store
- No plaintext credentials stored
- Encryption keys stored securely
- Automatic reconnection on app restart
- Each device has unique session data

### Security & Encryption

- Uses Noise Protocol for encryption
- Signal Protocol for end-to-end encryption
- Automatic key rotation
- Forward secrecy guaranteed
- Media encrypted with per-message keys

### Media Handling

- All media must be uploaded before sending
- Media downloads require valid `mediaKey` and checksums
- Media encryption is automatic
- Supported formats: JPEG, PNG, PDF, MP3, MP4, OGG, WEBP, etc.

### App State Synchronization

- Contacts, chat settings, and pin states sync automatically
- App state patches applied incrementally
- Full sync on first connection
- Critical for maintaining consistent state

### Error Handling

```go
// Always check connection status
if !client.IsConnected() {
    fmt.Println("Client is not connected")
}

// Handle send errors
resp, err := client.SendMessage(ctx, jid, msg)
if err != nil {
    // Check error type
    if errors.Is(err, whatsmeow.ErrNotConnected) {
        fmt.Println("Not connected to WhatsApp")
    }
}
```

### Rate Limiting

WhatsApp enforces rate limits:
- Message sending: ~100-200 messages per day for new numbers
- Group operations: Limited frequency
- Media uploads: Limited size and frequency
- Excessive use may result in temporary bans

## Production Considerations

### Graceful Shutdown

```go
// Always disconnect cleanly
defer client.Disconnect()

// Handle OS signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
<-sigChan

client.Disconnect()
```

### Logging Levels

```go
// Production: Use INFO or WARN
log := waLog.Stdout("Client", "INFO", true)

// Development: Use DEBUG
log := waLog.Stdout("Client", "DEBUG", true)

// Disable colors for log files
log := waLog.Stdout("Client", "INFO", false)
```

### Database Drivers

```go
// SQLite (suitable for single-instance)
import _ "github.com/mattn/go-sqlite3"
container, _ := sqlstore.New(ctx, "sqlite3", "file:whatsapp.db?_foreign_keys=on", log)

// PostgreSQL (suitable for multi-instance)
import _ "github.com/lib/pq"
container, _ := sqlstore.New(ctx, "postgres",
    "host=localhost user=wa password=pass dbname=whatsapp", log)
```

### Multi-Device Support

One container can manage multiple devices:

```go
// Get all devices
devices, err := container.GetAllDevices(ctx)

// Create new device
newDevice, err := container.NewDevice(ctx)

// Each device = separate WhatsApp connection
for _, dev := range devices {
    client := whatsmeow.NewClient(dev, log)
    client.Connect()
}
```

## Common Issues and Solutions

### "Store doesn't contain device JID"

**Problem**: Trying to connect before QR code login.

**Solution**: Always check `client.Store.ID == nil` before connecting.

### "Message decryption failed"

**Problem**: Missing encryption keys or corrupted session.

**Solution**:
- Ensure proper session restoration
- Check app state sync is working
- May require re-login (logout and scan QR again)

### "Not connected to WhatsApp"

**Problem**: Calling methods while disconnected.

**Solution**: Check `client.IsConnected()` before operations.

### Media Download Fails

**Problem**: Invalid media keys or expired URLs.

**Solution**:
- Download immediately upon receipt
- Store media keys with messages
- WhatsApp media URLs expire after ~30 days

## References and Resources

- **Official Documentation**: [pkg.go.dev/go.mau.fi/whatsmeow](https://pkg.go.dev/go.mau.fi/whatsmeow)
- **Source Code**: [github.com/tulir/whatsmeow](https://github.com/tulir/whatsmeow)
- **Example Code**: [github.com/tulir/whatsmeow/tree/main/mdtest](https://github.com/tulir/whatsmeow/tree/main/mdtest)
- **Protocol Buffers**: [github.com/tulir/whatsmeow/tree/main/binary/proto](https://github.com/tulir/whatsmeow/tree/main/binary/proto)
- **Support**: Matrix room #whatsmeow:maunium.net

## License

whatsmeow is licensed under the Mozilla Public License 2.0 (MPL-2.0).

---

**Last Updated**: 2025-11-13
**Library Version**: Latest as of documentation date
**Maintained By**: Tulir Asokan
