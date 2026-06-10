package renr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	waTypes "go.mau.fi/whatsmeow/types"
)

// avatarCacheTTL is the duration to cache avatar URLs (1 hour)
const avatarCacheTTL = 1 * time.Hour

// uploadMediaToS3 uploads media bytes to S3 and returns public URL
// This replaces the old downloadMedia() function - now we receive bytes from events
func (p *Plugin) uploadMediaToS3(data []byte, filename, mimetype, deviceID string) string {
	if !p.s3Enabled {
		p.logger.Errorf("⚠️  S3 upload skipped - RENR_PLUGIN_S3_ENABLED=false")
		return ""
	}

	if len(data) == 0 {
		p.logger.Errorf("⚠️  Empty media data, cannot upload")
		return ""
	}

	// Generate S3 key with device ID for organization
	key := fmt.Sprintf("whatsapp/%s/%s", deviceID, filename)

	p.logger.Infof("📤 Uploading to S3: key=%s, size=%d bytes, mimetype=%s",
		key, len(data), mimetype)

	// Upload to S3
	s3URL, err := p.uploadToS3(data, key, mimetype)
	if err != nil {
		p.logger.Errorf("❌ S3 upload failed: %v", err)
		return ""
	}

	p.logger.Infof("✅ S3 upload successful: %s", s3URL)
	return s3URL
}

// uploadToS3 uploads data to S3 using Renr plugin's configuration
// Updated to use RENR_PLUGIN_S3_* env vars and AWS SDK v2
func (p *Plugin) uploadToS3(data []byte, key, contentType string) (string, error) {
	if p.s3Client == nil {
		return "", fmt.Errorf("S3 client not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Calculate SHA256 for integrity
	hash := sha256.Sum256(data)
	sha256Hex := hex.EncodeToString(hash[:])

	// Upload to S3 with public-read ACL
	_, err := p.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(p.s3Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		ACL:         types.ObjectCannedACLPublicRead,
		Metadata: map[string]string{
			"sha256": sha256Hex,
			"source": "whatsapp-gateway-renr",
		},
	})

	if err != nil {
		return "", fmt.Errorf("PutObject failed: %w", err)
	}

	// Construct public URL based on URL style
	var publicURL string
	if p.s3Endpoint != "" {
		// Custom endpoint (MinIO, DigitalOcean Spaces, etc.)
		if p.s3URLStyle == "path" {
			publicURL = fmt.Sprintf("%s/%s/%s", p.s3Endpoint, p.s3Bucket, key)
		} else if p.s3URLStyle == "subdomain" {
			// remove https or http
			var protocol = ""
			if strings.Contains(p.s3Endpoint, "https://") {
				protocol = "https://"
			} else if strings.Contains(p.s3Endpoint, "http://") {
				protocol = "http://"
			}
			var endpoint = strings.Replace(p.s3Endpoint, protocol, "", 1)
			publicURL = fmt.Sprintf("%s%s.%s/%s", protocol, p.s3Bucket, endpoint, key)
		} else {
			publicURL = fmt.Sprintf("%s/%s", p.s3Endpoint, key)
		}
	} else {
		// AWS S3 standard endpoint
		publicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s",
			p.s3Bucket, p.s3Region, key)
	}

	return publicURL, nil
}

// handleMediaLegacy is a fallback for backward compatibility
// Downloads media from WhatsApp URL if Media.Data is not available
// DEPRECATED: Only used during migration period. Will be removed in Phase 3.
func (p *Plugin) handleMediaLegacy(whatsappURL string) string {
	p.logger.Warnf("⚠️  DEPRECATED: Legacy URL download called (Media.Data not available): %s", whatsappURL)

	// This function can be removed after full migration to Media.Data
	// For now, return empty string - plugins should handle missing media gracefully
	return ""
}

// getAvatarURL fetches a user's profile picture from WhatsApp, uploads it to S3, and returns the URL.
// Results are cached in memory for 1 hour to avoid repeated fetches.
// Returns empty string on any error (non-blocking).
//
// Parameters:
//   - deviceID: The device ID to use for fetching
//   - jid: The WhatsApp JID of the user whose avatar to fetch
//
// Cache key format: "deviceID:jid"
func (p *Plugin) getAvatarURL(deviceID string, jid waTypes.JID) string {
	// Skip if S3 is not enabled
	if !p.s3Enabled {
		return ""
	}

	// Create cache key
	cacheKey := fmt.Sprintf("%s:%s", deviceID, jid.String())

	// Check cache first (read lock)
	p.avatarCacheMu.RLock()
	if entry, ok := p.avatarCache[cacheKey]; ok {
		if time.Since(entry.FetchedAt) < avatarCacheTTL {
			p.avatarCacheMu.RUnlock()
			p.logger.Debugf("🖼️  Avatar cache hit: %s -> %s", cacheKey, entry.URL)
			return entry.URL
		}
	}
	p.avatarCacheMu.RUnlock()

	// Cache miss or expired - fetch from WhatsApp
	p.logger.Debugf("🖼️  Avatar cache miss, fetching from WhatsApp: %s", cacheKey)

	// Get WhatsApp client
	client, err := p.manager.GetClient(deviceID)
	if err != nil {
		p.logger.Debugf("⚠️  Cannot get client for avatar fetch: %v", err)
		return ""
	}

	// Get profile picture info from WhatsApp
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pictureInfo, err := client.GetProfilePictureInfo(ctx, jid, nil)
	if err != nil {
		p.logger.Debugf("⚠️  Failed to get profile picture info for %s: %v", jid.String(), err)
		// Cache empty result to avoid repeated failures
		p.cacheAvatarURL(cacheKey, "")
		return ""
	}

	// No profile picture available
	if pictureInfo == nil || pictureInfo.URL == "" {
		p.logger.Debugf("🖼️  No profile picture available for %s", jid.String())
		// Cache empty result
		p.cacheAvatarURL(cacheKey, "")
		return ""
	}

	// Download the profile picture from WhatsApp's URL
	p.logger.Debugf("🖼️  Downloading avatar from WhatsApp: %s", pictureInfo.URL)

	downloadCtx, downloadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer downloadCancel()

	req, err := p.httpClient.Get(pictureInfo.URL)
	if err != nil {
		p.logger.Warnf("⚠️  Failed to create request for avatar download: %v", err)
		p.cacheAvatarURL(cacheKey, "")
		return ""
	}
	defer req.Body.Close()

	if req.StatusCode != 200 {
		p.logger.Warnf("⚠️  Avatar download returned status %d", req.StatusCode)
		p.cacheAvatarURL(cacheKey, "")
		return ""
	}

	// Read the image data
	imageData, err := io.ReadAll(io.LimitReader(req.Body, 5*1024*1024)) // 5MB limit for avatars
	if err != nil {
		p.logger.Warnf("⚠️  Failed to read avatar data: %v", err)
		p.cacheAvatarURL(cacheKey, "")
		return ""
	}

	_ = downloadCtx // Used for documentation, actual timeout is on httpClient

	// Determine filename - use JID user part for organization
	// Format: whatsapp/{deviceID}/avatars/{jid_user}.jpg
	filename := fmt.Sprintf("%s.jpg", jid.User)
	s3Key := fmt.Sprintf("whatsapp/%s/avatars/%s", deviceID, filename)

	// Detect MIME type (most WhatsApp avatars are JPEG)
	mimetype := "image/jpeg"
	if len(imageData) > 4 && imageData[0] == 0x89 && imageData[1] == 0x50 {
		mimetype = "image/png"
		filename = fmt.Sprintf("%s.png", jid.User)
		s3Key = fmt.Sprintf("whatsapp/%s/avatars/%s", deviceID, filename)
	}

	// Upload to S3
	p.logger.Debugf("🖼️  Uploading avatar to S3: %s (size: %d bytes)", s3Key, len(imageData))

	s3URL, err := p.uploadToS3(imageData, s3Key, mimetype)
	if err != nil {
		p.logger.Warnf("⚠️  Failed to upload avatar to S3: %v", err)
		p.cacheAvatarURL(cacheKey, "")
		return ""
	}

	p.logger.Infof("✅ Avatar uploaded successfully: %s -> %s", jid.String(), s3URL)

	// Cache the result
	p.cacheAvatarURL(cacheKey, s3URL)

	return s3URL
}

// cacheAvatarURL stores an avatar URL in the cache
func (p *Plugin) cacheAvatarURL(key, url string) {
	p.avatarCacheMu.Lock()
	defer p.avatarCacheMu.Unlock()

	p.avatarCache[key] = avatarCacheEntry{
		URL:       url,
		FetchedAt: time.Now(),
	}
}
