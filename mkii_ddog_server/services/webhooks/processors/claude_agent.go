package processors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/httpclient"
	"github.com/Nokodoko/mkii_ddog_server/services/webhooks"
)

// ClaudeAgentProcessor invokes the Claude AI agent for RCA analysis
// NOTE: This processor is deprecated in favor of the agent orchestrator.
// It remains for backwards compatibility with legacy configurations.
type ClaudeAgentProcessor struct {
	agentURL string
	client   *http.Client
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

// invokeAgent calls the Claude agent sidecar with full payload
func (p *ClaudeAgentProcessor) invokeAgent(event *webhooks.WebhookEvent) (string, error) {
	req := claudeAnalysisRequest{
		Payload:      event.Payload,
		TemplateID:   "incident_report",
		Instructions: "Use the incident_report.json template to structure your analysis. You have access to dd_lib tools for querying Datadog APIs. Create new dd_lib functions if needed for analysis.",
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := p.client.Post(p.agentURL+"/analyze", "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var response claudeAnalysisResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %v", err)
	}

	if response.Error != "" {
		return "", fmt.Errorf("agent error: %s", response.Error)
	}

	return response.Analysis, nil
}

// Request/response types for Claude agent
type claudeAnalysisRequest struct {
	Payload      webhooks.WebhookPayload `json:"payload"`
	TemplateID   string                  `json:"template_id"`
	Instructions string                  `json:"instructions"`
}

type claudeAnalysisResponse struct {
	Success   bool   `json:"success"`
	MonitorID int64  `json:"monitorId"`
	Analysis  string `json:"analysis"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
}
