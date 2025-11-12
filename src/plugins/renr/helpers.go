package renr

import (
	"strings"

	waLog "go.mau.fi/whatsmeow/util/log"
)

// formatPhoneToJID converts phone number to WhatsApp JID
// Supports both individual and group chats
func formatPhoneToJID(phone string) string {
	// Remove any non-numeric characters except @
	if strings.Contains(phone, "@") {
		return phone // Already formatted
	}

	// Individual chat: phone@s.whatsapp.net
	// Group chat: phone@g.us
	// For now, assume individual chat
	return phone + "@s.whatsapp.net"
}

// getStringValue safely returns string value from pointer
func getStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// extractPhoneFromJID extracts phone number from WhatsApp JID
func extractPhoneFromJID(jid string) string {
	// JID format: "6281234567890@s.whatsapp.net"
	parts := strings.Split(jid, "@")
	if len(parts) > 0 {
		return parts[0]
	}
	return jid
}

// extractNameFromSender extracts name from sender JID
// Returns phone if name not available
func extractNameFromSender(sender string) string {
	return extractPhoneFromJID(sender)
}

// getExtension returns file extension from mime type
func getExtension(mimeType string) string {
	// Simple mime type to extension mapping
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "video/mp4":
		return "mp4"
	case "video/3gpp":
		return "3gp"
	case "audio/ogg":
		return "ogg"
	case "audio/mpeg":
		return "mp3"
	case "audio/mp4":
		return "m4a"
	case "application/pdf":
		return "pdf"
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return "docx"
	case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return "xlsx"
	default:
		return "bin"
	}
}

// queueLoggerAdapter adapts waLog.Logger to renrqueue.Logger
type queueLoggerAdapter struct {
	waLogger waLog.Logger
}

func (l *queueLoggerAdapter) Printf(format string, v ...interface{}) {
	l.waLogger.Infof(format, v...)
}

func (l *queueLoggerAdapter) Errorf(format string, v ...interface{}) {
	l.waLogger.Errorf(format, v...)
}

func (l *queueLoggerAdapter) Infof(format string, v ...interface{}) {
	l.waLogger.Infof(format, v...)
}
