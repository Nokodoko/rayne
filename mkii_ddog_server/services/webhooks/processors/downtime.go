package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/httpclient"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/keys"
	"github.com/Nokodoko/mkii_ddog_server/services/accounts"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// CredentialProvider interface at consumer side (interface ownership)
type CredentialProvider interface {
	GetByID(id int64) (*accounts.Account, error)
	GetDefault() *accounts.Account
}

// DowntimeProcessor creates automatic downtimes when monitors recover
type DowntimeProcessor struct {
	client   *http.Client
	accounts CredentialProvider
}

// NewDowntimeProcessor creates a new downtime processor (backward compatible)
func NewDowntimeProcessor() *DowntimeProcessor {
	return &DowntimeProcessor{
		client: httpclient.DatadogClient, // Use shared client with connection pooling
	}
}

// NewDowntimeProcessorWithAccounts creates a downtime processor with multi-account support
func NewDowntimeProcessorWithAccounts(accounts CredentialProvider) *DowntimeProcessor {
	return &DowntimeProcessor{
		client:   httpclient.DatadogClient,
		accounts: accounts,
	}
}

// Name returns the processor identifier
func (p *DowntimeProcessor) Name() string {
	return "downtime"
}

// CanProcess returns true if auto-downtime is enabled and monitor recovered (OK status)
func (p *DowntimeProcessor) CanProcess(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) bool {
	return config != nil &&
		config.AutoDowntime &&
		event.Payload.AlertStatus == "OK"
}

// Process creates a downtime for the recovered monitor
func (p *DowntimeProcessor) Process(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) webhooks.ProcessorResult {
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
	}

	duration := config.DowntimeDuration
	if duration == 0 {
		duration = 120 // Default 2 hours
	}

	// Get credentials for this event's account
	creds := p.getCredentials(event)

	err := p.createDowntime(event.Payload.MonitorID, event.Payload.Scope, duration, creds)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("created %d minute downtime for monitor %d", duration, event.Payload.MonitorID)
	return result
}

// getCredentials returns credentials for the event's account or default
func (p *DowntimeProcessor) getCredentials(event *webhooks.WebhookEvent) keys.Credentials {
	// If no account provider or no account ID, use default credentials
	if p.accounts == nil || event.AccountID == nil {
		return keys.Default()
	}

	// Try to get account-specific credentials
	account, err := p.accounts.GetByID(*event.AccountID)
	if err != nil || account == nil {
		// Fall back to default account
		account = p.accounts.GetDefault()
	}
	if account == nil {
		return keys.Default()
	}

	return keys.Credentials{
		APIKey:  account.APIKey,
		AppKey:  account.AppKey,
		BaseURL: account.BaseURL,
	}
}

// createDowntime creates a downtime via Datadog API
func (p *DowntimeProcessor) createDowntime(monitorID int64, scope string, durationMinutes int, creds keys.Credentials) error {
	now := time.Now().UTC()
	end := now.Add(time.Duration(durationMinutes) * time.Minute)

	request := downtimeRequest{
		Data: downtimeData{
			Type: "downtime",
			Attributes: downtimeAttributes{
				Message: fmt.Sprintf("Auto-created downtime after monitor recovery (ID: %d)", monitorID),
				MonitorIdentifier: monitorIdentifier{
					MonitorID: monitorID,
				},
				Scope: formatScope(scope),
				Schedule: downtimeSchedule{
					Start: now.Format(time.RFC3339),
					End:   end.Format(time.RFC3339),
				},
			},
		},
	}

	jsonBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	// Build API URL from account's base URL
	apiURL := creds.BuildURL(accounts.PathDowntime)

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("DD-API-KEY", creds.APIKey)
	req.Header.Set("DD-APPLICATION-KEY", creds.AppKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// formatScope converts comma-separated scope to Datadog format
func formatScope(scope string) string {
	if scope == "" {
		return "*"
	}

	parts := strings.Split(scope, ",")
	var cleanParts []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleanParts = append(cleanParts, part)
		}
	}

	if len(cleanParts) == 0 {
		return "*"
	}

	return strings.Join(cleanParts, " AND ")
}

// Datadog API types
type downtimeRequest struct {
	Data downtimeData `json:"data"`
}

type downtimeData struct {
	Type       string             `json:"type"`
	Attributes downtimeAttributes `json:"attributes"`
}

type downtimeAttributes struct {
	Message           string            `json:"message,omitempty"`
	MonitorIdentifier monitorIdentifier `json:"monitor_identifier"`
	Scope             string            `json:"scope"`
	Schedule          downtimeSchedule  `json:"schedule"`
}

type monitorIdentifier struct {
	MonitorID int64 `json:"monitor_id"`
}

type downtimeSchedule struct {
	Start string `json:"start"`
	End   string `json:"end"`
}
