package renr

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// RenrQueueMessage represents a message in the Renr queue persistence layer
type RenrQueueMessage struct {
	ID          int64
	DeviceID    string
	MessageID   string
	QueueTopic  string
	Payload     string
	Status      string // pending, processing, failed
	Attempts    int
	LastError   string
	CreatedAt   time.Time
	LastAttempt *time.Time
	NextRetry   *time.Time
}

// Database handles Renr queue message persistence
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
	CREATE TABLE IF NOT EXISTS renr_queue (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		device_id TEXT NOT NULL,
		message_id TEXT NOT NULL,
		queue_topic TEXT NOT NULL,
		payload TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'pending',
		attempts INTEGER NOT NULL DEFAULT 0,
		last_error TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_attempt DATETIME,
		next_retry DATETIME,

		UNIQUE(device_id, message_id)
	);

	CREATE INDEX IF NOT EXISTS idx_renr_queue_status ON renr_queue(status);
	CREATE INDEX IF NOT EXISTS idx_renr_queue_next_retry ON renr_queue(next_retry);
	CREATE INDEX IF NOT EXISTS idx_renr_queue_device_status ON renr_queue(device_id, status);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Database{db: db}, nil
}

// Enqueue adds a new message to the persistence queue
// Uses UNIQUE constraint on (device_id, message_id) to prevent duplicates
func (d *Database) Enqueue(deviceID, messageID, topic, payload string) error {
	_, err := d.db.Exec(`
		INSERT INTO renr_queue (device_id, message_id, queue_topic, payload, status)
		VALUES (?, ?, ?, ?, 'pending')
		ON CONFLICT(device_id, message_id) DO NOTHING
	`, deviceID, messageID, topic, payload)

	if err != nil {
		return fmt.Errorf("failed to enqueue message: %w", err)
	}

	return nil
}

// GetNextPendingForDevice retrieves the oldest pending message for a specific device
// Returns nil if no pending messages exist for the device
func (d *Database) GetNextPendingForDevice(deviceID string) (*RenrQueueMessage, error) {
	now := time.Now()

	row := d.db.QueryRow(`
		SELECT id, device_id, message_id, queue_topic, payload, status, attempts, last_error,
		       created_at, last_attempt, next_retry
		FROM renr_queue
		WHERE device_id = ?
		  AND status = 'pending'
		  AND (next_retry IS NULL OR next_retry <= ?)
		ORDER BY created_at ASC
		LIMIT 1
	`, deviceID, now)

	msg := &RenrQueueMessage{}
	var lastError sql.NullString
	var lastAttempt, nextRetry sql.NullTime

	err := row.Scan(
		&msg.ID, &msg.DeviceID, &msg.MessageID, &msg.QueueTopic, &msg.Payload,
		&msg.Status, &msg.Attempts, &lastError,
		&msg.CreatedAt, &lastAttempt, &nextRetry,
	)

	if err == sql.ErrNoRows {
		return nil, nil // No pending messages
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get next pending message: %w", err)
	}

	if lastError.Valid {
		msg.LastError = lastError.String
	}
	if lastAttempt.Valid {
		msg.LastAttempt = &lastAttempt.Time
	}
	if nextRetry.Valid {
		msg.NextRetry = &nextRetry.Time
	}

	return msg, nil
}

// GetAllPendingDevices returns a list of device IDs that have pending messages
func (d *Database) GetAllPendingDevices() ([]string, error) {
	now := time.Now()

	rows, err := d.db.Query(`
		SELECT DISTINCT device_id
		FROM renr_queue
		WHERE status = 'pending'
		  AND (next_retry IS NULL OR next_retry <= ?)
		ORDER BY device_id
	`, now)

	if err != nil {
		return nil, fmt.Errorf("failed to query pending devices: %w", err)
	}
	defer rows.Close()

	var devices []string
	for rows.Next() {
		var deviceID string
		if err := rows.Scan(&deviceID); err != nil {
			return nil, fmt.Errorf("failed to scan device ID: %w", err)
		}
		devices = append(devices, deviceID)
	}

	return devices, nil
}

// MarkProcessing marks a message as being processed
func (d *Database) MarkProcessing(id int64) error {
	_, err := d.db.Exec(`
		UPDATE renr_queue
		SET status = 'processing'
		WHERE id = ?
	`, id)

	return err
}

// MarkSuccess deletes the message from the queue (successful delivery)
func (d *Database) MarkSuccess(id int64) error {
	_, err := d.db.Exec(`
		DELETE FROM renr_queue
		WHERE id = ?
	`, id)

	return err
}

// MarkFailed marks a message as failed and schedules retry with exponential backoff
func (d *Database) MarkFailed(id int64, errorMsg string) error {
	// Get current attempts
	var attempts int
	err := d.db.QueryRow(`SELECT attempts FROM renr_queue WHERE id = ?`, id).Scan(&attempts)
	if err != nil {
		return fmt.Errorf("failed to get attempts: %w", err)
	}

	attempts++

	// Calculate exponential backoff: 1min, 2min, 4min, 8min, 16min, 32min, 64min, etc.
	backoffMinutes := 1 << (attempts - 1)
	if backoffMinutes > 1440 { // Cap at 24 hours (1440 minutes)
		backoffMinutes = 1440
	}

	nextRetry := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

	_, err = d.db.Exec(`
		UPDATE renr_queue
		SET status = 'pending',
		    attempts = ?,
		    last_error = ?,
		    last_attempt = CURRENT_TIMESTAMP,
		    next_retry = ?
		WHERE id = ?
	`, attempts, errorMsg, nextRetry, id)

	if err != nil {
		return fmt.Errorf("failed to mark message as failed: %w", err)
	}

	return nil
}

// CleanupOldFailed marks failed messages older than 24 hours as permanent failures
// and keeps them in the database for debugging purposes
func (d *Database) CleanupOldFailed() error {
	cutoff := time.Now().Add(-24 * time.Hour)

	result, err := d.db.Exec(`
		UPDATE renr_queue
		SET status = 'failed',
		    last_error = 'Permanent failure: message older than 24 hours'
		WHERE created_at < ?
		  AND status = 'pending'
		  AND attempts > 0
	`, cutoff)

	if err != nil {
		return fmt.Errorf("failed to cleanup old failed messages: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("[RENR-QUEUE] Marked %d messages as permanent failures (older than 24h)\n", rowsAffected)
	}

	return nil
}

// MarkAllPendingAsFailed marks all pending messages for a device as failed
// This is used when a device logs out to cleanup its queue
func (d *Database) MarkAllPendingAsFailed(deviceID string, reason string) error {
	result, err := d.db.Exec(`
		UPDATE renr_queue
		SET status = 'failed',
		    last_error = ?
		WHERE device_id = ?
		  AND status IN ('pending', 'processing')
	`, reason, deviceID)

	if err != nil {
		return fmt.Errorf("failed to mark pending messages as failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		fmt.Printf("[RENR-QUEUE] Marked %d pending messages as failed for device %s (reason: %s)\n", rowsAffected, deviceID, reason)
	}

	return nil
}

// GetStats returns statistics about the queue
func (d *Database) GetStats() (map[string]int64, error) {
	stats := make(map[string]int64)

	// Count by status
	rows, err := d.db.Query(`
		SELECT status, COUNT(*) as count
		FROM renr_queue
		GROUP BY status
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, fmt.Errorf("failed to scan stats: %w", err)
		}
		stats[status] = count
	}

	// Get total count
	var total int64
	err = d.db.QueryRow(`SELECT COUNT(*) FROM renr_queue`).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats["total"] = total

	return stats, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.db.Close()
}
