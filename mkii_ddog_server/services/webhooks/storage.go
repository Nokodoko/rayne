package webhooks

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
)

// Storage handles database operations for webhooks
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new webhook storage instance
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// InitTables creates the necessary database tables for webhooks
func (s *Storage) InitTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS webhook_events (
		id SERIAL PRIMARY KEY,
		alert_id BIGINT,
		alert_title TEXT,
		alert_message TEXT,
		alert_status VARCHAR(50),
		monitor_id BIGINT,
		monitor_name TEXT,
		monitor_type VARCHAR(100),
		tags TEXT[],
		event_timestamp BIGINT,
		event_type VARCHAR(100),
		priority VARCHAR(50),
		hostname VARCHAR(255),
		service VARCHAR(255),
		scope TEXT,
		transition_id VARCHAR(255),
		last_updated BIGINT,
		snapshot_url TEXT,
		link TEXT,
		org_id BIGINT,
		org_name VARCHAR(255),
		received_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		processed_at TIMESTAMP WITH TIME ZONE,
		status VARCHAR(50) DEFAULT 'pending',
		forwarded_to TEXT[],
		error_message TEXT,
		account_id BIGINT,
		account_name VARCHAR(255)
	);

	CREATE TABLE IF NOT EXISTS webhook_configs (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) UNIQUE NOT NULL,
		url TEXT NOT NULL,
		use_custom_payload BOOLEAN DEFAULT false,
		custom_payload TEXT,
		forward_urls TEXT[],
		auto_downtime BOOLEAN DEFAULT false,
		downtime_duration_minutes INT DEFAULT 120,
		notify_enabled BOOLEAN DEFAULT false,
		notify_numbers TEXT[],
		active BOOLEAN DEFAULT true,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_webhook_events_monitor_id ON webhook_events(monitor_id);
	CREATE INDEX IF NOT EXISTS idx_webhook_events_status ON webhook_events(status);
	CREATE INDEX IF NOT EXISTS idx_webhook_events_received_at ON webhook_events(received_at);
	CREATE INDEX IF NOT EXISTS idx_webhook_events_alert_status ON webhook_events(alert_status);
	CREATE INDEX IF NOT EXISTS idx_webhook_events_account_id ON webhook_events(account_id);
	`

	_, err := s.db.Exec(query)
	if err != nil {
		return err
	}

	// Add account columns if they don't exist (for existing tables)
	alterQuery := `
	DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'webhook_events' AND column_name = 'account_id') THEN
			ALTER TABLE webhook_events ADD COLUMN account_id BIGINT;
		END IF;
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns
					   WHERE table_name = 'webhook_events' AND column_name = 'account_name') THEN
			ALTER TABLE webhook_events ADD COLUMN account_name VARCHAR(255);
		END IF;
	END $$;
	`
	_, err = s.db.Exec(alterQuery)
	return err
}

// StoreEvent saves a webhook event to the database (backward compatible)
func (s *Storage) StoreEvent(payload WebhookPayload) (*WebhookEvent, error) {
	return s.StoreEventWithAccount(payload, nil, "")
}

// StoreEventWithAccount saves a webhook event with account association
func (s *Storage) StoreEventWithAccount(payload WebhookPayload, accountID *int64, accountName string) (*WebhookEvent, error) {
	query := `
	INSERT INTO webhook_events (
		alert_id, alert_title, alert_message, alert_status,
		monitor_id, monitor_name, monitor_type, tags,
		event_timestamp, event_type, priority, hostname,
		service, scope, transition_id, last_updated,
		snapshot_url, link, org_id, org_name,
		account_id, account_name
	) VALUES (
		$1, $2, $3, $4, $5, $6, $7, $8, $9, $10,
		$11, $12, $13, $14, $15, $16, $17, $18, $19, $20,
		$21, $22
	) RETURNING id, received_at`

	event := &WebhookEvent{
		Payload:     payload,
		Status:      "pending",
		AccountID:   accountID,
		AccountName: accountName,
	}

	err := s.db.QueryRow(
		query,
		payload.AlertID, payload.AlertTitle, payload.AlertMessage, payload.AlertStatus,
		payload.MonitorID, payload.MonitorName, payload.MonitorType, pq.Array(payload.Tags),
		payload.Timestamp, payload.EventType, payload.Priority, payload.Hostname,
		payload.Service, payload.Scope, payload.TransitionID, payload.LastUpdated,
		payload.SnapshotURL, payload.Link, payload.OrgID, payload.OrgName,
		accountID, accountName,
	).Scan(&event.ID, &event.ReceivedAt)

	if err != nil {
		return nil, err
	}

	return event, nil
}

// GetEventByID retrieves a webhook event by ID
func (s *Storage) GetEventByID(id int64) (*WebhookEvent, error) {
	query := `
	SELECT id, alert_id, alert_title, alert_message, alert_status,
		monitor_id, monitor_name, monitor_type, tags,
		event_timestamp, event_type, priority, hostname,
		service, scope, transition_id, last_updated,
		snapshot_url, link, org_id, org_name,
		received_at, processed_at, status, forwarded_to, error_message,
		account_id, account_name
	FROM webhook_events WHERE id = $1`

	event := &WebhookEvent{}
	var tags pq.StringArray
	var forwardedTo pq.StringArray
	var errorMsg sql.NullString
	var processedAt sql.NullTime
	var accountID sql.NullInt64
	var accountName sql.NullString

	err := s.db.QueryRow(query, id).Scan(
		&event.ID,
		&event.Payload.AlertID, &event.Payload.AlertTitle, &event.Payload.AlertMessage, &event.Payload.AlertStatus,
		&event.Payload.MonitorID, &event.Payload.MonitorName, &event.Payload.MonitorType, &tags,
		&event.Payload.Timestamp, &event.Payload.EventType, &event.Payload.Priority, &event.Payload.Hostname,
		&event.Payload.Service, &event.Payload.Scope, &event.Payload.TransitionID, &event.Payload.LastUpdated,
		&event.Payload.SnapshotURL, &event.Payload.Link, &event.Payload.OrgID, &event.Payload.OrgName,
		&event.ReceivedAt, &processedAt, &event.Status, &forwardedTo, &errorMsg,
		&accountID, &accountName,
	)

	if err != nil {
		return nil, err
	}

	event.Payload.Tags = tags
	event.ForwardedTo = forwardedTo
	if errorMsg.Valid {
		event.Error = errorMsg.String
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}
	if accountID.Valid {
		event.AccountID = &accountID.Int64
	}
	if accountName.Valid {
		event.AccountName = accountName.String
	}

	return event, nil
}

// GetRecentEvents retrieves recent webhook events
func (s *Storage) GetRecentEvents(limit int, offset int) ([]WebhookEvent, int, error) {
	countQuery := `SELECT COUNT(*) FROM webhook_events`
	var totalCount int
	if err := s.db.QueryRow(countQuery).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	query := `
	SELECT id, alert_id, alert_title, alert_message, alert_status,
		monitor_id, monitor_name, monitor_type, tags,
		event_timestamp, event_type, priority, hostname,
		service, scope, transition_id, last_updated,
		snapshot_url, link, org_id, org_name,
		received_at, processed_at, status, forwarded_to, error_message,
		account_id, account_name
	FROM webhook_events
	ORDER BY received_at DESC
	LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []WebhookEvent
	for rows.Next() {
		event := WebhookEvent{}
		var tags pq.StringArray
		var forwardedTo pq.StringArray
		var errorMsg sql.NullString
		var processedAt sql.NullTime
		var accountID sql.NullInt64
		var accountName sql.NullString

		err := rows.Scan(
			&event.ID,
			&event.Payload.AlertID, &event.Payload.AlertTitle, &event.Payload.AlertMessage, &event.Payload.AlertStatus,
			&event.Payload.MonitorID, &event.Payload.MonitorName, &event.Payload.MonitorType, &tags,
			&event.Payload.Timestamp, &event.Payload.EventType, &event.Payload.Priority, &event.Payload.Hostname,
			&event.Payload.Service, &event.Payload.Scope, &event.Payload.TransitionID, &event.Payload.LastUpdated,
			&event.Payload.SnapshotURL, &event.Payload.Link, &event.Payload.OrgID, &event.Payload.OrgName,
			&event.ReceivedAt, &processedAt, &event.Status, &forwardedTo, &errorMsg,
			&accountID, &accountName,
		)
		if err != nil {
			return nil, 0, err
		}

		event.Payload.Tags = tags
		event.ForwardedTo = forwardedTo
		if errorMsg.Valid {
			event.Error = errorMsg.String
		}
		if processedAt.Valid {
			event.ProcessedAt = &processedAt.Time
		}
		if accountID.Valid {
			event.AccountID = &accountID.Int64
		}
		if accountName.Valid {
			event.AccountName = accountName.String
		}

		events = append(events, event)
	}

	return events, totalCount, nil
}

// UpdateEventStatus updates the status of a webhook event
func (s *Storage) UpdateEventStatus(id int64, status string, forwardedTo []string, errorMsg string) error {
	now := time.Now()
	query := `
	UPDATE webhook_events
	SET status = $1, processed_at = $2, forwarded_to = $3, error_message = $4
	WHERE id = $5`

	var errMsgPtr *string
	if errorMsg != "" {
		errMsgPtr = &errorMsg
	}

	_, err := s.db.Exec(query, status, now, pq.Array(forwardedTo), errMsgPtr, id)
	return err
}

// GetEventsByMonitorID retrieves events for a specific monitor
func (s *Storage) GetEventsByMonitorID(monitorID int64, limit int) ([]WebhookEvent, error) {
	query := `
	SELECT id, alert_id, alert_title, alert_message, alert_status,
		monitor_id, monitor_name, received_at, status
	FROM webhook_events
	WHERE monitor_id = $1
	ORDER BY received_at DESC
	LIMIT $2`

	rows, err := s.db.Query(query, monitorID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []WebhookEvent
	for rows.Next() {
		event := WebhookEvent{}
		err := rows.Scan(
			&event.ID,
			&event.Payload.AlertID, &event.Payload.AlertTitle, &event.Payload.AlertMessage, &event.Payload.AlertStatus,
			&event.Payload.MonitorID, &event.Payload.MonitorName,
			&event.ReceivedAt, &event.Status,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

// SaveConfig saves a webhook configuration
func (s *Storage) SaveConfig(config WebhookConfig) (*WebhookConfig, error) {
	query := `
	INSERT INTO webhook_configs (
		name, url, use_custom_payload, custom_payload,
		forward_urls, auto_downtime, downtime_duration_minutes,
		notify_enabled, notify_numbers, active
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id, created_at`

	err := s.db.QueryRow(
		query,
		config.Name, config.URL, config.UseCustomPayload, config.CustomPayload,
		pq.Array(config.ForwardURLs), config.AutoDowntime, config.DowntimeDuration,
		config.NotifyEnabled, pq.Array(config.NotifyNumbers), config.Active,
	).Scan(&config.ID, &config.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &config, nil
}

// GetConfigByName retrieves a webhook configuration by name
func (s *Storage) GetConfigByName(name string) (*WebhookConfig, error) {
	query := `
	SELECT id, name, url, use_custom_payload, custom_payload,
		forward_urls, auto_downtime, downtime_duration_minutes,
		notify_enabled, notify_numbers, active, created_at
	FROM webhook_configs WHERE name = $1`

	config := &WebhookConfig{}
	var forwardURLs pq.StringArray
	var notifyNumbers pq.StringArray
	var customPayload sql.NullString

	err := s.db.QueryRow(query, name).Scan(
		&config.ID, &config.Name, &config.URL, &config.UseCustomPayload, &customPayload,
		&forwardURLs, &config.AutoDowntime, &config.DowntimeDuration,
		&config.NotifyEnabled, &notifyNumbers, &config.Active, &config.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	config.ForwardURLs = forwardURLs
	config.NotifyNumbers = notifyNumbers
	if customPayload.Valid {
		config.CustomPayload = customPayload.String
	}

	return config, nil
}

// GetActiveConfigs retrieves all active webhook configurations
func (s *Storage) GetActiveConfigs() ([]WebhookConfig, error) {
	query := `
	SELECT id, name, url, use_custom_payload, custom_payload,
		forward_urls, auto_downtime, downtime_duration_minutes,
		notify_enabled, notify_numbers, active, created_at
	FROM webhook_configs WHERE active = true`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var configs []WebhookConfig
	for rows.Next() {
		config := WebhookConfig{}
		var forwardURLs pq.StringArray
		var notifyNumbers pq.StringArray
		var customPayload sql.NullString

		err := rows.Scan(
			&config.ID, &config.Name, &config.URL, &config.UseCustomPayload, &customPayload,
			&forwardURLs, &config.AutoDowntime, &config.DowntimeDuration,
			&config.NotifyEnabled, &notifyNumbers, &config.Active, &config.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		config.ForwardURLs = forwardURLs
		config.NotifyNumbers = notifyNumbers
		if customPayload.Valid {
			config.CustomPayload = customPayload.String
		}

		configs = append(configs, config)
	}

	return configs, nil
}

// GetEventStats retrieves statistics about webhook events
func (s *Storage) GetEventStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total events
	var totalEvents int
	s.db.QueryRow(`SELECT COUNT(*) FROM webhook_events`).Scan(&totalEvents)
	stats["total_events"] = totalEvents

	// Events by status
	statusCounts := make(map[string]int)
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM webhook_events GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		var count int
		rows.Scan(&status, &count)
		statusCounts[status] = count
	}
	stats["by_status"] = statusCounts

	// Events by alert status
	alertCounts := make(map[string]int)
	rows2, err := s.db.Query(`SELECT alert_status, COUNT(*) FROM webhook_events GROUP BY alert_status`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()

	for rows2.Next() {
		var alertStatus string
		var count int
		rows2.Scan(&alertStatus, &count)
		alertCounts[alertStatus] = count
	}
	stats["by_alert_status"] = alertCounts

	// Events in last 24 hours
	var last24h int
	s.db.QueryRow(`SELECT COUNT(*) FROM webhook_events WHERE received_at > NOW() - INTERVAL '24 hours'`).Scan(&last24h)
	stats["last_24h"] = last24h

	return stats, nil
}

// Helper to convert config to JSON for storage
func (c *WebhookConfig) ToJSON() ([]byte, error) {
	return json.Marshal(c)
}
