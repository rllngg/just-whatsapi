package webhook

import (
	"fmt"
	"os"
)

// Config holds the webhook plugin configuration
type Config struct {
	WebhookURL string
	S3Config   S3Config
}

// S3Config holds AWS S3 configuration
type S3Config struct {
	Enabled         bool
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string // Optional: for S3-compatible services
}

// LoadConfig loads webhook configuration from environment variables
func LoadConfig() (*Config, error) {
	webhookURL := os.Getenv("PLUGINS_WEBHOOK_URL")
	if webhookURL == "" {
		return nil, fmt.Errorf("PLUGINS_WEBHOOK_URL is required")
	}

	s3Enabled := os.Getenv("PLUGINS_WEBHOOK_S3_ENABLED") == "true"

	config := &Config{
		WebhookURL: webhookURL,
		S3Config: S3Config{
			Enabled:         s3Enabled,
			Region:          os.Getenv("PLUGINS_WEBHOOK_S3_REGION"),
			Bucket:          os.Getenv("PLUGINS_WEBHOOK_S3_BUCKET"),
			AccessKeyID:     os.Getenv("PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY"),
			Endpoint:        os.Getenv("PLUGINS_WEBHOOK_S3_ENDPOINT"),
		},
	}

	// Validate S3 config if enabled
	if s3Enabled {
		if config.S3Config.Region == "" {
			return nil, fmt.Errorf("PLUGINS_WEBHOOK_S3_REGION is required when S3 is enabled")
		}
		if config.S3Config.Bucket == "" {
			return nil, fmt.Errorf("PLUGINS_WEBHOOK_S3_BUCKET is required when S3 is enabled")
		}
		if config.S3Config.AccessKeyID == "" {
			return nil, fmt.Errorf("PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID is required when S3 is enabled")
		}
		if config.S3Config.SecretAccessKey == "" {
			return nil, fmt.Errorf("PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY is required when S3 is enabled")
		}
	}

	return config, nil
}
