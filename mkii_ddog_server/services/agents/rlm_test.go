package agents

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

// mockSubAgent implements SubAgent for testing
type mockSubAgent struct {
	name       string
	response   string
	err        error
	delay      time.Duration
	callCount  int64
	lastQuery  string
}

func newMockSubAgent(name string, response string) *mockSubAgent {
	return &mockSubAgent{
		name:     name,
		response: response,
	}
}

func (m *mockSubAgent) Name() string {
	return m.name
}

func (m *mockSubAgent) Query(ctx context.Context, query string) (string, error) {
	atomic.AddInt64(&m.callCount, 1)
	m.lastQuery = query

	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}

	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

// rlmMockAgent implements Agent for RLM testing
type rlmMockAgent struct {
	name            string
	role            AgentRole
	queries         []SubQuery
	maxIterations   int
	currentIter     int
	analyzeDelay    time.Duration
	planCalls       int64
	analyzeCalls    int64
	concludeCalls   int64
}

func newRLMMockAgent(name string, role AgentRole, queries []SubQuery) *rlmMockAgent {
	return &rlmMockAgent{
		name:          name,
		role:          role,
		queries:       queries,
		maxIterations: 2,
	}
}

func (m *rlmMockAgent) Name() string {
	return m.name
}

func (m *rlmMockAgent) Role() AgentRole {
	return m.role
}

func (m *rlmMockAgent) Plan(ctx context.Context, event *types.AlertEvent, agentCtx AgentContext) AgentPlan {
	atomic.AddInt64(&m.planCalls, 1)
	m.currentIter = agentCtx.Iteration

	// Complete after maxIterations
	if m.currentIter >= m.maxIterations {
		return AgentPlan{
			Complete:  true,
			Reasoning: "max iterations reached",
		}
	}

	return AgentPlan{
		Complete:  false,
		Queries:   m.queries,
		Reasoning: "need more data",
	}
}

func (m *rlmMockAgent) Analyze(ctx context.Context, results []QueryResult, agentCtx AgentContext) AgentContext {
	atomic.AddInt64(&m.analyzeCalls, 1)

	if m.analyzeDelay > 0 {
		time.Sleep(m.analyzeDelay)
	}

	// Add findings from query results
	for _, r := range results {
		if r.Error == nil {
			agentCtx.Findings = append(agentCtx.Findings, Finding{
				Source:   r.Query.AgentName,
				Summary:  r.Result,
				Category: "query_result",
			})
		}
	}

	agentCtx.RootCause = "identified root cause"
	return agentCtx
}

func (m *rlmMockAgent) Conclude(ctx context.Context, agentCtx AgentContext) *AnalysisResult {
	atomic.AddInt64(&m.concludeCalls, 1)

	return &AnalysisResult{
		MonitorID:   agentCtx.Event.Payload.MonitorID,
		MonitorName: agentCtx.Event.Payload.MonitorName,
		AlertStatus: agentCtx.Event.Payload.AlertStatus,
		Success:     true,
		AgentRole:   m.role,
		RootCause:   agentCtx.RootCause,
		Summary:     "analysis complete",
		Findings:    agentCtx.Findings,
	}
}

func TestRLMCoordinator_Execute(t *testing.T) {
	coord := NewRLMCoordinator(5)

	subAgent := newMockSubAgent("logs", "log data here")
	coord.RegisterSubAgent(subAgent)

	queries := []SubQuery{
		{AgentName: "logs", Query: "get logs"},
	}
	agent := newRLMMockAgent("test", RoleGeneral, queries)

	event := &types.AlertEvent{
		ID: 1,
		Payload: types.AlertPayload{
			MonitorID:   123,
			MonitorName: "Test Monitor",
			AlertStatus: "Alert",
		},
	}

	result, err := coord.Execute(context.Background(), agent, event)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if !result.Success {
		t.Errorf("Expected success, got error: %s", result.Error)
	}
	if result.MonitorID != 123 {
		t.Errorf("MonitorID: expected 123, got %d", result.MonitorID)
	}
	if result.Iterations < 1 {
		t.Errorf("Should have at least 1 iteration, got %d", result.Iterations)
	}

	// Verify sub-agent was called
	if subAgent.callCount == 0 {
		t.Error("Sub-agent should have been called")
	}
}

func TestRLMCoordinator_RegisterSubAgent(t *testing.T) {
	coord := NewRLMCoordinator(5)

	sub1 := newMockSubAgent("sub1", "response1")
	sub2 := newMockSubAgent("sub2", "response2")

	coord.RegisterSubAgent(sub1)
	coord.RegisterSubAgent(sub2)

	agents := coord.ListSubAgents()
	if len(agents) != 2 {
		t.Errorf("Expected 2 sub-agents, got %d", len(agents))
	}

	found := map[string]bool{}
	for _, name := range agents {
		found[name] = true
	}
	if !found["sub1"] || !found["sub2"] {
		t.Errorf("Expected sub1 and sub2, got %v", agents)
	}
}

func TestRLMCoordinator_UnregisterSubAgent(t *testing.T) {
	coord := NewRLMCoordinator(5)

	sub := newMockSubAgent("test", "response")
	coord.RegisterSubAgent(sub)

	if len(coord.ListSubAgents()) != 1 {
		t.Error("Should have 1 sub-agent")
	}

	coord.UnregisterSubAgent("test")

	if len(coord.ListSubAgents()) != 0 {
		t.Error("Should have 0 sub-agents after unregister")
	}
}

func TestRLMCoordinator_SubAgentNotFound(t *testing.T) {
	coord := NewRLMCoordinator(5)

	// Don't register the sub-agent that will be queried
	queries := []SubQuery{
		{AgentName: "missing", Query: "get data"},
	}
	agent := newRLMMockAgent("test", RoleGeneral, queries)
	agent.maxIterations = 3 // Allow iteration with queries

	event := &types.AlertEvent{
		ID: 1,
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	result, err := coord.Execute(context.Background(), agent, event)

	// Should still succeed (non-required query)
	if err != nil {
		t.Logf("Error (may be expected): %v", err)
	}
	if result == nil {
		t.Fatal("Result should not be nil")
	}
}

func TestRLMCoordinator_RequiredQueryFailure(t *testing.T) {
	coord := NewRLMCoordinator(5)

	subAgent := newMockSubAgent("required", "")
	subAgent.err = errors.New("query failed")
	coord.RegisterSubAgent(subAgent)

	queries := []SubQuery{
		{AgentName: "required", Query: "get data", Required: true},
	}
	agent := newRLMMockAgent("test", RoleGeneral, queries)
	agent.maxIterations = 3 // Allow at least one iteration with queries

	event := &types.AlertEvent{
		ID: 1,
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	result, err := coord.Execute(context.Background(), agent, event)

	if err == nil {
		t.Error("Should return error for required query failure")
	}
	if result == nil {
		t.Fatal("Result should not be nil even on error")
	}
	if result.Success {
		t.Error("Should not be successful")
	}
}

func TestRLMCoordinator_ContextCancellation(t *testing.T) {
	coord := NewRLMCoordinator(5)

	slowSubAgent := newMockSubAgent("slow", "response")
	slowSubAgent.delay = 500 * time.Millisecond
	coord.RegisterSubAgent(slowSubAgent)

	queries := []SubQuery{
		{AgentName: "slow", Query: "slow query"},
	}
	agent := newRLMMockAgent("test", RoleGeneral, queries)
	agent.maxIterations = 3

	event := &types.AlertEvent{
		ID: 1,
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	result, err := coord.Execute(ctx, agent, event)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Logf("Expected DeadlineExceeded, got: %v", err)
	}
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.Success {
		t.Error("Should not be successful after cancellation")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("Should cancel faster, took %v", elapsed)
	}
}

func TestRLMCoordinator_MaxIterations(t *testing.T) {
	coord := NewRLMCoordinator(3) // Max 3 iterations

	queries := []SubQuery{} // No queries, but never complete
	agent := newRLMMockAgent("test", RoleGeneral, queries)
	agent.maxIterations = 100 // Agent wants to run forever

	// Override Plan to never complete
	originalMaxIter := agent.maxIterations
	agent.maxIterations = originalMaxIter

	event := &types.AlertEvent{
		ID: 1,
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	result, err := coord.Execute(context.Background(), agent, event)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Iterations > 3 {
		t.Errorf("Should not exceed max iterations, got %d", result.Iterations)
	}
}

func TestRLMCoordinator_ParallelSubQueries(t *testing.T) {
	coord := NewRLMCoordinator(5)

	// Create multiple slow sub-agents
	sub1 := newMockSubAgent("sub1", "response1")
	sub1.delay = 50 * time.Millisecond
	sub2 := newMockSubAgent("sub2", "response2")
	sub2.delay = 50 * time.Millisecond
	sub3 := newMockSubAgent("sub3", "response3")
	sub3.delay = 50 * time.Millisecond

	coord.RegisterSubAgent(sub1)
	coord.RegisterSubAgent(sub2)
	coord.RegisterSubAgent(sub3)

	queries := []SubQuery{
		{AgentName: "sub1", Query: "query1"},
		{AgentName: "sub2", Query: "query2"},
		{AgentName: "sub3", Query: "query3"},
	}
	agent := newRLMMockAgent("test", RoleGeneral, queries)
	agent.maxIterations = 3 // Allow iteration with queries

	event := &types.AlertEvent{
		ID: 1,
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	start := time.Now()
	result, err := coord.Execute(context.Background(), agent, event)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result == nil {
		t.Fatal("Result should not be nil")
	}

	// If parallel, should complete in ~50ms per iteration, not ~150ms
	if elapsed > 200*time.Millisecond {
		t.Errorf("Sub-queries should run in parallel, took %v", elapsed)
	}

	// All sub-agents should have been called at least once
	if sub1.callCount == 0 || sub2.callCount == 0 || sub3.callCount == 0 {
		t.Errorf("All sub-agents should be called at least once: sub1=%d, sub2=%d, sub3=%d",
			sub1.callCount, sub2.callCount, sub3.callCount)
	}
}

func TestNewRLMCoordinator(t *testing.T) {
	tests := []struct {
		maxIter  int
		expected int
	}{
		{5, 5},
		{0, 5},  // Should default to 5
		{-1, 5}, // Should default to 5
		{10, 10},
	}

	for _, tt := range tests {
		coord := NewRLMCoordinator(tt.maxIter)
		if coord.maxIterations != tt.expected {
			t.Errorf("maxIterations(%d): expected %d, got %d",
				tt.maxIter, tt.expected, coord.maxIterations)
		}
	}
}

func TestNewAgentContext(t *testing.T) {
	event := &types.AlertEvent{
		ID: 42,
		Payload: types.AlertPayload{
			MonitorID:   123,
			MonitorName: "Test",
		},
	}

	ctx := NewAgentContext(event)

	if ctx.Event != event {
		t.Error("Event not set correctly")
	}
	if ctx.Iteration != 0 {
		t.Errorf("Iteration should be 0, got %d", ctx.Iteration)
	}
	if ctx.QueryHistory == nil {
		t.Error("QueryHistory should be initialized")
	}
	if ctx.Findings == nil {
		t.Error("Findings should be initialized")
	}
	if ctx.Hypotheses == nil {
		t.Error("Hypotheses should be initialized")
	}
	if ctx.Metadata == nil {
		t.Error("Metadata should be initialized")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a longer string", 10, "this is a ..."},
		{"", 5, ""},
		{"abc", 0, "..."},
	}

	for _, tt := range tests {
		result := truncate(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncate(%q, %d): expected %q, got %q",
				tt.input, tt.maxLen, tt.expected, result)
		}
	}
}
