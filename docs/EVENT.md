This will document about event


follow @event.go


# Publisher
[CHECKED] Event will be published when device created (manager.go:124)
[CHECKED] Event will be published when device is disconnected (internet issue) (manager.go:79-84)
[CHECKED] Event will be published when device is logout (manager.go:86-95)
[CHECKED] Event will be published new message (from whatsapp) with atleast payload of device_id, message info, source (manager.go:67-77)
[CHECKED] Event will be published new message (from http handler) with atleast payload of device_id, message info, source (manager.go:298-302, 365-369, 432-436)

# Reply / Quoted context

`message.received` payloads carry two extra fields on `message`:

- `quoted` - set when the message is a reply. Contains `message_id`,
  `participant`, `chat_id`, `from_me`, `type` and `preview`. See
  `QuotedMessage` in events.go.
- `is_forwarded` - true when the message was forwarded.

Both are omitted when absent, so non-reply payloads are unchanged. Extraction is
type-agnostic (context_info.go), so replies of any message type are covered.
