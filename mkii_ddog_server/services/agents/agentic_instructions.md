# agentic_instructions.md

## Purpose
AI-powered agent framework for automated Root Cause Analysis (RCA) of Datadog alerts. Implements the Recursive Language Model (RLM) pattern: Plan -> Query -> Analyze -> Conclude, with role-based classification to route alerts to specialist agents.

## Technology
Go, context, sync, sync/atomic, net/http, encoding/json

## Contents
- `types.go` -- Agent and SubAgent interfaces, AgentRole constants, AgentContext, AgentPlan, SubQuery, QueryResult, Finding, AnalysisResult
- `orchestrator.go` -- AgentOrchestrator: semaphore-bounded concurrency, role classification, RLM coordination, recovery handling (ShouldRecover, Recover), failure alerting integration
- `classifier.go` -- RoleClassifier: rule-based alert routing by monitor type, tags, service, hostname
- `claude_agent.go` -- ClaudeAgent: Agent implementation that invokes Claude AI sidecar at /analyze and /recover. Handles error classification fields (error_type, retries_exhausted, failure_event, failure_notebook) from sidecar responses
- `failure_alerter.go` -- FailureAlerter: creates Datadog events via Events API when agent analysis fails. Best-effort alerting that provides visibility into pipeline failures even when the sidecar is unreachable
- `rlm.go` -- RLMCoordinator: implements Plan->Query->Analyze->Conclude loop with sub-agent fan-out

## Key Functions
- `NewAgentOrchestrator(config) *AgentOrchestrator` -- Creates orchestrator with bounded concurrency (default: 3) and FailureAlerter
- `(o *AgentOrchestrator) Analyze(ctx, event) (*AnalysisResult, error)` -- Single entry point for all agent analysis. On failure, fires FailureAlerter.ReportFailure() in a goroutine
- `(o *AgentOrchestrator) ShouldAnalyze(event) bool` -- Returns true for "Alert", "Warn", or "Triggered" status (checks both alert_status and ALERT_STATE fields)
- `(o *AgentOrchestrator) ShouldRecover(event) bool` -- Returns true for "OK", "Recovered", or "Resolved" status (checks both alert_status and ALERT_STATE fields)
- `(o *AgentOrchestrator) Recover(ctx, event) (*AnalysisResult, error)` -- Notifies agent sidecar that a monitor recovered, triggering notebook lifecycle update (ACTIVE -> RESOLVED)
- `(o *AgentOrchestrator) RegisterAgent(agent)` -- Registers specialist agent for a role
- `NewRoleClassifier() *RoleClassifier` -- Creates classifier with default rules
- `(c *RoleClassifier) Classify(event) AgentRole` -- Determines agent role from monitor type, tags, service, hostname
- `NewRLMCoordinator(maxIterations) *RLMCoordinator` -- Creates RLM loop coordinator (default: 5 iterations)
- `(r *RLMCoordinator) Execute(ctx, agent, event) (*AnalysisResult, error)` -- Runs the RLM loop
- `NewClaudeAgent(role) *ClaudeAgent` -- Creates Claude-based agent for a specific role
- `NewDefaultClaudeAgent() *ClaudeAgent` -- Creates general-purpose Claude agent
- `(a *ClaudeAgent) InvokeRecovery(ctx, event) error` -- Calls the sidecar /recover endpoint to update existing notebook status
- `NewFailureAlerter() *FailureAlerter` -- Creates alerter using DD_API_KEY/DD_APP_KEY from env
- `(fa *FailureAlerter) ReportFailure(ctx, result, err)` -- Creates Datadog event with error details, monitor info, and agent role tags (best-effort, errors logged not propagated)

## Data Types
- `Agent` -- interface: Name(), Role(), Plan(ctx, event, agentCtx), Analyze(ctx, results, agentCtx), Conclude(ctx, agentCtx)
- `SubAgent` -- interface: Name(), Query(ctx, query) (string, error)
- `AgentRole` -- string: RoleInfrastructure, RoleApplication, RoleNetwork, RoleDatabase, RoleLogs, RoleGeneral
- `AgentContext` -- struct: Event, Iteration, QueryHistory, Findings, Hypotheses, RootCause, Recommendations, Metadata
- `AgentPlan` -- struct: Complete, Queries []SubQuery, Reasoning
- `SubQuery` -- struct: AgentName, Query, Priority, Required
- `QueryResult` -- struct: Query, Result, Error, Duration, Timestamp
- `Finding` -- struct: Source, Category, Summary, Details, Severity, Timestamp, Metadata
- `AnalysisResult` -- struct: MonitorID, MonitorName, AlertStatus, Success, AgentRole, RootCause, Summary, Findings, Recommendations, Iterations, Duration, Error, StartedAt, CompletedAt
- `OrchestratorConfig` -- struct: MaxConcurrent (default 3), RLMMaxIterations (default 5)
- `RoleClassifier` -- struct: monitorTypeRules, tagRules, servicePatterns, hostnamePatterns (all map[string]AgentRole)
- `FailureAlerter` -- struct: enabled, apiKey, appKey, apiURL, httpClient. Uses DD_SITE env (default: ddog-gov.com)
- `datadogEvent` -- struct: Title, Text, Priority, Tags, AlertType, SourceTypeName (Datadog Events API v1 payload)
- `claudeResponse` -- struct: includes ErrorType, RetriesExhausted, FailureEvent, FailureNotebook fields for failure alerting

## Logging
Uses `log.Printf` with prefixes: `[AGENT-ORCH]`, `[RLM]`, `[FAILURE-ALERTER]`

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
