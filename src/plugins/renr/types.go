package renr

// QueueChatPerson represents a person in chat
type QueueChatPerson struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
	Email string `json:"email"`
}

// QueueChatMessage represents a chat message in queue
type QueueChatMessage struct {
	ChannelID int64           `json:"channel_id"`
	MessageID string          `json:"message_id"`
	Ref       string          `json:"ref"`
	To        QueueChatPerson `json:"to"`
	From      QueueChatPerson `json:"from"`
	ReplyTo   *string         `json:"reply_to,omitempty"`
	Body      QueueChatBody   `json:"body"`
	Timestamp int64           `json:"timestamp"`
}

// QueueChatBody represents message content
type QueueChatBody struct {
	Type     string      `json:"type"`              // "text", "image", "document", "video", "audio"
	Content  string      `json:"content"`           // Message text or caption
	Files    []QueueFile `json:"files,omitempty"`   // DEPRECATED - ignore
	FilesURL []string    `json:"filesURL,omitempty"` // Use this for file URLs
}

// QueueFile - DEPRECATED, ignore this field
type QueueFile struct {
	Base64 string `json:"base64"`
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	Type   string `json:"type"`
}

// WhatsAppQRRequest represents incoming QR request from queue
type WhatsAppQRRequest struct {
	Channel ChannelInfo `json:"channel"`
}

// ChannelInfo contains channel information
type ChannelInfo struct {
	ID       int64  `json:"id"`
	Metadata string `json:"metadata"`
	Status   string `json:"status"`
}

// QueueChannelUpdate represents outgoing channel update
type QueueChannelUpdate struct {
	ChannelID   int64   `json:"channel_id"`
	Status      string  `json:"status"`      // WAITING_INTEGRATION, CONNECTED, ERROR
	Reference   string  `json:"reference"`
	Data        *string `json:"data,omitempty"`       // Base64 QR code
	ErrorReason *string `json:"errorReason,omitempty"`
}
