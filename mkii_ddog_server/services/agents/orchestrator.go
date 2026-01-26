package agents

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

// AgentOrchestrator is the single entry point for all agent-based analysis.
// It provides semaphore-bounded concurrency, role classification, and RLM coordination.
type AgentOrchestrator struct {
	classifier     *RoleClassifier
	agents         map[AgentRole]Agent
	defaultAgent   Agent
	rlmCoordinator *RLMCoordinator
	semaphore      chan struct{}
	mu             sync.RWMutex

	// Metrics
	activeCount    int64
	totalProcessed int64
	totalErrors    int64
}

// OrchestratorConfig holds configuration for the agent orchestrator
type OrchestratorConfig struct {
	// MaxConcurrent is the maximum number of concurrent agent analyses
	// Default: 3
	MaxConcurrent int

	// RLMMaxIterations is the maximum number of RLM iterations per analysis
	// Default: 5
	RLMMaxIterations int
}

// DefaultOrchestratorConfig returns sensible defaults
func DefaultOrchestratorConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxConcurrent:    3,
		RLMMaxIterations: 5,
	}
}

// NewAgentOrchestrator creates a new agent orchestrator
func NewAgentOrchestrator(config OrchestratorConfig) *AgentOrchestrator {
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 3
	}
	if config.RLMMaxIterations <= 0 {
		config.RLMMaxIterations = 5
	}

	return &AgentOrchestrator{
		classifier:     NewRoleClassifier(),
		agents:         make(map[AgentRole]Agent),
		rlmCoordinator: NewRLMCoordinator(config.RLMMaxIterations),
		semaphore:      make(chan struct{}, config.MaxConcurrent),
	}
}

// RegisterAgent adds a specialist agent for a specific role
func (o *AgentOrchestrator) RegisterAgent(agent Agent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.agents[agent.Role()] = agent
	log.Printf("[AGENT-ORCH] Registered agent: %s (role: %s)", agent.Name(), agent.Role())
}

// SetDefaultAgent sets the fallback agent for unclassified alerts
func (o *AgentOrchestrator) SetDefaultAgent(agent Agent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.defaultAgent = agent
	log.Printf("[AGENT-ORCH] Set default agent: %s", agent.Name())
}

// RegisterSubAgent adds a sub-agent to the RLM coordinator
func (o *AgentOrchestrator) RegisterSubAgent(subAgent SubAgent) {
	o.rlmCoordinator.RegisterSubAgent(subAgent)
}

// Analyze performs a bounded agent analysis on a webhook event.
// This is the single entry point - all agent calls should go through here.
func (o *AgentOrchestrator) Analyze(ctx context.Context, event *types.AlertEvent) (*AnalysisResult, error) {
	// Acquire semaphore (bounded concurrency)
	select {
	case o.semaphore <- struct{}{}:
		defer func() { <-o.semaphore }()
	case <-ctx.Done():
		log.Printf("[AGENT-ORCH] Context cancelled while waiting for semaphore: monitor %d",
			event.Payload.MonitorID)
		return nil, ctx.Err()
	}

	atomic.AddInt64(&o.activeCount, 1)
	defer atomic.AddInt64(&o.activeCount, -1)

	log.Printf("[AGENT-ORCH] Starting analysis for monitor %d (active: %d)",
		event.Payload.MonitorID, atomic.LoadInt64(&o.activeCount))

	// Classify the alert to determine which agent to use
	role := o.classifier.Classify(event)
	log.Printf("[AGENT-ORCH] Classified monitor %d as role: %s", event.Payload.MonitorID, role)

	// Get the appropriate agent
	agent := o.getAgent(role)
	if agent == nil {
		atomic.AddInt64(&o.totalErrors, 1)
		log.Printf("[AGENT-ORCH] No agent available for role %s (monitor %d)", role, event.Payload.MonitorID)
		return &AnalysisResult{
			MonitorID:   event.Payload.MonitorID,
			MonitorName: event.Payload.MonitorName,
			AlertStatus: event.Payload.AlertStatus,
			Success:     false,
			AgentRole:   role,
			Error:       "no agent available for role: " + string(role),
			StartedAt:   time.Now(),
			CompletedAt: time.Now(),
		}, nil
	}

	// Execute the RLM loop
	result, err := o.rlmCoordinator.Execute(ctx, agent, event)

	atomic.AddInt64(&o.totalProcessed, 1)
	if err != nil || (result != nil && !result.Success) {
		atomic.AddInt64(&o.totalErrors, 1)
	}

	if err != nil {
		log.Printf("[AGENT-ORCH] Analysis failed for monitor %d: %v", event.Payload.MonitorID, err)
		return result, err
	}

	log.Printf("[AGENT-ORCH] Analysis completed for monitor %d: success=%v, iterations=%d, duration=%v",
		event.Payload.MonitorID, result.Success, result.Iterations, result.Duration)

	return result, nil
}

// ShouldAnalyze determines if an event should trigger agent analysis
func (o *AgentOrchestrator) ShouldAnalyze(event *types.AlertEvent) bool {
	status := event.Payload.AlertStatus
	return status == "Alert" || status == "Warn"
}

// getAgent returns the appropriate agent for a role
func (o *AgentOrchestrator) getAgent(role AgentRole) Agent {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if agent, ok := o.agents[role]; ok {
		return agent
	}
	return o.defaultAgent
}

// Stats returns current orchestrator statistics
func (o *AgentOrchestrator) Stats() OrchestratorStats {
	o.mu.RLock()
	agentCount := len(o.agents)
	o.mu.RUnlock()

	return OrchestratorStats{
		ActiveAnalyses:  atomic.LoadInt64(&o.activeCount),
		MaxConcurrent:   cap(o.semaphore),
		TotalProcessed:  atomic.LoadInt64(&o.totalProcessed),
		TotalErrors:     atomic.LoadInt64(&o.totalErrors),
		RegisteredAgents: agentCount,
		SubAgents:       o.rlmCoordinator.ListSubAgents(),
	}
}

// OrchestratorStats holds orchestrator statistics
type OrchestratorStats struct {
	ActiveAnalyses   int64    `json:"active_analyses"`
	MaxConcurrent    int      `json:"max_concurrent"`
	TotalProcessed   int64    `json:"total_processed"`
	TotalErrors      int64    `json:"total_errors"`
	RegisteredAgents int      `json:"registered_agents"`
	SubAgents        []string `json:"sub_agents"`
}
