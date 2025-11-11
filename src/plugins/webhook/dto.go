package webhook

// WebhookPayload is the root structure for all webhook deliveries
type WebhookPayload struct {
	// Event name (e.g., "device.created", "message.received")
	Event string `json:"event"`
	// Payload contains the event-specific data
	Payload interface{} `json:"payload"`
	// Timestamp when the webhook was created (Unix timestamp)
	Timestamp int64 `json:"timestamp"`
}

// Device Lifecycle Event DTOs

// DeviceCreatedPayload is sent when a new device is created and awaiting QR scan
type DeviceCreatedPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// Status indicates the current state of the device ("pending")
	Status string `json:"status"`
}

// DeviceQRReadyPayload is sent when QR code is generated and ready for scanning
type DeviceQRReadyPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// Status indicates the current state of the device ("qr_ready")
	Status string `json:"status"`
}

// DeviceConnectedPayload is sent when device successfully connects to WhatsApp
type DeviceConnectedPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// DeviceJID is the WhatsApp JID (phone number identifier) for this device
	DeviceJID string `json:"device_jid,omitempty"`
	// Status indicates the current state of the device ("connected")
	Status string `json:"status"`
}

// DeviceDisconnectedPayload is sent when device loses connection to WhatsApp
type DeviceDisconnectedPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// Reason explains why the device disconnected (e.g., "connection_lost")
	Reason string `json:"reason"`
}

// DeviceLogoutPayload is sent when device is logged out from WhatsApp
type DeviceLogoutPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// Reason explains why the device was logged out
	Reason string `json:"reason"`
}

// DeviceRestoredPayload is sent when device is successfully restored from database
type DeviceRestoredPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// DeviceJID is the WhatsApp JID (phone number identifier) for this device
	DeviceJID string `json:"device_jid"`
	// Status indicates the current state of the device ("connected")
	Status string `json:"status"`
}

// DeviceRestoreFailedPayload is sent when device restoration fails
type DeviceRestoreFailedPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// Error message explaining why restoration failed
	Error string `json:"error"`
}

// Message Event DTOs

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
	// MediaURL is the S3 URL where the image is stored (if S3 is enabled)
	MediaURL string `json:"media_url,omitempty"`
}

// VideoContent contains video message metadata
type VideoContent struct {
	// URL is the WhatsApp media URL for the video
	URL string `json:"url,omitempty"`
	// Mimetype is the video MIME type (e.g., "video/mp4")
	Mimetype string `json:"mimetype"`
	// Caption is the optional video caption
	Caption string `json:"caption,omitempty"`
	// MediaURL is the S3 URL where the video is stored (if S3 is enabled)
	MediaURL string `json:"media_url,omitempty"`
}

// AudioContent contains audio message metadata
type AudioContent struct {
	// URL is the WhatsApp media URL for the audio
	URL string `json:"url,omitempty"`
	// Mimetype is the audio MIME type (e.g., "audio/ogg")
	Mimetype string `json:"mimetype"`
	// MediaURL is the S3 URL where the audio is stored (if S3 is enabled)
	MediaURL string `json:"media_url,omitempty"`
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
	// MediaURL is the S3 URL where the document is stored (if S3 is enabled)
	MediaURL string `json:"media_url,omitempty"`
}

// MessageContent represents the content of a WhatsApp message
type MessageContent struct {
	// MessageID is the unique WhatsApp message identifier
	MessageID string `json:"message_id"`
	// ChatID is the WhatsApp chat identifier (phone number or group ID)
	ChatID string `json:"chat_id"`
	// Sender is the WhatsApp JID of the message sender
	Sender string `json:"sender"`
	// Timestamp when the message was sent (Unix timestamp)
	Timestamp int64 `json:"timestamp"`
	// FromMe indicates if the message was sent by this device
	FromMe bool `json:"from_me"`
	// Type indicates the message type: "text", "image", "video", "audio", "document", "unknown"
	Type string `json:"type"`
	// Text contains text message content (only present if Type is "text")
	Text *TextContent `json:"text,omitempty"`
	// Image contains image message metadata (only present if Type is "image")
	Image *ImageContent `json:"image,omitempty"`
	// Video contains video message metadata (only present if Type is "video")
	Video *VideoContent `json:"video,omitempty"`
	// Audio contains audio message metadata (only present if Type is "audio")
	Audio *AudioContent `json:"audio,omitempty"`
	// Document contains document message metadata (only present if Type is "document")
	Document *DocumentContent `json:"document,omitempty"`
	// HasMedia indicates if this message contains media content
	HasMedia bool `json:"has_media,omitempty"`
}

// MessageReceivedPayload is sent when a message is received from WhatsApp
type MessageReceivedPayload struct {
	// DeviceID is the unique identifier for the device that received the message
	DeviceID string `json:"device_id"`
	// Message contains the full message content and metadata
	Message MessageContent `json:"message"`
	// Source indicates where the message came from ("whatsapp")
	Source string `json:"source"`
}

// MessageReadPayload is sent when a read receipt is received
type MessageReadPayload struct {
	// DeviceID is the unique identifier for this device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier
	ChatID string `json:"chat_id"`
	// Sender is the WhatsApp JID that sent the receipt
	Sender string `json:"sender"`
	// MessageIDs is the list of message IDs that were read
	MessageIDs []string `json:"message_ids"`
	// Timestamp when the receipt was sent (Unix timestamp)
	Timestamp int64 `json:"timestamp"`
	// Type is the receipt type
	Type string `json:"type"`
}

// Message Sent Event DTOs

// MessageTextSentPayload is sent when a text message is successfully sent
type MessageTextSentPayload struct {
	// DeviceID is the unique identifier for the device that sent the message
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where the message was sent
	ChatID string `json:"chat_id"`
	// MessageID is the unique WhatsApp message identifier
	MessageID string `json:"message_id"`
	// Source indicates where the send request originated ("http")
	Source string `json:"source"`
}

// MessageTextFailedPayload is sent when sending a text message fails
type MessageTextFailedPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where the message was attempted
	ChatID string `json:"chat_id"`
	// Error message explaining why the send failed
	Error string `json:"error"`
}

// MessageImageSentPayload is sent when an image message is successfully sent
type MessageImageSentPayload struct {
	// DeviceID is the unique identifier for the device that sent the message
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where the image was sent
	ChatID string `json:"chat_id"`
	// MessageID is the unique WhatsApp message identifier
	MessageID string `json:"message_id"`
	// Source indicates where the send request originated ("http")
	Source string `json:"source"`
}

// MessageImageFailedPayload is sent when sending an image message fails
type MessageImageFailedPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where the image was attempted
	ChatID string `json:"chat_id"`
	// Error message explaining why the send failed
	Error string `json:"error"`
}

// MessageFileSentPayload is sent when a file/document message is successfully sent
type MessageFileSentPayload struct {
	// DeviceID is the unique identifier for the device that sent the message
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where the file was sent
	ChatID string `json:"chat_id"`
	// MessageID is the unique WhatsApp message identifier
	MessageID string `json:"message_id"`
	// Source indicates where the send request originated ("http")
	Source string `json:"source"`
}

// MessageFileFailedPayload is sent when sending a file/document message fails
type MessageFileFailedPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where the file was attempted
	ChatID string `json:"chat_id"`
	// Error message explaining why the send failed
	Error string `json:"error"`
}

// Presence Event DTOs

// MessagePresenceSentPayload is sent when a presence update (typing indicator) is sent
type MessagePresenceSentPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where presence was sent
	ChatID string `json:"chat_id"`
}

// MessagePresenceFailedPayload is sent when sending presence update fails
type MessagePresenceFailedPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier where presence was attempted
	ChatID string `json:"chat_id"`
	// Error message explaining why the send failed
	Error string `json:"error"`
}

// Reaction Event DTOs

// MessageReactionSentPayload is sent when a reaction is successfully sent to a message
type MessageReactionSentPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier
	ChatID string `json:"chat_id"`
	// TargetMessage is the message ID being reacted to
	TargetMessage string `json:"target_message"`
	// ReactionMessage is the message ID of the reaction itself
	ReactionMessage string `json:"reaction_message"`
}

// MessageReactionFailedPayload is sent when sending a reaction fails
type MessageReactionFailedPayload struct {
	// DeviceID is the unique identifier for the device
	DeviceID string `json:"device_id"`
	// ChatID is the WhatsApp chat identifier
	ChatID string `json:"chat_id"`
	// MessageID is the message that was attempted to be reacted to
	MessageID string `json:"message_id"`
	// Error message explaining why the reaction failed
	Error string `json:"error"`
}
