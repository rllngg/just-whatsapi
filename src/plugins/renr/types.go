package renr

// QueueChatPerson represents a person in chat
type QueueChatPerson struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarURL,omitempty"` // S3 URL of profile picture
}

// QuotedData describes the message a reply is quoting. Mirrors
// whatsapp.QuotedMessage for the queue contract.
type QuotedData struct {
	MessageID   string `json:"message_id"`
	Participant string `json:"participant,omitempty"` // Sender JID of the quoted message
	Phone       string `json:"phone,omitempty"`       // Phone number extracted from Participant
	ChatID      string `json:"chat_id,omitempty"`     // Only set for cross-chat quotes
	FromMe      bool   `json:"from_me"`
	Type        string `json:"type,omitempty"`    // Quoted message type
	Preview     string `json:"preview,omitempty"` // Short text/caption excerpt
}

// QueueChatMessage represents a chat message in queue
type QueueChatMessage struct {
	ChannelID int64           `json:"channel_id"`
	MessageID string          `json:"message_id"`
	Ref       string          `json:"ref"`
	To        QueueChatPerson `json:"to"`
	From      QueueChatPerson `json:"from"`
	// ReplyTo is the quoted message ID. Retained for backward compatibility;
	// new consumers should prefer Quoted.
	ReplyTo *string `json:"reply_to,omitempty"`
	// Quoted carries the full reply context so consumers can render a quote
	// bubble without a lookup
	Quoted    *QuotedData   `json:"quoted,omitempty"`
	Body      QueueChatBody `json:"body"`
	Timestamp int64         `json:"timestamp"`
}

// QueueChatBody represents message content
type QueueChatBody struct {
	Type     string      `json:"type"`               // "text", "image", "document", "video", "audio", "sticker", "contact", "contacts", "location", "live_location", "reaction", "protocol", "poll", "poll_vote"
	Content  string      `json:"content"`            // Message text or caption
	Files    []QueueFile `json:"files,omitempty"`    // DEPRECATED - ignore
	FilesURL []string    `json:"filesURL,omitempty"` // Use this for file URLs

	// Structured data fields for new message types (all optional)
	Contact  *ContactData  `json:"contact,omitempty"`
	Contacts *ContactsData `json:"contacts,omitempty"`
	Location *LocationData `json:"location,omitempty"`
	Reaction *ReactionData `json:"reaction,omitempty"`
	Protocol *ProtocolData `json:"protocol,omitempty"`
	Poll     *PollData     `json:"poll,omitempty"`
	PollVote *PollVoteData `json:"poll_vote,omitempty"`
}

// ContactData represents a single contact vCard
type ContactData struct {
	DisplayName string `json:"display_name"`
	VCard       string `json:"vcard"` // Full vCard 3.0 format
}

// ContactsData represents multiple contacts
type ContactsData struct {
	DisplayName string   `json:"display_name"`
	Contacts    []string `json:"contacts"` // Array of vCard strings
}

// LocationData represents a location (static or live)
type LocationData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Name      string  `json:"name,omitempty"`
	Address   string  `json:"address,omitempty"`
	URL       string  `json:"url,omitempty"` // Map URL
	IsLive    bool    `json:"is_live"`       // true for live location
}

// ReactionData represents a reaction to a message
type ReactionData struct {
	Emoji           string `json:"emoji"` // Empty string = reaction removed
	TargetMessageID string `json:"target_message_id"`
	TargetSender    string `json:"target_sender,omitempty"`
}

// ProtocolData represents protocol messages (deletions, etc.)
type ProtocolData struct {
	ProtocolType    string `json:"protocol_type"`               // "revoke", "ephemeral_setting", etc.
	TargetMessageID string `json:"target_message_id,omitempty"` // For revoke
}

// PollData represents a poll creation
type PollData struct {
	Name            string       `json:"name"` // Poll question
	Options         []PollOption `json:"options"`
	SelectableCount int          `json:"selectable_count"` // Max selections allowed
}

// PollOption represents a single poll option
type PollOption struct {
	Name string `json:"name"` // Option text
}

// PollVoteData represents a vote on a poll
type PollVoteData struct {
	PollMessageID   string   `json:"poll_message_id"`
	SelectedOptions []string `json:"selected_options"` // May contain "[encrypted - requires decryption]"
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
	Name        string  `json:"name"`
	Status      string  `json:"status"` // WAITING_INTEGRATION, CONNECTED, ERROR, DISCONNECTED
	Reference   string  `json:"reference"`
	Data        *string `json:"data,omitempty"` // Base64 QR code
	ErrorReason *string `json:"errorReason,omitempty"`
}

// ChannelDisconnectRequest represents incoming disconnect request from queue
type ChannelDisconnectRequest struct {
	ChannelID   int64  `json:"channel_id"`
	Status      string `json:"status"`      // Should be "DISCONNECTED"
	Reference   string `json:"reference"`   // device_id to disconnect
	ErrorReason string `json:"errorReason"` // Reason for disconnect (required, can be empty string)
}

// QueueMessageRead represents a message read receipt in queue
type QueueMessageRead struct {
	ChannelID   int64           `json:"channelId"`
	From        QueueChatPerson `json:"from"`
	To          QueueChatPerson `json:"to"`
	Timestamp   int64           `json:"timestamp"`
	ErrorReason string          `json:"errorReason"`
}

// QueueMessageReadIds represents message read receipt with IDs for queue
type QueueMessageReadIds struct {
	ConversationID int64   `json:"conversation_id"`
	MessageIDs     []int64 `json:"message_ids"`
	ReadAt         string  `json:"read_at"` // ISO 8601 format for LocalDateTime
}
