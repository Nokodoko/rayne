package agents

import (
	"context"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

// AgentRole identifies the type of specialist agent for alert routing
type AgentRole string

const (
	RoleInfrastructure AgentRole = "infrastructure"
	RoleApplication    AgentRole = "application"
	RoleNetwork        AgentRole = "network"
	RoleDatabase       AgentRole = "database"
	RoleLogs           AgentRole = "logs"
	RoleWatchdog       AgentRole = "watchdog" // Datadog Watchdog anomaly detection monitors
	RoleGeneral        AgentRole = "general"  // Fallback for unclassified alerts
)

// Agent defines the interface for specialist agents that can analyze alerts
type Agent interface {
	// Name returns the unique identifier for this agent
	Name() string

	// Role returns the specialist role this agent handles
	Role() AgentRole

	// Plan determines what queries/actions are needed to analyze the alert
	Plan(ctx context.Context, event *types.AlertEvent, agentCtx AgentContext) AgentPlan

	// Analyze processes query results and updates the agent context
	Analyze(ctx context.Context, results []QueryResult, agentCtx AgentContext) AgentContext

	// Conclude generates the final analysis result
	Conclude(ctx context.Context, agentCtx AgentContext) *AnalysisResult
}

// SubAgent defines the interface for data-fetching sub-agents
type SubAgent interface {
	// Name returns the unique identifier for this sub-agent
	Name() string

	// Query executes a query and returns the result
	Query(ctx context.Context, query string) (string, error)
}

// AgentContext holds the accumulated state during RLM iterations
type AgentContext struct {
	Event          *types.AlertEvent
	Iteration      int
	QueryHistory   []QueryResult
	Findings       []Finding
	Hypotheses     []string
	RootCause      string
	Recommendations []string
	Metadata       map[string]interface{}
}

// NewAgentContext creates a new agent context for an event
func NewAgentContext(event *types.AlertEvent) AgentContext {
	return AgentContext{
		Event:        event,
		Iteration:    0,
		QueryHistory: make([]QueryResult, 0),
		Findings:     make([]Finding, 0),
		Hypotheses:   make([]string, 0),
		Metadata:     make(map[string]interface{}),
	}
}

// AgentPlan contains the decisions from the Plan phase
type AgentPlan struct {
	// Complete indicates whether analysis is done
	Complete bool

	// Queries lists the sub-agent queries to execute
	Queries []SubQuery

	// Reasoning explains why these queries were chosen
	Reasoning string
}

// SubQuery represents a request to a sub-agent
type SubQuery struct {
	// AgentName identifies which sub-agent to use
	AgentName string

	// Query is the specific query/request to execute
	Query string

	// Priority determines execution order (lower = higher priority)
	Priority int

	// Required indicates if the analysis should fail without this result
	Required bool
}

// QueryResult contains the outcome of a sub-agent query
type QueryResult struct {
	Query     SubQuery
	Result    string
	Error     error
	Duration  time.Duration
	Timestamp time.Time
}

// Finding represents a discovered fact during analysis
type Finding struct {
	Source     string                 `json:"source"`      // Which sub-agent/query produced this
	Category   string                 `json:"category"`    // e.g., "metric", "log", "trace", "config"
	Summary    string                 `json:"summary"`     // Brief description
	Details    string                 `json:"details"`     // Full details
	Severity   string                 `json:"severity"`    // "info", "warning", "critical"
	Timestamp  time.Time              `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AnalysisResult contains the final output of an agent analysis
type AnalysisResult struct {
	// Event identification
	MonitorID   int64     `json:"monitor_id"`
	MonitorName string    `json:"monitor_name"`
	AlertStatus string    `json:"alert_status"`

	// Analysis outcome
	Success     bool      `json:"success"`
	AgentRole   AgentRole `json:"agent_role"`
	RootCause   string    `json:"root_cause"`
	Summary     string    `json:"summary"`
	Details     string    `json:"details"`

	// Supporting data
	Findings       []Finding `json:"findings"`
	Recommendations []string  `json:"recommendations"`

	// Execution metadata
	Iterations int           `json:"iterations"`
	Duration   time.Duration `json:"duration"`
	Error      string        `json:"error,omitempty"`

	// Timestamps
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
}

// JobResult represents the result of a dispatched webhook job
type JobResult struct {
	EventID     int64
	Success     bool
	ProcessedBy []string
	Errors      []string
	Duration    time.Duration
}

// DispatcherStats holds statistics about the dispatcher
type DispatcherStats struct {
	QueueSize       int   `json:"queue_size"`
	QueueCapacity   int   `json:"queue_capacity"`
	ActiveWorkers   int   `json:"active_workers"`
	TotalWorkers    int   `json:"total_workers"`
	ProcessedCount  int64 `json:"processed_count"`
	ErrorCount      int64 `json:"error_count"`
	DroppedCount    int64 `json:"dropped_count"`
}
