package whatsapp

import (
	"testing"
)

// TestDetectMimetypeJPEG tests JPEG detection
func TestDetectMimetypeJPEG(t *testing.T) {
	// JPEG magic bytes: FF D8 FF
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	mimetype := detectMimetype(jpegData)
	if mimetype != "image/jpeg" {
		t.Errorf("Expected image/jpeg, got %s", mimetype)
	}
}

// TestDetectMimetypePNG tests PNG detection
func TestDetectMimetypePNG(t *testing.T) {
	// PNG magic bytes: 89 50 4E 47 0D 0A 1A 0A
	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	mimetype := detectMimetype(pngData)
	if mimetype != "image/png" {
		t.Errorf("Expected image/png, got %s", mimetype)
	}
}

// TestDetectMimetypeGIF tests GIF detection
func TestDetectMimetypeGIF(t *testing.T) {
	// GIF magic bytes: 47 49 46 38 (GIF8)
	gifData := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	mimetype := detectMimetype(gifData)
	if mimetype != "image/gif" {
		t.Errorf("Expected image/gif, got %s", mimetype)
	}
}

// TestDetectMimetypeWebP tests WebP detection
func TestDetectMimetypeWebP(t *testing.T) {
	// WebP magic bytes: RIFF....WEBP
	webpData := []byte{0x52, 0x49, 0x46, 0x46, 0x00, 0x00, 0x00, 0x00, 0x57, 0x45, 0x42, 0x50}
	mimetype := detectMimetype(webpData)
	if mimetype != "image/webp" {
		t.Errorf("Expected image/webp, got %s", mimetype)
	}
}

// TestDetectMimetypeMP4 tests MP4 detection
func TestDetectMimetypeMP4(t *testing.T) {
	// MP4 magic bytes: contains 'ftyp' box
	mp4Data := []byte{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6F, 0x6D}
	mimetype := detectMimetype(mp4Data)
	if mimetype != "video/mp4" {
		t.Errorf("Expected video/mp4, got %s", mimetype)
	}
}

// TestDetectMimetypePDF tests PDF detection
func TestDetectMimetypePDF(t *testing.T) {
	// PDF magic bytes: %PDF
	pdfData := []byte{0x25, 0x50, 0x44, 0x46, 0x2D, 0x31, 0x2E, 0x34, 0x0A, 0x00, 0x00, 0x00}
	mimetype := detectMimetype(pdfData)
	if mimetype != "application/pdf" {
		t.Errorf("Expected application/pdf, got %s", mimetype)
	}
}

// TestDetectMimetypeOGG tests OGG detection
func TestDetectMimetypeOGG(t *testing.T) {
	// OGG magic bytes: OggS
	oggData := []byte{0x4F, 0x67, 0x67, 0x53, 0x00, 0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	mimetype := detectMimetype(oggData)
	if mimetype != "audio/ogg" {
		t.Errorf("Expected audio/ogg, got %s", mimetype)
	}
}

// TestDetectMimetypeMP3WithID3 tests MP3 with ID3 header
func TestDetectMimetypeMP3WithID3(t *testing.T) {
	// MP3 with ID3 header: ID3
	mp3Data := []byte{0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	mimetype := detectMimetype(mp3Data)
	if mimetype != "audio/mpeg" {
		t.Errorf("Expected audio/mpeg, got %s", mimetype)
	}
}

// TestDetectMimetypeMP3WithFrameSync tests MP3 with frame sync
func TestDetectMimetypeMP3WithFrameSync(t *testing.T) {
	// MP3 frame sync: FF FB or FF F3
	mp3Data := []byte{0xFF, 0xFB, 0x90, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	mimetype := detectMimetype(mp3Data)
	if mimetype != "audio/mpeg" {
		t.Errorf("Expected audio/mpeg, got %s", mimetype)
	}
}

// TestDetectMimetypeUnknown tests unknown file types
func TestDetectMimetypeUnknown(t *testing.T) {
	// Random data
	unknownData := []byte{0x12, 0x34, 0x56, 0x78, 0x9A, 0xBC, 0xDE, 0xF0, 0x11, 0x22, 0x33, 0x44}
	mimetype := detectMimetype(unknownData)
	if mimetype != "application/octet-stream" {
		t.Errorf("Expected application/octet-stream, got %s", mimetype)
	}
}

// TestDetectMimetypeTooSmall tests data that's too small
func TestDetectMimetypeTooSmall(t *testing.T) {
	smallData := []byte{0xFF, 0xD8}
	mimetype := detectMimetype(smallData)
	if mimetype != "application/octet-stream" {
		t.Errorf("Expected application/octet-stream for small data, got %s", mimetype)
	}
}

// TestGetExtensionFromMimetypeImages tests image extensions
func TestGetExtensionFromMimetypeImages(t *testing.T) {
	tests := []struct {
		mimetype  string
		extension string
	}{
		{"image/jpeg", ".jpg"},
		{"image/png", ".png"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
	}

	for _, tt := range tests {
		ext := getExtensionFromMimetype(tt.mimetype)
		if ext != tt.extension {
			t.Errorf("getExtensionFromMimetype(%s) = %s, want %s", tt.mimetype, ext, tt.extension)
		}
	}
}

// TestGetExtensionFromMimetypeVideos tests video extensions
func TestGetExtensionFromMimetypeVideos(t *testing.T) {
	tests := []struct {
		mimetype  string
		extension string
	}{
		{"video/mp4", ".mp4"},
		{"video/3gpp", ".3gp"},
	}

	for _, tt := range tests {
		ext := getExtensionFromMimetype(tt.mimetype)
		if ext != tt.extension {
			t.Errorf("getExtensionFromMimetype(%s) = %s, want %s", tt.mimetype, ext, tt.extension)
		}
	}
}

// TestGetExtensionFromMimetypeAudio tests audio extensions
func TestGetExtensionFromMimetypeAudio(t *testing.T) {
	tests := []struct {
		mimetype  string
		extension string
	}{
		{"audio/ogg", ".ogg"},
		{"audio/mpeg", ".mp3"},
		{"audio/mp4", ".m4a"},
		{"audio/aac", ".aac"},
	}

	for _, tt := range tests {
		ext := getExtensionFromMimetype(tt.mimetype)
		if ext != tt.extension {
			t.Errorf("getExtensionFromMimetype(%s) = %s, want %s", tt.mimetype, ext, tt.extension)
		}
	}
}

// TestGetExtensionFromMimetypeDocuments tests document extensions
func TestGetExtensionFromMimetypeDocuments(t *testing.T) {
	tests := []struct {
		mimetype  string
		extension string
	}{
		{"application/pdf", ".pdf"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", ".docx"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", ".xlsx"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", ".pptx"},
		{"application/msword", ".doc"},
		{"application/vnd.ms-excel", ".xls"},
		{"application/vnd.ms-powerpoint", ".ppt"},
		{"application/zip", ".zip"},
		{"application/x-rar-compressed", ".rar"},
		{"text/plain", ".txt"},
	}

	for _, tt := range tests {
		ext := getExtensionFromMimetype(tt.mimetype)
		if ext != tt.extension {
			t.Errorf("getExtensionFromMimetype(%s) = %s, want %s", tt.mimetype, ext, tt.extension)
		}
	}
}

// TestGetExtensionFromMimetypeUnknown tests unknown mimetype
func TestGetExtensionFromMimetypeUnknown(t *testing.T) {
	ext := getExtensionFromMimetype("application/unknown")
	if ext != ".bin" {
		t.Errorf("Expected .bin for unknown mimetype, got %s", ext)
	}
}

// TestMaxMediaSizeConstant tests that the constant is set correctly
func TestMaxMediaSizeConstant(t *testing.T) {
	expected := int64(10 * 1024 * 1024) // 10MB
	if MaxMediaSize != expected {
		t.Errorf("MaxMediaSize = %d, want %d", MaxMediaSize, expected)
	}
}

// TestDetectMimetypeConsistency tests that detection is consistent
func TestDetectMimetypeConsistency(t *testing.T) {
	jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}

	// Call multiple times
	result1 := detectMimetype(jpegData)
	result2 := detectMimetype(jpegData)
	result3 := detectMimetype(jpegData)

	if result1 != result2 || result2 != result3 {
		t.Errorf("Inconsistent detection: %s, %s, %s", result1, result2, result3)
	}
}

// TestGetExtensionPrefixesDot tests that all extensions start with dot
func TestGetExtensionPrefixesDot(t *testing.T) {
	mimetypes := []string{
		"image/jpeg",
		"video/mp4",
		"audio/ogg",
		"application/pdf",
		"application/unknown",
	}

	for _, mimetype := range mimetypes {
		ext := getExtensionFromMimetype(mimetype)
		if len(ext) == 0 || ext[0] != '.' {
			t.Errorf("Extension for %s doesn't start with dot: %s", mimetype, ext)
		}
	}
}

// TestDetectMimetypeMP4Variations tests different MP4 variants
func TestDetectMimetypeMP4Variations(t *testing.T) {
	// ftyp at different positions (within first 8 bytes)
	variations := [][]byte{
		{0x00, 0x00, 0x00, 0x20, 0x66, 0x74, 0x79, 0x70, 0x69, 0x73, 0x6F, 0x6D}, // Position 4
		{0x00, 0x00, 0x00, 0x18, 0x66, 0x74, 0x79, 0x70, 0x4D, 0x34, 0x56, 0x20}, // Position 4
		{0x66, 0x74, 0x79, 0x70, 0x33, 0x67, 0x70, 0x35, 0x00, 0x00, 0x00, 0x00}, // Position 0
	}

	for i, data := range variations {
		mimetype := detectMimetype(data)
		if mimetype != "video/mp4" {
			t.Errorf("Variation %d: expected video/mp4, got %s", i, mimetype)
		}
	}
}
