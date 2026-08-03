package whatsapp

import (
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// quotedPreviewMaxLen caps the inline quote preview shipped to consumers.
const quotedPreviewMaxLen = 512

// contextInfoCarrier is satisfied by every waE2E message variant that can carry
// a ContextInfo, and therefore can be a reply.
//
// Conversation (a plain string), ReactionMessage and ProtocolMessage do NOT
// implement it - a reply is structurally impossible for those types.
type contextInfoCarrier interface {
	GetContextInfo() *waProto.ContextInfo
}

// extractContextInfo returns the ContextInfo of whichever variant is populated
// on msg, or nil if none carries one. msg must already be unwrapped from its
// container messages (DeviceSent, Ephemeral, ViewOnce, ...).
//
// Note: every slice element below is a typed pointer, so the interface value is
// never a bare nil. Generated protobuf getters are nil-receiver safe, which is
// why an explicit nil check on c would be both wrong (typed nils compare false)
// and unnecessary.
func extractContextInfo(msg *waProto.Message) *waProto.ContextInfo {
	if msg == nil {
		return nil
	}

	for _, c := range []contextInfoCarrier{
		msg.GetExtendedTextMessage(), msg.GetImageMessage(), msg.GetVideoMessage(),
		msg.GetAudioMessage(), msg.GetDocumentMessage(), msg.GetStickerMessage(),
		msg.GetContactMessage(), msg.GetContactsArrayMessage(), msg.GetLocationMessage(),
		msg.GetLiveLocationMessage(), msg.GetPollCreationMessage(), msg.GetGroupInviteMessage(),
		msg.GetButtonsMessage(), msg.GetListMessage(), msg.GetTemplateMessage(),
		msg.GetInteractiveMessage(), msg.GetProductMessage(), msg.GetOrderMessage(),
		msg.GetEventMessage(), msg.GetAlbumMessage(),
	} {
		if ci := c.GetContextInfo(); ci != nil {
			return ci
		}
	}

	return nil
}

// selfJID returns this device's own JID in non-AD form, or an empty string when
// the client is not logged in. Nil-guarded because callers include code paths
// that are exercised in tests without a live client.
func selfJID(client *whatsmeow.Client) string {
	if client == nil || client.Store == nil || client.Store.ID == nil {
		return ""
	}
	return client.Store.ID.ToNonAD().String()
}

// buildQuotedMessage converts a ContextInfo into the DTO shipped to consumers.
//
// Returns nil when ci carries no quote. StanzaID is the reply discriminator:
// ContextInfo is also present on forwarded messages, ad replies and
// disappearing-mode notices, none of which are replies.
func buildQuotedMessage(ci *waProto.ContextInfo, selfJID string) *QuotedMessage {
	if ci == nil || ci.GetStanzaID() == "" {
		return nil
	}

	participant := ci.GetParticipant()

	quoted := &QuotedMessage{
		MessageID:   ci.GetStanzaID(),
		Participant: participant,
		ChatID:      ci.GetRemoteJID(),
		// ContextInfo has no fromMe field, so it is derived by comparing the
		// quoted sender against this device's own JID.
		FromMe: selfJID != "" && participant == selfJID,
	}

	quoted.Type, quoted.Preview = classifyQuotedMessage(ci.GetQuotedMessage())

	return quoted
}

// classifyQuotedMessage returns the type and a short text preview for an
// embedded quoted message, using the same type vocabulary as MessagePayload.
//
// It deliberately performs no media download: the embedded QuotedMessage often
// lacks usable media keys, and downloading here would add a network round trip
// to every inbound message.
func classifyQuotedMessage(m *waProto.Message) (string, string) {
	if m == nil {
		return "", ""
	}

	switch {
	case m.GetConversation() != "":
		return "text", truncateRunes(m.GetConversation(), quotedPreviewMaxLen)
	case m.GetExtendedTextMessage() != nil:
		return "text", truncateRunes(m.GetExtendedTextMessage().GetText(), quotedPreviewMaxLen)
	case m.GetImageMessage() != nil:
		return "image", truncateRunes(m.GetImageMessage().GetCaption(), quotedPreviewMaxLen)
	case m.GetVideoMessage() != nil:
		return "video", truncateRunes(m.GetVideoMessage().GetCaption(), quotedPreviewMaxLen)
	case m.GetAudioMessage() != nil:
		return "audio", ""
	case m.GetDocumentMessage() != nil:
		doc := m.GetDocumentMessage()
		preview := doc.GetCaption()
		if preview == "" {
			preview = doc.GetFileName()
		}
		return "document", truncateRunes(preview, quotedPreviewMaxLen)
	case m.GetStickerMessage() != nil:
		return "sticker", ""
	case m.GetContactMessage() != nil:
		return "contact", truncateRunes(m.GetContactMessage().GetDisplayName(), quotedPreviewMaxLen)
	case m.GetContactsArrayMessage() != nil:
		return "contacts", truncateRunes(m.GetContactsArrayMessage().GetDisplayName(), quotedPreviewMaxLen)
	case m.GetLocationMessage() != nil:
		return "location", truncateRunes(m.GetLocationMessage().GetName(), quotedPreviewMaxLen)
	case m.GetLiveLocationMessage() != nil:
		return "live_location", truncateRunes(m.GetLiveLocationMessage().GetCaption(), quotedPreviewMaxLen)
	case m.GetPollCreationMessage() != nil:
		return "poll", truncateRunes(m.GetPollCreationMessage().GetName(), quotedPreviewMaxLen)
	default:
		return "unknown", ""
	}
}

// truncateRunes truncates s to at most n runes, never splitting a UTF-8
// sequence. A non-positive n returns an empty string.
func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}

	count := 0
	for i := range s {
		if count == n {
			return s[:i]
		}
		count++
	}

	return s
}
