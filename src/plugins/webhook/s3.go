package webhook

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client handles file uploads to AWS S3
type S3Client struct {
	client *s3.Client
	bucket string
	config S3Config
}

// NewS3Client creates a new S3 client
func NewS3Client(cfg S3Config) (*S3Client, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	ctx := context.Background()

	// Create AWS config
	awsConfig, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client options
	s3Options := func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		}
	}

	client := s3.NewFromConfig(awsConfig, s3Options)

	return &S3Client{
		client: client,
		bucket: cfg.Bucket,
		config: cfg,
	}, nil
}

// UploadFile uploads a file to S3 and returns the public URL
func (s *S3Client) UploadFile(ctx context.Context, data []byte, filename string, contentType string) (string, error) {
	if s == nil {
		return "", nil
	}

	// Generate a unique key with timestamp
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("whatsapp-media/%d-%s", timestamp, filepath.Base(filename))

	// Log upload details
	fmt.Printf("[S3] Uploading file: %s (size: %d bytes, type: %s)\n", filename, len(data), contentType)
	fmt.Printf("[S3] S3 key: %s, bucket: %s\n", key, s.bucket)

	// Upload to S3
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		fmt.Printf("[S3] Failed to upload file %s: %v\n", filename, err)
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Generate URL
	var url string
	if s.config.Endpoint != "" {
		url = fmt.Sprintf("%s/%s/%s", s.config.Endpoint, s.bucket, key)
	} else {
		url = fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.config.Region, key)
	}

	fmt.Printf("[S3] File uploaded successfully: %s -> %s\n", filename, url)
	return url, nil
}
