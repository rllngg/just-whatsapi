package webhook

import (
	"fmt"
	"os"
	"strings"
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
	URLStyle        string // URL format style: "path", "subdomain", or "auto"
}

// LoadConfig loads webhook configuration from environment variables
func LoadConfig() (*Config, error) {
	webhookURL := os.Getenv("PLUGINS_WEBHOOK_URL")
	if webhookURL == "" {
		fmt.Println("⚠️  WARNING: PLUGINS_WEBHOOK_URL is not set")
		fmt.Println("⚠️  Webhook plugin will run in STORE-ONLY mode")
		fmt.Println("⚠️  Events will be stored in SQLite but not delivered to any endpoint")
	}

	s3Enabled := os.Getenv("PLUGINS_WEBHOOK_S3_ENABLED") == "true"

	// Load URL style with default
	urlStyle := os.Getenv("PLUGINS_WEBHOOK_S3_URL_STYLE")
	if urlStyle == "" {
		urlStyle = "path" // Default to path-style for backward compatibility
	}

	config := &Config{
		WebhookURL: webhookURL,
		S3Config: S3Config{
			Enabled:         s3Enabled,
			Region:          os.Getenv("PLUGINS_WEBHOOK_S3_REGION"),
			Bucket:          os.Getenv("PLUGINS_WEBHOOK_S3_BUCKET"),
			AccessKeyID:     os.Getenv("PLUGINS_WEBHOOK_S3_ACCESS_KEY_ID"),
			SecretAccessKey: os.Getenv("PLUGINS_WEBHOOK_S3_SECRET_ACCESS_KEY"),
			Endpoint:        os.Getenv("PLUGINS_WEBHOOK_S3_ENDPOINT"),
			URLStyle:        urlStyle,
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

		// Validate URL style
		validStyles := map[string]bool{"path": true, "subdomain": true, "auto": true}
		if !validStyles[config.S3Config.URLStyle] {
			return nil, fmt.Errorf("PLUGINS_WEBHOOK_S3_URL_STYLE must be one of: path, subdomain, auto (got: %s)", config.S3Config.URLStyle)
		}

		// Warn if subdomain style used with DNS-incompatible bucket name
		if config.S3Config.URLStyle == "subdomain" && containsInvalidDNSChars(config.S3Config.Bucket) {
			fmt.Printf("⚠️  WARNING: Bucket '%s' contains characters invalid for subdomain URLs\n", config.S3Config.Bucket)
			fmt.Printf("⚠️  Subdomain-style URLs may not work. Consider using URL_STYLE=path\n")
		}
	}

	return config, nil
}

// containsInvalidDNSChars checks if a bucket name contains characters invalid for DNS/subdomain usage
func containsInvalidDNSChars(bucket string) bool {
	// Underscores not DNS-safe
	if strings.Contains(bucket, "_") {
		return true
	}
	// Consecutive dots not allowed
	if strings.Contains(bucket, "..") {
		return true
	}
	// Cannot start or end with hyphen
	if strings.HasPrefix(bucket, "-") || strings.HasSuffix(bucket, "-") {
		return true
	}
	// Length constraints
	if len(bucket) < 3 || len(bucket) > 63 {
		return true
	}
	// Check for uppercase letters (not DNS-safe)
	if strings.ToLower(bucket) != bucket {
		return true
	}
	return false
}
