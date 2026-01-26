package types

import "time"

// AlertPayload contains the data from an incoming alert/webhook
// This is the shared type that both webhooks and agents packages use
type AlertPayload struct {
	// Standard Datadog fields
	AlertID       int64    `json:"alert_id"`
	AlertTitle    string   `json:"alert_title"`
	AlertMessage  string   `json:"alert_message"`
	AlertStatus   string   `json:"alert_status"` // "Alert", "OK", "Warn", "No Data"
	MonitorID     int64    `json:"monitor_id"`
	MonitorName   string   `json:"monitor_name"`
	MonitorType   string   `json:"monitor_type"`
	Tags          []string `json:"tags"`
	Timestamp     int64    `json:"timestamp"`
	EventType     string   `json:"event_type"`
	Priority      string   `json:"priority"`
	Hostname      string   `json:"hostname"`
	Service       string   `json:"service"`
	Scope         string   `json:"scope"`
	TransitionID  string   `json:"transition_id"`
	LastUpdated   int64    `json:"last_updated"`
	SnapshotURL   string   `json:"snapshot_url"`
	Link          string   `json:"link"`
	OrgID         int64    `json:"org_id"`
	OrgName       string   `json:"org_name"`

	// Custom fields from Terraform webhook config
	AlertState          string `json:"ALERT_STATE"`
	AlertTitleCustom    string `json:"ALERT_TITLE"`
	ApplicationLongname string `json:"APPLICATION_LONGNAME"`
	ApplicationTeam     string `json:"APPLICATION_TEAM"`
	DetailedDescription string `json:"DETAILED_DESCRIPTION"`
	Impact              string `json:"IMPACT"`
	Metric              string `json:"METRIC"`
	SupportGroup        string `json:"SUPPORT_GROUP"`
	Threshold           string `json:"THRESHOLD"`
	Value               string `json:"VALUE"`
	Urgency             string `json:"URGENCY"`
}

// AlertEvent represents a stored alert event
type AlertEvent struct {
	ID          int64        `json:"id"`
	Payload     AlertPayload `json:"payload"`
	ReceivedAt  time.Time    `json:"received_at"`
	ProcessedAt *time.Time   `json:"processed_at,omitempty"`
	Status      string       `json:"status"`
	ForwardedTo []string     `json:"forwarded_to,omitempty"`
	Error       string       `json:"error,omitempty"`
}
