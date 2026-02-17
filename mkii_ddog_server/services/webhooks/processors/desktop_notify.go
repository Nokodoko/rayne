package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

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

// DesktopNotifyProcessor sends notifications to local desktop notification servers
type DesktopNotifyProcessor struct {
	serverURLs []string
	client     *http.Client
}

// NewDesktopNotifyProcessor creates a new desktop notification processor
// Supports multiple servers via comma-separated NOTIFY_SERVER_URLS or single NOTIFY_SERVER_URL
func NewDesktopNotifyProcessor() *DesktopNotifyProcessor {
	var urls []string

	// Check for multiple URLs first
	if urlList := os.Getenv("NOTIFY_SERVER_URLS"); urlList != "" {
		for _, u := range splitAndTrim(urlList, ",") {
			if u != "" {
				urls = append(urls, u)
			}
		}
	}

	// Fallback to single URL
	if len(urls) == 0 {
		url := os.Getenv("NOTIFY_SERVER_URL")
		if url == "" {
			url = "http://host.minikube.internal:9999"
		}
		urls = append(urls, url)
	}

	return &DesktopNotifyProcessor{
		serverURLs: urls,
		client:     &http.Client{Timeout: 5 * time.Second}, // Simple client without tracing
	}
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := make([]string, 0)
	for _, p := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
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
	log.Printf("[NOTIFY-PROC] Processing event %d", event.ID)
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
	}

	// Classify the monitor type for the notification
	monitorType := classifyForNotification(&event.Payload)

	// Forward the full custom payload to notify-server
	err := p.sendNotification(event.Payload, monitorType)
	if err != nil {
		log.Printf("[NOTIFY-PROC] Error sending notification: %v", err)
		result.Success = false
		result.Error = err.Error()
		return result
	}
	log.Printf("[NOTIFY-PROC] Notification sent successfully")

	title := resolveTitle(event.Payload)

	result.Success = true
	result.Message = fmt.Sprintf("notification sent: %s [%s]", title, monitorType)
	return result
}

// classifyForNotification determines the monitor type label for desktop notifications.
func classifyForNotification(payload *webhooks.WebhookPayload) string {
	if webhooks.IsWatchdogMonitor(payload) {
		return "watchdog"
	}

	mt := strings.ToLower(strings.TrimSpace(payload.MonitorType))
	if mt != "" {
		return mt
	}

	return "alert_monitor"
}

// resolveTitle extracts the best available title from a webhook payload.
// Watchdog alerts arrive with empty standard fields, so we fall back through
// several fields until we find content.
func resolveTitle(p webhooks.WebhookPayload) string {
	if p.MonitorName != "" {
		return p.MonitorName
	}
	if p.AlertTitleCustom != "" {
		return p.AlertTitleCustom
	}
	if p.AlertTitle != "" {
		return p.AlertTitle
	}
	if p.DetailedDescription != "" {
		// Use first line only — DETAILED_DESCRIPTION can be very long
		if i := strings.IndexByte(p.DetailedDescription, '\n'); i > 0 {
			return p.DetailedDescription[:i]
		}
		return p.DetailedDescription
	}
	return "Datadog Webhook"
}

// sendNotification sends the notification to all configured servers
func (p *DesktopNotifyProcessor) sendNotification(webhookPayload webhooks.WebhookPayload, monitorType string) error {
	log.Printf("[NOTIFY] Sending to %d servers: %v", len(p.serverURLs), p.serverURLs)

	title := resolveTitle(webhookPayload)

	// Include monitor type in the notification title so users can distinguish webhook types
	payload := map[string]string{
		"title":   fmt.Sprintf("💣 [%s] %s", monitorType, title),
		"message": fmt.Sprintf("[%s] %s", monitorType, title),
		"urgency": "critical",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	// Send to all configured servers
	var lastErr error
	successCount := 0
	for _, serverURL := range p.serverURLs {
		log.Printf("[NOTIFY] Sending to: %s", serverURL)
		resp, err := p.client.Post(serverURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			log.Printf("[NOTIFY] Error sending to %s: %v", serverURL, err)
			lastErr = fmt.Errorf("request to %s failed: %v", serverURL, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Printf("[NOTIFY] Server %s returned HTTP %d", serverURL, resp.StatusCode)
			lastErr = fmt.Errorf("server %s returned HTTP %d", serverURL, resp.StatusCode)
			continue
		}
		log.Printf("[NOTIFY] Success from %s", serverURL)
		successCount++
	}

	// Return error only if all servers failed
	if successCount == 0 && lastErr != nil {
		return lastErr
	}

	return nil
}
