package agents

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

// RLMCoordinator implements the Recursive Language Model pattern
// for multi-step agent collaboration: Plan → Query → Analyze → Conclude
type RLMCoordinator struct {
	maxIterations int
	subAgents     map[string]SubAgent
	mu            sync.RWMutex
}

// NewRLMCoordinator creates a new RLM coordinator
func NewRLMCoordinator(maxIterations int) *RLMCoordinator {
	if maxIterations <= 0 {
		maxIterations = 5
	}
	return &RLMCoordinator{
		maxIterations: maxIterations,
		subAgents:     make(map[string]SubAgent),
	}
}

// RegisterSubAgent adds a sub-agent to the coordinator
func (r *RLMCoordinator) RegisterSubAgent(agent SubAgent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subAgents[agent.Name()] = agent
	log.Printf("[RLM] Registered sub-agent: %s", agent.Name())
}

// UnregisterSubAgent removes a sub-agent
func (r *RLMCoordinator) UnregisterSubAgent(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subAgents, name)
	log.Printf("[RLM] Unregistered sub-agent: %s", name)
}

// ListSubAgents returns the names of registered sub-agents
func (r *RLMCoordinator) ListSubAgents() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.subAgents))
	for name := range r.subAgents {
		names = append(names, name)
	}
	return names
}

// Execute runs the RLM loop for a specialist agent
func (r *RLMCoordinator) Execute(ctx context.Context, agent Agent, event *types.AlertEvent) (*AnalysisResult, error) {
	startTime := time.Now()
	agentCtx := NewAgentContext(event)

	log.Printf("[RLM] Starting analysis for monitor %d with %s agent (max %d iterations)",
		event.Payload.MonitorID, agent.Name(), r.maxIterations)

	for iteration := 0; iteration < r.maxIterations; iteration++ {
		select {
		case <-ctx.Done():
			return r.buildCancelledResult(event, agent, agentCtx, startTime, ctx.Err()), ctx.Err()
		default:
		}

		agentCtx.Iteration = iteration + 1
		log.Printf("[RLM] Iteration %d/%d for monitor %d", agentCtx.Iteration, r.maxIterations, event.Payload.MonitorID)

		// PLAN: Determine what queries are needed
		plan := agent.Plan(ctx, event, agentCtx)
		log.Printf("[RLM] Plan: %d queries, complete=%v, reason=%s",
			len(plan.Queries), plan.Complete, truncate(plan.Reasoning, 100))

		// Check if analysis is complete
		if plan.Complete {
			result := agent.Conclude(ctx, agentCtx)
			result.Iterations = agentCtx.Iteration
			result.Duration = time.Since(startTime)
			result.StartedAt = startTime
			result.CompletedAt = time.Now()
			log.Printf("[RLM] Analysis complete for monitor %d after %d iterations",
				event.Payload.MonitorID, result.Iterations)
			return result, nil
		}

		// Skip query phase if no queries
		if len(plan.Queries) == 0 {
			continue
		}

		// QUERY: Fan-out to sub-agents
		results := r.executeSubQueries(ctx, plan.Queries)
		agentCtx.QueryHistory = append(agentCtx.QueryHistory, results...)

		// Check for required query failures
		for _, result := range results {
			if result.Query.Required && result.Error != nil {
				log.Printf("[RLM] Required query failed: %s - %v", result.Query.AgentName, result.Error)
				return r.buildErrorResult(event, agent, agentCtx, startTime, result.Error), result.Error
			}
		}

		// ANALYZE: Process results and update context
		agentCtx = agent.Analyze(ctx, results, agentCtx)
	}

	// Max iterations reached - conclude with available data
	log.Printf("[RLM] Max iterations reached for monitor %d, concluding with available data",
		event.Payload.MonitorID)

	result := agent.Conclude(ctx, agentCtx)
	result.Iterations = r.maxIterations
	result.Duration = time.Since(startTime)
	result.StartedAt = startTime
	result.CompletedAt = time.Now()
	return result, nil
}

// executeSubQueries runs queries concurrently using fan-out pattern
func (r *RLMCoordinator) executeSubQueries(ctx context.Context, queries []SubQuery) []QueryResult {
	r.mu.RLock()
	subAgents := make(map[string]SubAgent, len(r.subAgents))
	for k, v := range r.subAgents {
		subAgents[k] = v
	}
	r.mu.RUnlock()

	results := make(chan QueryResult, len(queries))
	var wg sync.WaitGroup

	for _, q := range queries {
		wg.Add(1)
		go func(query SubQuery) {
			defer wg.Done()

			result := QueryResult{
				Query:     query,
				Timestamp: time.Now(),
			}

			agent, ok := subAgents[query.AgentName]
			if !ok {
				result.Error = &subAgentNotFoundError{query.AgentName}
				result.Duration = 0
				log.Printf("[RLM] Sub-agent not found: %s", query.AgentName)
				results <- result
				return
			}

			startTime := time.Now()
			queryResult, err := agent.Query(ctx, query.Query)
			result.Duration = time.Since(startTime)
			result.Result = queryResult
			result.Error = err

			if err != nil {
				log.Printf("[RLM] Query failed: %s/%s - %v (took %v)",
					query.AgentName, truncate(query.Query, 50), err, result.Duration)
			} else {
				log.Printf("[RLM] Query succeeded: %s/%s (took %v, %d bytes)",
					query.AgentName, truncate(query.Query, 50), result.Duration, len(queryResult))
			}

			results <- result
		}(q)
	}

	// Fan-in: collect all results
	go func() {
		wg.Wait()
		close(results)
	}()

	var collected []QueryResult
	for result := range results {
		collected = append(collected, result)
	}

	return collected
}

// buildCancelledResult creates a result for cancelled analysis
func (r *RLMCoordinator) buildCancelledResult(
	event *types.AlertEvent,
	agent Agent,
	agentCtx AgentContext,
	startTime time.Time,
	err error,
) *AnalysisResult {
	return &AnalysisResult{
		MonitorID:   event.Payload.MonitorID,
		MonitorName: event.Payload.MonitorName,
		AlertStatus: event.Payload.AlertStatus,
		Success:     false,
		AgentRole:   agent.Role(),
		Summary:     "Analysis cancelled",
		Details:     "The analysis was cancelled before completion",
		Findings:    agentCtx.Findings,
		Iterations:  agentCtx.Iteration,
		Duration:    time.Since(startTime),
		Error:       err.Error(),
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}
}

// buildErrorResult creates a result for failed analysis
func (r *RLMCoordinator) buildErrorResult(
	event *types.AlertEvent,
	agent Agent,
	agentCtx AgentContext,
	startTime time.Time,
	err error,
) *AnalysisResult {
	return &AnalysisResult{
		MonitorID:   event.Payload.MonitorID,
		MonitorName: event.Payload.MonitorName,
		AlertStatus: event.Payload.AlertStatus,
		Success:     false,
		AgentRole:   agent.Role(),
		Summary:     "Analysis failed",
		Details:     "A required query failed during analysis",
		Findings:    agentCtx.Findings,
		Iterations:  agentCtx.Iteration,
		Duration:    time.Since(startTime),
		Error:       err.Error(),
		StartedAt:   startTime,
		CompletedAt: time.Now(),
	}
}

// subAgentNotFoundError indicates a requested sub-agent doesn't exist
type subAgentNotFoundError struct {
	name string
}

func (e *subAgentNotFoundError) Error() string {
	return "sub-agent not found: " + e.name
}

// truncate shortens a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
