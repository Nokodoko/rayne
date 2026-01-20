package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// ForwardingProcessor forwards webhook events to configured URLs
type ForwardingProcessor struct {
	client *http.Client
}

// NewForwardingProcessor creates a new forwarding processor
func NewForwardingProcessor() *ForwardingProcessor {
	return &ForwardingProcessor{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name returns the processor identifier
func (p *ForwardingProcessor) Name() string {
	return "forwarding"
}

// CanProcess returns true if there are URLs configured for forwarding
func (p *ForwardingProcessor) CanProcess(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) bool {
	return config != nil && len(config.ForwardURLs) > 0
}

// Process forwards the webhook payload to all configured URLs
func (p *ForwardingProcessor) Process(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) webhooks.ProcessorResult {
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
		Success:       true,
	}

	var forwardedTo []string
	var errors []string

	for _, url := range config.ForwardURLs {
		if err := p.forwardToURL(event.Payload, url); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", url, err))
		} else {
			forwardedTo = append(forwardedTo, url)
		}
	}

	result.ForwardedTo = forwardedTo

	if len(errors) > 0 {
		result.Error = fmt.Sprintf("some forwards failed: %v", errors)
		if len(forwardedTo) == 0 {
			result.Success = false
		}
	}

	if len(forwardedTo) > 0 {
		result.Message = fmt.Sprintf("forwarded to %d URLs", len(forwardedTo))
	}

	return result
}

// forwardToURL sends the webhook payload to a single URL
func (p *ForwardingProcessor) forwardToURL(payload webhooks.WebhookPayload, url string) error {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	resp, err := p.client.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}
