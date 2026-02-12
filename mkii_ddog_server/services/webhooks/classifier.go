package webhooks

import (
	"strings"
)

// ClassifyMonitorType determines the monitor classification for routing.
// Returns "watchdog" for Watchdog monitors, empty string for standard monitors.
// This is used by the orchestrator to route webhooks to the appropriate
// analysis endpoint (watchdog vs standard RCA).
func ClassifyMonitorType(payload *WebhookPayload) string {
	if IsWatchdogMonitor(payload) {
		return "watchdog"
	}
	return ""
}

// IsWatchdogMonitor determines if a webhook payload is from a Datadog Watchdog monitor.
// Watchdog is Datadog's AI-based anomaly detection that automatically detects
// irregularities across metrics, logs, and APM without manual configuration.
//
// Detection heuristics:
// - monitor_type field contains "watchdog"
// - monitor_name or alert_title contains "watchdog"
// - tags contain "source:watchdog", "monitor_type:watchdog", or "created_by:watchdog"
func IsWatchdogMonitor(payload *WebhookPayload) bool {
	mt := strings.ToLower(payload.MonitorType)
	if mt == "watchdog" || strings.Contains(mt, "watchdog") {
		return true
	}

	if strings.Contains(strings.ToLower(payload.MonitorName), "watchdog") {
		return true
	}

	if strings.Contains(strings.ToLower(payload.AlertTitle), "watchdog") {
		return true
	}

	// Check custom ALERT_TITLE field (Terraform webhook format)
	if strings.Contains(strings.ToLower(payload.AlertTitleCustom), "watchdog") {
		return true
	}

	for _, tag := range payload.Tags {
		tagLower := strings.ToLower(tag)
		if tagLower == "source:watchdog" || tagLower == "monitor_type:watchdog" || tagLower == "created_by:watchdog" {
			return true
		}
	}

	return false
}
