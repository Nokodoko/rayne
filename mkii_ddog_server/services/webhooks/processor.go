package webhooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Processor handles async webhook processing
type Processor struct {
	storage      *Storage
	downtimeSvc  *DowntimeService
}

// NewProcessor creates a new webhook processor
func NewProcessor(storage *Storage, downtimeSvc *DowntimeService) *Processor {
	return &Processor{
		storage:      storage,
		downtimeSvc:  downtimeSvc,
	}
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
