package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/httpclient"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// parseApplicationTeam extracts application_team from alert_title tags
// Format: "... on application_team:value,other_tag:value,..."
func parseApplicationTeam(alertTitle string) string {
	re := regexp.MustCompile(`application_team:([^,\s]+)`)
	matches := re.FindStringSubmatch(alertTitle)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// DesktopNotifyProcessor sends notifications to a local desktop notification server
type DesktopNotifyProcessor struct {
	serverURL string
	client    *http.Client
}

// NewDesktopNotifyProcessor creates a new desktop notification processor
func NewDesktopNotifyProcessor() *DesktopNotifyProcessor {
	url := os.Getenv("NOTIFY_SERVER_URL")
	if url == "" {
		url = "http://host.minikube.internal:9999"
	}
	return &DesktopNotifyProcessor{
		serverURL: url,
		client:    httpclient.NotifyClient, // Use shared client with connection pooling
	}
}

// Name returns the processor identifier
func (p *DesktopNotifyProcessor) Name() string {
	return "desktop_notify"
}

// CanProcess always returns true - we want notifications for all events
func (p *DesktopNotifyProcessor) CanProcess(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) bool {
	// Desktop notifications are always enabled (config-independent)
	// You could add a config flag like config.DesktopNotify if needed
	return true
}

// Process sends a desktop notification
func (p *DesktopNotifyProcessor) Process(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) webhooks.ProcessorResult {
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
	}

	// Forward the full custom payload to notify-server
	err := p.sendNotification(event.Payload)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	title := event.Payload.AlertTitleCustom
	if title == "" {
		title = event.Payload.MonitorName
	}
	if title == "" {
		title = "Datadog Webhook"
	}

	result.Success = true
	result.Message = fmt.Sprintf("notification sent: %s", title)
	return result
}

// sendNotification sends the notification to the server
func (p *DesktopNotifyProcessor) sendNotification(webhookPayload webhooks.WebhookPayload) error {
	// Get application_team - first try direct field, then parse from alert_title
	applicationTeam := webhookPayload.ApplicationTeam
	if applicationTeam == "" {
		// Try parsing from alert_title (format: "... application_team:value,...")
		applicationTeam = parseApplicationTeam(webhookPayload.AlertTitle)
	}
	if applicationTeam == "" {
		// Also try the custom alert title field
		applicationTeam = parseApplicationTeam(webhookPayload.AlertTitleCustom)
	}
	if applicationTeam == "" {
		applicationTeam = "unknown"
	}

	// Simple payload with bomb emoji and application team
	payload := map[string]string{
		"title":   "💣 " + applicationTeam,
		"message": applicationTeam,
		"urgency": "critical",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	resp, err := p.client.Post(p.serverURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned HTTP %d", resp.StatusCode)
	}

	return nil
}
