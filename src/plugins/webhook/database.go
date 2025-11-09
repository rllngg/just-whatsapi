package webhook

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// WebhookQueue represents a webhook delivery job in the queue
type WebhookQueue struct {
	ID          int64
	Event       string
	Payload     string
	WebhookURL  string
	Status      string // pending, processing, success, failed
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	LastAttempt *time.Time
	NextRetry   *time.Time
	CompletedAt *time.Time
}

// Database handles webhook queue persistence
type Database struct {
	db *sql.DB
}

// NewDatabase creates a new database connection and initializes schema
func NewDatabase(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	schema := `
	CREATE TABLE IF NOT EXISTS webhook_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event TEXT NOT NULL,
		payload TEXT NOT NULL,
		webhook_url TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		attempts INTEGER NOT NULL DEFAULT 0,
		last_error TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_attempt DATETIME,
		next_retry DATETIME,
		completed_at DATETIME
	);

	CREATE INDEX IF NOT EXISTS idx_webhook_queue_status ON webhook_queue(status);
	CREATE INDEX IF NOT EXISTS idx_webhook_queue_next_retry ON webhook_queue(next_retry);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Database{db: db}, nil
}

// Enqueue adds a new webhook delivery job to the queue
func (d *Database) Enqueue(event string, payload map[string]interface{}, webhookURL string) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	_, err = d.db.Exec(`
		INSERT INTO webhook_queue (event, payload, webhook_url, status)
		VALUES (?, ?, ?, 'pending')
	`, event, string(payloadJSON), webhookURL)

	if err != nil {
		return fmt.Errorf("failed to enqueue webhook: %w", err)
	}

	return nil
}

// GetPending retrieves pending webhooks that are ready to be processed
func (d *Database) GetPending(limit int) ([]*WebhookQueue, error) {
	now := time.Now()

	rows, err := d.db.Query(`
		SELECT id, event, payload, webhook_url, status, attempts, last_error, 
		       created_at, last_attempt, next_retry, completed_at
		FROM webhook_queue
		WHERE status IN ('pending', 'failed')
		  AND (next_retry IS NULL OR next_retry <= ?)
		  AND created_at >= ?
		ORDER BY created_at ASC
		LIMIT ?
	`, now, now.Add(-24*time.Hour), limit)

	if err != nil {
		return nil, fmt.Errorf("failed to query pending webhooks: %w", err)
	}
	defer rows.Close()

	var webhooks []*WebhookQueue
	for rows.Next() {
		wh := &WebhookQueue{}
		var lastAttempt, nextRetry, completedAt sql.NullTime

		err := rows.Scan(
			&wh.ID, &wh.Event, &wh.Payload, &wh.WebhookURL,
			&wh.Status, &wh.Attempts, &wh.LastError,
			&wh.CreatedAt, &lastAttempt, &nextRetry, &completedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan webhook: %w", err)
		}

		if lastAttempt.Valid {
			wh.LastAttempt = &lastAttempt.Time
		}
		if nextRetry.Valid {
			wh.NextRetry = &nextRetry.Time
		}
		if completedAt.Valid {
			wh.CompletedAt = &completedAt.Time
		}

		webhooks = append(webhooks, wh)
	}

	return webhooks, nil
}

// MarkProcessing marks a webhook as being processed
func (d *Database) MarkProcessing(id int64) error {
	_, err := d.db.Exec(`
		UPDATE webhook_queue
		SET status = 'processing'
		WHERE id = ?
	`, id)

	return err
}

// MarkSuccess marks a webhook as successfully delivered
func (d *Database) MarkSuccess(id int64) error {
	_, err := d.db.Exec(`
		UPDATE webhook_queue
		SET status = 'success',
		    completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, id)

	return err
}

// MarkFailed marks a webhook as failed and schedules retry with exponential backoff
func (d *Database) MarkFailed(id int64, errorMsg string) error {
	// Get current attempts
	var attempts int
	err := d.db.QueryRow(`SELECT attempts FROM webhook_queue WHERE id = ?`, id).Scan(&attempts)
	if err != nil {
		return fmt.Errorf("failed to get attempts: %w", err)
	}

	attempts++

	// Calculate exponential backoff: 1min, 2min, 4min, 8min, 16min, 32min, 64min, etc.
	backoffMinutes := 1 << (attempts - 1)
	if backoffMinutes > 1440 { // Cap at 24 hours
		backoffMinutes = 1440
	}

	nextRetry := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

	_, err = d.db.Exec(`
		UPDATE webhook_queue
		SET status = 'failed',
		    attempts = ?,
		    last_error = ?,
		    last_attempt = CURRENT_TIMESTAMP,
		    next_retry = ?
		WHERE id = ?
	`, attempts, errorMsg, nextRetry, id)

	return err
}

// CleanupOld removes webhooks older than the specified duration
func (d *Database) CleanupOld(olderThan time.Duration) error {
	cutoff := time.Now().Add(-olderThan)

	_, err := d.db.Exec(`
		DELETE FROM webhook_queue
		WHERE created_at < ?
		  AND status IN ('success', 'failed')
	`, cutoff)

	return err
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}
