package whatsapp

import (
	"encoding/json"
	"testing"
)

// TestMediaDataJSONMarshaling tests JSON marshaling/unmarshaling of MediaData
func TestMediaDataJSONMarshaling(t *testing.T) {
	original := &MediaData{
		Data:        []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG magic bytes
		Mimetype:    "image/jpeg",
		Filename:    "test.jpg",
		Size:        1024,
		SHA256:      "abc123def456",
		WhatsAppURL: "https://example.com/media/test",
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal MediaData: %v", err)
	}

	// Unmarshal back
	var decoded MediaData
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal MediaData: %v", err)
	}

	// Verify fields
	if decoded.Mimetype != original.Mimetype {
		t.Errorf("Mimetype mismatch: got %s, want %s", decoded.Mimetype, original.Mimetype)
	}
	if decoded.Filename != original.Filename {
		t.Errorf("Filename mismatch: got %s, want %s", decoded.Filename, original.Filename)
	}
	if decoded.Size != original.Size {
		t.Errorf("Size mismatch: got %d, want %d", decoded.Size, original.Size)
	}
	if decoded.SHA256 != original.SHA256 {
		t.Errorf("SHA256 mismatch: got %s, want %s", decoded.SHA256, original.SHA256)
	}
	if decoded.WhatsAppURL != original.WhatsAppURL {
		t.Errorf("WhatsAppURL mismatch: got %s, want %s", decoded.WhatsAppURL, original.WhatsAppURL)
	}
}

// TestImageContentWithMedia tests ImageContent with Media field
func TestImageContentWithMedia(t *testing.T) {
	mediaData := &MediaData{
		Data:        []byte{0xFF, 0xD8, 0xFF},
		Mimetype:    "image/jpeg",
		Filename:    "photo.jpg",
		Size:        2048,
		SHA256:      "hash123",
		WhatsAppURL: "https://wa.me/media/test",
	}

	imageContent := &ImageContent{
		Media:    mediaData,
		URL:      "https://wa.me/media/test", // Deprecated
		Mimetype: "image/jpeg",
		Caption:  "Test image",
	}

	// Marshal to JSON
	data, err := json.Marshal(imageContent)
	if err != nil {
		t.Fatalf("Failed to marshal ImageContent: %v", err)
	}

	// Unmarshal back
	var decoded ImageContent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal ImageContent: %v", err)
	}

	// Verify Media field is populated
	if decoded.Media == nil {
		t.Fatal("Media field is nil after unmarshaling")
	}
	if decoded.Media.Mimetype != "image/jpeg" {
		t.Errorf("Media mimetype mismatch: got %s, want image/jpeg", decoded.Media.Mimetype)
	}
	if decoded.Media.Size != 2048 {
		t.Errorf("Media size mismatch: got %d, want 2048", decoded.Media.Size)
	}

	// Verify backward compatibility fields
	if decoded.URL != imageContent.URL {
		t.Errorf("URL mismatch: got %s, want %s", decoded.URL, imageContent.URL)
	}
	if decoded.Caption != "Test image" {
		t.Errorf("Caption mismatch: got %s, want 'Test image'", decoded.Caption)
	}
}

// TestVideoContentWithMedia tests VideoContent with Media field
func TestVideoContentWithMedia(t *testing.T) {
	mediaData := &MediaData{
		Data:     []byte{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70}, // MP4 magic
		Mimetype: "video/mp4",
		Filename: "video.mp4",
		Size:     5120,
		SHA256:   "videohash",
	}

	videoContent := &VideoContent{
		Media:    mediaData,
		Mimetype: "video/mp4",
		Caption:  "Test video",
	}

	// Marshal and unmarshal
	data, err := json.Marshal(videoContent)
	if err != nil {
		t.Fatalf("Failed to marshal VideoContent: %v", err)
	}

	var decoded VideoContent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal VideoContent: %v", err)
	}

	if decoded.Media == nil {
		t.Fatal("Media field is nil")
	}
	if decoded.Media.Filename != "video.mp4" {
		t.Errorf("Filename mismatch: got %s, want video.mp4", decoded.Media.Filename)
	}
}

// TestAudioContentWithMedia tests AudioContent with Media field
func TestAudioContentWithMedia(t *testing.T) {
	mediaData := &MediaData{
		Data:     []byte{0x4F, 0x67, 0x67, 0x53}, // OGG magic
		Mimetype: "audio/ogg",
		Filename: "voice.ogg",
		Size:     3072,
		SHA256:   "audiohash",
	}

	audioContent := &AudioContent{
		Media:    mediaData,
		Mimetype: "audio/ogg",
	}

	data, err := json.Marshal(audioContent)
	if err != nil {
		t.Fatalf("Failed to marshal AudioContent: %v", err)
	}

	var decoded AudioContent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal AudioContent: %v", err)
	}

	if decoded.Media == nil {
		t.Fatal("Media field is nil")
	}
	if decoded.Media.Mimetype != "audio/ogg" {
		t.Errorf("Mimetype mismatch: got %s, want audio/ogg", decoded.Media.Mimetype)
	}
}

// TestDocumentContentWithMedia tests DocumentContent with Media field
func TestDocumentContentWithMedia(t *testing.T) {
	mediaData := &MediaData{
		Data:     []byte{0x25, 0x50, 0x44, 0x46}, // PDF magic
		Mimetype: "application/pdf",
		Filename: "document.pdf",
		Size:     10240,
		SHA256:   "dochash",
	}

	docContent := &DocumentContent{
		Media:    mediaData,
		Mimetype: "application/pdf",
		Filename: "document.pdf",
		Caption:  "Important document",
	}

	data, err := json.Marshal(docContent)
	if err != nil {
		t.Fatalf("Failed to marshal DocumentContent: %v", err)
	}

	var decoded DocumentContent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal DocumentContent: %v", err)
	}

	if decoded.Media == nil {
		t.Fatal("Media field is nil")
	}
	if decoded.Media.Filename != "document.pdf" {
		t.Errorf("Filename mismatch: got %s, want document.pdf", decoded.Media.Filename)
	}
}

// TestStickerContentWithMedia tests StickerContent with Media field
func TestStickerContentWithMedia(t *testing.T) {
	mediaData := &MediaData{
		Data:     []byte{0x52, 0x49, 0x46, 0x46}, // RIFF (WebP starts with RIFF)
		Mimetype: "image/webp",
		Filename: "sticker.webp",
		Size:     512,
		SHA256:   "stickerhash",
	}

	stickerContent := &StickerContent{
		Media:    mediaData,
		Mimetype: "image/webp",
	}

	data, err := json.Marshal(stickerContent)
	if err != nil {
		t.Fatalf("Failed to marshal StickerContent: %v", err)
	}

	var decoded StickerContent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal StickerContent: %v", err)
	}

	if decoded.Media == nil {
		t.Fatal("Media field is nil")
	}
	if decoded.Media.Mimetype != "image/webp" {
		t.Errorf("Mimetype mismatch: got %s, want image/webp", decoded.Media.Mimetype)
	}
}

// TestMessagePayloadWithMediaContent tests MessagePayload with media content
func TestMessagePayloadWithMediaContent(t *testing.T) {
	mediaData := &MediaData{
		Data:     []byte{0xFF, 0xD8, 0xFF},
		Mimetype: "image/jpeg",
		Filename: "test.jpg",
		Size:     1024,
		SHA256:   "hash",
	}

	payload := MessagePayload{
		MessageID: "msg123",
		ChatID:    "12345@s.whatsapp.net",
		Type:      "image",
		Timestamp: 1699999999000,
		HasMedia:  true,
		Image: &ImageContent{
			Media:    mediaData,
			Mimetype: "image/jpeg",
			Caption:  "Test",
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal MessagePayload: %v", err)
	}

	var decoded MessagePayload
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal MessagePayload: %v", err)
	}

	if decoded.Type != "image" {
		t.Errorf("Type mismatch: got %s, want image", decoded.Type)
	}
	if !decoded.HasMedia {
		t.Error("HasMedia should be true")
	}
	if decoded.Image == nil {
		t.Fatal("Image field is nil")
	}
	if decoded.Image.Media == nil {
		t.Fatal("Image.Media field is nil")
	}
	if decoded.Image.Media.Size != 1024 {
		t.Errorf("Media size mismatch: got %d, want 1024", decoded.Image.Media.Size)
	}
}

// TestMessageMediaDownloadFailedEvent tests the download failure event
func TestMessageMediaDownloadFailedEvent(t *testing.T) {
	event := MessageMediaDownloadFailedEvent{
		DeviceID:  "device123",
		MessageID: "msg456",
		ChatID:    "chat789@s.whatsapp.net",
		MediaType: "image",
		Error:     "network timeout",
		Timestamp: 1699999999000,
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal event: %v", err)
	}

	var decoded MessageMediaDownloadFailedEvent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal event: %v", err)
	}

	if decoded.DeviceID != "device123" {
		t.Errorf("DeviceID mismatch: got %s, want device123", decoded.DeviceID)
	}
	if decoded.MediaType != "image" {
		t.Errorf("MediaType mismatch: got %s, want image", decoded.MediaType)
	}
	if decoded.Error != "network timeout" {
		t.Errorf("Error mismatch: got %s, want 'network timeout'", decoded.Error)
	}
}

// TestBackwardCompatibility tests that old consumers using only URL field still work
func TestBackwardCompatibility(t *testing.T) {
	// Simulate old-style ImageContent (only URL, no Media)
	oldStyle := &ImageContent{
		URL:      "https://wa.me/media/old",
		Mimetype: "image/jpeg",
		Caption:  "Old style",
	}

	data, err := json.Marshal(oldStyle)
	if err != nil {
		t.Fatalf("Failed to marshal old-style content: %v", err)
	}

	var decoded ImageContent
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal old-style content: %v", err)
	}

	// Old consumers should still be able to read URL field
	if decoded.URL != "https://wa.me/media/old" {
		t.Errorf("URL not preserved: got %s", decoded.URL)
	}
	if decoded.Caption != "Old style" {
		t.Errorf("Caption not preserved: got %s", decoded.Caption)
	}
	// Media field should be nil for old-style messages
	if decoded.Media != nil {
		t.Error("Media field should be nil for old-style messages")
	}
}
