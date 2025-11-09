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
