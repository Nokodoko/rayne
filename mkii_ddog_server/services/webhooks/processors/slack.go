package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// SlackProcessor sends webhook notifications to Slack channels.
//
// EXAMPLE TEMPLATE: Copy and modify this file to create new integrations
// (PagerDuty, Discord, Teams, OpsGenie, etc.)
//
// To enable, add to the processor registry in cmd/api/api.go:
//
//	slackProc := processors.NewSlackProcessor()
//	webhookProcessor.Register(slackProc)
//
// Environment variables:
//
//	SLACK_WEBHOOK_URL - Slack incoming webhook URL
//	SLACK_CHANNEL     - Optional channel override (default: webhook default)
type SlackProcessor struct {
	webhookURL string
	channel    string
	client     *http.Client
}

// NewSlackProcessor creates a new Slack notification processor
func NewSlackProcessor() *SlackProcessor {
	return &SlackProcessor{
		webhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
		channel:    os.Getenv("SLACK_CHANNEL"),
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// NewSlackProcessorWithConfig creates a Slack processor with explicit configuration
func NewSlackProcessorWithConfig(webhookURL, channel string) *SlackProcessor {
	return &SlackProcessor{
		webhookURL: webhookURL,
		channel:    channel,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Name returns the processor identifier
func (p *SlackProcessor) Name() string {
	return "slack"
}

// CanProcess returns true if Slack is configured and event is actionable
func (p *SlackProcessor) CanProcess(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) bool {
	// Only process if Slack webhook URL is configured
	if p.webhookURL == "" {
		return false
	}

	// Process Alert and Warn statuses (customize as needed)
	status := event.Payload.AlertStatus
	return status == "Alert" || status == "Warn" || status == "OK"
}

// Process sends the webhook event to Slack
func (p *SlackProcessor) Process(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) webhooks.ProcessorResult {
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
	}

	message := p.buildSlackMessage(event)

	err := p.sendToSlack(message)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	result.Success = true
	result.Message = fmt.Sprintf("sent to Slack: %s", event.Payload.MonitorName)
	result.ForwardedTo = []string{p.webhookURL}
	return result
}

// buildSlackMessage creates a Slack message payload from the webhook event
func (p *SlackProcessor) buildSlackMessage(event *webhooks.WebhookEvent) slackMessage {
	// Choose color based on alert status
	color := "#36a64f" // green for OK
	switch event.Payload.AlertStatus {
	case "Alert":
		color = "#ff0000" // red
	case "Warn":
		color = "#ffcc00" // yellow
	case "No Data":
		color = "#808080" // gray
	}

	// Build attachment
	attachment := slackAttachment{
		Color:      color,
		Title:      event.Payload.MonitorName,
		TitleLink:  event.Payload.Link,
		Text:       event.Payload.AlertMessage,
		Footer:     "Datadog via Rayne",
		FooterIcon: "https://www.datadoghq.com/favicon.ico",
		Timestamp:  event.Payload.Timestamp,
		Fields: []slackField{
			{Title: "Status", Value: event.Payload.AlertStatus, Short: true},
			{Title: "Monitor ID", Value: fmt.Sprintf("%d", event.Payload.MonitorID), Short: true},
			{Title: "Hostname", Value: event.Payload.Hostname, Short: true},
			{Title: "Service", Value: event.Payload.Service, Short: true},
		},
	}

	// Add scope if present
	if event.Payload.Scope != "" {
		attachment.Fields = append(attachment.Fields, slackField{
			Title: "Scope",
			Value: event.Payload.Scope,
			Short: false,
		})
	}

	msg := slackMessage{
		Attachments: []slackAttachment{attachment},
	}

	// Override channel if configured
	if p.channel != "" {
		msg.Channel = p.channel
	}

	return msg
}

// sendToSlack posts the message to the Slack webhook
func (p *SlackProcessor) sendToSlack(message slackMessage) error {
	jsonBody, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %v", err)
	}

	resp, err := p.client.Post(p.webhookURL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("Slack API returned HTTP %d", resp.StatusCode)
	}

	return nil
}

// Slack API types
type slackMessage struct {
	Channel     string            `json:"channel,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []slackAttachment `json:"attachments,omitempty"`
}

type slackAttachment struct {
	Color      string       `json:"color,omitempty"`
	Title      string       `json:"title,omitempty"`
	TitleLink  string       `json:"title_link,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []slackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
}

type slackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}
