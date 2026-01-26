package agents

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
)

// mockAgent implements Agent interface for testing
type mockAgent struct {
	name           string
	role           AgentRole
	planComplete   bool
	analyzeDelay   time.Duration
	concludeResult *AnalysisResult
	planCalls      int64
	analyzeCalls   int64
	concludeCalls  int64
}

func newMockAgent(name string, role AgentRole) *mockAgent {
	return &mockAgent{
		name: name,
		role: role,
		concludeResult: &AnalysisResult{
			Success:   true,
			AgentRole: role,
			Summary:   "mock analysis",
		},
	}
}

func (m *mockAgent) Name() string {
	return m.name
}

func (m *mockAgent) Role() AgentRole {
	return m.role
}

func (m *mockAgent) Plan(ctx context.Context, event *types.AlertEvent, agentCtx AgentContext) AgentPlan {
	atomic.AddInt64(&m.planCalls, 1)
	return AgentPlan{
		Complete:  m.planComplete || agentCtx.Iteration > 0,
		Reasoning: "mock plan",
	}
}

func (m *mockAgent) Analyze(ctx context.Context, results []QueryResult, agentCtx AgentContext) AgentContext {
	atomic.AddInt64(&m.analyzeCalls, 1)
	if m.analyzeDelay > 0 {
		time.Sleep(m.analyzeDelay)
	}
	agentCtx.RootCause = "mock root cause"
	return agentCtx
}

func (m *mockAgent) Conclude(ctx context.Context, agentCtx AgentContext) *AnalysisResult {
	atomic.AddInt64(&m.concludeCalls, 1)
	result := *m.concludeResult
	result.MonitorID = agentCtx.Event.Payload.MonitorID
	result.MonitorName = agentCtx.Event.Payload.MonitorName
	return &result
}

func TestAgentOrchestrator_RegisterAgent(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewAgentOrchestrator(config)

	agent := newMockAgent("test-infra", RoleInfrastructure)
	orch.RegisterAgent(agent)

	stats := orch.Stats()
	if stats.RegisteredAgents != 1 {
		t.Errorf("Expected 1 registered agent, got %d", stats.RegisteredAgents)
	}
}

func TestAgentOrchestrator_SetDefaultAgent(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewAgentOrchestrator(config)

	defaultAgent := newMockAgent("default", RoleGeneral)
	orch.SetDefaultAgent(defaultAgent)

	// Create event that won't match any specific role
	event := &types.AlertEvent{
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	result, err := orch.Analyze(context.Background(), event)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.AgentRole != RoleGeneral {
		t.Errorf("Expected RoleGeneral, got %s", result.AgentRole)
	}
}

func TestAgentOrchestrator_ShouldAnalyze(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewAgentOrchestrator(config)

	tests := []struct {
		status   string
		expected bool
	}{
		{"Alert", true},
		{"Warn", true},
		{"OK", false},
		{"No Data", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					AlertStatus: tt.status,
				},
			}
			result := orch.ShouldAnalyze(event)
			if result != tt.expected {
				t.Errorf("ShouldAnalyze(%s): expected %v, got %v", tt.status, tt.expected, result)
			}
		})
	}
}

func TestAgentOrchestrator_BoundedConcurrency(t *testing.T) {
	config := OrchestratorConfig{
		MaxConcurrent:    2,
		RLMMaxIterations: 1,
	}
	orch := NewAgentOrchestrator(config)

	// Create a slow agent with delay to ensure overlapping execution
	slowAgent := newMockAgent("slow", RoleGeneral)
	slowAgent.analyzeDelay = 50 * time.Millisecond
	slowAgent.planComplete = true // Complete immediately after analyze
	orch.SetDefaultAgent(slowAgent)

	var wg sync.WaitGroup
	var maxActive int64
	var mu sync.Mutex
	var activeCount int64

	// Launch more goroutines than MaxConcurrent
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					MonitorID:   int64(id),
					AlertStatus: "Alert",
				},
			}

			orch.Analyze(context.Background(), event)
		}(i)
	}

	// Monitor the active count via orchestrator stats
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				stats := orch.Stats()
				mu.Lock()
				if stats.ActiveAnalyses > maxActive {
					maxActive = stats.ActiveAnalyses
				}
				current := atomic.LoadInt64(&activeCount)
				if current > maxActive {
					maxActive = current
				}
				mu.Unlock()
				time.Sleep(5 * time.Millisecond)
			}
		}
	}()

	wg.Wait()
	close(done)

	mu.Lock()
	max := maxActive
	mu.Unlock()

	// Due to semaphore, should never exceed MaxConcurrent
	if max > int64(config.MaxConcurrent) {
		t.Errorf("Max concurrent %d exceeded limit %d", max, config.MaxConcurrent)
	}
}

func TestAgentOrchestrator_ContextCancellation(t *testing.T) {
	config := OrchestratorConfig{
		MaxConcurrent:    1,
		RLMMaxIterations: 5,
	}
	orch := NewAgentOrchestrator(config)

	slowAgent := newMockAgent("slow", RoleGeneral)
	slowAgent.analyzeDelay = 500 * time.Millisecond
	orch.SetDefaultAgent(slowAgent)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	event := &types.AlertEvent{
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	start := time.Now()
	_, err := orch.Analyze(ctx, event)
	elapsed := time.Since(start)

	if err != context.DeadlineExceeded {
		t.Logf("Expected context.DeadlineExceeded, got %v (elapsed: %v)", err, elapsed)
	}

	if elapsed > 200*time.Millisecond {
		t.Errorf("Should have cancelled faster, took %v", elapsed)
	}
}

func TestAgentOrchestrator_Stats(t *testing.T) {
	config := OrchestratorConfig{
		MaxConcurrent:    5,
		RLMMaxIterations: 3,
	}
	orch := NewAgentOrchestrator(config)

	agent1 := newMockAgent("infra", RoleInfrastructure)
	agent2 := newMockAgent("app", RoleApplication)
	orch.RegisterAgent(agent1)
	orch.RegisterAgent(agent2)

	stats := orch.Stats()

	if stats.MaxConcurrent != 5 {
		t.Errorf("MaxConcurrent: expected 5, got %d", stats.MaxConcurrent)
	}
	if stats.RegisteredAgents != 2 {
		t.Errorf("RegisteredAgents: expected 2, got %d", stats.RegisteredAgents)
	}
	if stats.ActiveAnalyses != 0 {
		t.Errorf("ActiveAnalyses: expected 0, got %d", stats.ActiveAnalyses)
	}
	if stats.TotalProcessed != 0 {
		t.Errorf("TotalProcessed: expected 0, got %d", stats.TotalProcessed)
	}
}

func TestAgentOrchestrator_RoleRouting(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewAgentOrchestrator(config)

	infraAgent := newMockAgent("infra", RoleInfrastructure)
	appAgent := newMockAgent("app", RoleApplication)
	dbAgent := newMockAgent("db", RoleDatabase)

	orch.RegisterAgent(infraAgent)
	orch.RegisterAgent(appAgent)
	orch.RegisterAgent(dbAgent)

	tests := []struct {
		name        string
		monitorType string
		expected    AgentRole
	}{
		{"APM routes to Application", "apm", RoleApplication},
		{"Metric routes to Infrastructure", "metric", RoleInfrastructure},
		{"DBM routes to Database", "dbm", RoleDatabase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &types.AlertEvent{
				Payload: types.AlertPayload{
					MonitorID:   123,
					MonitorType: tt.monitorType,
					AlertStatus: "Alert",
				},
			}

			result, err := orch.Analyze(context.Background(), event)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}
			if result.AgentRole != tt.expected {
				t.Errorf("Expected role %s, got %s", tt.expected, result.AgentRole)
			}
		})
	}
}

func TestAgentOrchestrator_NoAgentAvailable(t *testing.T) {
	config := DefaultOrchestratorConfig()
	orch := NewAgentOrchestrator(config)

	// Don't register any agents or set default

	event := &types.AlertEvent{
		Payload: types.AlertPayload{
			MonitorID:   123,
			AlertStatus: "Alert",
		},
	}

	result, err := orch.Analyze(context.Background(), event)
	if err != nil {
		t.Fatalf("Should not return error: %v", err)
	}
	if result.Success {
		t.Error("Should not be successful without agent")
	}
	if result.Error == "" {
		t.Error("Should have error message")
	}
}

func TestDefaultOrchestratorConfig(t *testing.T) {
	config := DefaultOrchestratorConfig()

	if config.MaxConcurrent <= 0 {
		t.Error("MaxConcurrent should be > 0")
	}
	if config.RLMMaxIterations <= 0 {
		t.Error("RLMMaxIterations should be > 0")
	}
}

func TestNewAgentOrchestrator_DefaultValues(t *testing.T) {
	// Test with zero values
	config := OrchestratorConfig{
		MaxConcurrent:    0,
		RLMMaxIterations: 0,
	}
	orch := NewAgentOrchestrator(config)

	stats := orch.Stats()
	if stats.MaxConcurrent != 3 { // Default
		t.Errorf("Expected default MaxConcurrent 3, got %d", stats.MaxConcurrent)
	}
}
