package whatsapp

import (
	"testing"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

func testJID(user, server string) types.JID {
	return types.JID{User: user, Server: server}
}

func textMessage(s string) *waProto.Message {
	return &waProto.Message{Conversation: proto.String(s)}
}

func TestMessageCacheRoundTrip(t *testing.T) {
	c := newMessageCache(10, time.Hour)
	chat := testJID("628111", types.DefaultUserServer)
	sender := testJID("628222", types.DefaultUserServer)

	c.Put("dev1", chat, "MSG1", sender, false, textMessage("original"))

	entry, ok := c.Get("dev1", chat, "MSG1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if entry.Content.GetConversation() != "original" {
		t.Errorf("content = %q", entry.Content.GetConversation())
	}
	if entry.Participant.User != "628222" {
		t.Errorf("participant = %s", entry.Participant)
	}
	if entry.FromMe {
		t.Error("expected FromMe false")
	}
}

func TestMessageCacheMiss(t *testing.T) {
	c := newMessageCache(10, time.Hour)
	chat := testJID("628111", types.DefaultUserServer)

	if _, ok := c.Get("dev1", chat, "NOPE"); ok {
		t.Error("expected miss for unknown message ID")
	}
	if _, ok := c.Get("dev1", chat, ""); ok {
		t.Error("expected miss for empty message ID")
	}
}

func TestMessageCacheNilReceiverSafe(t *testing.T) {
	var c *messageCache
	chat := testJID("628111", types.DefaultUserServer)

	c.Put("dev1", chat, "MSG1", chat, false, textMessage("x"))
	if _, ok := c.Get("dev1", chat, "MSG1"); ok {
		t.Error("expected miss from nil cache")
	}
	if c.Len() != 0 {
		t.Error("expected zero length from nil cache")
	}
}

func TestMessageCacheKeyIsolation(t *testing.T) {
	c := newMessageCache(10, time.Hour)
	chatA := testJID("628111", types.DefaultUserServer)
	chatB := testJID("628333", types.DefaultUserServer)
	sender := testJID("628222", types.DefaultUserServer)

	c.Put("dev1", chatA, "SAME", sender, false, textMessage("from dev1 chatA"))
	c.Put("dev2", chatA, "SAME", sender, false, textMessage("from dev2 chatA"))
	c.Put("dev1", chatB, "SAME", sender, false, textMessage("from dev1 chatB"))

	cases := []struct {
		device string
		chat   types.JID
		want   string
	}{
		{"dev1", chatA, "from dev1 chatA"},
		{"dev2", chatA, "from dev2 chatA"},
		{"dev1", chatB, "from dev1 chatB"},
	}

	for _, tc := range cases {
		entry, ok := c.Get(tc.device, tc.chat, "SAME")
		if !ok {
			t.Fatalf("expected hit for %s/%s", tc.device, tc.chat)
		}
		if got := entry.Content.GetConversation(); got != tc.want {
			t.Errorf("%s/%s = %q, want %q", tc.device, tc.chat, got, tc.want)
		}
	}
}

// TestMessageCacheKeyNormalisesAD ensures a chat JID carrying device/agent parts
// resolves to the same entry as its non-AD form.
func TestMessageCacheKeyNormalisesAD(t *testing.T) {
	c := newMessageCache(10, time.Hour)
	plain := testJID("628111", types.DefaultUserServer)
	withDevice := types.JID{User: "628111", Server: types.DefaultUserServer, Device: 3}

	c.Put("dev1", withDevice, "MSG1", plain, false, textMessage("hi"))

	if _, ok := c.Get("dev1", plain, "MSG1"); !ok {
		t.Error("expected AD and non-AD chat JIDs to share a cache key")
	}
}

func TestMessageCacheTTLExpiry(t *testing.T) {
	c := newMessageCache(10, time.Hour)
	chat := testJID("628111", types.DefaultUserServer)

	c.Put("dev1", chat, "MSG1", chat, false, textMessage("old"))

	// Backdate the entry rather than sleeping.
	key := messageCacheKey("dev1", chat, "MSG1")
	c.mu.Lock()
	c.entries[key].StoredAt = time.Now().Add(-2 * time.Hour)
	c.mu.Unlock()

	if _, ok := c.Get("dev1", chat, "MSG1"); ok {
		t.Error("expected expired entry to miss")
	}
}

func TestMessageCacheFIFOEviction(t *testing.T) {
	c := newMessageCache(3, time.Hour)
	chat := testJID("628111", types.DefaultUserServer)

	for _, id := range []string{"M1", "M2", "M3", "M4"} {
		c.Put("dev1", chat, id, chat, false, textMessage(id))
	}

	if c.Len() != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", c.Len())
	}
	if _, ok := c.Get("dev1", chat, "M1"); ok {
		t.Error("expected oldest entry M1 to be evicted")
	}
	for _, id := range []string{"M2", "M3", "M4"} {
		if _, ok := c.Get("dev1", chat, id); !ok {
			t.Errorf("expected %s to still be cached", id)
		}
	}
}

// TestMessageCacheOverwriteDoesNotEvict verifies that re-Putting an existing key
// updates in place instead of consuming another eviction slot.
func TestMessageCacheOverwriteDoesNotEvict(t *testing.T) {
	c := newMessageCache(2, time.Hour)
	chat := testJID("628111", types.DefaultUserServer)

	c.Put("dev1", chat, "M1", chat, false, textMessage("v1"))
	c.Put("dev1", chat, "M2", chat, false, textMessage("v2"))
	c.Put("dev1", chat, "M1", chat, false, textMessage("v1-updated"))

	entry, ok := c.Get("dev1", chat, "M1")
	if !ok {
		t.Fatal("expected M1 to still be cached")
	}
	if entry.Content.GetConversation() != "v1-updated" {
		t.Errorf("expected updated content, got %q", entry.Content.GetConversation())
	}
	if _, ok := c.Get("dev1", chat, "M2"); !ok {
		t.Error("expected M2 to survive an overwrite of M1")
	}
}

func TestTrimForQuoteNil(t *testing.T) {
	if got := trimForQuote(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestTrimForQuoteStripsNestedQuote is the guard against quadratic growth when
// replying to a reply to a reply.
func TestTrimForQuoteStripsNestedQuote(t *testing.T) {
	msg := &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String("photo"),
			JPEGThumbnail: []byte{1, 2, 3, 4},
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String("PARENT"),
				QuotedMessage: textMessage("the grandparent message"),
			},
		},
	}

	trimmed := trimForQuote(msg)
	if trimmed == nil {
		t.Fatal("expected trimmed message")
	}

	img := trimmed.GetImageMessage()
	if img.GetJPEGThumbnail() != nil {
		t.Error("expected JPEGThumbnail to be stripped")
	}
	if img.GetContextInfo().GetQuotedMessage() != nil {
		t.Error("expected nested QuotedMessage to be stripped")
	}
	if img.GetContextInfo().GetStanzaID() != "PARENT" {
		t.Error("expected StanzaID to be preserved")
	}
	if img.GetCaption() != "photo" {
		t.Error("expected caption to be preserved")
	}

	// The original must not be mutated.
	if msg.GetImageMessage().GetJPEGThumbnail() == nil {
		t.Error("trimForQuote mutated its input")
	}
	if msg.GetImageMessage().GetContextInfo().GetQuotedMessage() == nil {
		t.Error("trimForQuote mutated its input's nested quote")
	}
}

func TestTrimForQuoteStripsExtendedTextNestedQuote(t *testing.T) {
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:          proto.String("reply"),
			JPEGThumbnail: []byte{9, 9},
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String("PARENT"),
				QuotedMessage: textMessage("grandparent"),
			},
		},
	}

	trimmed := trimForQuote(msg)
	ext := trimmed.GetExtendedTextMessage()

	if ext.GetJPEGThumbnail() != nil {
		t.Error("expected JPEGThumbnail to be stripped")
	}
	if ext.GetContextInfo().GetQuotedMessage() != nil {
		t.Error("expected nested QuotedMessage to be stripped")
	}
	if ext.GetText() != "reply" {
		t.Error("expected text to be preserved")
	}
}

func TestMessageCacheEnvParsing(t *testing.T) {
	sizeTests := []struct {
		raw  string
		want int
	}{
		{"", 500},
		{"1000", 1000},
		{"abc", 500},
		{"0", 500},
		{"-5", 500},
	}
	for _, tt := range sizeTests {
		if got := messageCacheSizeFromEnv(tt.raw, 500); got != tt.want {
			t.Errorf("messageCacheSizeFromEnv(%q) = %d, want %d", tt.raw, got, tt.want)
		}
	}

	ttlTests := []struct {
		raw  string
		want time.Duration
	}{
		{"", time.Hour},
		{"30m", 30 * time.Minute},
		{"nonsense", time.Hour},
		{"0s", time.Hour},
		{"-1h", time.Hour},
	}
	for _, tt := range ttlTests {
		if got := messageCacheTTLFromEnv(tt.raw, time.Hour); got != tt.want {
			t.Errorf("messageCacheTTLFromEnv(%q) = %v, want %v", tt.raw, got, tt.want)
		}
	}
}

func TestNewMessageCacheDefaults(t *testing.T) {
	c := newMessageCache(0, 0)
	if c.max != defaultMessageCacheSize {
		t.Errorf("max = %d, want %d", c.max, defaultMessageCacheSize)
	}
	if c.ttl != defaultMessageCacheTTL {
		t.Errorf("ttl = %v, want %v", c.ttl, defaultMessageCacheTTL)
	}
}
