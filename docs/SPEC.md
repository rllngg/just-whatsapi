Please follow TDD (Integrated Test Only)




Goal Implement These endpoint

- Each time please publish event follow @event.go
-


# Send Message (textOnly)

POST /message/text
Body
{
  "device_id": "6289685028129@s.whatsapp.net",
  "chat_id": "6289685028129@s.whatsapp.net",
  "message": "selamat malam",
  "reply_message_id": "3EB089B9D6ADD58153C561",
  "is_forwarded": false,
  "duration": 3600 // Disappearing message duration in seconds (optional)
}
Result
{
  "message_id": "MESSAGE_ID"
}

# Send Message Image

POST /message/image
Body
{
  "device_id": "6289685028129@s.whatsapp.net",
  "chat_id": "6289685028129@s.whatsapp.net",
  "file_url": "https://test.com/image.png",
  "caption": "selamat malam",
  "reply_message_id": "3EB089B9D6ADD58153C561",
  "is_forwarded": false,
  "view_once": false,
  "duration": 3600 // Disappearing message duration in seconds (optional)
}
Result
{
  "message_id": "MESSAGE_ID"
}
# Send Message File
POST /message/file
Body
{
  "device_id": "6289685028129@s.whatsapp.net",
  "chat_id": "6289685028129@s.whatsapp.net",
  "file_url": "https://test.com/image.png",
  "caption": "selamat malam",
  "reply_message_id": "3EB089B9D6ADD58153C561",
  "is_forwarded": false,
  "duration": 3600 // Disappearing message duration in seconds (optional)
}
Result
{
  "message_id": "MESSAGE_ID"
}


# Send Message Presence
POST /message/presence
Body
{
  "device_id": "6289685028129@s.whatsapp.net",
  "chat_id": "6289685028129@s.whatsapp.net",
  "is_forwarded": false
}
Result
{
  "message_id": "MESSAGE_ID"
}


# Send Message Emoji
POST /message/emoji
Body
{
  "device_id": "6289685028129@s.whatsapp.net",
  "chat_id": "6289685028129@s.whatsapp.net",
  "message_id": "1231231232131",
  "is_forwarded": false
}
Result
{
  "message_id": "MESSAGE_ID"
}

# Fetch Message History

POST /message/history
Body
{
  "device_id": "device_1",
  "chat_id": "6289685028129@s.whatsapp.net",
  "before_message_id": "3EB089B9D6ADD58153C561", // Optional - for pagination, fetches messages before this ID
  "count": 50 // Optional - number of messages to fetch (default: 50, max: 100)
}
Result
{
  "request_id": "550e8400-e29b-41d4-a716-446655440000",
  "messages": [], // Empty initially - messages arrive via events.HistorySync and webhooks
  "count": 0
}

**Important Notes:**
- This endpoint sends a history sync request to your primary WhatsApp device (phone)
- The response indicates the request was sent successfully with a unique `request_id` for tracking
- **Actual messages will arrive asynchronously** via:
  - `events.HistorySync` events in the event bus
  - Webhook delivery (if webhook plugin enabled)
  - Event topic: `message.history.synced`
- **Requirements:**
  - Your primary WhatsApp device (phone) must be connected and online
  - The primary device must have the requested message history available
  - Device must be in "connected" status
- **Pagination:** Use `before_message_id` to fetch older messages:
  1. First request: omit `before_message_id` to get most recent messages
  2. Subsequent requests: use the oldest `message_id` from previous response as `before_message_id`
- **Recommended batch size:** 50 messages per request
- **Response time:** Depends on network and primary device availability (usually 2-10 seconds)

**Events Published:**
- `message.history.requested` - When the request is sent
- `message.history.synced` - When messages are successfully synced (contains actual messages)
- `message.history.failed` - If the sync request fails

---

# Supported Message Types

The WhatsApp Gateway supports the following message types. When a message is received, it will be published as a `message.received` event with the `MessagePayload` structure containing type-specific content.

## Text Messages
**Type:** `text`

Basic text messages or extended text with formatting/replies.

**Payload Fields:**
- `text.content` - The message text

## Media Messages

### Image Messages
**Type:** `image`

Image files with optional caption.

**Payload Fields:**
- `image.url` - WhatsApp media URL
- `image.mimetype` - Image MIME type (e.g., "image/jpeg")
- `image.caption` - Optional caption
- `has_media` - true

### Video Messages
**Type:** `video`

Video files with optional caption.

**Payload Fields:**
- `video.url` - WhatsApp media URL
- `video.mimetype` - Video MIME type (e.g., "video/mp4")
- `video.caption` - Optional caption
- `has_media` - true

### Audio Messages
**Type:** `audio`

Audio files or voice messages.

**Payload Fields:**
- `audio.url` - WhatsApp media URL
- `audio.mimetype` - Audio MIME type (e.g., "audio/ogg")
- `has_media` - true

### Document Messages
**Type:** `document`

File attachments with optional caption.

**Payload Fields:**
- `document.url` - WhatsApp media URL
- `document.mimetype` - Document MIME type (e.g., "application/pdf")
- `document.caption` - Optional caption
- `document.filename` - Original filename
- `has_media` - true

### Sticker Messages
**Type:** `sticker`

Sticker images (usually WebP format).

**Payload Fields:**
- `sticker.url` - WhatsApp media URL
- `sticker.mimetype` - Sticker MIME type (e.g., "image/webp")
- `sticker.caption` - Optional caption (rarely used)
- `has_media` - true

## Contact Messages

### Single Contact
**Type:** `contact`

A single contact card shared.

**Payload Fields:**
- `contact.display_name` - Contact's display name
- `contact.vcard` - Full vCard data (VCF format)

**Example vCard:**
```
BEGIN:VCARD
VERSION:3.0
FN:John Doe
TEL;TYPE=CELL:+1234567890
END:VCARD
```

### Multiple Contacts
**Type:** `contacts`

Multiple contact cards shared in one message.

**Payload Fields:**
- `contacts.display_name` - Display name for the contact array
- `contacts.contacts[]` - Array of vCard strings

## Location Messages

### Static Location
**Type:** `location`

A fixed location pin.

**Payload Fields:**
- `location.latitude` - Location latitude (float64)
- `location.longitude` - Location longitude (float64)
- `location.name` - Optional location name
- `location.address` - Optional full address
- `location.url` - Optional map URL
- `location.is_live` - false

**Example:**
```json
{
  "type": "location",
  "location": {
    "latitude": -6.2088,
    "longitude": 106.8456,
    "name": "Jakarta",
    "address": "Jakarta, Indonesia",
    "is_live": false
  }
}
```

### Live Location
**Type:** `live_location`

Live location sharing (updates periodically).

**Payload Fields:**
- `location.latitude` - Current latitude (float64)
- `location.longitude` - Current longitude (float64)
- `location.name` - Optional caption/description
- `location.is_live` - true

**Note:** Live location updates will arrive as separate `message.received` events as the location changes.

## Reaction Messages
**Type:** `reaction`

Emoji reaction to another message.

**Payload Fields:**
- `reaction.emoji` - The emoji used (empty string = removed reaction)
- `reaction.target_message_id` - ID of the message being reacted to
- `reaction.target_sender` - Optional sender of the target message

**Example - Adding Reaction:**
```json
{
  "type": "reaction",
  "reaction": {
    "emoji": "👍",
    "target_message_id": "3EB089B9D6ADD58153C561",
    "target_sender": "6281234567890@s.whatsapp.net"
  }
}
```

**Example - Removing Reaction:**
```json
{
  "type": "reaction",
  "reaction": {
    "emoji": "",
    "target_message_id": "3EB089B9D6ADD58153C561"
  }
}
```

## Protocol Messages
**Type:** `protocol`

System/protocol messages like deletions, ephemeral settings.

**Payload Fields:**
- `protocol.type` - Protocol message type
- `protocol.target_message_id` - Optional target message ID (for revoke)

**Protocol Types:**
- `revoke` - Message deleted by sender
- `ephemeral_setting` - Disappearing messages setting changed
- `ephemeral_sync_response` - Ephemeral message sync
- `history_sync_notification` - History sync notification
- `app_state_sync_key_share` - App state key sharing
- `initial_security_notification_setting_sync` - Security notification sync
- `app_state_fatal_exception_notification` - App state error

**Example - Message Deletion:**
```json
{
  "type": "protocol",
  "protocol": {
    "type": "revoke",
    "target_message_id": "3EB089B9D6ADD58153C561"
  }
}
```

## Poll Messages

### Poll Creation
**Type:** `poll`

A poll created in the chat.

**Payload Fields:**
- `poll.name` - Poll question
- `poll.options[]` - Array of poll options
- `poll.options[].name` - Option text
- `poll.selectable_count` - Maximum number of options that can be selected

**Example:**
```json
{
  "type": "poll",
  "poll": {
    "name": "What's your favorite color?",
    "options": [
      {"name": "Red"},
      {"name": "Blue"},
      {"name": "Green"}
    ],
    "selectable_count": 1
  }
}
```

### Poll Vote
**Type:** `poll_vote`

A vote cast on a poll.

**Payload Fields:**
- `poll_vote.poll_message_id` - ID of the original poll message
- `poll_vote.selected_options[]` - Array of selected option names

**Note:** Poll votes are encrypted by WhatsApp. The gateway currently marks them as `"[encrypted - requires decryption]"`. Full vote decryption will be implemented in a future update.

**Example:**
```json
{
  "type": "poll_vote",
  "poll_vote": {
    "poll_message_id": "3EB089B9D6ADD58153C561",
    "selected_options": ["[encrypted - requires decryption]"]
  }
}
```

## Unknown Messages
**Type:** `unknown`

Any message type not yet implemented or recognized.

**No additional fields.** Check logs for raw message details.

---

# Message Payload Structure

All incoming messages have this base structure:

```json
{
  "message_id": "3EB089B9D6ADD58153C561",
  "chat_id": "6281234567890@s.whatsapp.net",
  "chat_name": "John Doe",
  "sender": "6281234567890@s.whatsapp.net",
  "sender_name": "John Doe",
  "timestamp": 1699999999000,
  "push_name": "John",
  "from_me": false,
  "is_group": false,
  "type": "text|image|video|audio|document|sticker|contact|contacts|location|live_location|reaction|poll|poll_vote|protocol|unknown",

  // Type-specific content (only one populated based on type)
  "text": {...},
  "image": {...},
  "video": {...},
  "audio": {...},
  "document": {...},
  "sticker": {...},
  "contact": {...},
  "contacts": {...},
  "location": {...},
  "reaction": {...},
  "protocol": {...},
  "poll": {...},
  "poll_vote": {...},

  "has_media": true|false
}
```

**Common Fields:**
- `message_id` - Unique WhatsApp message identifier
- `chat_id` - Chat JID (format: `number@s.whatsapp.net` for DM, `groupid@g.us` for group)
- `chat_name` - Group name (for groups) or contact name (for DMs)
- `sender` - Sender's JID
- `sender_name` - Full name or PushName of sender
- `timestamp` - Unix timestamp in milliseconds
- `push_name` - Original PushName from WhatsApp
- `from_me` - Whether the message was sent by this device
- `is_group` - Whether this is a group chat
- `type` - Message type identifier
- `has_media` - Whether this message contains downloadable media

---

# Event Publishing

When messages are received, the gateway publishes a `message.received` event with the following structure:

```json
{
  "device_id": "device_1",
  "message": {
    // MessagePayload structure as documented above
  },
  "source": "whatsapp"
}
```

**Event Topics:**
- `message.received` - All incoming messages (all types)

**Webhook Delivery:**
If the webhook plugin is enabled, all message types will be delivered to the configured webhook URL as HTTP POST requests with the event payload as JSON.
