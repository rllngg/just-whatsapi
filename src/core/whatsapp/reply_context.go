package whatsapp

import (
	"context"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// buildOutgoingContextInfo builds the ContextInfo for a reply and/or a forward.
//
// It returns nil when neither applies, which is the correct "no context" value
// for the proto fields callers assign it to.
//
// WhatsApp needs three things for a quote to render: StanzaID, Participant, and
// a non-nil QuotedMessage. Omitting QuotedMessage makes clients drop the quote
// bar silently, so an empty stub is used on a cache miss - the recipient's own
// client renders the preview from its local copy, and the embedded copy is only
// a fallback. RemoteJID is deliberately left unset: setting it makes clients
// treat the quote as cross-chat.
func (m *Manager) buildOutgoingContextInfo(
	ctx context.Context,
	deviceID string,
	device *Device,
	chatJID types.JID,
	replyMessageID string,
	replyParticipant string,
	isForwarded bool,
) *waProto.ContextInfo {
	if replyMessageID == "" && !isForwarded {
		return nil
	}

	ci := &waProto.ContextInfo{}

	if isForwarded {
		ci.IsForwarded = proto.Bool(true)
		ci.ForwardingScore = proto.Uint32(1)
	}

	if replyMessageID == "" {
		return ci
	}

	cached, hasCached := m.msgCache.Get(deviceID, chatJID, replyMessageID)

	participant := m.resolveReplyParticipant(ctx, device, chatJID, replyParticipant, cached, hasCached)

	quoted := &waProto.Message{Conversation: proto.String("")}
	if hasCached && cached.Content != nil {
		quoted = cached.Content
	}

	ci.StanzaID = proto.String(replyMessageID)
	ci.Participant = proto.String(participant.String())
	ci.QuotedMessage = quoted

	return ci
}

// cacheSentMessage remembers a message we just sent, so that a reply quoting it
// can embed real content rather than the empty stub.
func (m *Manager) cacheSentMessage(deviceID string, device *Device, chatJID types.JID, msgID string, msg *waProto.Message) {
	if msgID == "" || device == nil || device.Client == nil || device.Client.Store == nil {
		return
	}

	own := device.Client.Store.ID
	if own == nil {
		return
	}

	m.msgCache.Put(deviceID, chatJID, msgID, own.ToNonAD(), true, msg)
}

// resolveReplyParticipant determines the JID of the quoted message's sender,
// preferring what the caller supplied, then the cache, then chat-shape
// heuristics.
func (m *Manager) resolveReplyParticipant(
	ctx context.Context,
	device *Device,
	chatJID types.JID,
	replyParticipant string,
	cached *messageCacheEntry,
	hasCached bool,
) types.JID {
	var participant types.JID

	if replyParticipant != "" {
		// ParseJID accepts a bare string by treating it as a server with an
		// empty user, so a non-empty User is what actually validates it.
		parsed, err := types.ParseJID(replyParticipant)
		switch {
		case err != nil:
			if m.logger != nil {
				m.logger.Warnf("Invalid reply_participant %q, falling back: %v", replyParticipant, err)
			}
		case parsed.User == "":
			if m.logger != nil {
				m.logger.Warnf("reply_participant %q has no user part, falling back", replyParticipant)
			}
		default:
			participant = parsed.ToNonAD()
		}
	}

	if participant.IsEmpty() && hasCached && !cached.Participant.IsEmpty() {
		participant = cached.Participant
	}

	if participant.IsEmpty() {
		if chatJID.Server == types.GroupServer {
			// A group quote needs the original sender, which we do not have.
			// Send anyway with the group JID: the quote renders imperfectly,
			// whereas refusing would lose the message entirely.
			if m.logger != nil {
				m.logger.Warnf("Reply in group %s has no known participant; quote may render incompletely", chatJID)
			}
			participant = chatJID.ToNonAD()
		} else {
			// In a DM the quoted message is almost always the other party's.
			participant = chatJID.ToNonAD()
		}
	}

	return m.normalizeAddressingMode(ctx, device, chatJID, participant)
}

// normalizeAddressingMode aligns the participant's addressing mode (PN vs LID)
// with the chat's. whatsmeow does not do this on send, so a mismatched
// participant would produce a quote the recipient cannot resolve.
func (m *Manager) normalizeAddressingMode(
	ctx context.Context,
	device *Device,
	chatJID types.JID,
	participant types.JID,
) types.JID {
	if device == nil || device.Client == nil || device.Client.Store == nil {
		return participant
	}

	// Only user JIDs have an alternate form; group JIDs are left alone.
	if chatJID.Server != types.DefaultUserServer && chatJID.Server != types.HiddenUserServer {
		return participant
	}
	if participant.Server == chatJID.Server {
		return participant
	}

	alt, err := device.Client.Store.GetAltJID(ctx, participant)
	if err != nil {
		if m.logger != nil {
			m.logger.Warnf("Failed to resolve alternate JID for %s: %v", participant, err)
		}
		return participant
	}
	if alt.IsEmpty() {
		return participant
	}

	return alt.ToNonAD()
}
