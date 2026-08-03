package renr

import (
	"encoding/json"
	"strings"
	"testing"

	"whatsapp/src/core/whatsapp"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// newTestPlugin builds a Plugin with S3 disabled, which short-circuits
// getAvatarURL before it touches the manager, so the converter can be exercised
// without a live WhatsApp client.
func newTestPlugin() *Plugin {
	return &Plugin{
		logger:    waLog.Noop,
		s3Enabled: false,
	}
}

func textReceivedEvent() whatsapp.MessageReceivedEvent {
	return whatsapp.MessageReceivedEvent{
		DeviceID: "123",
		Message: whatsapp.MessagePayload{
			MessageID:  "MSG1",
			ChatID:     "6281111111111@s.whatsapp.net",
			ChatName:   "John",
			Sender:     "6282222222222@s.whatsapp.net",
			SenderName: "Jane",
			Timestamp:  1699999999,
			Type:       "text",
			Text:       &whatsapp.TextContent{Content: "my reply"},
		},
	}
}

func TestConvertToQueueChatMessageWithQuote(t *testing.T) {
	p := newTestPlugin()

	event := textReceivedEvent()
	event.Message.Quoted = &whatsapp.QuotedMessage{
		MessageID:   "PARENT",
		Participant: "6283333333333@s.whatsapp.net",
		FromMe:      false,
		Type:        "image",
		Preview:     "the original caption",
	}

	queueMsg, err := p.convertToQueueChatMessage(event)
	if err != nil {
		t.Fatalf("convertToQueueChatMessage failed: %v", err)
	}

	if queueMsg.ReplyTo == nil {
		t.Fatal("expected ReplyTo to be set")
	}
	if *queueMsg.ReplyTo != "PARENT" {
		t.Errorf("ReplyTo = %q, want PARENT", *queueMsg.ReplyTo)
	}

	if queueMsg.Quoted == nil {
		t.Fatal("expected Quoted to be set")
	}
	if queueMsg.Quoted.MessageID != "PARENT" {
		t.Errorf("Quoted.MessageID = %q", queueMsg.Quoted.MessageID)
	}
	if *queueMsg.ReplyTo != queueMsg.Quoted.MessageID {
		t.Error("expected ReplyTo and Quoted.MessageID to agree")
	}
	if queueMsg.Quoted.Participant != "6283333333333@s.whatsapp.net" {
		t.Errorf("Quoted.Participant = %q", queueMsg.Quoted.Participant)
	}
	if queueMsg.Quoted.Phone != "6283333333333" {
		t.Errorf("Quoted.Phone = %q, want 6283333333333", queueMsg.Quoted.Phone)
	}
	if queueMsg.Quoted.Type != "image" {
		t.Errorf("Quoted.Type = %q", queueMsg.Quoted.Type)
	}
	if queueMsg.Quoted.Preview != "the original caption" {
		t.Errorf("Quoted.Preview = %q", queueMsg.Quoted.Preview)
	}
}

func TestConvertToQueueChatMessageWithoutQuote(t *testing.T) {
	p := newTestPlugin()

	queueMsg, err := p.convertToQueueChatMessage(textReceivedEvent())
	if err != nil {
		t.Fatalf("convertToQueueChatMessage failed: %v", err)
	}

	if queueMsg.ReplyTo != nil {
		t.Errorf("expected ReplyTo nil, got %q", *queueMsg.ReplyTo)
	}
	if queueMsg.Quoted != nil {
		t.Errorf("expected Quoted nil, got %+v", queueMsg.Quoted)
	}

	// The serialized form must be unchanged for non-replies.
	data, err := json.Marshal(queueMsg)
	if err != nil {
		t.Fatalf("failed to marshal queue message: %v", err)
	}
	if strings.Contains(string(data), "reply_to") {
		t.Errorf("expected no reply_to key, got: %s", data)
	}
	if strings.Contains(string(data), "quoted") {
		t.Errorf("expected no quoted key, got: %s", data)
	}
}

// TestConvertToQueueChatMessageLIDParticipant covers a quoted sender addressed
// by LID rather than phone number.
func TestConvertToQueueChatMessageLIDParticipant(t *testing.T) {
	p := newTestPlugin()

	event := textReceivedEvent()
	event.Message.Quoted = &whatsapp.QuotedMessage{
		MessageID:   "PARENT",
		Participant: "219408454660313:46@lid",
		Type:        "text",
	}

	queueMsg, err := p.convertToQueueChatMessage(event)
	if err != nil {
		t.Fatalf("convertToQueueChatMessage failed: %v", err)
	}

	if queueMsg.Quoted.Phone != "219408454660313" {
		t.Errorf("Quoted.Phone = %q, want the LID user without the device suffix", queueMsg.Quoted.Phone)
	}
}

func TestQuotedMessageID(t *testing.T) {
	replyTo := "FROM_REPLY_TO"

	tests := []struct {
		name string
		msg  QueueChatMessage
		want string
	}{
		{"empty", QueueChatMessage{}, ""},
		{"flat_only", QueueChatMessage{ReplyTo: &replyTo}, "FROM_REPLY_TO"},
		{"structured_only", QueueChatMessage{Quoted: &QuotedData{MessageID: "FROM_QUOTED"}}, "FROM_QUOTED"},
		{
			"flat_wins",
			QueueChatMessage{ReplyTo: &replyTo, Quoted: &QuotedData{MessageID: "FROM_QUOTED"}},
			"FROM_REPLY_TO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quotedMessageID(tt.msg); got != tt.want {
				t.Errorf("quotedMessageID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestQuotedParticipant(t *testing.T) {
	tests := []struct {
		name string
		msg  QueueChatMessage
		want string
	}{
		{"no_quote", QueueChatMessage{}, ""},
		{"empty_quote", QueueChatMessage{Quoted: &QuotedData{}}, ""},
		{
			"participant",
			QueueChatMessage{Quoted: &QuotedData{Participant: "628111@s.whatsapp.net"}},
			"628111@s.whatsapp.net",
		},
		{
			"phone_promoted_to_jid",
			QueueChatMessage{Quoted: &QuotedData{Phone: "628111"}},
			"628111@s.whatsapp.net",
		},
		{
			"participant_wins",
			QueueChatMessage{Quoted: &QuotedData{Participant: "628111@s.whatsapp.net", Phone: "628999"}},
			"628111@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quotedParticipant(tt.msg); got != tt.want {
				t.Errorf("quotedParticipant() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestQueueChatMessageQuoteRoundTrip verifies the outbound direction: a producer
// echoing a quoted object back must deserialize into the same fields.
func TestQueueChatMessageQuoteRoundTrip(t *testing.T) {
	payload := `{
		"channel_id": 123,
		"message_id": "MSG1",
		"ref": "123",
		"to": {"id": "6281111111111@s.whatsapp.net", "name": "John", "phone": "6281111111111", "email": ""},
		"from": {"id": "6282222222222@s.whatsapp.net", "name": "Jane", "phone": "6282222222222", "email": ""},
		"reply_to": "PARENT",
		"quoted": {
			"message_id": "PARENT",
			"participant": "6283333333333@s.whatsapp.net",
			"from_me": false,
			"type": "text",
			"preview": "the original"
		},
		"body": {"type": "text", "content": "my reply"},
		"timestamp": 1699999999
	}`

	var msg QueueChatMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if quotedMessageID(msg) != "PARENT" {
		t.Errorf("quotedMessageID() = %q", quotedMessageID(msg))
	}
	if quotedParticipant(msg) != "6283333333333@s.whatsapp.net" {
		t.Errorf("quotedParticipant() = %q", quotedParticipant(msg))
	}
	if msg.Quoted.Preview != "the original" {
		t.Errorf("Quoted.Preview = %q", msg.Quoted.Preview)
	}
}

// TestQueueChatMessageReplyToOnly covers producers that have not migrated to the
// structured quote yet.
func TestQueueChatMessageReplyToOnly(t *testing.T) {
	payload := `{"channel_id":123,"message_id":"MSG1","ref":"123","reply_to":"PARENT","body":{"type":"text","content":"hi"},"timestamp":1}`

	var msg QueueChatMessage
	if err := json.Unmarshal([]byte(payload), &msg); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if quotedMessageID(msg) != "PARENT" {
		t.Errorf("quotedMessageID() = %q, want PARENT", quotedMessageID(msg))
	}
	// No participant: the manager falls back to its cache, then to heuristics.
	if got := quotedParticipant(msg); got != "" {
		t.Errorf("quotedParticipant() = %q, want empty", got)
	}
}
