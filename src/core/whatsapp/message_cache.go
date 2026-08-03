package whatsapp

import (
	"strconv"
	"sync"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

const (
	defaultMessageCacheSize = 2000
	defaultMessageCacheTTL  = 24 * time.Hour
)

// messageCacheEntry is a trimmed copy of a message kept so that an outbound
// reply can embed a faithful quote.
type messageCacheEntry struct {
	Content     *waProto.Message
	Participant types.JID
	FromMe      bool
	StoredAt    time.Time
}

// messageCache stores recently seen messages keyed by device, chat and message
// ID. It is bounded by both size (FIFO eviction) and age (TTL).
//
// This lives in core rather than in a plugin because the send path is core and
// is also reached from the HTTP gateway, which has the same problem of knowing
// only a message ID when building a reply.
type messageCache struct {
	mu      sync.RWMutex
	entries map[string]*messageCacheEntry
	order   []string
	max     int
	ttl     time.Duration
}

// newMessageCache creates a cache holding at most max entries for at most ttl.
// Non-positive values fall back to the defaults.
func newMessageCache(max int, ttl time.Duration) *messageCache {
	if max <= 0 {
		max = defaultMessageCacheSize
	}
	if ttl <= 0 {
		ttl = defaultMessageCacheTTL
	}

	return &messageCache{
		entries: make(map[string]*messageCacheEntry, max),
		order:   make([]string, 0, max),
		max:     max,
		ttl:     ttl,
	}
}

// messageCacheKey builds the composite cache key. The chat JID is normalised to
// its non-AD form so that messages stored from different device sessions of the
// same chat collide correctly.
func messageCacheKey(deviceID string, chat types.JID, msgID string) string {
	return deviceID + "|" + chat.ToNonAD().String() + "|" + msgID
}

// Put stores a trimmed copy of content. A nil receiver is a no-op so callers
// need no guard.
func (c *messageCache) Put(deviceID string, chat types.JID, msgID string, participant types.JID, fromMe bool, content *waProto.Message) {
	if c == nil || msgID == "" {
		return
	}

	key := messageCacheKey(deviceID, chat, msgID)

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.entries[key]; !exists {
		// Evict oldest entries until there is room for one more.
		for len(c.order) >= c.max && len(c.order) > 0 {
			oldest := c.order[0]
			c.order = c.order[1:]
			delete(c.entries, oldest)
		}
		c.order = append(c.order, key)
	}

	c.entries[key] = &messageCacheEntry{
		Content:     trimForQuote(content),
		Participant: participant.ToNonAD(),
		FromMe:      fromMe,
		StoredAt:    time.Now(),
	}
}

// Get returns the cached entry, or false when absent or expired.
func (c *messageCache) Get(deviceID string, chat types.JID, msgID string) (*messageCacheEntry, bool) {
	if c == nil || msgID == "" {
		return nil, false
	}

	key := messageCacheKey(deviceID, chat, msgID)

	c.mu.RLock()
	entry, ok := c.entries[key]
	c.mu.RUnlock()

	if !ok {
		return nil, false
	}
	if time.Since(entry.StoredAt) > c.ttl {
		return nil, false
	}

	return entry, true
}

// Len reports the number of entries currently held, expired ones included.
func (c *messageCache) Len() int {
	if c == nil {
		return 0
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}

// trimForQuote returns a copy of msg suitable for embedding as a quote. It
// strips inline thumbnails and, critically, any nested quoted message: without
// that a reply to a reply to a reply would grow quadratically.
func trimForQuote(msg *waProto.Message) *waProto.Message {
	if msg == nil {
		return nil
	}

	clone, ok := proto.Clone(msg).(*waProto.Message)
	if !ok || clone == nil {
		return nil
	}

	if m := clone.GetImageMessage(); m != nil {
		m.JPEGThumbnail = nil
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetVideoMessage(); m != nil {
		m.JPEGThumbnail = nil
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetDocumentMessage(); m != nil {
		m.JPEGThumbnail = nil
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetStickerMessage(); m != nil {
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetAudioMessage(); m != nil {
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetLocationMessage(); m != nil {
		m.JPEGThumbnail = nil
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetLiveLocationMessage(); m != nil {
		m.JPEGThumbnail = nil
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetExtendedTextMessage(); m != nil {
		m.JPEGThumbnail = nil
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetContactMessage(); m != nil {
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetContactsArrayMessage(); m != nil {
		stripNestedQuote(m.GetContextInfo())
	}
	if m := clone.GetPollCreationMessage(); m != nil {
		stripNestedQuote(m.GetContextInfo())
	}

	return clone
}

// stripNestedQuote removes an embedded quoted message from ci.
func stripNestedQuote(ci *waProto.ContextInfo) {
	if ci != nil {
		ci.QuotedMessage = nil
	}
}

// messageCacheSizeFromEnv parses a positive integer, returning fallback when the
// value is unset or invalid.
func messageCacheSizeFromEnv(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

// messageCacheTTLFromEnv parses a Go duration, returning fallback when the value
// is unset or invalid.
func messageCacheTTLFromEnv(raw string, fallback time.Duration) time.Duration {
	if raw == "" {
		return fallback
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
