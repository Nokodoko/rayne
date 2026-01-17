package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// ClaudeAnalysisRequest is the request payload for the Claude agent
type ClaudeAnalysisRequest struct {
	MonitorID   int64    `json:"monitorId"`
	AlertStatus string   `json:"alertStatus"`
	MonitorName string   `json:"monitorName"`
	Scope       string   `json:"scope"`
	Tags        []string `json:"tags,omitempty"`
}

// ClaudeAnalysisResponse is the response from the Claude agent
type ClaudeAnalysisResponse struct {
	Success   bool   `json:"success"`
	MonitorID int64  `json:"monitorId"`
	Analysis  string `json:"analysis"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}

// Processor handles async webhook processing
type Processor struct {
	storage        *Storage
	downtimeSvc    *DowntimeService
	claudeAgentURL string
}

// NewProcessor creates a new webhook processor
func NewProcessor(storage *Storage, downtimeSvc *DowntimeService) *Processor {
	claudeURL := os.Getenv("CLAUDE_AGENT_URL")
	if claudeURL == "" {
		claudeURL = "http://localhost:9000"
	}
	return &Processor{
		storage:        storage,
		downtimeSvc:    downtimeSvc,
		claudeAgentURL: claudeURL,
	}
}

// invokeClaudeAgent calls the sidecar Claude agent for RCA analysis
func (p *Processor) invokeClaudeAgent(event *WebhookEvent) (*ClaudeAnalysisResponse, error) {
	req := ClaudeAnalysisRequest{
		MonitorID:   event.Payload.MonitorID,
		AlertStatus: event.Payload.AlertStatus,
		MonitorName: event.Payload.MonitorName,
		Scope:       event.Payload.Scope,
		Tags:        event.Payload.Tags,
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	client := &http.Client{Timeout: 120 * time.Second} // Long timeout for AI analysis
	resp, err := client.Post(p.claudeAgentURL+"/analyze", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call claude agent: %v", err)
	}
	defer resp.Body.Close()

	var result ClaudeAnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("claude agent error: %s", result.Error)
	}

	return &result, nil
}

// Process handles a webhook event asynchronously
func (p *Processor) Process(event *WebhookEvent) {
	log.Printf("Processing webhook event ID: %d, Monitor: %s, Status: %s",
		event.ID, event.Payload.MonitorName, event.Payload.AlertStatus)

	var forwardedTo []string
	var processingError string

	// Get active configurations
	configs, err := p.storage.GetActiveConfigs()
	if err != nil {
		log.Printf("Error getting configs: %v", err)
		processingError = fmt.Sprintf("failed to get configs: %v", err)
		p.storage.UpdateEventStatus(event.ID, "failed", forwardedTo, processingError)
		return
	}

	// Process each active configuration
	for _, config := range configs {
		// Forward to configured URLs
		if len(config.ForwardURLs) > 0 {
			for _, url := range config.ForwardURLs {
				if err := p.forward(event.Payload, url); err != nil {
					log.Printf("Failed to forward to %s: %v", url, err)
				} else {
					forwardedTo = append(forwardedTo, url)
					log.Printf("Forwarded webhook to: %s", url)
				}
			}
		}

		// Trigger automatic downtime if configured and monitor recovered
		if config.AutoDowntime && event.Payload.AlertStatus == "OK" {
			duration := config.DowntimeDuration
			if duration == 0 {
				duration = 120 // Default 2 hours
			}
			if err := p.downtimeSvc.CreateForMonitor(event.Payload.MonitorID, event.Payload.Scope, duration); err != nil {
				log.Printf("Failed to create downtime: %v", err)
			} else {
				log.Printf("Created downtime for monitor %d", event.Payload.MonitorID)
			}
		}
	}

	// Invoke Claude agent for RCA analysis on Alert/Warn status
	if event.Payload.AlertStatus == "Alert" || event.Payload.AlertStatus == "Warn" {
		go func() {
			log.Printf("Invoking Claude agent for RCA analysis on event %d", event.ID)
			analysis, err := p.invokeClaudeAgent(event)
			if err != nil {
				log.Printf("Claude agent failed for event %d: %v", event.ID, err)
				return
			}
			// Truncate analysis for logging
			analysisPreview := analysis.Analysis
			if len(analysisPreview) > 200 {
				analysisPreview = analysisPreview[:200] + "..."
			}
			log.Printf("Claude analysis completed for event %d: %s", event.ID, analysisPreview)
			// TODO: Store analysis in vector DB, create notebook via Datadog API
		}()
	}

	// Update event status
	status := "processed"
	if processingError != "" {
		status = "failed"
	}
	p.storage.UpdateEventStatus(event.ID, status, forwardedTo, processingError)
}

// forward sends webhook payload to a URL
func (p *Processor) forward(payload WebhookPayload, url string) error {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to forward: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("forward failed with status: %d", resp.StatusCode)
	}

	return nil
}

// ProcessPending processes all pending webhook events
func (p *Processor) ProcessPending() error {
	events, _, err := p.storage.GetRecentEvents(100, 0)
	if err != nil {
		return err
	}

	for _, event := range events {
		if event.Status == "pending" {
			go p.Process(&event)
		}
	}

	return nil
}
