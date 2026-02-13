package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// FailureAlerter creates Datadog events when agent analysis fails.
// This provides visibility into pipeline failures even when the sidecar
// itself was unreachable or returned an error.
type FailureAlerter struct {
	enabled    bool
	apiKey     string
	appKey     string
	apiURL     string
	httpClient *http.Client
}

// NewFailureAlerter creates a FailureAlerter if DD keys are available
func NewFailureAlerter() *FailureAlerter {
	apiKey := os.Getenv("DD_API_KEY")
	appKey := os.Getenv("DD_APP_KEY")

	ddSite := os.Getenv("DD_SITE")
	if ddSite == "" {
		ddSite = "ddog-gov.com"
	}
	apiURL := fmt.Sprintf("https://api.%s", ddSite)

	return &FailureAlerter{
		enabled: apiKey != "" && appKey != "",
		apiKey:  apiKey,
		appKey:  appKey,
		apiURL:  apiURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// datadogEvent represents the Datadog Events API payload
type datadogEvent struct {
	Title          string   `json:"title"`
	Text           string   `json:"text"`
	Priority       string   `json:"priority"`
	Tags           []string `json:"tags"`
	AlertType      string   `json:"alert_type"`
	SourceTypeName string   `json:"source_type_name"`
}

// ReportFailure creates a Datadog event recording an agent analysis failure.
// It is best-effort: errors are logged but not propagated.
func (fa *FailureAlerter) ReportFailure(ctx context.Context, result *AnalysisResult, err error) {
	if !fa.enabled {
		log.Printf("[FAILURE-ALERTER] Skipping - DD keys not configured")
		return
	}

	monitorID := int64(0)
	monitorName := "Unknown"
	agentRole := AgentRole("unknown")
	if result != nil {
		monitorID = result.MonitorID
		monitorName = result.MonitorName
		agentRole = result.AgentRole
	}

	errMsg := "unknown error"
	if err != nil {
		errMsg = err.Error()
	} else if result != nil && result.Error != "" {
		errMsg = result.Error
	}

	title := fmt.Sprintf("[Agent Analysis Failure] %s", monitorName)
	text := fmt.Sprintf("## Agent Analysis Failure\n\n"+
		"**Monitor:** %s (ID: %d)\n"+
		"**Agent Role:** %s\n"+
		"**Error:** %s\n"+
		"**Timestamp:** %s\n",
		monitorName, monitorID, agentRole, errMsg, time.Now().UTC().Format(time.RFC3339))

	event := datadogEvent{
		Title:    title,
		Text:     text,
		Priority: "normal",
		Tags: []string{
			"service:rayne",
			"source:agent_orchestrator",
			fmt.Sprintf("monitor_id:%d", monitorID),
			fmt.Sprintf("agent_role:%s", agentRole),
		},
		AlertType:      "error",
		SourceTypeName: "custom",
	}

	payload, jsonErr := json.Marshal(event)
	if jsonErr != nil {
		log.Printf("[FAILURE-ALERTER] Failed to marshal event: %v", jsonErr)
		return
	}

	url := fmt.Sprintf("%s/api/v1/events", fa.apiURL)
	req, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(payload))
	if reqErr != nil {
		log.Printf("[FAILURE-ALERTER] Failed to create request: %v", reqErr)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", fa.apiKey)
	req.Header.Set("DD-APPLICATION-KEY", fa.appKey)

	resp, doErr := fa.httpClient.Do(req)
	if doErr != nil {
		log.Printf("[FAILURE-ALERTER] Failed to send event: %v", doErr)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 || resp.StatusCode == 202 {
		log.Printf("[FAILURE-ALERTER] Event created for monitor %d (%s)", monitorID, monitorName)
	} else {
		log.Printf("[FAILURE-ALERTER] API returned %d for monitor %d", resp.StatusCode, monitorID)
	}
}
