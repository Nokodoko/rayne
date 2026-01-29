package webhooks

import "time"

// WebhookProcessor defines the interface for webhook processing plugins.
// Implement this interface to add new webhook processing capabilities
// (e.g., Slack notifications, PagerDuty integration, custom forwarding).
type WebhookProcessor interface {
	// Name returns a unique identifier for this processor
	Name() string

	// CanProcess determines if this processor should handle the given event.
	// Return true if this processor is relevant for the event based on
	// alert status, tags, monitor type, or other criteria.
	CanProcess(event *WebhookEvent, config *WebhookConfig) bool

	// Process handles the webhook event. It should perform its specific
	// action (forward, notify, create downtime, etc.) and return a result.
	// Processing happens asynchronously - errors are logged but don't block other processors.
	Process(event *WebhookEvent, config *WebhookConfig) ProcessorResult
}

// ProcessorResult contains the outcome of a processor's execution
type ProcessorResult struct {
	ProcessorName string   `json:"processor_name"`
	Success       bool     `json:"success"`
	Message       string   `json:"message,omitempty"`
	Error         string   `json:"error,omitempty"`
	ForwardedTo   []string `json:"forwarded_to,omitempty"` // URLs that received the webhook
}

// WebhookPayload represents the incoming webhook data from Datadog
type WebhookPayload struct {
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

// WebhookEvent represents a stored webhook event
type WebhookEvent struct {
	ID          int64          `json:"id"`
	Payload     WebhookPayload `json:"payload"`
	ReceivedAt  time.Time      `json:"received_at"`
	ProcessedAt *time.Time     `json:"processed_at,omitempty"`
	Status      string         `json:"status"` // "pending", "processing", "processed", "failed"
	ForwardedTo []string       `json:"forwarded_to,omitempty"`
	Error       string         `json:"error,omitempty"`
	AccountID   *int64         `json:"account_id,omitempty"`
	AccountName string         `json:"account_name,omitempty"`
}

// WebhookConfig represents configuration for a webhook endpoint
type WebhookConfig struct {
	ID               int64    `json:"id"`
	Name             string   `json:"name"`
	URL              string   `json:"url"`
	UseCustomPayload bool     `json:"use_custom_payload"`
	CustomPayload    string   `json:"custom_payload,omitempty"`
	ForwardURLs      []string `json:"forward_urls,omitempty"`
	AutoDowntime     bool     `json:"auto_downtime"`
	DowntimeDuration int      `json:"downtime_duration_minutes,omitempty"` // Duration in minutes
	NotifyEnabled    bool     `json:"notify_enabled"`
	NotifyNumbers    []string `json:"notify_numbers,omitempty"`
	Active           bool     `json:"active"`
	CreatedAt        time.Time `json:"created_at"`
}

// CreateWebhookRequest represents a request to create a webhook in Datadog
type CreateWebhookRequest struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	UseCustomPayload bool   `json:"use_custom_payload"`
	CustomPayload    string `json:"custom_payload,omitempty"`
}

// WebhookResponse represents the response from creating a webhook
type WebhookResponse struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	UseCustomPayload bool   `json:"use_custom_payload"`
}

// WebhookEventListResponse represents a list of webhook events
type WebhookEventListResponse struct {
	Events     []WebhookEvent `json:"events"`
	TotalCount int            `json:"total_count"`
	Page       int            `json:"page"`
	PerPage    int            `json:"per_page"`
}
