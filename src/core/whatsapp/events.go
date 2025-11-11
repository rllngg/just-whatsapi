package whatsapp

// BaseEvent contains common fields for all events
type BaseEvent struct {
	DeviceID string `json:"device_id"`
}

// Device Lifecycle Events

// DeviceCreatedEvent is published when a new device is created
type DeviceCreatedEvent struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"` // "pending"
}

// DeviceQRReadyEvent is published when QR code is ready for scanning
type DeviceQRReadyEvent struct {
	DeviceID string `json:"device_id"`
	Status   string `json:"status"` // "qr_ready"
}

// DeviceConnectedEvent is published when device successfully connects
type DeviceConnectedEvent struct {
	DeviceID  string `json:"device_id"`
	DeviceJID string `json:"device_jid,omitempty"`
	Status    string `json:"status"` // "connected"
}

// DeviceDisconnectedEvent is published when device loses connection
type DeviceDisconnectedEvent struct {
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason"` // "connection_lost"
}

// DeviceLogoutEvent is published when device is logged out
type DeviceLogoutEvent struct {
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason"` // logout reason from WhatsApp
}

// DeviceRestoredEvent is published when device is restored from database
type DeviceRestoredEvent struct {
	DeviceID  string `json:"device_id"`
	DeviceJID string `json:"device_jid"`
	Status    string `json:"status"` // "connected"
}

// DeviceRestoreFailedEvent is published when device restoration fails
type DeviceRestoreFailedEvent struct {
	DeviceID string `json:"device_id"`
	Error    string `json:"error"`
}

// Message Events

// TextContent contains the text message content
type TextContent struct {
	// Content is the actual text message
	Content string `json:"content"`
}

// ImageContent contains image message metadata
type ImageContent struct {
	// URL is the WhatsApp media URL for the image
	URL string `json:"url,omitempty"`
	// Mimetype is the image MIME type (e.g., "image/jpeg")
	Mimetype string `json:"mimetype"`
	// Caption is the optional image caption
	Caption string `json:"caption,omitempty"`
}

// VideoContent contains video message metadata
type VideoContent struct {
	// URL is the WhatsApp media URL for the video
	URL string `json:"url,omitempty"`
	// Mimetype is the video MIME type (e.g., "video/mp4")
	Mimetype string `json:"mimetype"`
	// Caption is the optional video caption
	Caption string `json:"caption,omitempty"`
}

// AudioContent contains audio message metadata
type AudioContent struct {
	// URL is the WhatsApp media URL for the audio
	URL string `json:"url,omitempty"`
	// Mimetype is the audio MIME type (e.g., "audio/ogg")
	Mimetype string `json:"mimetype"`
}

// DocumentContent contains document message metadata
type DocumentContent struct {
	// URL is the WhatsApp media URL for the document
	URL string `json:"url,omitempty"`
	// Mimetype is the document MIME type (e.g., "application/pdf")
	Mimetype string `json:"mimetype"`
	// Caption is the optional document caption
	Caption string `json:"caption,omitempty"`
	// Filename is the original filename of the document
	Filename string `json:"filename"`
}

// MessagePayload represents the message content structure
type MessagePayload struct {
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id"`
	Sender    string `json:"sender"`
	Timestamp int64  `json:"timestamp"`
	FromMe    bool   `json:"from_me"`
	Type      string `json:"type"` // "text", "image", "video", "audio", "document", "unknown"
	Text      *TextContent     `json:"text,omitempty"`
	Image     *ImageContent    `json:"image,omitempty"`
	Video     *VideoContent    `json:"video,omitempty"`
	Audio     *AudioContent    `json:"audio,omitempty"`
	Document  *DocumentContent `json:"document,omitempty"`
	HasMedia  bool             `json:"has_media,omitempty"`
}

// MessageReceivedEvent is published when a message is received from WhatsApp
type MessageReceivedEvent struct {
	DeviceID string         `json:"device_id"`
	Message  MessagePayload `json:"message"`
	Source   string         `json:"source"` // "whatsapp"
}

// MessageReadEvent is published when a read receipt is received
type MessageReadEvent struct {
	DeviceID   string   `json:"device_id"`
	ChatID     string   `json:"chat_id"`
	Sender     string   `json:"sender"`
	MessageIDs []string `json:"message_ids"`
	Timestamp  int64    `json:"timestamp"`
	Type       string   `json:"type"` // receipt type
}

// Message Sent Events

// MessageTextSentEvent is published when a text message is successfully sent
type MessageTextSentEvent struct {
	DeviceID  string `json:"device_id"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Source    string `json:"source"` // "http"
}

// MessageTextFailedEvent is published when sending a text message fails
type MessageTextFailedEvent struct {
	DeviceID string `json:"device_id"`
	ChatID   string `json:"chat_id"`
	Error    string `json:"error"`
}

// MessageImageSentEvent is published when an image message is successfully sent
type MessageImageSentEvent struct {
	DeviceID  string `json:"device_id"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Source    string `json:"source"` // "http"
}

// MessageImageFailedEvent is published when sending an image message fails
type MessageImageFailedEvent struct {
	DeviceID string `json:"device_id"`
	ChatID   string `json:"chat_id"`
	Error    string `json:"error"`
}

// MessageFileSentEvent is published when a file message is successfully sent
type MessageFileSentEvent struct {
	DeviceID  string `json:"device_id"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Source    string `json:"source"` // "http"
}

// MessageFileFailedEvent is published when sending a file message fails
type MessageFileFailedEvent struct {
	DeviceID string `json:"device_id"`
	ChatID   string `json:"chat_id"`
	Error    string `json:"error"`
}

// Presence Events

// MessagePresenceSentEvent is published when presence (typing indicator) is sent
type MessagePresenceSentEvent struct {
	DeviceID string `json:"device_id"`
	ChatID   string `json:"chat_id"`
}

// MessagePresenceFailedEvent is published when sending presence fails
type MessagePresenceFailedEvent struct {
	DeviceID string `json:"device_id"`
	ChatID   string `json:"chat_id"`
	Error    string `json:"error"`
}

// Reaction Events

// MessageReactionSentEvent is published when a reaction is successfully sent
type MessageReactionSentEvent struct {
	DeviceID        string `json:"device_id"`
	ChatID          string `json:"chat_id"`
	TargetMessage   string `json:"target_message"`   // The message being reacted to
	ReactionMessage string `json:"reaction_message"` // The reaction message ID
}

// MessageReactionFailedEvent is published when sending a reaction fails
type MessageReactionFailedEvent struct {
	DeviceID  string `json:"device_id"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Error     string `json:"error"`
}
