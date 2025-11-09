package whatsapp

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DeviceMapping represents the mapping between device_id and WhatsApp JID
type DeviceMapping struct {
	DeviceID  string
	DeviceJID string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// DeviceDatabase handles device mapping persistence
type DeviceDatabase struct {
	db *sql.DB
}

// NewDeviceDatabase creates a new device database connection and initializes schema
func NewDeviceDatabase(dbPath string) (*DeviceDatabase, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize schema
	schema := `
	CREATE TABLE IF NOT EXISTS device_mapping (
		device_id TEXT PRIMARY KEY,
		device_jid TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_device_jid ON device_mapping(device_jid);
	`

	_, err = db.Exec(schema)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &DeviceDatabase{db: db}, nil
}

// SaveMapping saves or updates a device mapping
func (d *DeviceDatabase) SaveMapping(deviceID, deviceJID string) error {
	_, err := d.db.Exec(`
		INSERT INTO device_mapping (device_id, device_jid, created_at, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(device_id) DO UPDATE SET
			device_jid = excluded.device_jid,
			updated_at = CURRENT_TIMESTAMP
	`, deviceID, deviceJID)

	if err != nil {
		return fmt.Errorf("failed to save device mapping: %w", err)
	}

	return nil
}

// GetMapping retrieves a device mapping by device_id
func (d *DeviceDatabase) GetMapping(deviceID string) (*DeviceMapping, error) {
	mapping := &DeviceMapping{}
	err := d.db.QueryRow(`
		SELECT device_id, device_jid, created_at, updated_at
		FROM device_mapping
		WHERE device_id = ?
	`, deviceID).Scan(&mapping.DeviceID, &mapping.DeviceJID, &mapping.CreatedAt, &mapping.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device mapping: %w", err)
	}

	return mapping, nil
}

// GetMappingByJID retrieves a device mapping by WhatsApp JID
func (d *DeviceDatabase) GetMappingByJID(deviceJID string) (*DeviceMapping, error) {
	mapping := &DeviceMapping{}
	err := d.db.QueryRow(`
		SELECT device_id, device_jid, created_at, updated_at
		FROM device_mapping
		WHERE device_jid = ?
	`, deviceJID).Scan(&mapping.DeviceID, &mapping.DeviceJID, &mapping.CreatedAt, &mapping.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get device mapping by JID: %w", err)
	}

	return mapping, nil
}

// GetAllMappings retrieves all device mappings
func (d *DeviceDatabase) GetAllMappings() ([]*DeviceMapping, error) {
	rows, err := d.db.Query(`
		SELECT device_id, device_jid, created_at, updated_at
		FROM device_mapping
		ORDER BY created_at ASC
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to query device mappings: %w", err)
	}
	defer rows.Close()

	var mappings []*DeviceMapping
	for rows.Next() {
		mapping := &DeviceMapping{}
		err := rows.Scan(&mapping.DeviceID, &mapping.DeviceJID, &mapping.CreatedAt, &mapping.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan device mapping: %w", err)
		}
		mappings = append(mappings, mapping)
	}

	return mappings, nil
}

// DeleteMapping removes a device mapping by device_id
func (d *DeviceDatabase) DeleteMapping(deviceID string) error {
	_, err := d.db.Exec(`
		DELETE FROM device_mapping
		WHERE device_id = ?
	`, deviceID)

	if err != nil {
		return fmt.Errorf("failed to delete device mapping: %w", err)
	}

	return nil
}

// Close closes the database connection
func (d *DeviceDatabase) Close() error {
	return d.db.Close()
}
