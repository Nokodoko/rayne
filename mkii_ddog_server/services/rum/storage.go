package rum

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Storage handles database operations for RUM
type Storage struct {
	db *sql.DB
}

// NewStorage creates a new RUM storage instance
func NewStorage(db *sql.DB) *Storage {
	return &Storage{db: db}
}

// InitTables creates the necessary database tables for RUM
func (s *Storage) InitTables() error {
	query := `
	CREATE TABLE IF NOT EXISTS rum_visitors (
		id SERIAL PRIMARY KEY,
		uuid VARCHAR(36) UNIQUE NOT NULL,
		first_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		last_seen TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		session_count INT DEFAULT 1,
		total_views INT DEFAULT 0,
		user_agent TEXT,
		ip_hash VARCHAR(64),
		country VARCHAR(100),
		city VARCHAR(100)
	);

	CREATE TABLE IF NOT EXISTS rum_sessions (
		id SERIAL PRIMARY KEY,
		visitor_uuid VARCHAR(36) REFERENCES rum_visitors(uuid) ON DELETE CASCADE,
		session_id VARCHAR(36) UNIQUE NOT NULL,
		start_time TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		end_time TIMESTAMP WITH TIME ZONE,
		page_views INT DEFAULT 0,
		duration_ms BIGINT DEFAULT 0,
		referrer TEXT,
		entry_page TEXT,
		exit_page TEXT,
		user_agent TEXT,
		device_type VARCHAR(50),
		browser VARCHAR(100),
		os VARCHAR(100)
	);

	CREATE TABLE IF NOT EXISTS rum_events (
		id SERIAL PRIMARY KEY,
		visitor_uuid VARCHAR(36) REFERENCES rum_visitors(uuid) ON DELETE CASCADE,
		session_id VARCHAR(36) REFERENCES rum_sessions(session_id) ON DELETE CASCADE,
		event_type VARCHAR(50) NOT NULL,
		timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		page_url TEXT,
		page_title TEXT,
		action_name TEXT,
		action_type VARCHAR(50),
		error_message TEXT,
		duration_ms BIGINT,
		metadata JSONB
	);

	CREATE INDEX IF NOT EXISTS idx_rum_visitors_uuid ON rum_visitors(uuid);
	CREATE INDEX IF NOT EXISTS idx_rum_visitors_last_seen ON rum_visitors(last_seen);
	CREATE INDEX IF NOT EXISTS idx_rum_sessions_visitor ON rum_sessions(visitor_uuid);
	CREATE INDEX IF NOT EXISTS idx_rum_sessions_start ON rum_sessions(start_time);
	CREATE INDEX IF NOT EXISTS idx_rum_events_session ON rum_events(session_id);
	CREATE INDEX IF NOT EXISTS idx_rum_events_timestamp ON rum_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_rum_events_type ON rum_events(event_type);
	`

	_, err := s.db.Exec(query)
	return err
}

// CreateVisitor creates a new visitor record
func (s *Storage) CreateVisitor(uuid, userAgent, ipHash string) error {
	query := `
	INSERT INTO rum_visitors (uuid, user_agent, ip_hash)
	VALUES ($1, $2, $3)`

	_, err := s.db.Exec(query, uuid, userAgent, ipHash)
	return err
}

// GetVisitorByUUID retrieves a visitor by UUID
func (s *Storage) GetVisitorByUUID(uuid string) (*Visitor, error) {
	query := `
	SELECT id, uuid, first_seen, last_seen, session_count, total_views,
		COALESCE(user_agent, ''), COALESCE(ip_hash, ''),
		COALESCE(country, ''), COALESCE(city, '')
	FROM rum_visitors WHERE uuid = $1`

	visitor := &Visitor{}
	err := s.db.QueryRow(query, uuid).Scan(
		&visitor.ID, &visitor.UUID, &visitor.FirstSeen, &visitor.LastSeen,
		&visitor.SessionCount, &visitor.TotalViews, &visitor.UserAgent,
		&visitor.IPHash, &visitor.Country, &visitor.City,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return visitor, nil
}

// UpdateVisitorLastSeen updates the last_seen timestamp and increments session count
func (s *Storage) UpdateVisitorLastSeen(uuid string) error {
	query := `
	UPDATE rum_visitors
	SET last_seen = NOW(), session_count = session_count + 1
	WHERE uuid = $1`

	_, err := s.db.Exec(query, uuid)
	return err
}

// IncrementVisitorViews increments the total views for a visitor
func (s *Storage) IncrementVisitorViews(uuid string) error {
	query := `UPDATE rum_visitors SET total_views = total_views + 1 WHERE uuid = $1`
	_, err := s.db.Exec(query, uuid)
	return err
}

// CreateSession creates a new session record
func (s *Storage) CreateSession(visitorUUID, sessionID, referrer, entryPage, userAgent string) error {
	deviceType, browser, os := parseUserAgent(userAgent)

	query := `
	INSERT INTO rum_sessions (visitor_uuid, session_id, referrer, entry_page, user_agent, device_type, browser, os)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err := s.db.Exec(query, visitorUUID, sessionID, referrer, entryPage, userAgent, deviceType, browser, os)
	return err
}

// GetSessionByID retrieves a session by its ID
func (s *Storage) GetSessionByID(sessionID string) (*Session, error) {
	query := `
	SELECT id, visitor_uuid, session_id, start_time, end_time, page_views,
		duration_ms, COALESCE(referrer, ''), COALESCE(entry_page, ''),
		COALESCE(exit_page, ''), COALESCE(user_agent, ''),
		COALESCE(device_type, ''), COALESCE(browser, ''), COALESCE(os, '')
	FROM rum_sessions WHERE session_id = $1`

	session := &Session{}
	var endTime sql.NullTime

	err := s.db.QueryRow(query, sessionID).Scan(
		&session.ID, &session.VisitorUUID, &session.SessionID, &session.StartTime, &endTime,
		&session.PageViews, &session.DurationMs, &session.Referrer, &session.EntryPage,
		&session.ExitPage, &session.UserAgent, &session.DeviceType, &session.Browser, &session.OS,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if endTime.Valid {
		session.EndTime = &endTime.Time
	}

	return session, nil
}

// UpdateSession updates session data
func (s *Storage) UpdateSession(sessionID string, pageViews int, durationMs int64) error {
	query := `
	UPDATE rum_sessions
	SET page_views = $1, duration_ms = $2
	WHERE session_id = $3`

	_, err := s.db.Exec(query, pageViews, durationMs, sessionID)
	return err
}

// EndSession marks a session as ended
func (s *Storage) EndSession(sessionID, exitPage string) error {
	query := `
	UPDATE rum_sessions
	SET end_time = NOW(), exit_page = $1,
		duration_ms = EXTRACT(EPOCH FROM (NOW() - start_time)) * 1000
	WHERE session_id = $2`

	_, err := s.db.Exec(query, exitPage, sessionID)
	return err
}

// IncrementSessionPageViews increments page views for a session
func (s *Storage) IncrementSessionPageViews(sessionID string) error {
	query := `UPDATE rum_sessions SET page_views = page_views + 1 WHERE session_id = $1`
	_, err := s.db.Exec(query, sessionID)
	return err
}

// StoreEvent saves a RUM event
func (s *Storage) StoreEvent(event RUMEvent) error {
	var metadataJSON []byte
	if event.Metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(event.Metadata)
		if err != nil {
			return err
		}
	}

	query := `
	INSERT INTO rum_events (
		visitor_uuid, session_id, event_type, page_url, page_title,
		action_name, action_type, error_message, duration_ms, metadata
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := s.db.Exec(query,
		event.VisitorUUID, event.SessionID, event.EventType, event.PageURL,
		event.PageTitle, event.ActionName, event.ActionType, event.ErrorMsg,
		event.Duration, metadataJSON,
	)

	// Increment page views if this is a view event
	if event.EventType == "view" || event.EventType == "page_view" {
		s.IncrementSessionPageViews(event.SessionID)
		s.IncrementVisitorViews(event.VisitorUUID)
	}

	return err
}

// GetEventsBySession retrieves events for a session
func (s *Storage) GetEventsBySession(sessionID string) ([]RUMEvent, error) {
	query := `
	SELECT id, visitor_uuid, session_id, event_type, timestamp,
		COALESCE(page_url, ''), COALESCE(page_title, ''),
		COALESCE(action_name, ''), COALESCE(action_type, ''),
		COALESCE(error_message, ''), COALESCE(duration_ms, 0), metadata
	FROM rum_events
	WHERE session_id = $1
	ORDER BY timestamp ASC`

	rows, err := s.db.Query(query, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []RUMEvent
	for rows.Next() {
		event := RUMEvent{}
		var metadataJSON []byte

		err := rows.Scan(
			&event.ID, &event.VisitorUUID, &event.SessionID, &event.EventType,
			&event.Timestamp, &event.PageURL, &event.PageTitle, &event.ActionName,
			&event.ActionType, &event.ErrorMsg, &event.Duration, &metadataJSON,
		)
		if err != nil {
			return nil, err
		}

		if len(metadataJSON) > 0 {
			json.Unmarshal(metadataJSON, &event.Metadata)
		}

		events = append(events, event)
	}

	return events, nil
}

// CountUniqueVisitors counts unique visitors in a time range
func (s *Storage) CountUniqueVisitors(from, to time.Time) (int, error) {
	query := `
	SELECT COUNT(DISTINCT uuid) FROM rum_visitors
	WHERE first_seen >= $1 AND first_seen <= $2`

	var count int
	err := s.db.QueryRow(query, from, to).Scan(&count)
	return count, err
}

// GetAnalytics retrieves analytics data for a time range
func (s *Storage) GetAnalytics(from, to time.Time) (*VisitorAnalytics, error) {
	analytics := &VisitorAnalytics{
		Period:    from.Format("2006-01-02") + " to " + to.Format("2006-01-02"),
		ByDevice:  make(map[string]int),
		ByBrowser: make(map[string]int),
		ByCountry: make(map[string]int),
	}

	// Unique visitors
	s.db.QueryRow(`
		SELECT COUNT(DISTINCT visitor_uuid) FROM rum_sessions
		WHERE start_time >= $1 AND start_time <= $2
	`, from, to).Scan(&analytics.UniqueVisitors)

	// Total sessions
	s.db.QueryRow(`
		SELECT COUNT(*) FROM rum_sessions
		WHERE start_time >= $1 AND start_time <= $2
	`, from, to).Scan(&analytics.TotalSessions)

	// Total page views
	s.db.QueryRow(`
		SELECT COALESCE(SUM(page_views), 0) FROM rum_sessions
		WHERE start_time >= $1 AND start_time <= $2
	`, from, to).Scan(&analytics.TotalPageViews)

	// Average session duration
	s.db.QueryRow(`
		SELECT COALESCE(AVG(duration_ms), 0) FROM rum_sessions
		WHERE start_time >= $1 AND start_time <= $2 AND duration_ms > 0
	`, from, to).Scan(&analytics.AvgSessionDuration)

	// New vs returning visitors
	s.db.QueryRow(`
		SELECT COUNT(*) FROM rum_visitors
		WHERE first_seen >= $1 AND first_seen <= $2
	`, from, to).Scan(&analytics.NewVisitors)

	analytics.ReturningVisitors = analytics.UniqueVisitors - analytics.NewVisitors
	if analytics.ReturningVisitors < 0 {
		analytics.ReturningVisitors = 0
	}

	// Top pages
	rows, err := s.db.Query(`
		SELECT page_url, COALESCE(page_title, ''), COUNT(*) as views
		FROM rum_events
		WHERE event_type IN ('view', 'page_view') AND timestamp >= $1 AND timestamp <= $2
		GROUP BY page_url, page_title
		ORDER BY views DESC
		LIMIT 10
	`, from, to)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var stat PageStat
			rows.Scan(&stat.PageURL, &stat.PageTitle, &stat.Views)
			analytics.TopPages = append(analytics.TopPages, stat)
		}
	}

	// By device type
	rows2, err := s.db.Query(`
		SELECT COALESCE(device_type, 'Unknown'), COUNT(*)
		FROM rum_sessions
		WHERE start_time >= $1 AND start_time <= $2
		GROUP BY device_type
	`, from, to)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var device string
			var count int
			rows2.Scan(&device, &count)
			analytics.ByDevice[device] = count
		}
	}

	// By browser
	rows3, err := s.db.Query(`
		SELECT COALESCE(browser, 'Unknown'), COUNT(*)
		FROM rum_sessions
		WHERE start_time >= $1 AND start_time <= $2
		GROUP BY browser
	`, from, to)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var browser string
			var count int
			rows3.Scan(&browser, &count)
			analytics.ByBrowser[browser] = count
		}
	}

	return analytics, nil
}

// GetRecentSessions retrieves recent sessions
func (s *Storage) GetRecentSessions(limit, offset int) ([]Session, error) {
	query := `
	SELECT id, visitor_uuid, session_id, start_time, end_time, page_views,
		duration_ms, COALESCE(referrer, ''), COALESCE(entry_page, ''),
		COALESCE(exit_page, ''), COALESCE(user_agent, ''),
		COALESCE(device_type, ''), COALESCE(browser, ''), COALESCE(os, '')
	FROM rum_sessions
	ORDER BY start_time DESC
	LIMIT $1 OFFSET $2`

	rows, err := s.db.Query(query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		session := Session{}
		var endTime sql.NullTime

		err := rows.Scan(
			&session.ID, &session.VisitorUUID, &session.SessionID, &session.StartTime, &endTime,
			&session.PageViews, &session.DurationMs, &session.Referrer, &session.EntryPage,
			&session.ExitPage, &session.UserAgent, &session.DeviceType, &session.Browser, &session.OS,
		)
		if err != nil {
			return nil, err
		}

		if endTime.Valid {
			session.EndTime = &endTime.Time
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// parseUserAgent is a simple user agent parser (can be enhanced with a proper library)
func parseUserAgent(ua string) (deviceType, browser, os string) {
	deviceType = "desktop"
	browser = "Unknown"
	os = "Unknown"

	// Simple device detection
	if contains(ua, "Mobile") || contains(ua, "Android") || contains(ua, "iPhone") {
		deviceType = "mobile"
	} else if contains(ua, "Tablet") || contains(ua, "iPad") {
		deviceType = "tablet"
	}

	// Simple browser detection
	if contains(ua, "Chrome") && !contains(ua, "Edge") {
		browser = "Chrome"
	} else if contains(ua, "Firefox") {
		browser = "Firefox"
	} else if contains(ua, "Safari") && !contains(ua, "Chrome") {
		browser = "Safari"
	} else if contains(ua, "Edge") {
		browser = "Edge"
	}

	// Simple OS detection
	if contains(ua, "Windows") {
		os = "Windows"
	} else if contains(ua, "Mac OS") || contains(ua, "Macintosh") {
		os = "macOS"
	} else if contains(ua, "Linux") && !contains(ua, "Android") {
		os = "Linux"
	} else if contains(ua, "Android") {
		os = "Android"
	} else if contains(ua, "iOS") || contains(ua, "iPhone") || contains(ua, "iPad") {
		os = "iOS"
	}

	return
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
