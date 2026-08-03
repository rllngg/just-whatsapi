package whatsapp

import (
	"strings"
	"testing"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

func ctxWithStanza(id string) *waProto.ContextInfo {
	return &waProto.ContextInfo{StanzaID: proto.String(id)}
}

func TestExtractContextInfoNil(t *testing.T) {
	if ci := extractContextInfo(nil); ci != nil {
		t.Errorf("expected nil for nil message, got %v", ci)
	}
}

func TestExtractContextInfoPlainConversation(t *testing.T) {
	// A plain Conversation cannot carry a ContextInfo, so it can never be a reply.
	msg := &waProto.Message{Conversation: proto.String("hello")}
	if ci := extractContextInfo(msg); ci != nil {
		t.Errorf("expected nil for plain conversation, got %v", ci)
	}
}

// TestExtractContextInfoTypedNilSafe guards the typed-nil subtlety in
// extractContextInfo: the interface slice holds typed pointers, so entries are
// never bare nil. Generated getters are nil-receiver safe, which is what makes
// the loop correct without an explicit nil check.
func TestExtractContextInfoTypedNilSafe(t *testing.T) {
	msg := &waProto.Message{ImageMessage: nil, VideoMessage: nil, ExtendedTextMessage: nil}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("extractContextInfo panicked on typed nils: %v", r)
		}
	}()

	if ci := extractContextInfo(msg); ci != nil {
		t.Errorf("expected nil, got %v", ci)
	}
}

// TestExtractContextInfoAllVariants would catch a whatsmeow upgrade that drops
// GetContextInfo from any carrier type we rely on.
func TestExtractContextInfoAllVariants(t *testing.T) {
	tests := []struct {
		name string
		msg  *waProto.Message
	}{
		{"extended_text", &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"image", &waProto.Message{ImageMessage: &waProto.ImageMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"video", &waProto.Message{VideoMessage: &waProto.VideoMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"audio", &waProto.Message{AudioMessage: &waProto.AudioMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"document", &waProto.Message{DocumentMessage: &waProto.DocumentMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"sticker", &waProto.Message{StickerMessage: &waProto.StickerMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"contact", &waProto.Message{ContactMessage: &waProto.ContactMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"contacts_array", &waProto.Message{ContactsArrayMessage: &waProto.ContactsArrayMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"location", &waProto.Message{LocationMessage: &waProto.LocationMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"live_location", &waProto.Message{LiveLocationMessage: &waProto.LiveLocationMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"poll_creation", &waProto.Message{PollCreationMessage: &waProto.PollCreationMessage{ContextInfo: ctxWithStanza("ABC")}}},
		{"group_invite", &waProto.Message{GroupInviteMessage: &waProto.GroupInviteMessage{ContextInfo: ctxWithStanza("ABC")}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ci := extractContextInfo(tt.msg)
			if ci == nil {
				t.Fatalf("expected ContextInfo for %s, got nil", tt.name)
			}
			if ci.GetStanzaID() != "ABC" {
				t.Errorf("expected StanzaID ABC, got %q", ci.GetStanzaID())
			}
		})
	}
}

// TestExtractContextInfoNoQuote covers a ContextInfo that is present but is not
// a reply: StanzaID is the reply discriminator, not the presence of ContextInfo.
func TestExtractContextInfoNoQuote(t *testing.T) {
	msg := &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        proto.String("forwarded"),
			ContextInfo: &waProto.ContextInfo{IsForwarded: proto.Bool(true)},
		},
	}

	ci := extractContextInfo(msg)
	if ci == nil {
		t.Fatal("expected ContextInfo to be returned")
	}
	if !ci.GetIsForwarded() {
		t.Error("expected IsForwarded true")
	}
	if q := buildQuotedMessage(ci, ""); q != nil {
		t.Errorf("expected nil quoted message when StanzaID is empty, got %+v", q)
	}
}

func TestBuildQuotedMessageNil(t *testing.T) {
	if q := buildQuotedMessage(nil, "me@s.whatsapp.net"); q != nil {
		t.Errorf("expected nil for nil ContextInfo, got %+v", q)
	}
}

func TestBuildQuotedMessageFields(t *testing.T) {
	ci := &waProto.ContextInfo{
		StanzaID:      proto.String("3EB0ABCDEF"),
		Participant:   proto.String("6281111111111@s.whatsapp.net"),
		RemoteJID:     proto.String("status@broadcast"),
		QuotedMessage: &waProto.Message{Conversation: proto.String("original text")},
	}

	q := buildQuotedMessage(ci, "6289999999999@s.whatsapp.net")
	if q == nil {
		t.Fatal("expected quoted message")
	}
	if q.MessageID != "3EB0ABCDEF" {
		t.Errorf("MessageID = %q", q.MessageID)
	}
	if q.Participant != "6281111111111@s.whatsapp.net" {
		t.Errorf("Participant = %q", q.Participant)
	}
	if q.ChatID != "status@broadcast" {
		t.Errorf("ChatID = %q", q.ChatID)
	}
	if q.Type != "text" {
		t.Errorf("Type = %q", q.Type)
	}
	if q.Preview != "original text" {
		t.Errorf("Preview = %q", q.Preview)
	}
	if q.FromMe {
		t.Error("expected FromMe false when participant differs from self")
	}
}

func TestBuildQuotedMessageFromMe(t *testing.T) {
	self := "6289999999999@s.whatsapp.net"
	ci := &waProto.ContextInfo{
		StanzaID:    proto.String("ID1"),
		Participant: proto.String(self),
	}

	if q := buildQuotedMessage(ci, self); q == nil || !q.FromMe {
		t.Errorf("expected FromMe true when participant equals self")
	}

	// An unknown self JID must not produce a false positive.
	ci2 := &waProto.ContextInfo{StanzaID: proto.String("ID2")}
	if q := buildQuotedMessage(ci2, ""); q == nil || q.FromMe {
		t.Errorf("expected FromMe false when self JID is unknown")
	}
}

func TestClassifyQuotedMessage(t *testing.T) {
	tests := []struct {
		name        string
		msg         *waProto.Message
		wantType    string
		wantPreview string
	}{
		{"nil", nil, "", ""},
		{"conversation", &waProto.Message{Conversation: proto.String("hi")}, "text", "hi"},
		{
			"extended_text",
			&waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{Text: proto.String("hey")}},
			"text", "hey",
		},
		{
			"image_with_caption",
			&waProto.Message{ImageMessage: &waProto.ImageMessage{Caption: proto.String("a photo")}},
			"image", "a photo",
		},
		{
			"video",
			&waProto.Message{VideoMessage: &waProto.VideoMessage{Caption: proto.String("clip")}},
			"video", "clip",
		},
		{"audio", &waProto.Message{AudioMessage: &waProto.AudioMessage{}}, "audio", ""},
		{
			"document_falls_back_to_filename",
			&waProto.Message{DocumentMessage: &waProto.DocumentMessage{FileName: proto.String("report.pdf")}},
			"document", "report.pdf",
		},
		{
			"document_prefers_caption",
			&waProto.Message{DocumentMessage: &waProto.DocumentMessage{
				Caption:  proto.String("Q3 numbers"),
				FileName: proto.String("report.pdf"),
			}},
			"document", "Q3 numbers",
		},
		{"sticker", &waProto.Message{StickerMessage: &waProto.StickerMessage{}}, "sticker", ""},
		{
			"location",
			&waProto.Message{LocationMessage: &waProto.LocationMessage{Name: proto.String("Office")}},
			"location", "Office",
		},
		{
			"poll",
			&waProto.Message{PollCreationMessage: &waProto.PollCreationMessage{Name: proto.String("Lunch?")}},
			"poll", "Lunch?",
		},
		{"unknown", &waProto.Message{ReactionMessage: &waProto.ReactionMessage{}}, "unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotPreview := classifyQuotedMessage(tt.msg)
			if gotType != tt.wantType {
				t.Errorf("type = %q, want %q", gotType, tt.wantType)
			}
			if gotPreview != tt.wantPreview {
				t.Errorf("preview = %q, want %q", gotPreview, tt.wantPreview)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"under_limit", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"ascii_truncated", "hello world", 5, "hello"},
		{"zero", "hello", 0, ""},
		{"negative", "hello", -1, ""},
		{"empty", "", 5, ""},
		{"emoji", "👍👍👍👍", 2, "👍👍"},
		{"cjk", "日本語テスト", 3, "日本語"},
		{"mixed", "a👍日b", 3, "a👍日"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRunes(tt.in, tt.n)
			if got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.in, tt.n, got, tt.want)
			}
			if !utf8ValidString(got) {
				t.Errorf("result %q is not valid UTF-8", got)
			}
		})
	}
}

func TestTruncateRunesRespectsPreviewCap(t *testing.T) {
	long := strings.Repeat("あ", quotedPreviewMaxLen+50)
	got := truncateRunes(long, quotedPreviewMaxLen)

	if n := len([]rune(got)); n != quotedPreviewMaxLen {
		t.Errorf("expected %d runes, got %d", quotedPreviewMaxLen, n)
	}
	if !utf8ValidString(got) {
		t.Error("truncated string is not valid UTF-8")
	}
}

func utf8ValidString(s string) bool {
	for _, r := range s {
		if r == '�' {
			return false
		}
	}
	return true
}

func TestSelfJIDNilClient(t *testing.T) {
	if got := selfJID(nil); got != "" {
		t.Errorf("expected empty string for nil client, got %q", got)
	}
}
