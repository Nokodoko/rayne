package rum

import "time"

// Visitor represents a unique visitor tracked by RUM
type Visitor struct {
	ID           int64     `json:"id"`
	UUID         string    `json:"uuid"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	SessionCount int       `json:"session_count"`
	TotalViews   int       `json:"total_views"`
	UserAgent    string    `json:"user_agent,omitempty"`
	IPHash       string    `json:"ip_hash,omitempty"`
	Country      string    `json:"country,omitempty"`
	City         string    `json:"city,omitempty"`
}

// Session represents a visitor session
type Session struct {
	ID          int64      `json:"id"`
	VisitorUUID string     `json:"visitor_uuid"`
	SessionID   string     `json:"session_id"`
	StartTime   time.Time  `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	PageViews   int        `json:"page_views"`
	DurationMs  int64      `json:"duration_ms"`
	Referrer    string     `json:"referrer,omitempty"`
	EntryPage   string     `json:"entry_page,omitempty"`
	ExitPage    string     `json:"exit_page,omitempty"`
	UserAgent   string     `json:"user_agent,omitempty"`
	DeviceType  string     `json:"device_type,omitempty"`
	Browser     string     `json:"browser,omitempty"`
	OS          string     `json:"os,omitempty"`
}

// RUMEvent represents a Real User Monitoring event
type RUMEvent struct {
	ID          int64                  `json:"id,omitempty"`
	VisitorUUID string                 `json:"visitor_uuid"`
	SessionID   string                 `json:"session_id"`
	EventType   string                 `json:"event_type"` // "view", "action", "error", "resource", "long_task"
	Timestamp   time.Time              `json:"timestamp"`
	PageURL     string                 `json:"page_url,omitempty"`
	PageTitle   string                 `json:"page_title,omitempty"`
	ActionName  string                 `json:"action_name,omitempty"`
	ActionType  string                 `json:"action_type,omitempty"`
	ErrorMsg    string                 `json:"error_message,omitempty"`
	Duration    int64                  `json:"duration_ms,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// VisitorInitRequest is sent when initializing a visitor
type VisitorInitRequest struct {
	ExistingUUID string `json:"existing_uuid,omitempty"`
	UserAgent    string `json:"user_agent,omitempty"`
	Referrer     string `json:"referrer,omitempty"`
	EntryPage    string `json:"entry_page,omitempty"`
}

// VisitorInitResponse is returned when initializing a visitor
type VisitorInitResponse struct {
	VisitorUUID string `json:"visitor_uuid"`
	SessionID   string `json:"session_id"`
	IsNew       bool   `json:"is_new"`
	Message     string `json:"message,omitempty"`
}

// TrackEventRequest is sent when tracking a RUM event
type TrackEventRequest struct {
	VisitorUUID string                 `json:"visitor_uuid"`
	SessionID   string                 `json:"session_id"`
	EventType   string                 `json:"event_type"`
	PageURL     string                 `json:"page_url,omitempty"`
	PageTitle   string                 `json:"page_title,omitempty"`
	ActionName  string                 `json:"action_name,omitempty"`
	ActionType  string                 `json:"action_type,omitempty"`
	ErrorMsg    string                 `json:"error_message,omitempty"`
	Duration    int64                  `json:"duration_ms,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SessionEndRequest is sent when ending a session
type SessionEndRequest struct {
	VisitorUUID string `json:"visitor_uuid"`
	SessionID   string `json:"session_id"`
	ExitPage    string `json:"exit_page,omitempty"`
}

// VisitorAnalytics contains analytics data for visitors
type VisitorAnalytics struct {
	UniqueVisitors    int                    `json:"unique_visitors"`
	TotalSessions     int                    `json:"total_sessions"`
	TotalPageViews    int                    `json:"total_page_views"`
	AvgSessionDuration float64               `json:"avg_session_duration_ms"`
	NewVisitors       int                    `json:"new_visitors"`
	ReturningVisitors int                    `json:"returning_visitors"`
	TopPages          []PageStat             `json:"top_pages,omitempty"`
	ByDevice          map[string]int         `json:"by_device,omitempty"`
	ByBrowser         map[string]int         `json:"by_browser,omitempty"`
	ByCountry         map[string]int         `json:"by_country,omitempty"`
	Period            string                 `json:"period"`
}

// PageStat represents page view statistics
type PageStat struct {
	PageURL   string `json:"page_url"`
	PageTitle string `json:"page_title,omitempty"`
	Views     int    `json:"views"`
}

// TimeRange represents a time range for analytics queries
type TimeRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}
