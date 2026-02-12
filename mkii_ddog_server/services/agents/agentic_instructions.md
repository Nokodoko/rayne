# agentic_instructions.md

## Purpose
AI-powered agent framework for automated Root Cause Analysis (RCA) of Datadog alerts. Implements the Recursive Language Model (RLM) pattern: Plan -> Query -> Analyze -> Conclude, with role-based classification to route alerts to specialist agents.

## Technology
Go, context, sync, sync/atomic, net/http, encoding/json

## Contents
- `types.go` -- Agent and SubAgent interfaces, AgentRole constants, AgentContext, AgentPlan, SubQuery, QueryResult, Finding, AnalysisResult
- `orchestrator.go` -- AgentOrchestrator: semaphore-bounded concurrency, role classification, RLM coordination
- `classifier.go` -- RoleClassifier: rule-based alert routing by monitor type, tags, service, hostname
- `claude_agent.go` -- ClaudeAgent: Agent implementation that invokes Claude AI sidecar at /analyze
- `rlm.go` -- RLMCoordinator: implements Plan->Query->Analyze->Conclude loop with sub-agent fan-out

## Key Functions
- `NewAgentOrchestrator(config) *AgentOrchestrator` -- Creates orchestrator with bounded concurrency (default: 3)
- `(o *AgentOrchestrator) Analyze(ctx, event) (*AnalysisResult, error)` -- Single entry point for all agent analysis
- `(o *AgentOrchestrator) ShouldAnalyze(event) bool` -- Returns true for "Alert" or "Warn" status
- `(o *AgentOrchestrator) RegisterAgent(agent)` -- Registers specialist agent for a role
- `NewRoleClassifier() *RoleClassifier` -- Creates classifier with default rules
- `(c *RoleClassifier) Classify(event) AgentRole` -- Determines agent role from monitor type, tags, service, hostname
- `NewRLMCoordinator(maxIterations) *RLMCoordinator` -- Creates RLM loop coordinator (default: 5 iterations)
- `(r *RLMCoordinator) Execute(ctx, agent, event) (*AnalysisResult, error)` -- Runs the RLM loop
- `NewClaudeAgent(role) *ClaudeAgent` -- Creates Claude-based agent for a specific role
- `NewDefaultClaudeAgent() *ClaudeAgent` -- Creates general-purpose Claude agent

## Data Types
- `Agent` -- interface: Name(), Role(), Plan(ctx, event, agentCtx), Analyze(ctx, results, agentCtx), Conclude(ctx, agentCtx)
- `SubAgent` -- interface: Name(), Query(ctx, query) (string, error)
- `AgentRole` -- string: RoleInfrastructure, RoleApplication, RoleNetwork, RoleDatabase, RoleLogs, RoleGeneral
- `AgentContext` -- struct: Event, Iteration, QueryHistory, Findings, Hypotheses, RootCause, Recommendations, Metadata
- `AgentPlan` -- struct: Complete, Queries []SubQuery, Reasoning
- `SubQuery` -- struct: AgentName, Query, Priority, Required
- `QueryResult` -- struct: Query, Result, Error, Duration, Timestamp
- `Finding` -- struct: Source, Category, Summary, Details, Severity, Timestamp, Metadata
- `AnalysisResult` -- struct: MonitorID, MonitorName, AlertStatus, Success, AgentRole, RootCause, Summary, Findings, Recommendations, Iterations, Duration
- `OrchestratorConfig` -- struct: MaxConcurrent (default 3), RLMMaxIterations (default 5)
- `RoleClassifier` -- struct: monitorTypeRules, tagRules, servicePatterns, hostnamePatterns (all map[string]AgentRole)

## Logging
Uses `log.Printf` with prefixes: `[AGENT-ORCH]`, `[RLM]`

## CRUD Entry Points
- **Create**: Implement `Agent` interface for new specialist roles, register via `orchestrator.RegisterAgent()`
- **Read**: Call `orchestrator.Analyze(ctx, event)` from webhook processing pipeline
- **Update**: Add classification rules to `classifier.go`, adjust RLM iteration limits
- **Delete**: Unregister agents by removing `RegisterAgent()` calls

## Style Guide
- Semaphore pattern for bounded concurrency: `chan struct{}`
- Fan-out/fan-in for sub-agent queries with `sync.WaitGroup` and channels
- Context propagation and cancellation checking at each RLM iteration
- Error wrapping with `fmt.Errorf("...: %w", err)`
- Representative snippet:

```go
func (o *AgentOrchestrator) Analyze(ctx context.Context, event *types.AlertEvent) (*AnalysisResult, error) {
	select {
	case o.semaphore <- struct{}{}:
		defer func() { <-o.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	atomic.AddInt64(&o.activeCount, 1)
	defer atomic.AddInt64(&o.activeCount, -1)

	role := o.classifier.Classify(event)
	agent := o.getAgent(role)
	if agent == nil {
		return &AnalysisResult{Success: false, Error: "no agent available"}, nil
	}

	return o.rlmCoordinator.Execute(ctx, agent, event)
}
```
