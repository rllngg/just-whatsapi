# Webhook Plugin Documentation

## Goal
Making Webhook available for this application with AWS S3 Integration for media files.

## Configuration

Webhook should be configured via `.env` with:
- `PLUGINS_WEBHOOK_URL` - Your webhook endpoint URL
- `PLUGINS_WEBHOOK_S3_*` - AWS S3 configuration for uploading media files before sending HTTP requests

## Implementation Status

### ✅ All Events Implemented

| Event | Status | Location | Description |
|-------|--------|----------|-------------|
| Device Created | ✅ | manager.go:141 | When a new device is created |
| Device Connected | ✅ | manager.go:169 | When device successfully connects |
| Device Disconnected | ✅ | manager.go:151 | When device loses connection (internet issue) |
| Device Logout | ✅ | manager.go:162 | When device is logged out |
| Device QR Ready | ✅ | manager.go:154 | When QR code is ready for scanning |
| Message Received | ✅ | manager.go:143 | New message from WhatsApp |
| Message Read | ✅ | manager.go:176 | Read receipt from someone |
| Text Message Sent | ✅ | manager.go:386 | Text message sent via HTTP handler |
| Text Message Failed | ✅ | manager.go:377 | Text message failed to send |
| Image Message Sent | ✅ | manager.go:453 | Image message sent via HTTP handler |
| Image Message Failed | ✅ | manager.go:444 | Image message failed to send |
| File Message Sent | ✅ | manager.go:519 | File message sent via HTTP handler |
| File Message Failed | ✅ | manager.go:510 | File message failed to send |
| Presence Sent | ✅ | manager.go:563 | Presence update sent |
| Presence Failed | ✅ | manager.go:545, 555 | Presence update failed |
| Reaction Sent | ✅ | manager.go:606 | Reaction sent to message |
| Reaction Failed | ✅ | manager.go:597 | Reaction failed to send |

## Features

✅ SQLite database for webhook queue with retry logic  
✅ Retry failed webhooks until success or max 1 day  
✅ Exponential backoff: 1min → 2min → 4min → 8min → 16min → 32min → ... → 24 hours  
✅ S3 upload for image/media messages before webhook delivery  

---

# Webhook Payload Formats

All webhooks follow this base structure:

```json
{
  "event": "event.name",
  "device_id": "device_1",
  "payload": { /* event-specific data */ },
  "timestamp": 1234567890
}
```

## 1. Device Events

### 1.1 Device Created
**Event:** `device.created`  
**Triggered:** When a new WhatsApp device is created and QR code generation starts

```json
{
  "event": "device.created",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "status": "pending"
  },
  "timestamp": 1699123456
}
```

**Payload Fields:**
- `device_id` (string): Unique identifier for the device
- `status` (string): Current status - always "pending" at creation

---

### 1.2 Device QR Ready
**Event:** `device.qr_ready`  
**Triggered:** When QR code is generated and ready to be scanned

```json
{
  "event": "device.qr_ready",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "status": "qr_ready"
  },
  "timestamp": 1699123456
}
```

**Payload Fields:**
- `device_id` (string): Unique identifier for the device
- `status` (string): Current status - "qr_ready"

---

### 1.3 Device Connected
**Event:** `device.connected`  
**Triggered:** When device successfully connects to WhatsApp (after QR scan or auto-reconnect)

```json
{
  "event": "device.connected",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "status": "connected"
  },
  "timestamp": 1699123456
}
```

**Payload Fields:**
- `device_id` (string): Unique identifier for the device
- `status` (string): Current status - "connected"

---

### 1.4 Device Disconnected
**Event:** `device.disconnected`  
**Triggered:** When device loses connection (internet issue, network problem)

```json
{
  "event": "device.disconnected",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "reason": "connection_lost"
  },
  "timestamp": 1699123456
}
```

**Payload Fields:**
- `device_id` (string): Unique identifier for the device
- `reason` (string): Reason for disconnection - "connection_lost"

---

### 1.5 Device Logout
**Event:** `device.logout`  
**Triggered:** When device is logged out (user logged out from phone, device removed)

```json
{
  "event": "device.logout",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "reason": "LOGGED_OUT"
  },
  "timestamp": 1699123456
}
```

**Payload Fields:**
- `device_id` (string): Unique identifier for the device
- `reason` (string): Logout reason from WhatsApp protocol

---

## 2. Message Received Events

### 2.1 Text Message Received
**Event:** `message.received`  
**Triggered:** When a text message is received from WhatsApp  
**Source:** `whatsapp`

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "message": {
      "message_id": "3EB0C127E5F4A2D5E456",
      "chat_id": "6281234567890@s.whatsapp.net",
      "sender": "6281234567890@s.whatsapp.net",
      "timestamp": 1699123456,
      "from_me": false,
      "type": "text",
      "text": {
        "content": "Hello, how are you?"
      }
    },
    "source": "whatsapp"
  },
  "timestamp": 1699123460
}
```

**Message Fields:**
- `message_id` (string): Unique WhatsApp message ID
- `chat_id` (string): Chat identifier (phone@s.whatsapp.net for individual, group_id@g.us for groups)
- `sender` (string): Sender's JID
- `timestamp` (int64): Message timestamp (Unix seconds)
- `from_me` (boolean): Whether the message was sent by this device
- `type` (string): Message type - "text"
- `text.content` (string): The actual text content
- `source` (string): Always "whatsapp" for received messages

---

### 2.2 Image Message Received
**Event:** `message.received`  
**Triggered:** When an image is received from WhatsApp  
**Note:** Image is automatically uploaded to S3 if enabled

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "message": {
      "message_id": "3EB0C127E5F4A2D5E456",
      "chat_id": "6281234567890@s.whatsapp.net",
      "sender": "6281234567890@s.whatsapp.net",
      "timestamp": 1699123456,
      "from_me": false,
      "type": "image",
      "has_media": true,
      "image": {
        "mimetype": "image/jpeg",
        "caption": "Check this out!",
        "url": "https://mmg.whatsapp.net/d/f/At5...",
        "media_url": "https://your-bucket.s3.amazonaws.com/whatsapp-media/1699123456-image.jpg"
      }
    },
    "source": "whatsapp"
  },
  "timestamp": 1699123460
}
```

**Image Fields:**
- `has_media` (boolean): Always true for media messages
- `image.mimetype` (string): MIME type (image/jpeg, image/png, etc.)
- `image.caption` (string): Image caption if provided
- `image.url` (string): WhatsApp's media URL (temporary)
- `image.media_url` (string): S3 URL (only if S3 is enabled and upload successful)

---

### 2.3 Video Message Received
**Event:** `message.received`  
**Triggered:** When a video is received from WhatsApp

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "message": {
      "message_id": "3EB0C127E5F4A2D5E456",
      "chat_id": "6281234567890@s.whatsapp.net",
      "sender": "6281234567890@s.whatsapp.net",
      "timestamp": 1699123456,
      "from_me": false,
      "type": "video",
      "has_media": true,
      "video": {
        "mimetype": "video/mp4",
        "caption": "Amazing video!",
        "url": "https://mmg.whatsapp.net/d/f/At5...",
        "media_url": "https://your-bucket.s3.amazonaws.com/whatsapp-media/1699123456-video.mp4"
      }
    },
    "source": "whatsapp"
  },
  "timestamp": 1699123460
}
```

---

### 2.4 Audio Message Received
**Event:** `message.received`  
**Triggered:** When an audio file or voice note is received

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "message": {
      "message_id": "3EB0C127E5F4A2D5E456",
      "chat_id": "6281234567890@s.whatsapp.net",
      "sender": "6281234567890@s.whatsapp.net",
      "timestamp": 1699123456,
      "from_me": false,
      "type": "audio",
      "has_media": true,
      "audio": {
        "mimetype": "audio/ogg; codecs=opus",
        "url": "https://mmg.whatsapp.net/d/f/At5...",
        "media_url": "https://your-bucket.s3.amazonaws.com/whatsapp-media/1699123456-audio.ogg"
      }
    },
    "source": "whatsapp"
  },
  "timestamp": 1699123460
}
```

---

### 2.5 Document Message Received
**Event:** `message.received`  
**Triggered:** When a document/file is received

```json
{
  "event": "message.received",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "message": {
      "message_id": "3EB0C127E5F4A2D5E456",
      "chat_id": "6281234567890@s.whatsapp.net",
      "sender": "6281234567890@s.whatsapp.net",
      "timestamp": 1699123456,
      "from_me": false,
      "type": "document",
      "has_media": true,
      "document": {
        "mimetype": "application/pdf",
        "caption": "Important document",
        "filename": "report.pdf",
        "url": "https://mmg.whatsapp.net/d/f/At5...",
        "media_url": "https://your-bucket.s3.amazonaws.com/whatsapp-media/1699123456-report.pdf"
      }
    },
    "source": "whatsapp"
  },
  "timestamp": 1699123460
}
```

**Document Fields:**
- `document.filename` (string): Original filename

---

### 2.6 Message Read Receipt
**Event:** `message.read`  
**Triggered:** When someone reads your message(s)

```json
{
  "event": "message.read",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "sender": "6281234567890@s.whatsapp.net",
    "message_ids": [
      "3EB0C127E5F4A2D5E456",
      "3EB0C127E5F4A2D5E457"
    ],
    "timestamp": 1699123456,
    "type": "read"
  },
  "timestamp": 1699123460
}
```

**Payload Fields:**
- `chat_id` (string): Chat where messages were read
- `sender` (string): Who read the messages
- `message_ids` (array): List of message IDs that were read
- `timestamp` (int64): When the read receipt was received
- `type` (string): Receipt type

---

## 3. Message Sent Events (via HTTP API)

### 3.1 Text Message Sent Successfully
**Event:** `message.text.sent`  
**Triggered:** When text message is successfully sent via HTTP handler  
**Source:** `http`

```json
{
  "event": "message.text.sent",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "message_id": "3EB0C127E5F4A2D5E456",
    "source": "http"
  },
  "timestamp": 1699123460
}
```

**Payload Fields:**
- `device_id` (string): Device that sent the message
- `chat_id` (string): Destination chat
- `message_id` (string): WhatsApp message ID assigned after sending
- `source` (string): Always "http" for API-sent messages

---

### 3.2 Text Message Failed
**Event:** `message.text.failed`  
**Triggered:** When text message fails to send via HTTP handler

```json
{
  "event": "message.text.failed",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "error": "failed to send message: connection timeout"
  },
  "timestamp": 1699123460
}
```

**Payload Fields:**
- `error` (string): Error message describing what went wrong

---

### 3.3 Image Message Sent Successfully
**Event:** `message.image.sent`  
**Triggered:** When image message is successfully sent via HTTP handler

```json
{
  "event": "message.image.sent",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "message_id": "3EB0C127E5F4A2D5E456",
    "source": "http"
  },
  "timestamp": 1699123460
}
```

---

### 3.4 Image Message Failed
**Event:** `message.image.failed`  
**Triggered:** When image message fails to send

```json
{
  "event": "message.image.failed",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "error": "failed to send image: file too large"
  },
  "timestamp": 1699123460
}
```

---

### 3.5 File/Document Message Sent Successfully
**Event:** `message.file.sent`  
**Triggered:** When file/document message is successfully sent

```json
{
  "event": "message.file.sent",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "message_id": "3EB0C127E5F4A2D5E456",
    "source": "http"
  },
  "timestamp": 1699123460
}
```

---

### 3.6 File/Document Message Failed
**Event:** `message.file.failed`  
**Triggered:** When file/document message fails to send

```json
{
  "event": "message.file.failed",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "error": "failed to send file: invalid file format"
  },
  "timestamp": 1699123460
}
```

---

### 3.7 Presence Update Sent
**Event:** `message.presence.sent`  
**Triggered:** When typing indicator is successfully sent

```json
{
  "event": "message.presence.sent",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net"
  },
  "timestamp": 1699123460
}
```

---

### 3.8 Presence Update Failed
**Event:** `message.presence.failed`  
**Triggered:** When typing indicator fails to send

```json
{
  "event": "message.presence.failed",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "error": "failed to send presence: not connected"
  },
  "timestamp": 1699123460
}
```

---

### 3.9 Reaction Sent Successfully
**Event:** `message.reaction.sent`  
**Triggered:** When reaction to a message is successfully sent

```json
{
  "event": "message.reaction.sent",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "target_message": "3EB0C127E5F4A2D5E456",
    "reaction_message": "3EB0C127E5F4A2D5E999"
  },
  "timestamp": 1699123460
}
```

**Payload Fields:**
- `target_message` (string): ID of the message being reacted to
- `reaction_message` (string): ID of the reaction message

---

### 3.10 Reaction Failed
**Event:** `message.reaction.failed`  
**Triggered:** When reaction fails to send

```json
{
  "event": "message.reaction.failed",
  "device_id": "device_1",
  "payload": {
    "device_id": "device_1",
    "chat_id": "6281234567890@s.whatsapp.net",
    "message_id": "3EB0C127E5F4A2D5E456",
    "error": "failed to send reaction: message not found"
  },
  "timestamp": 1699123460
}
```

---

## Notes

### S3 Media Upload
- When S3 is enabled (`PLUGINS_WEBHOOK_S3_ENABLED=true`), all media files (images, videos, audio, documents) are automatically uploaded to S3
- The S3 URL is added to the payload as `media_url` field
- Original WhatsApp URL remains in the `url` field
- Upload happens **before** the webhook is sent
- If S3 upload fails, the webhook is still sent without the `media_url` field

### Retry Logic
- Failed webhooks are retried with exponential backoff: **1min → 2min → 4min → 8min → 16min → 32min → ... → 24 hours**
- Maximum retry period: **24 hours** from initial attempt
- Webhooks older than 24 hours are automatically discarded
- Successfully delivered webhooks are kept for **7 days** then cleaned up
- Retry status is persisted in SQLite database (`webhook_queue.db`)

### Webhook Security
- Always use **HTTPS** for production webhook URLs
- Implement signature verification on your webhook endpoint
- Validate the `device_id` matches your expected devices
- Check `timestamp` to prevent replay attacks

### Message Types
Supported message types in `message.received` event:
- `text` - Plain text messages
- `image` - Image files (JPEG, PNG, etc.)
- `video` - Video files (MP4, etc.)
- `audio` - Audio files and voice notes
- `document` - Documents and files
- `unknown` - Unsupported message types

### JID Format
- Individual chats: `phone_number@s.whatsapp.net` (e.g., `6281234567890@s.whatsapp.net`)
- Group chats: `group_id@g.us`
- Broadcast lists: `broadcast_id@broadcast`
