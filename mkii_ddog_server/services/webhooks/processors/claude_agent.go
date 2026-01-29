package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/httpclient"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/keys"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// ClaudeAgentProcessor invokes the Claude AI agent for RCA analysis
// NOTE: This processor is deprecated in favor of the agent orchestrator.
// It remains for backwards compatibility with legacy configurations.
type ClaudeAgentProcessor struct {
	agentURL string
	client   *http.Client
	accounts CredentialProvider
}

// NewClaudeAgentProcessor creates a new Claude agent processor
func NewClaudeAgentProcessor() *ClaudeAgentProcessor {
	url := os.Getenv("CLAUDE_AGENT_URL")
	if url == "" {
		url = "http://localhost:9000"
	}
	return &ClaudeAgentProcessor{
		agentURL: url,
		client:   httpclient.AgentClient, // Use shared client with connection pooling
	}
}

// NewClaudeAgentProcessorWithAccounts creates a Claude agent processor with multi-account support
func NewClaudeAgentProcessorWithAccounts(accounts CredentialProvider) *ClaudeAgentProcessor {
	url := os.Getenv("CLAUDE_AGENT_URL")
	if url == "" {
		url = "http://localhost:9000"
	}
	return &ClaudeAgentProcessor{
		agentURL: url,
		client:   httpclient.AgentClient,
		accounts: accounts,
	}
}

// Name returns the processor identifier
func (p *ClaudeAgentProcessor) Name() string {
	return "claude_agent"
}

// CanProcess returns true for Alert or Warn statuses (incidents needing RCA)
func (p *ClaudeAgentProcessor) CanProcess(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) bool {
	status := event.Payload.AlertStatus
	return status == "Alert" || status == "Warn"
}

// Process invokes the Claude agent for root cause analysis
func (p *ClaudeAgentProcessor) Process(event *webhooks.WebhookEvent, config *webhooks.WebhookConfig) webhooks.ProcessorResult {
	result := webhooks.ProcessorResult{
		ProcessorName: p.Name(),
	}

	analysis, err := p.invokeAgent(event)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result
	}

	result.Success = true
	// Truncate for result message
	preview := analysis
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	result.Message = fmt.Sprintf("RCA analysis completed: %s", preview)
	return result
}

// getCredentials returns credentials for the event's account or default
func (p *ClaudeAgentProcessor) getCredentials(event *webhooks.WebhookEvent) keys.Credentials {
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

// invokeAgent calls the Claude agent sidecar with full payload
func (p *ClaudeAgentProcessor) invokeAgent(event *webhooks.WebhookEvent) (string, error) {
	// Get credentials for this event's account
	creds := p.getCredentials(event)

	req := claudeAnalysisRequest{
		Payload:      event.Payload,
		TemplateID:   "incident_report",
		Instructions: "Use the incident_report.json template to structure your analysis. You have access to dd_lib tools for querying Datadog APIs. Create new dd_lib functions if needed for analysis.",
		Credentials: &agentCredentials{
			APIKey:  creds.APIKey,
			AppKey:  creds.AppKey,
			BaseURL: creds.BaseURL,
		},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := p.client.Post(p.agentURL+"/analyze", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var response claudeAnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	if response.Error != "" {
		return "", fmt.Errorf("agent error: %s", response.Error)
	}

	return response.Analysis, nil
}

// agentCredentials holds credentials to pass to the Claude agent sidecar
type agentCredentials struct {
	APIKey  string `json:"api_key"`
	AppKey  string `json:"app_key"`
	BaseURL string `json:"base_url"`
}

// Request/response types for Claude agent
type claudeAnalysisRequest struct {
	Payload      webhooks.WebhookPayload `json:"payload"`
	TemplateID   string                  `json:"template_id"`
	Instructions string                  `json:"instructions"`
	Credentials  *agentCredentials       `json:"credentials,omitempty"`
}

type claudeAnalysisResponse struct {
	Success   bool   `json:"success"`
	MonitorID int64  `json:"monitorId"`
	Analysis  string `json:"analysis"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}
