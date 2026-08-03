package whatsapp

import "go.mau.fi/whatsmeow/types"

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
	DeviceID    string `json:"device_id"`
	PhoneNumber string `json:"phone_number"`
	DeviceLID   string `json:"device_lid"`
	DeviceJID   string `json:"device_jid,omitempty"`
	Status      string `json:"status"` // "connected"
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

// MediaData contains downloaded media content from WhatsApp
// This struct provides the actual media bytes along with metadata,
// enabling plugins to handle media without downloading from WhatsApp URLs
type MediaData struct {
	// Data is the raw bytes of the media file
	Data []byte `json:"data,omitempty"`
	// Mimetype is the detected MIME type (e.g., "image/jpeg", "video/mp4")
	Mimetype string `json:"mimetype"`
	// Filename is the suggested filename with extension
	Filename string `json:"filename"`
	// Size is the size of the media in bytes
	Size int64 `json:"size"`
	// SHA256 is the hex-encoded SHA256 hash of the media data
	SHA256 string `json:"sha256"`
	// WhatsAppURL is the original WhatsApp media URL (DEPRECATED - use Data field instead)
	WhatsAppURL string `json:"whatsapp_url,omitempty"`
}

// TextContent contains the text message content
type TextContent struct {
	// Content is the actual text message
	Content string `json:"content"`
}

// ImageContent contains image message metadata
type ImageContent struct {
	// Media contains the downloaded image data and metadata (preferred)
	Media *MediaData `json:"media,omitempty"`
	// URL is the WhatsApp media URL for the image (DEPRECATED - use Media field instead)
	URL string `json:"url,omitempty"`
	// Mimetype is the image MIME type (e.g., "image/jpeg")
	Mimetype string `json:"mimetype"`
	// Caption is the optional image caption
	Caption string `json:"caption,omitempty"`
}

// VideoContent contains video message metadata
type VideoContent struct {
	// Media contains the downloaded video data and metadata (preferred)
	Media *MediaData `json:"media,omitempty"`
	// URL is the WhatsApp media URL for the video (DEPRECATED - use Media field instead)
	URL string `json:"url,omitempty"`
	// Mimetype is the video MIME type (e.g., "video/mp4")
	Mimetype string `json:"mimetype"`
	// Caption is the optional video caption
	Caption string `json:"caption,omitempty"`
}

// AudioContent contains audio message metadata
type AudioContent struct {
	// Media contains the downloaded audio data and metadata (preferred)
	Media *MediaData `json:"media,omitempty"`
	// URL is the WhatsApp media URL for the audio (DEPRECATED - use Media field instead)
	URL string `json:"url,omitempty"`
	// Mimetype is the audio MIME type (e.g., "audio/ogg")
	Mimetype string `json:"mimetype"`
}

// DocumentContent contains document message metadata
type DocumentContent struct {
	// Media contains the downloaded document data and metadata (preferred)
	Media *MediaData `json:"media,omitempty"`
	// URL is the WhatsApp media URL for the document (DEPRECATED - use Media field instead)
	URL string `json:"url,omitempty"`
	// Mimetype is the document MIME type (e.g., "application/pdf")
	Mimetype string `json:"mimetype"`
	// Caption is the optional document caption
	Caption string `json:"caption,omitempty"`
	// Filename is the original filename of the document
	Filename string `json:"filename"`
}

// StickerContent contains sticker message metadata
type StickerContent struct {
	// Media contains the downloaded sticker data and metadata (preferred)
	Media *MediaData `json:"media,omitempty"`
	// URL is the WhatsApp media URL for the sticker (DEPRECATED - use Media field instead)
	URL string `json:"url,omitempty"`
	// Mimetype is the sticker MIME type (e.g., "image/webp")
	Mimetype string `json:"mimetype"`
	// Caption is the optional sticker caption (rarely used)
	Caption string `json:"caption,omitempty"`
}

// ContactContent contains contact card information
type ContactContent struct {
	// DisplayName is the contact's display name
	DisplayName string `json:"display_name"`
	// VCard is the full vCard data
	VCard string `json:"vcard"`
}

// ContactsContent contains multiple contacts
type ContactsContent struct {
	// DisplayName is the name shown for the contacts array
	DisplayName string `json:"display_name"`
	// Contacts is an array of vCard strings
	Contacts []string `json:"contacts"`
}

// LocationContent contains location information
type LocationContent struct {
	// Latitude is the location's latitude
	Latitude float64 `json:"latitude"`
	// Longitude is the location's longitude
	Longitude float64 `json:"longitude"`
	// Name is the optional location name
	Name string `json:"name,omitempty"`
	// Address is the optional full address
	Address string `json:"address,omitempty"`
	// URL is the optional map URL
	URL string `json:"url,omitempty"`
	// IsLive indicates if this is a live location
	IsLive bool `json:"is_live"`
}

// ReactionContent contains reaction information
type ReactionContent struct {
	// Emoji is the emoji used for the reaction (empty string means removed)
	Emoji string `json:"emoji"`
	// TargetMessageID is the ID of the message being reacted to
	TargetMessageID string `json:"target_message_id"`
	// TargetSender is the sender of the target message
	TargetSender string `json:"target_sender,omitempty"`
}

// ProtocolContent contains protocol message information (deletions, etc.)
type ProtocolContent struct {
	// Type is the protocol message type ("revoke", "ephemeral_setting", etc.)
	Type string `json:"type"`
	// TargetMessageID is the ID of the affected message (for revoke)
	TargetMessageID string `json:"target_message_id,omitempty"`
}

// PollOption represents a single poll option
type PollOption struct {
	// Name is the option text
	Name string `json:"name"`
}

// PollContent contains poll information
type PollContent struct {
	// Name is the poll question
	Name string `json:"name"`
	// Options are the available poll options
	Options []PollOption `json:"options"`
	// SelectableCount is the maximum number of options that can be selected
	SelectableCount int `json:"selectable_count"`
}

// PollVoteContent contains poll vote information
type PollVoteContent struct {
	// PollMessageID is the ID of the original poll message
	PollMessageID string `json:"poll_message_id"`
	// SelectedOptions are the option names that were selected
	SelectedOptions []string `json:"selected_options"`
}

// QuotedMessage describes the message that a reply is quoting. It is populated
// from waProto.ContextInfo on any message variant that can carry one.
type QuotedMessage struct {
	// MessageID is the WhatsApp stanza ID of the quoted message
	MessageID string `json:"message_id"`
	// Participant is the JID of the quoted message's sender. Required to render
	// group replies, and required to build a valid outbound quote.
	Participant string `json:"participant,omitempty"`
	// ChatID is ContextInfo.RemoteJID, set only for cross-chat quotes such as
	// status replies
	ChatID string `json:"chat_id,omitempty"`
	// FromMe indicates the quoted message was sent by this device
	FromMe bool `json:"from_me"`
	// Type is the quoted message's type, using the same vocabulary as
	// MessagePayload.Type
	Type string `json:"type,omitempty"`
	// Preview is a short text or caption excerpt so consumers can render a
	// quote bubble without a database lookup
	Preview string `json:"preview,omitempty"`
}

// MessagePayload represents the message content structure
type MessagePayload struct {
	MessageID  string           `json:"message_id"`
	ChatID     string           `json:"chat_id"`
	ChatName   string           `json:"chat_name"` // Group name for groups, contact name for DM
	Sender     string           `json:"sender"`
	SenderName string           `json:"sender_name"` // Full name or PushName of sender
	Timestamp  int64            `json:"timestamp"`
	PushName   string           `json:"push_name"` // Original PushName from WhatsApp
	FromMe     bool             `json:"from_me"`
	IsGroup    bool             `json:"is_group"`
	Type       string           `json:"type"` // "text", "image", "video", "audio", "document", "sticker", "contact", "contacts", "location", "live_location", "reaction", "poll", "poll_vote", "protocol", "unknown"
	Text       *TextContent     `json:"text,omitempty"`
	Image      *ImageContent    `json:"image,omitempty"`
	Video      *VideoContent    `json:"video,omitempty"`
	Audio      *AudioContent    `json:"audio,omitempty"`
	Document   *DocumentContent `json:"document,omitempty"`
	Sticker    *StickerContent  `json:"sticker,omitempty"`
	Contact    *ContactContent  `json:"contact,omitempty"`
	Contacts   *ContactsContent `json:"contacts,omitempty"`
	Location   *LocationContent `json:"location,omitempty"`
	Reaction   *ReactionContent `json:"reaction,omitempty"`
	Protocol   *ProtocolContent `json:"protocol,omitempty"`
	Poll       *PollContent     `json:"poll,omitempty"`
	PollVote   *PollVoteContent `json:"poll_vote,omitempty"`
	HasMedia   bool             `json:"has_media,omitempty"`
	// Quoted is set when this message is a reply to another message
	Quoted *QuotedMessage `json:"quoted,omitempty"`
	// IsForwarded reports ContextInfo.IsForwarded
	IsForwarded bool `json:"is_forwarded,omitempty"`
}

// MessageReceivedEvent is published when a message is received from WhatsApp
type MessageReceivedEvent struct {
	DeviceID     string         `json:"device_id"`
	Message      MessagePayload `json:"message"`
	Sender       types.JID      `json:"sender"`
	Conversation types.JID      `json:"conversation"`
	Source       string         `json:"source"` // "whatsapp"
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

// MessageVideoSentEvent is published when a video message is successfully sent
type MessageVideoSentEvent struct {
	DeviceID  string `json:"device_id"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	Source    string `json:"source"` // "http"
}

// MessageVideoFailedEvent is published when sending a video message fails
type MessageVideoFailedEvent struct {
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

// Message History Events

// MessageHistoryRequestedEvent is published when history sync is requested
type MessageHistoryRequestedEvent struct {
	DeviceID        string `json:"device_id"`
	ChatID          string `json:"chat_id"`
	BeforeMessageID string `json:"before_message_id,omitempty"`
	Count           int    `json:"count"`
	RequestID       string `json:"request_id"` // Unique ID to correlate request/response
}

// MessageHistorySyncedEvent is published when history sync completes successfully
type MessageHistorySyncedEvent struct {
	DeviceID  string           `json:"device_id"`
	ChatID    string           `json:"chat_id,omitempty"`
	Messages  []MessagePayload `json:"messages"`
	RequestID string           `json:"request_id"` // Correlate with request
	Count     int              `json:"count"`      // Number of messages synced
}

// MessageHistoryFailedEvent is published when history sync fails
type MessageHistoryFailedEvent struct {
	DeviceID  string `json:"device_id"`
	ChatID    string `json:"chat_id,omitempty"`
	RequestID string `json:"request_id"`
	Error     string `json:"error"`
}

// MessageMediaDownloadFailedEvent is published when media download from WhatsApp fails
type MessageMediaDownloadFailedEvent struct {
	DeviceID  string `json:"device_id"`
	MessageID string `json:"message_id"`
	ChatID    string `json:"chat_id,omitempty"`
	MediaType string `json:"media_type"` // "image", "video", "audio", "document", "sticker"
	Error     string `json:"error"`
	Timestamp int64  `json:"timestamp"`
}
