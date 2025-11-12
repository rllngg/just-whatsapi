package renr

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// handleMedia downloads media and uploads to S3, returns URL
func (p *Plugin) handleMedia(mediaURL, mediaType, messageID, deviceID string) string {
	// If no media URL, return empty
	if mediaURL == "" {
		p.logger.Warnf("No media URL provided for %s message", mediaType)
		return ""
	}

	// Check if S3 is enabled
	if !p.isS3Enabled() {
		p.logger.Warnf("S3 not enabled, using original media URL")
		return mediaURL
	}

	// Download media
	data, contentType, err := p.downloadMedia(mediaURL)
	if err != nil {
		p.logger.Errorf("Failed to download media: %v", err)
		return mediaURL // Fallback to original URL
	}

	// Generate filename
	ext := getExtension(contentType)
	filename := fmt.Sprintf("%s.%s", messageID, ext)
	key := fmt.Sprintf("whatsapp/%s/%s", deviceID, filename)

	// Upload to S3
	s3URL, err := p.uploadToS3(data, key, contentType)
	if err != nil {
		p.logger.Errorf("Failed to upload to S3: %v", err)
		return mediaURL // Fallback to original URL
	}

	p.logger.Infof("Uploaded media to S3: %s", s3URL)
	return s3URL
}

// isS3Enabled checks if S3 configuration is available
func (p *Plugin) isS3Enabled() bool {
	return os.Getenv("PLUGINS_WEBHOOK_S3_ENABLED") == "true"
}

// downloadMedia downloads media from URL
func (p *Plugin) downloadMedia(url string) ([]byte, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return data, contentType, nil
}

// uploadToS3 uploads data to S3 and returns public URL
func (p *Plugin) uploadToS3(data []byte, key, contentType string) (string, error) {
	// Get S3 configuration from environment
	region := os.Getenv("PLUGINS_WEBHOOK_S3_REGION")
	bucket := os.Getenv("PLUGINS_WEBHOOK_S3_BUCKET")
	accessKey := os.Getenv("PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID")
	secretKey := os.Getenv("PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY")
	endpoint := os.Getenv("PLUGINS_WEBHOOK_S3_ENDPOINT")

	if region == "" || bucket == "" || accessKey == "" || secretKey == "" {
		return "", fmt.Errorf("S3 configuration incomplete")
	}

	// Create AWS session
	awsConfig := &aws.Config{
		Region:      aws.String(region),
		Credentials: credentials.NewStaticCredentials(accessKey, secretKey, ""),
	}

	// If custom endpoint (MinIO), set it
	if endpoint != "" {
		awsConfig.Endpoint = aws.String(endpoint)
		awsConfig.S3ForcePathStyle = aws.Bool(true)
	}

	sess, err := session.NewSession(awsConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create AWS session: %w", err)
	}

	// Create S3 client
	svc := s3.New(sess)

	// Upload file
	_, err = svc.PutObjectWithContext(context.Background(), &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"),
	})

	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Generate public URL
	var publicURL string
	if endpoint != "" {
		// MinIO or custom S3 endpoint
		publicURL = fmt.Sprintf("%s/%s/%s", endpoint, bucket, key)
	} else {
		// AWS S3
		publicURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", bucket, region, key)
	}

	return publicURL, nil
}

// getFilenameFromURL extracts filename from URL
func getFilenameFromURL(url string) string {
	return filepath.Base(url)
}
