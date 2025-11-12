package renrqueue

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// FlexibleTime wraps time.Time to handle multiple timestamp formats during JSON unmarshaling
type FlexibleTime struct {
	time.Time
}

// UnmarshalJSON implements custom unmarshaling for FlexibleTime
func (ft *FlexibleTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	if s == "null" || s == "" {
		return nil
	}

	// Try multiple timestamp formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05.999999999",   // Without timezone
		"2006-01-02T15:04:05.999999",      // Microseconds without timezone
		"2006-01-02T15:04:05.999",         // Milliseconds without timezone
		"2006-01-02T15:04:05",             // Seconds without timezone
	}

	var err error
	for _, format := range formats {
		var t time.Time
		t, err = time.Parse(format, s)
		if err == nil {
			ft.Time = t
			return nil
		}
	}

	return fmt.Errorf("unable to parse time %q: %w", s, err)
}

// MarshalJSON implements custom marshaling for FlexibleTime
func (ft FlexibleTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(ft.Time)
}

// QueueStatus represents the current status of a queue message
type QueueStatus string

const (
	StatusPending    QueueStatus = "PENDING"
	StatusProcessing QueueStatus = "PROCESSING"
	StatusCompleted  QueueStatus = "COMPLETED"
	StatusFailed     QueueStatus = "FAILED"
)

// EnqueueRequest represents the request body for enqueueing a message
type EnqueueRequest struct {
	Ref              *string `json:"ref,omitempty"`
	Payload          string  `json:"payload"`
	TimeoutInSeconds *int    `json:"timeoutInSeconds,omitempty"`
}

// MarkFailedRequest represents the request body for marking a message as failed
type MarkFailedRequest struct {
	Reason string `json:"reason"`
}

// QueueItemResponse represents a queue item in responses
type QueueItemResponse struct {
	ID          int64        `json:"id"`
	Topic       string       `json:"topic"`
	Ref         *string      `json:"ref,omitempty"`
	Payload     *string      `json:"payload,omitempty"`
	Status      QueueStatus  `json:"status"`
	RetryCount  int          `json:"retryCount"`
	CreatedDate FlexibleTime `json:"createdDate"`
	ExpiredAt   *FlexibleTime `json:"expiredAt,omitempty"`
	Response    *string      `json:"response,omitempty"`
}

// QueueFailureResult represents the result of marking a message as failed
type QueueFailureResult struct {
	ID             int64         `json:"id"`
	Status         QueueStatus   `json:"status"`
	Response       string        `json:"response"`
	RetryCount     int           `json:"retryCount"`
	WillRetry      bool          `json:"willRetry"`
	NextRetryAt    *FlexibleTime `json:"nextRetryAt,omitempty"`
	NewQueueItemID *int64        `json:"newQueueItemId,omitempty"`
}

// QueueStats represents statistics for a queue topic
type QueueStats struct {
	Topic      string `json:"topic"`
	Pending    int64  `json:"pending"`
	Processing int64  `json:"processing"`
	Completed  int64  `json:"completed"`
	Failed     int64  `json:"failed"`
}

// RetryResult represents the result of retrying a failed message
type RetryResult struct {
	OriginalID   int64             `json:"originalId"`
	NewQueueItem QueueItemResponse `json:"newQueueItem"`
}

// PagedQueueItems represents a paginated list of queue items
type PagedQueueItems struct {
	Content       []QueueItemResponse `json:"content"`
	TotalElements int64               `json:"totalElements"`
	TotalPages    int                 `json:"totalPages"`
	CurrentPage   int                 `json:"currentPage"`
	PageSize      int                 `json:"pageSize"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Timestamp FlexibleTime `json:"timestamp"`
	Status    int          `json:"status"`
	Error     string       `json:"error"`
	Message   string       `json:"message"`
	Path      string       `json:"path"`
}

// Client represents the queue system API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Username   string
	Password   string
}

// NewClient creates a new queue system API client
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Username: username,
		Password: password,
	}
}

// LoadEnv loads environment variables from .env file
// Returns error if .env file exists but cannot be loaded
// Silently continues if .env file doesn't exist
func LoadEnv(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{".env"}
	}
	return godotenv.Load(paths...)
}

// NewClientFromEnv creates a new client using environment variables
// Environment variables:
//   - RENR_QUEUE_URL: Base URL for the queue API (default: http://localhost:8080)
//   - RENR_QUEUE_USERNAME: Username for basic authentication (required)
//   - RENR_QUEUE_PASSWORD: Password for basic authentication (required)
func NewClientFromEnv() (*Client, error) {
	baseURL := os.Getenv("RENR_QUEUE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	username := os.Getenv("RENR_QUEUE_USERNAME")
	if username == "" {
		return nil, fmt.Errorf("RENR_QUEUE_USERNAME environment variable is required")
	}

	password := os.Getenv("RENR_QUEUE_PASSWORD")
	if password == "" {
		return nil, fmt.Errorf("RENR_QUEUE_PASSWORD environment variable is required")
	}

	return NewClient(baseURL, username, password), nil
}

// doRequest performs an HTTP request with Basic Auth
func (c *Client) doRequest(method, path string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.Username, c.Password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	return resp, nil
}

// handleResponse processes the HTTP response and unmarshals JSON
func (c *Client) handleResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
		}
		return fmt.Errorf("API error (status %d): %s - %s", errResp.Status, errResp.Error, errResp.Message)
	}

	if result != nil && len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Enqueue adds a new message to the queue for the specified topic
func (c *Client) Enqueue(topic string, req EnqueueRequest) (*QueueItemResponse, error) {
	path := fmt.Sprintf("/system/queue/%s", url.PathEscape(topic))
	resp, err := c.doRequest(http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}

	var result QueueItemResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Pull retrieves the next pending message from the queue and marks it as PROCESSING
func (c *Client) Pull(topic string, ref *string) (*QueueItemResponse, error) {
	path := fmt.Sprintf("/system/queue/%s/pull", url.PathEscape(topic))

	// Add ref query parameter if provided
	if ref != nil && *ref != "" {
		path += "?ref=" + url.QueryEscape(*ref)
	}

	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	// Handle 204 No Content
	if resp.StatusCode == http.StatusNoContent {
		resp.Body.Close()
		return nil, nil
	}

	var result QueueItemResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Complete marks a message as successfully processed
func (c *Client) Complete(topic string, id int64) (*QueueItemResponse, error) {
	path := fmt.Sprintf("/system/queue/%s/%d/complete", url.PathEscape(topic), id)
	resp, err := c.doRequest(http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}

	var result QueueItemResponse
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// MarkAsFailed marks a message as failed and automatically creates retry if retries available
func (c *Client) MarkAsFailed(topic string, id int64, req MarkFailedRequest) (*QueueFailureResult, error) {
	path := fmt.Sprintf("/system/queue/%s/%d/failed", url.PathEscape(topic), id)
	resp, err := c.doRequest(http.MethodPost, path, req)
	if err != nil {
		return nil, err
	}

	var result QueueFailureResult
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetStats retrieves statistics for a specific queue topic
func (c *Client) GetStats(topic string) (*QueueStats, error) {
	path := fmt.Sprintf("/system/queue/%s/stats", url.PathEscape(topic))
	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result QueueStats
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetFailedMessages retrieves all failed messages for a topic (dead letter queue)
func (c *Client) GetFailedMessages(topic string, page, size int) (*PagedQueueItems, error) {
	path := fmt.Sprintf("/system/queue/%s/failed?page=%d&size=%d",
		url.PathEscape(topic), page, size)

	resp, err := c.doRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var result PagedQueueItems
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// RetryFailed manually retries a failed message from the dead letter queue
func (c *Client) RetryFailed(topic string, id int64) (*RetryResult, error) {
	path := fmt.Sprintf("/system/queue/%s/%d/retry", url.PathEscape(topic), id)
	resp, err := c.doRequest(http.MethodPost, path, nil)
	if err != nil {
		return nil, err
	}

	var result RetryResult
	if err := c.handleResponse(resp, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

// Helper function to create string pointer
func StringPtr(s string) *string {
	return &s
}

// Helper function to create int pointer
func IntPtr(i int) *int {
	return &i
}
