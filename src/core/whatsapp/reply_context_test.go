package whatsapp

import (
	"context"
	"testing"
	"time"

	"go.mau.fi/whatsmeow/types"
)

// newTestManager builds a Manager with only the fields buildOutgoingContextInfo
// needs, so the builder can be exercised without a live WhatsApp client.
func newTestManager() *Manager {
	return &Manager{
		devices:  make(map[string]*Device),
		msgCache: newMessageCache(100, time.Hour),
	}
}

func TestBuildOutgoingContextInfoNoReplyNoForward(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	if ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "", "", false); ci != nil {
		t.Errorf("expected nil when there is no reply and no forward, got %+v", ci)
	}
}

func TestBuildOutgoingContextInfoForwardOnly(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "", "", true)
	if ci == nil {
		t.Fatal("expected ContextInfo for a forward")
	}
	if !ci.GetIsForwarded() {
		t.Error("expected IsForwarded true")
	}
	if ci.GetForwardingScore() != 1 {
		t.Errorf("ForwardingScore = %d, want 1", ci.GetForwardingScore())
	}
	if ci.StanzaID != nil {
		t.Error("expected StanzaID to be unset for a forward with no reply")
	}
	if ci.QuotedMessage != nil {
		t.Error("expected QuotedMessage to be unset for a forward with no reply")
	}
}

func TestBuildOutgoingContextInfoExplicitParticipant(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "MSG1", "628222@s.whatsapp.net", false)
	if ci == nil {
		t.Fatal("expected ContextInfo")
	}
	if ci.GetStanzaID() != "MSG1" {
		t.Errorf("StanzaID = %q", ci.GetStanzaID())
	}
	if ci.GetParticipant() != "628222@s.whatsapp.net" {
		t.Errorf("Participant = %q", ci.GetParticipant())
	}
	// A non-nil QuotedMessage is mandatory or clients drop the quote bar.
	if ci.QuotedMessage == nil {
		t.Error("expected non-nil QuotedMessage")
	}
	// RemoteJID must stay unset, else clients treat the quote as cross-chat.
	if ci.RemoteJID != nil {
		t.Errorf("expected RemoteJID unset, got %q", ci.GetRemoteJID())
	}
}

func TestBuildOutgoingContextInfoDMFallback(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "MSG1", "", false)
	if ci == nil {
		t.Fatal("expected ContextInfo")
	}
	if ci.GetParticipant() != "628111@s.whatsapp.net" {
		t.Errorf("expected DM fallback to the chat JID, got %q", ci.GetParticipant())
	}
}

// TestBuildOutgoingContextInfoGroupNoParticipant asserts we still send: a quote
// with an imperfect participant renders badly, but refusing loses the message.
func TestBuildOutgoingContextInfoGroupNoParticipant(t *testing.T) {
	m := newTestManager()
	group := testJID("120363000000000000", types.GroupServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, group, "MSG1", "", false)
	if ci == nil {
		t.Fatal("expected ContextInfo even without a known participant")
	}
	if ci.GetStanzaID() != "MSG1" {
		t.Errorf("StanzaID = %q", ci.GetStanzaID())
	}
	if ci.GetParticipant() == "" {
		t.Error("expected some participant to be set")
	}
}

func TestBuildOutgoingContextInfoCacheHit(t *testing.T) {
	m := newTestManager()
	group := testJID("120363000000000000", types.GroupServer)
	sender := testJID("628222", types.DefaultUserServer)

	m.msgCache.Put("dev1", group, "MSG1", sender, false, textMessage("original text"))

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, group, "MSG1", "", false)
	if ci == nil {
		t.Fatal("expected ContextInfo")
	}
	if ci.GetParticipant() != "628222@s.whatsapp.net" {
		t.Errorf("expected participant from cache, got %q", ci.GetParticipant())
	}
	if got := ci.GetQuotedMessage().GetConversation(); got != "original text" {
		t.Errorf("expected cached content, got %q", got)
	}
}

func TestBuildOutgoingContextInfoCacheMissStub(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "UNKNOWN", "", false)
	if ci == nil {
		t.Fatal("expected ContextInfo")
	}
	if ci.QuotedMessage == nil {
		t.Fatal("expected a stub QuotedMessage on cache miss")
	}
	if got := ci.GetQuotedMessage().GetConversation(); got != "" {
		t.Errorf("expected empty stub content, got %q", got)
	}
}

// TestBuildOutgoingContextInfoExplicitBeatsCache pins the resolution order.
func TestBuildOutgoingContextInfoExplicitBeatsCache(t *testing.T) {
	m := newTestManager()
	group := testJID("120363000000000000", types.GroupServer)
	cachedSender := testJID("628222", types.DefaultUserServer)

	m.msgCache.Put("dev1", group, "MSG1", cachedSender, false, textMessage("cached"))

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, group, "MSG1", "628333@s.whatsapp.net", false)
	if ci.GetParticipant() != "628333@s.whatsapp.net" {
		t.Errorf("expected the explicit participant to win, got %q", ci.GetParticipant())
	}
	// Content still comes from the cache.
	if got := ci.GetQuotedMessage().GetConversation(); got != "cached" {
		t.Errorf("expected cached content, got %q", got)
	}
}

func TestBuildOutgoingContextInfoInvalidParticipantFallsBack(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "MSG1", "not a jid", false)
	if ci == nil {
		t.Fatal("expected ContextInfo")
	}
	if ci.GetParticipant() != "628111@s.whatsapp.net" {
		t.Errorf("expected fallback to the chat JID, got %q", ci.GetParticipant())
	}
}

func TestBuildOutgoingContextInfoReplyAndForward(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	ci := m.buildOutgoingContextInfo(context.Background(), "dev1", nil, chat, "MSG1", "", true)
	if ci == nil {
		t.Fatal("expected ContextInfo")
	}
	if !ci.GetIsForwarded() {
		t.Error("expected IsForwarded true")
	}
	if ci.GetStanzaID() != "MSG1" {
		t.Error("expected the reply fields to be set alongside the forward flag")
	}
}

// TestNormalizeAddressingModeNilDevice covers the unit-test path where no live
// client exists to resolve an alternate JID.
func TestNormalizeAddressingModeNilDevice(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.HiddenUserServer)
	participant := testJID("628222", types.DefaultUserServer)

	got := m.normalizeAddressingMode(context.Background(), nil, chat, participant)
	if got.String() != participant.String() {
		t.Errorf("expected the participant unchanged without a client, got %s", got)
	}
}

func TestNormalizeAddressingModeGroupChatUnchanged(t *testing.T) {
	m := newTestManager()
	group := testJID("120363000000000000", types.GroupServer)
	participant := testJID("628222", types.DefaultUserServer)

	got := m.normalizeAddressingMode(context.Background(), nil, group, participant)
	if got.String() != participant.String() {
		t.Errorf("expected group chats to leave the participant unchanged, got %s", got)
	}
}

func TestCacheSentMessageNilDevice(t *testing.T) {
	m := newTestManager()
	chat := testJID("628111", types.DefaultUserServer)

	// Must not panic and must store nothing.
	m.cacheSentMessage("dev1", nil, chat, "MSG1", textMessage("x"))
	if m.msgCache.Len() != 0 {
		t.Error("expected nothing to be cached without a device")
	}
}
