package renr

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

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
