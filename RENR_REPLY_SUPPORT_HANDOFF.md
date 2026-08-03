# Reply / Quoted Message Support — Handoff to Renr Team

**Branch:** `t3code/renr-reply-comment-support`
**Scope:** WhatsApp Gateway only. Renr-side changes are still required — see [What You Need To Do](#what-you-need-to-do).

---

## 1. Problem

A reply sent in WhatsApp arrived at Renr as an ordinary chat message. The "replying to X" relationship was lost, so threads read as unrelated messages.

Root cause was on the gateway side, in both directions:

- **Inbound:** the gateway discarded WhatsApp's `ContextInfo` entirely. The `reply_to` field existed in the queue contract but was **hardcoded to `null`** — there was a literal `// TODO: Extract reply_to` in the code. Every reply that ever reached Renr had `reply_to: null`.
- **Outbound:** the gateway accepted `reply_to` and mapped it to an internal `reply_message_id` field that **no send function ever read**. Replies sent from Renr were delivered to WhatsApp as plain messages, with no quote.

Both are now fixed.

---

## 2. What Changed

### 2.1 Inbound — `channel-chat-message-incoming`

Two fields are now populated when a message is a reply:

| Field | Before | After |
|-------|--------|-------|
| `reply_to` | always `null` | quoted message ID, or **key omitted** when not a reply |
| `quoted` | did not exist | full quote context object, or **key omitted** when not a reply |

Example reply payload:

```json
{
  "channel_id": 123,
  "message_id": "3EB0CHILD",
  "ref": "123",
  "to":   { "id": "6281234567890@s.whatsapp.net", "name": "John Doe",  "phone": "6281234567890", "email": "" },
  "from": { "id": "6289876543210@s.whatsapp.net", "name": "Jane Smith", "phone": "6289876543210", "email": "" },
  "reply_to": "3EB0PARENT",
  "quoted": {
    "message_id":  "3EB0PARENT",
    "participant": "6289876543210@s.whatsapp.net",
    "phone":       "6289876543210",
    "from_me":     false,
    "type":        "text",
    "preview":     "The original message being replied to"
  },
  "body": { "type": "text", "content": "Yes, I agree!", "files": null, "filesURL": null },
  "timestamp": 1699999999
}
```

#### `quoted` field reference

| Field | Type | Notes |
|-------|------|-------|
| `message_id` | string | WhatsApp stanza ID of the quoted message. Always present |
| `participant` | string | JID of the quoted message's **sender**. Critical for group replies |
| `phone` | string | User part extracted from `participant` (LID device suffix stripped) |
| `chat_id` | string | Only set for cross-chat quotes, e.g. status replies. Usually absent |
| `from_me` | bool | `true` when *we* sent the quoted message |
| `type` | string | Quoted message type — same vocabulary as `body.type` |
| `preview` | string | Text/caption excerpt, truncated to **512 runes** (rune-safe, never splits UTF-8) |

**Notes on `preview`:** derived from the copy of the original message that WhatsApp embeds in the reply. It is never a media download. So a quoted image yields its caption, a quoted document yields its caption or filename, and a quoted sticker/audio yields an empty string. `preview` may legitimately be `""` — always fall back to rendering by `type`.

**Notes on `participant`:** in a **group**, this is the individual member who sent the original message, not the group JID. This is the field that makes "Alice replied to Bob" renderable. In a **DM** it is the other party (or our own JID when `from_me` is true).

#### Coverage

Reply context is extracted **type-agnostically** — every WhatsApp message type that can carry a quote is covered (text, image, video, audio, document, sticker, contact, contacts, location, live location, poll, group invite, and interactive/template variants). This applies to both "reply *to* a media message" and "reply *with* a media message".

Not applicable by protocol design: reactions and protocol messages (deletes) cannot be replies. Reactions already carry their own `body.reaction.target_message_id`.

Messages arriving via history sync go through the same converter, so historical replies also carry `quoted`.

### 2.2 Outbound — `channel-chat-message-outgoing`

To send a reply, include `reply_to`. Optionally include `quoted.participant`.

```json
{
  "channel_id": 123,
  "to": { "id": "6281234567890@s.whatsapp.net" },
  "reply_to": "3EB0PARENT",
  "quoted": {
    "message_id":  "3EB0PARENT",
    "participant": "6289876543210@s.whatsapp.net"
  },
  "body": { "type": "text", "content": "Replying to your message" }
}
```

Accepted forms, in precedence order:

1. `reply_to` — the flat ID. Takes precedence if both are present.
2. `quoted.message_id` — used when `reply_to` is absent.
3. `quoted.participant`, else `quoted.phone` — used to identify the original sender.

**`quoted.participant` is optional but strongly recommended for group replies.** When it is omitted the gateway resolves the sender by:

1. Looking up its in-memory message cache (recent messages, both sent and received).
2. For a **DM**, assuming the other party — correct in nearly all cases.
3. For a **group** with a cache miss, falling back to the group JID and logging a warning. **The quote will render imperfectly.** The gateway still sends the message rather than dropping it.

So: echoing `quoted.participant` back from the inbound payload is the single most valuable thing you can do for group reply fidelity.

**Supported for reply:** `text`, `image`, `document`, `video`, `audio`, `file`.
**Ignored for reply:** `reaction` — it carries its own target.

### 2.3 Webhook consumers

Webhook delivery is a generic passthrough, so webhook payloads gain the same `quoted` object plus an `is_forwarded` boolean on `message`, with no webhook-side changes needed.

### 2.4 HTTP API

- `reply_message_id` on `/message/text`, `/message/image`, `/message/file` was previously accepted and **silently ignored**. It now actually produces a quoted reply.
- New optional `reply_participant` field on the same endpoints, same semantics as `quoted.participant`.

---

## 3. What You Need To Do

### Required — before deploy

1. **Check unknown-property handling.** A new `quoted` key now appears on inbound payloads. If your deserializer has `FAIL_ON_UNKNOWN_PROPERTIES` enabled (Jackson default is *enabled*), inbound message processing will start throwing. Either add the field or disable that behavior.

2. **Check every `reply_to` assumption.** This field was previously `null` on 100% of messages. Any code that assumed it — a null check treated as dead, a non-null branch never exercised, a `NOT NULL` column, an index, a test fixture — is now live for the first time. This is the only non-additive part of the change.

### Recommended

3. **Render the quote bubble** from `quoted` — you have `message_id`, sender (`participant` / `phone`), `type` and `preview`, so no database lookup is needed. This is what actually resolves the reported confusion; the gateway now supplies the data, but the UI has to use it.

4. **Handle a quoted parent you don't have.** `quoted.message_id` may reference a message that predates the channel connection, or was sent from the phone directly and never synced. Render from `preview` + `type` rather than assuming the parent exists locally.

5. **Echo `quoted.participant` on outbound replies.** Cheap, and it is what makes group replies quote the correct person.

6. **Treat `preview` as possibly empty.** Sticker, audio, and caption-less media quotes have no text. Fall back to a type label ("Photo", "Voice message", …).

### Suggested test cases

| Case | Expect |
|------|--------|
| Reply in a **DM** | `quoted.message_id` matches parent, `preview` populated |
| Reply in a **group** | `quoted.participant` is the *member* JID, not the group JID |
| Reply **to** an image | `quoted.type == "image"`, `preview` is the caption (may be `""`) |
| Reply **with** an image | `body.type == "image"` and `quoted` still present |
| Reply to **our own** message | `quoted.from_me == true` |
| Plain non-reply message | neither `reply_to` nor `quoted` key present |
| Outbound with `reply_to` + `quoted.participant` | recipient's WhatsApp shows a real quote bubble |
| Outbound with `reply_to` only | still quotes, via cache/DM fallback |

---

## 4. Compatibility Summary

| Change | Breaking? |
|--------|-----------|
| New `quoted` key on inbound | Additive — breaks only strict unknown-property parsers |
| `reply_to` now populated | **Behavioral** — was always `null`, now sometimes set |
| `quoted` accepted on outbound | Additive — ignored if you don't send it |
| Existing `reply_to`-only producers | Unchanged, still work |
| Non-reply payloads | Byte-identical to before — both keys omitted |

---

## 5. Known Limitations

- **The quote cache is in-memory.** A gateway restart clears it. Replies still send correctly after a restart, but a group reply whose `quoted.participant` you did not supply may attribute the quote imperfectly until that chat sees traffic again. Supplying `quoted.participant` makes this a non-issue. SQLite persistence is a deferred follow-up.
- **`preview` is best-effort.** It comes from WhatsApp's embedded copy of the original, which is occasionally absent; `type` and `preview` are then empty while `message_id` is still correct.
- **Swagger is partially stale.** `reply_participant` was added by hand to the three swagger files. Full regeneration is blocked by a tooling version mismatch (CLI v1.16.4 vs library v1.8.1 pinned in `go.mod`) and is tracked separately — it does not affect runtime behavior.

---

## 6. Verification Status

`go build ./...` clean, `go vet ./...` clean, **129 tests passing** (up from 39; 90 added).

Covered: context extraction across all carrier types, typed-nil safety, reply-vs-forward discrimination, quote classification, rune-safe truncation, cache round trip / key isolation / TTL / FIFO eviction, nested-quote stripping, outbound participant resolution and all fallback paths, and JSON round trips for both DTOs including the omitted-when-absent property.

Not yet done: end-to-end manual test against a live paired device — needs a real WhatsApp connection.
