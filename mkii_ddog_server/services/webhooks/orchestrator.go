package webhooks

import (
	"context"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/Nokodoko/mkii_ddog_server/cmd/types"
	"github.com/Nokodoko/mkii_ddog_server/services/agents"
)

// ProcessorOrchestrator manages webhook processing with tiered execution:
// - Tier 1 (fast): Parallel execution of quick processors
// - Tier 2 (slow): Bounded execution of agent analysis
type ProcessorOrchestrator struct {
	fastProcessors []WebhookProcessor
	agentOrch      *agents.AgentOrchestrator
	storage        *Storage
	notifier       *Notifier
	mu             sync.RWMutex
}

// OrchestratorResult contains the results of processing a webhook
type OrchestratorResult struct {
	ProcessedBy []string
	Errors      []string
	AgentResult *agents.AnalysisResult
}

// NewProcessorOrchestrator creates a new orchestrator
func NewProcessorOrchestrator(storage *Storage, agentOrch *agents.AgentOrchestrator) *ProcessorOrchestrator {
	return &ProcessorOrchestrator{
		fastProcessors: make([]WebhookProcessor, 0),
		agentOrch:      agentOrch,
		storage:        storage,
		notifier:       NewNotifier(),
	}
}

// RegisterFastProcessor adds a fast processor (desktop notify, forwarding, downtime)
func (o *ProcessorOrchestrator) RegisterFastProcessor(processor WebhookProcessor) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.fastProcessors = append(o.fastProcessors, processor)
	log.Printf("[ORCHESTRATOR] Registered fast processor: %s", processor.Name())
}

// ListProcessors returns the names of all registered processors
func (o *ProcessorOrchestrator) ListProcessors() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()

	names := make([]string, 0, len(o.fastProcessors)+1)
	for _, p := range o.fastProcessors {
		names = append(names, p.Name())
	}
	names = append(names, "agent_orchestrator")
	return names
}

// Process handles a webhook event with tiered execution
func (o *ProcessorOrchestrator) Process(ctx context.Context, event *WebhookEvent) OrchestratorResult {
	result := OrchestratorResult{
		ProcessedBy: make([]string, 0),
		Errors:      make([]string, 0),
	}

	log.Printf("[ORCHESTRATOR] Processing event %d (status: %s)",
		event.ID, event.Payload.AlertStatus)

	// Get active configurations (handle nil storage for testing)
	var configs []WebhookConfig
	var err error
	if o.storage != nil && o.storage.db != nil {
		configs, err = o.storage.GetActiveConfigs()
		if err != nil {
			log.Printf("[ORCHESTRATOR] Error getting configs: %v", err)
			result.Errors = append(result.Errors, "failed to get configs: "+err.Error())
			o.storage.UpdateEventStatus(event.ID, "failed", nil, err.Error())
			return result
		}
	}

	// Use a default empty config if none exist
	if len(configs) == 0 {
		configs = []WebhookConfig{{}}
	}

	// --- TIER 1: Fast processors in parallel ---
	o.mu.RLock()
	processors := make([]WebhookProcessor, len(o.fastProcessors))
	copy(processors, o.fastProcessors)
	o.mu.RUnlock()

	var fastResults []ProcessorResult
	var forwardedTo []string

	for _, config := range configs {
		configCopy := config
		tier1Results := o.executeFastProcessors(ctx, event, &configCopy, processors)
		fastResults = append(fastResults, tier1Results...)

		for _, r := range tier1Results {
			if r.Success {
				result.ProcessedBy = append(result.ProcessedBy, r.ProcessorName)
			} else if r.Error != "" {
				result.Errors = append(result.Errors, r.ProcessorName+": "+r.Error)
			}
			forwardedTo = append(forwardedTo, r.ForwardedTo...)
		}
	}

	// --- TIER 2: Agent analysis (bounded by semaphore) ---
	// Convert to AlertEvent for agent processing
	alertEvent := toAlertEvent(event)
	if o.agentOrch != nil && o.agentOrch.ShouldAnalyze(alertEvent) {
		log.Printf("[ORCHESTRATOR] Triggering agent analysis for event %d", event.ID)

		agentResult, err := o.agentOrch.Analyze(ctx, alertEvent)
		result.AgentResult = agentResult

		if err != nil {
			log.Printf("[ORCHESTRATOR] Agent analysis failed for event %d: %v", event.ID, err)
			result.Errors = append(result.Errors, "agent_analysis: "+err.Error())
		} else if agentResult != nil {
			result.ProcessedBy = append(result.ProcessedBy, "agent_"+string(agentResult.AgentRole))
			if !agentResult.Success && agentResult.Error != "" {
				result.Errors = append(result.Errors, "agent_analysis: "+agentResult.Error)
			}

			// Send desktop notification when a notebook is created
			if agentResult.NotebookURL != "" && o.notifier != nil {
				o.notifier.NotifyNotebookCreated(
					agentResult.MonitorName,
					string(agentResult.AgentRole),
					agentResult.NotebookURL,
				)
			}
		}
	} else if o.agentOrch != nil && o.agentOrch.ShouldRecover(alertEvent) {
		// Recovery event: notify the agent sidecar to resolve the existing notebook
		log.Printf("[ORCHESTRATOR] Triggering recovery for event %d (monitor %d, status: %s)",
			event.ID, alertEvent.Payload.MonitorID, alertEvent.Payload.AlertStatus)

		recoverResult, err := o.agentOrch.Recover(ctx, alertEvent)
		if err != nil {
			log.Printf("[ORCHESTRATOR] Recovery failed for event %d: %v", event.ID, err)
			result.Errors = append(result.Errors, "agent_recovery: "+err.Error())
		} else if recoverResult != nil {
			result.ProcessedBy = append(result.ProcessedBy, "agent_recovery")
			if !recoverResult.Success && recoverResult.Error != "" {
				result.Errors = append(result.Errors, "agent_recovery: "+recoverResult.Error)
			}
		}
	}

	// Determine final status
	status := "processed"
	var errorMsg string

	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			if errorMsg != "" {
				errorMsg += "; "
			}
			errorMsg += e
		}
		// Only mark as failed if ALL processing failed
		if len(result.ProcessedBy) == 0 {
			status = "failed"
		}
	}

	if o.storage != nil && o.storage.db != nil {
		o.storage.UpdateEventStatus(event.ID, status, forwardedTo, errorMsg)
	}

	log.Printf("[ORCHESTRATOR] Event %d processed: status=%s, processors=%v, errors=%d",
		event.ID, status, result.ProcessedBy, len(result.Errors))

	return result
}

// executeFastProcessors runs fast processors in parallel using fan-out
func (o *ProcessorOrchestrator) executeFastProcessors(
	ctx context.Context,
	event *WebhookEvent,
	config *WebhookConfig,
	processors []WebhookProcessor,
) []ProcessorResult {
	// Find applicable processors
	var applicable []WebhookProcessor
	for _, proc := range processors {
		if proc.CanProcess(event, config) {
			applicable = append(applicable, proc)
		}
	}

	if len(applicable) == 0 {
		return nil
	}

	// Fan-out: execute all in parallel
	resultsCh := make(chan ProcessorResult, len(applicable))
	var wg sync.WaitGroup

	for _, proc := range applicable {
		wg.Add(1)
		go func(p WebhookProcessor) {
			defer wg.Done()

			// Check context before processing
			select {
			case <-ctx.Done():
				resultsCh <- ProcessorResult{
					ProcessorName: p.Name(),
					Success:       false,
					Error:         "context cancelled",
				}
				return
			default:
			}

			log.Printf("[ORCHESTRATOR] Running fast processor %s for event %d", p.Name(), event.ID)
			result := p.Process(event, config)

			if result.Success {
				log.Printf("[ORCHESTRATOR] Fast processor %s succeeded: %s", p.Name(), result.Message)
			} else {
				log.Printf("[ORCHESTRATOR] Fast processor %s failed: %s", p.Name(), result.Error)
			}

			resultsCh <- result
		}(proc)
	}

	// Fan-in: collect results
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	var results []ProcessorResult
	for r := range resultsCh {
		results = append(results, r)
	}

	return results
}

// ProcessPending reprocesses all pending webhook events
func (o *ProcessorOrchestrator) ProcessPending(ctx context.Context) error {
	events, _, err := o.storage.GetRecentEvents(100, 0)
	if err != nil {
		return err
	}

	count := 0
	for _, event := range events {
		if event.Status == "pending" {
			eventCopy := event
			go func() {
				o.Process(ctx, &eventCopy)
			}()
			count++
		}
	}

	log.Printf("[ORCHESTRATOR] Reprocessing %d pending events", count)
	return nil
}

// monitorTypePattern matches service values that are actually Datadog monitor types
// rather than real application service names.
var monitorTypePattern = regexp.MustCompile(
	`(?i)^(http-check|process-check|tcp-check|dns-check|ssl-check|grpc-check|service-check|custom-check|metric alert|query alert|composite|synthetics|event-v2 alert|watchdog)$`,
)

// resolveServiceName determines the actual service name from a webhook payload.
// Custom Datadog webhook templates set APPLICATION_TEAM to the real service/team
// name, while the standard service field often contains the monitor type
// (e.g., "http-check") instead of the actual service.
// Priority: APPLICATION_TEAM > scope tag > service (if not monitor type) > fallback.
func resolveServiceName(p WebhookPayload) string {
	// 1. APPLICATION_TEAM is the most reliable source
	if appTeam := strings.TrimSpace(p.ApplicationTeam); appTeam != "" {
		return appTeam
	}

	// 2. Check scope for application_team tag
	if p.Scope != "" {
		re := regexp.MustCompile(`application_team:([^,\s]+)`)
		if m := re.FindStringSubmatch(p.Scope); len(m) > 1 {
			return m[1]
		}
	}

	// 3. Check tags for application_team
	for _, tag := range p.Tags {
		if strings.HasPrefix(tag, "application_team:") {
			if val := strings.TrimPrefix(tag, "application_team:"); val != "" {
				return val
			}
		}
	}

	// 4. Use service only if it doesn't look like a monitor type
	if svc := strings.TrimSpace(p.Service); svc != "" && !monitorTypePattern.MatchString(svc) {
		return svc
	}

	// 5. Fallback to raw service
	if p.Service != "" {
		return p.Service
	}
	return ""
}

// toAlertEvent converts a WebhookEvent to an AlertEvent for agent processing.
// Custom webhook templates populate uppercase fields (ALERT_TITLE, ALERT_STATE)
// while leaving standard lowercase fields empty. This function fills in standard
// fields from their uppercase counterparts when empty so downstream code works.
func toAlertEvent(event *WebhookEvent) *types.AlertEvent {
	p := event.Payload

	// Fill standard fields from custom/uppercase equivalents when empty
	alertTitle := p.AlertTitle
	if alertTitle == "" {
		alertTitle = p.AlertTitleCustom
	}
	alertStatus := p.AlertStatus
	if alertStatus == "" && p.AlertState == "Triggered" {
		alertStatus = "Alert"
	}

	// Resolve the actual service name (APPLICATION_TEAM > scope tag > service)
	resolvedService := resolveServiceName(p)
	if resolvedService != p.Service {
		log.Printf("[ORCHESTRATOR] Resolved service: %q (raw: %q, APPLICATION_TEAM: %q)",
			resolvedService, p.Service, p.ApplicationTeam)
	}

	return &types.AlertEvent{
		ID: event.ID,
		Payload: types.AlertPayload{
			AlertID:             p.AlertID,
			AlertTitle:          alertTitle,
			AlertMessage:        p.AlertMessage,
			AlertStatus:         alertStatus,
			MonitorID:           p.MonitorID,
			MonitorName:         p.MonitorName,
			MonitorType:         p.MonitorType,
			Tags:                p.Tags,
			Timestamp:           p.Timestamp,
			EventType:           p.EventType,
			Priority:            p.Priority,
			Hostname:            p.Hostname,
			Service:             resolvedService,
			Scope:               p.Scope,
			TransitionID:        p.TransitionID,
			LastUpdated:         p.LastUpdated,
			SnapshotURL:         p.SnapshotURL,
			Link:                p.Link,
			OrgID:               p.OrgID,
			OrgName:             p.OrgName,
			AlertState:          p.AlertState,
			AlertTitleCustom:    p.AlertTitleCustom,
			ApplicationLongname: p.ApplicationLongname,
			ApplicationTeam:     p.ApplicationTeam,
			DetailedDescription: p.DetailedDescription,
			Impact:              p.Impact,
			Metric:              p.Metric,
			SupportGroup:        p.SupportGroup,
			Threshold:           p.Threshold,
			Value:               p.Value,
			Urgency:             p.Urgency,
		},
		ReceivedAt:  event.ReceivedAt,
		ProcessedAt: event.ProcessedAt,
		Status:      event.Status,
		ForwardedTo: event.ForwardedTo,
		Error:       event.Error,
	}
}
