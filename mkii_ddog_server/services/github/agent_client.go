package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// AgentClient communicates with the Claude agent sidecar for issue processing.
type AgentClient struct {
	agentURL string
	client   *http.Client
}

// NewAgentClient creates a new agent client.
func NewAgentClient() *AgentClient {
	url := os.Getenv("CLAUDE_AGENT_URL")
	if url == "" {
		url = "http://localhost:9000"
	}
	return &AgentClient{
		agentURL: url,
		client:   &http.Client{Timeout: 15 * time.Minute},
	}
}

// ProcessIssue sends a GitHub issue to the sidecar for Claude Code processing.
func (c *AgentClient) ProcessIssue(ctx context.Context, req AgentProcessRequest) (*AgentProcessResponse, error) {
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.agentURL+"/github/process-issue", bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("agent returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var response AgentProcessResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &response, nil
}
