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
// Handles multiple formats:
// - Individual: "6281234567890@s.whatsapp.net"
// - Group: "120363423115728010@g.us"
// - LID (Linked Device): "219408454660313:46@lid"
func extractPhoneFromJID(jid string) string {
	// Split by @ to get the identifier part
	parts := strings.Split(jid, "@")
	if len(parts) == 0 {
		return jid
	}

	phone := parts[0]

	// Remove LID suffix (everything after colon) if present
	// Example: "219408454660313:46" -> "219408454660313"
	if strings.Contains(phone, ":") {
		phone = strings.Split(phone, ":")[0]
	}

	return phone
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

// getStickerFilename determines the appropriate filename for stickers
// based on MIME type and original filename
// - If MIME type indicates a standard image format (webp, png, jpeg, gif): keep original extension
// - If MIME type is binary/octet-stream or file extension is .bin: use .sticker extension
// This handles animated stickers which are often zip archives disguised as .bin files
func getStickerFilename(originalFilename, mimetype string) string {
	// Check MIME type first
	switch mimetype {
	case "image/webp", "image/png", "image/jpeg", "image/gif":
		// Standard image formats - use original filename
		return originalFilename
	case "application/octet-stream", "application/zip", "application/x-zip-compressed":
		// Binary or zip - use .sticker extension
		return replaceExtension(originalFilename, "sticker")
	default:
		// Check if original file has .bin extension
		if strings.HasSuffix(strings.ToLower(originalFilename), ".bin") {
			return replaceExtension(originalFilename, "sticker")
		}
		// Keep original filename for other cases
		return originalFilename
	}
}

// replaceExtension replaces file extension with new extension
// Example: replaceExtension("file.bin", "sticker") -> "file.sticker"
func replaceExtension(filename, newExt string) string {
	// Find last dot
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		// No extension, just append
		return filename + "." + newExt
	}
	// Replace extension
	return filename[:lastDot+1] + newExt
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
