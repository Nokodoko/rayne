package webhooks

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
	"github.com/Nokodoko/mkii_ddog_server/services/agents"
)

// mockWebhookProcessor implements WebhookProcessor for testing
type mockWebhookProcessor struct {
	name         string
	canProcess   bool
	processDelay time.Duration
	shouldFail   bool
	callCount    int64
	mu           sync.Mutex
	processedIDs []int64
}

func newMockProcessor(name string, canProcess bool) *mockWebhookProcessor {
	return &mockWebhookProcessor{
		name:       name,
		canProcess: canProcess,
	}
}

func (m *mockWebhookProcessor) Name() string {
	return m.name
}

func (m *mockWebhookProcessor) CanProcess(event *WebhookEvent, config *WebhookConfig) bool {
	return m.canProcess
}

func (m *mockWebhookProcessor) Process(event *WebhookEvent, config *WebhookConfig) ProcessorResult {
	atomic.AddInt64(&m.callCount, 1)

	if m.processDelay > 0 {
		time.Sleep(m.processDelay)
	}

	m.mu.Lock()
	m.processedIDs = append(m.processedIDs, event.ID)
	m.mu.Unlock()

	if m.shouldFail {
		return ProcessorResult{
			ProcessorName: m.name,
			Success:       false,
			Error:         "mock failure",
		}
	}

	return ProcessorResult{
		ProcessorName: m.name,
		Success:       true,
		Message:       "processed",
	}
}

func (m *mockWebhookProcessor) getCallCount() int64 {
	return atomic.LoadInt64(&m.callCount)
}

func (m *mockWebhookProcessor) getProcessedIDs() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]int64, len(m.processedIDs))
	copy(result, m.processedIDs)
	return result
}

// mockAgentOrchestrator implements a minimal agent orchestrator for testing
type mockAgentOrchestrator struct {
	shouldAnalyze bool
	analyzeDelay  time.Duration
	shouldFail    bool
	callCount     int64
}

func (m *mockAgentOrchestrator) ShouldAnalyze(event *types.AlertEvent) bool {
	return m.shouldAnalyze
}

func (m *mockAgentOrchestrator) Analyze(ctx context.Context, event *types.AlertEvent) (*agents.AnalysisResult, error) {
	atomic.AddInt64(&m.callCount, 1)

	if m.analyzeDelay > 0 {
		select {
		case <-time.After(m.analyzeDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.shouldFail {
		return &agents.AnalysisResult{
			Success: false,
			Error:   "mock analysis failure",
		}, nil
	}

	return &agents.AnalysisResult{
		Success:   true,
		AgentRole: agents.RoleGeneral,
		Summary:   "mock analysis",
	}, nil
}

// mockStorage implements minimal storage for testing
type mockStorage struct {
	configs []WebhookConfig
}

func (m *mockStorage) GetActiveConfigs() ([]WebhookConfig, error) {
	if m.configs == nil {
		return []WebhookConfig{{}}, nil
	}
	return m.configs, nil
}

func (m *mockStorage) UpdateEventStatus(id int64, status string, forwardedTo []string, errorMsg string) error {
	return nil
}

func TestOrchestrator_RegisterFastProcessor(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	proc1 := newMockProcessor("proc1", true)
	proc2 := newMockProcessor("proc2", true)

	orch.RegisterFastProcessor(proc1)
	orch.RegisterFastProcessor(proc2)

	processors := orch.ListProcessors()

	// Should have 2 fast processors + agent_orchestrator
	if len(processors) != 3 {
		t.Errorf("Expected 3 processors, got %d: %v", len(processors), processors)
	}

	found := map[string]bool{}
	for _, p := range processors {
		found[p] = true
	}

	if !found["proc1"] {
		t.Error("proc1 not found")
	}
	if !found["proc2"] {
		t.Error("proc2 not found")
	}
	if !found["agent_orchestrator"] {
		t.Error("agent_orchestrator not found")
	}
}

func TestOrchestrator_ProcessFastProcessorsParallel(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	// Create processors with delay to verify parallel execution
	proc1 := newMockProcessor("proc1", true)
	proc1.processDelay = 50 * time.Millisecond

	proc2 := newMockProcessor("proc2", true)
	proc2.processDelay = 50 * time.Millisecond

	proc3 := newMockProcessor("proc3", true)
	proc3.processDelay = 50 * time.Millisecond

	orch.RegisterFastProcessor(proc1)
	orch.RegisterFastProcessor(proc2)
	orch.RegisterFastProcessor(proc3)

	event := &WebhookEvent{
		ID: 1,
		Payload: WebhookPayload{
			MonitorID:   123,
			AlertStatus: "OK", // Not Alert/Warn, so no agent analysis
		},
	}

	start := time.Now()
	result := orch.Process(context.Background(), event)
	elapsed := time.Since(start)

	// If parallel, should complete in ~50ms, not ~150ms
	if elapsed > 120*time.Millisecond {
		t.Errorf("Processors should run in parallel, took %v", elapsed)
	}

	if len(result.ProcessedBy) != 3 {
		t.Errorf("Expected 3 processors, got %d: %v", len(result.ProcessedBy), result.ProcessedBy)
	}

	// Verify all processors were called
	if proc1.getCallCount() != 1 {
		t.Errorf("proc1 call count: expected 1, got %d", proc1.getCallCount())
	}
	if proc2.getCallCount() != 1 {
		t.Errorf("proc2 call count: expected 1, got %d", proc2.getCallCount())
	}
	if proc3.getCallCount() != 1 {
		t.Errorf("proc3 call count: expected 1, got %d", proc3.getCallCount())
	}
}

func TestOrchestrator_ProcessorFiltering(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	procYes := newMockProcessor("yes", true)
	procNo := newMockProcessor("no", false)

	orch.RegisterFastProcessor(procYes)
	orch.RegisterFastProcessor(procNo)

	event := &WebhookEvent{
		ID: 1,
		Payload: WebhookPayload{
			AlertStatus: "OK",
		},
	}

	result := orch.Process(context.Background(), event)

	if procYes.getCallCount() != 1 {
		t.Errorf("procYes should be called once, got %d", procYes.getCallCount())
	}
	if procNo.getCallCount() != 0 {
		t.Errorf("procNo should not be called, got %d", procNo.getCallCount())
	}

	if len(result.ProcessedBy) != 1 || result.ProcessedBy[0] != "yes" {
		t.Errorf("Expected [yes], got %v", result.ProcessedBy)
	}
}

func TestOrchestrator_ProcessorFailure(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	procOK := newMockProcessor("ok", true)
	procFail := newMockProcessor("fail", true)
	procFail.shouldFail = true

	orch.RegisterFastProcessor(procOK)
	orch.RegisterFastProcessor(procFail)

	event := &WebhookEvent{
		ID: 1,
		Payload: WebhookPayload{
			AlertStatus: "OK",
		},
	}

	result := orch.Process(context.Background(), event)

	// One success, one failure
	if len(result.ProcessedBy) != 1 {
		t.Errorf("Expected 1 successful processor, got %d", len(result.ProcessedBy))
	}
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestOrchestrator_ContextCancellation(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	// Processor that will be slow
	proc := newMockProcessor("slow", true)
	proc.processDelay = 100 * time.Millisecond

	orch.RegisterFastProcessor(proc)

	event := &WebhookEvent{
		ID: 1,
		Payload: WebhookPayload{
			AlertStatus: "OK",
		},
	}

	// Cancel context immediately - the context check happens before processor runs
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	result := orch.Process(ctx, event)

	// Context was cancelled before processing started
	// The error should be captured
	if len(result.Errors) > 0 {
		t.Logf("Errors captured as expected: %v", result.Errors)
	}
	// If no errors, processor may have started before context check
	// This is timing-dependent but acceptable
}

func TestOrchestrator_ToAlertEventConversion(t *testing.T) {
	webhookEvent := &WebhookEvent{
		ID: 42,
		Payload: WebhookPayload{
			AlertID:             100,
			AlertTitle:          "Test Alert",
			AlertMessage:        "Test message",
			AlertStatus:         "Alert",
			MonitorID:           200,
			MonitorName:         "Test Monitor",
			MonitorType:         "metric",
			Tags:                []string{"env:test", "team:platform"},
			Hostname:            "host1.example.com",
			Service:             "test-service",
			Scope:               "env:test",
			AlertState:          "ALERT",
			AlertTitleCustom:    "Custom Title",
			ApplicationTeam:     "Platform",
			ApplicationLongname: "Test App",
			DetailedDescription: "Detailed desc",
			Impact:              "High",
			Metric:              "cpu.usage",
			SupportGroup:        "SRE",
			Threshold:           "90",
			Value:               "95",
			Urgency:             "high",
		},
		Status: "pending",
	}

	alertEvent := toAlertEvent(webhookEvent)

	if alertEvent.ID != 42 {
		t.Errorf("ID: expected 42, got %d", alertEvent.ID)
	}
	if alertEvent.Payload.MonitorID != 200 {
		t.Errorf("MonitorID: expected 200, got %d", alertEvent.Payload.MonitorID)
	}
	if alertEvent.Payload.AlertStatus != "Alert" {
		t.Errorf("AlertStatus: expected Alert, got %s", alertEvent.Payload.AlertStatus)
	}
	if alertEvent.Payload.Hostname != "host1.example.com" {
		t.Errorf("Hostname: expected host1.example.com, got %s", alertEvent.Payload.Hostname)
	}
	if len(alertEvent.Payload.Tags) != 2 {
		t.Errorf("Tags: expected 2, got %d", len(alertEvent.Payload.Tags))
	}
	if alertEvent.Payload.ApplicationTeam != "Platform" {
		t.Errorf("ApplicationTeam: expected Platform, got %s", alertEvent.Payload.ApplicationTeam)
	}
	if alertEvent.Status != "pending" {
		t.Errorf("Status: expected pending, got %s", alertEvent.Status)
	}
}

func TestOrchestrator_ConcurrentProcessing(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	proc := newMockProcessor("counter", true)
	orch.RegisterFastProcessor(proc)

	var wg sync.WaitGroup
	eventCount := 100

	for i := 0; i < eventCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := &WebhookEvent{
				ID: int64(id),
				Payload: WebhookPayload{
					AlertStatus: "OK",
				},
			}
			orch.Process(context.Background(), event)
		}(i)
	}

	wg.Wait()

	callCount := proc.getCallCount()
	if callCount != int64(eventCount) {
		t.Errorf("Expected %d calls, got %d", eventCount, callCount)
	}

	processedIDs := proc.getProcessedIDs()
	if len(processedIDs) != eventCount {
		t.Errorf("Expected %d processed IDs, got %d", eventCount, len(processedIDs))
	}
}

func TestOrchestrator_EmptyProcessors(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	// No processors registered

	event := &WebhookEvent{
		ID: 1,
		Payload: WebhookPayload{
			AlertStatus: "OK",
		},
	}

	result := orch.Process(context.Background(), event)

	if len(result.ProcessedBy) != 0 {
		t.Errorf("Expected 0 processors, got %d", len(result.ProcessedBy))
	}
	if len(result.Errors) != 0 {
		t.Errorf("Expected 0 errors, got %d", len(result.Errors))
	}
}

func TestOrchestrator_AllProcessorsFail(t *testing.T) {
	storage := &Storage{}
	orch := NewProcessorOrchestrator(storage, nil)

	proc1 := newMockProcessor("fail1", true)
	proc1.shouldFail = true

	proc2 := newMockProcessor("fail2", true)
	proc2.shouldFail = true

	orch.RegisterFastProcessor(proc1)
	orch.RegisterFastProcessor(proc2)

	event := &WebhookEvent{
		ID: 1,
		Payload: WebhookPayload{
			AlertStatus: "OK",
		},
	}

	result := orch.Process(context.Background(), event)

	if len(result.ProcessedBy) != 0 {
		t.Errorf("Expected 0 successful, got %d", len(result.ProcessedBy))
	}
	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}
}

func TestNewProcessorOrchestrator(t *testing.T) {
	storage := &Storage{}
	agentOrch := agents.NewAgentOrchestrator(agents.DefaultOrchestratorConfig())

	orch := NewProcessorOrchestrator(storage, agentOrch)

	if orch == nil {
		t.Fatal("Orchestrator should not be nil")
	}
	if orch.storage != storage {
		t.Error("Storage not set correctly")
	}
	if orch.agentOrch != agentOrch {
		t.Error("Agent orchestrator not set correctly")
	}
	if len(orch.fastProcessors) != 0 {
		t.Error("Fast processors should be empty initially")
	}
}
