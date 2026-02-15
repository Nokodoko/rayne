package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils/httpclient"
)

// ClaudeAgent implements the Agent interface using the Claude AI sidecar
type ClaudeAgent struct {
	agentURL string
	role     AgentRole
	name     string
}

// NewClaudeAgent creates a new Claude-based agent for a specific role
func NewClaudeAgent(role AgentRole) *ClaudeAgent {
	url := os.Getenv("CLAUDE_AGENT_URL")
	if url == "" {
		url = "http://localhost:9000"
	}

	return &ClaudeAgent{
		agentURL: url,
		role:     role,
		name:     fmt.Sprintf("claude-%s", role),
	}
}

// NewDefaultClaudeAgent creates a general-purpose Claude agent
func NewDefaultClaudeAgent() *ClaudeAgent {
	return NewClaudeAgent(RoleGeneral)
}

// Name returns the agent's unique identifier
func (a *ClaudeAgent) Name() string {
	return a.name
}

// Role returns the agent's specialist role
func (a *ClaudeAgent) Role() AgentRole {
	return a.role
}

// Plan determines what queries are needed (for Claude, we do single-shot analysis)
func (a *ClaudeAgent) Plan(ctx context.Context, event *types.AlertEvent, agentCtx AgentContext) AgentPlan {
	// First iteration: delegate to Claude AI sidecar (Analyze phase calls invokeAnalysis)
	// RLM sets Iteration to 1 before the first Plan call
	if agentCtx.Iteration <= 1 {
		return AgentPlan{
			Complete:  false,
			Queries:   []SubQuery{}, // No sub-queries needed — sidecar call happens in Analyze()
			Reasoning: "Delegating to Claude AI sidecar for comprehensive analysis",
		}
	}

	// After sidecar analysis, mark as complete
	return AgentPlan{
		Complete:  true,
		Reasoning: "Analysis completed via Claude sidecar",
	}
}

// Analyze processes query results (minimal for Claude since it's single-shot)
func (a *ClaudeAgent) Analyze(ctx context.Context, results []QueryResult, agentCtx AgentContext) AgentContext {
	// For Claude, we perform the actual analysis here
	analysis, err := a.invokeAnalysis(ctx, agentCtx.Event)
	if err != nil {
		agentCtx.Findings = append(agentCtx.Findings, Finding{
			Source:    a.name,
			Category:  "error",
			Summary:   "Analysis invocation failed",
			Details:   err.Error(),
			Severity:  "warning",
			Timestamp: time.Now(),
		})
		return agentCtx
	}

	// Store the analysis result
	agentCtx.RootCause = analysis
	agentCtx.Findings = append(agentCtx.Findings, Finding{
		Source:    a.name,
		Category:  "analysis",
		Summary:   "Claude AI analysis",
		Details:   analysis,
		Severity:  "info",
		Timestamp: time.Now(),
	})

	return agentCtx
}

// Conclude generates the final analysis result
func (a *ClaudeAgent) Conclude(ctx context.Context, agentCtx AgentContext) *AnalysisResult {
	event := agentCtx.Event

	summary := "Analysis completed"
	if agentCtx.RootCause != "" {
		if len(agentCtx.RootCause) > 200 {
			summary = agentCtx.RootCause[:200] + "..."
		} else {
			summary = agentCtx.RootCause
		}
	}

	return &AnalysisResult{
		MonitorID:       event.Payload.MonitorID,
		MonitorName:     event.Payload.MonitorName,
		AlertStatus:     event.Payload.AlertStatus,
		Success:         agentCtx.RootCause != "",
		AgentRole:       a.role,
		RootCause:       agentCtx.RootCause,
		Summary:         summary,
		Details:         agentCtx.RootCause,
		Findings:        agentCtx.Findings,
		Recommendations: agentCtx.Recommendations,
	}
}

// invokeAnalysis calls the Claude agent sidecar.
// Routes watchdog monitors to /watchdog endpoint, all others to /analyze.
func (a *ClaudeAgent) invokeAnalysis(ctx context.Context, event *types.AlertEvent) (string, error) {
	payload := event.Payload

	// Use fallbacks for monitor_id and monitor_name
	monitorID := payload.MonitorID
	if monitorID == 0 {
		monitorID = payload.AlertID
	}
	monitorName := payload.MonitorName
	if monitorName == "" {
		monitorName = payload.AlertTitleCustom
	}
	if monitorName == "" {
		monitorName = payload.AlertTitle
	}

	req := claudeRequest{
		Payload: claudePayload{
			MonitorID:           monitorID,
			MonitorName:         monitorName,
			AlertStatus:         payload.AlertStatus,
			Hostname:            payload.Hostname,
			Service:             payload.Service,
			Scope:               payload.Scope,
			Tags:                payload.Tags,
			AlertState:          payload.AlertState,
			AlertTitle:          payload.AlertTitleCustom,
			ApplicationTeam:     payload.ApplicationTeam,
			ApplicationLongname: payload.ApplicationLongname,
			DetailedDescription: payload.DetailedDescription,
			Impact:              payload.Impact,
			Metric:              payload.Metric,
			SupportGroup:        payload.SupportGroup,
			Threshold:           payload.Threshold,
			Value:               payload.Value,
			Urgency:             payload.Urgency,
		},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Route watchdog monitors to the /watchdog endpoint
	endpoint := "/analyze"
	if a.role == RoleWatchdog {
		endpoint = "/watchdog"
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.agentURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.AgentClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var response claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if response.Error != "" {
		errorDetail := response.Error
		if response.ErrorType != "" {
			errorDetail = fmt.Sprintf("[%s] %s", response.ErrorType, response.Error)
		}
		if response.RetriesExhausted {
			errorDetail += " (retries exhausted)"
		}
		return "", fmt.Errorf("agent error: %s", errorDetail)
	}

	return response.Analysis, nil
}

// InvokeRecovery calls the Claude agent sidecar's /recover endpoint
// to update an existing notebook when a monitor recovers.
func (a *ClaudeAgent) InvokeRecovery(ctx context.Context, event *types.AlertEvent) error {
	payload := event.Payload

	monitorID := payload.MonitorID
	if monitorID == 0 {
		monitorID = payload.AlertID
	}
	monitorName := payload.MonitorName
	if monitorName == "" {
		monitorName = payload.AlertTitleCustom
	}
	if monitorName == "" {
		monitorName = payload.AlertTitle
	}

	req := claudeRequest{
		Payload: claudePayload{
			MonitorID:           monitorID,
			MonitorName:         monitorName,
			AlertStatus:         payload.AlertStatus,
			Hostname:            payload.Hostname,
			Service:             payload.Service,
			Scope:               payload.Scope,
			Tags:                payload.Tags,
			AlertState:          payload.AlertState,
			AlertTitle:          payload.AlertTitleCustom,
			ApplicationTeam:     payload.ApplicationTeam,
			ApplicationLongname: payload.ApplicationLongname,
			DetailedDescription: payload.DetailedDescription,
			Impact:              payload.Impact,
			Metric:              payload.Metric,
			SupportGroup:        payload.SupportGroup,
			Threshold:           payload.Threshold,
			Value:               payload.Value,
			Urgency:             payload.Urgency,
		},
	}

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal recovery request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.agentURL+"/recover", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create recovery request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpclient.AgentClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("recovery request failed: %w", err)
	}
	defer resp.Body.Close()

	var response claudeResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode recovery response: %w", err)
	}

	if response.Error != "" {
		return fmt.Errorf("recovery error: %s", response.Error)
	}

	return nil
}

// Claude sidecar request/response types
type claudeRequest struct {
	Payload claudePayload `json:"payload"`
}

type claudePayload struct {
	MonitorID           int64    `json:"monitor_id"`
	MonitorName         string   `json:"monitor_name"`
	AlertStatus         string   `json:"alert_status"`
	Hostname            string   `json:"hostname"`
	Service             string   `json:"service"`
	Scope               string   `json:"scope"`
	Tags                []string `json:"tags"`
	AlertState          string   `json:"ALERT_STATE"`
	AlertTitle          string   `json:"ALERT_TITLE"`
	ApplicationTeam     string   `json:"APPLICATION_TEAM"`
	ApplicationLongname string   `json:"APPLICATION_LONGNAME"`
	DetailedDescription string   `json:"DETAILED_DESCRIPTION"`
	Impact              string   `json:"IMPACT"`
	Metric              string   `json:"METRIC"`
	SupportGroup        string   `json:"SUPPORT_GROUP"`
	Threshold           string   `json:"THRESHOLD"`
	Value               string   `json:"VALUE"`
	Urgency             string   `json:"URGENCY"`
}

type claudeResponse struct {
	Success   bool   `json:"success"`
	MonitorID int64  `json:"monitorId"`
	Analysis  string `json:"analysis"`
	Error     string `json:"error,omitempty"`
	Timestamp string `json:"timestamp"`
	Notebook  *struct {
		URL string `json:"url"`
	} `json:"notebook,omitempty"`

	// Failure alerting fields (populated on error responses)
	ErrorType        string `json:"error_type,omitempty"`
	RetriesExhausted bool   `json:"retries_exhausted,omitempty"`
	FailureEvent     *struct {
		ID interface{} `json:"id,omitempty"`
	} `json:"failure_event,omitempty"`
	FailureNotebook *struct {
		ID  interface{} `json:"id,omitempty"`
		URL string      `json:"url,omitempty"`
	} `json:"failure_notebook,omitempty"`
}
