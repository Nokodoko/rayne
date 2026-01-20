package webhooks

import (
	"log"
	"sync"
)

// Processor orchestrates webhook processing using registered WebhookProcessors
type Processor struct {
	storage    *Storage
	processors []WebhookProcessor
	mu         sync.RWMutex
}

// NewProcessor creates a new webhook processor with default processors registered
func NewProcessor(storage *Storage, defaultProcessors ...WebhookProcessor) *Processor {
	p := &Processor{
		storage:    storage,
		processors: make([]WebhookProcessor, 0),
	}

	// Register default processors
	for _, proc := range defaultProcessors {
		p.Register(proc)
	}

	return p
}

// Register adds a new processor to the registry.
// Processors are executed in registration order.
func (p *Processor) Register(processor WebhookProcessor) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.processors = append(p.processors, processor)
	log.Printf("[PROCESSOR] Registered processor: %s", processor.Name())
}

// Unregister removes a processor by name
func (p *Processor) Unregister(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	for i, proc := range p.processors {
		if proc.Name() == name {
			p.processors = append(p.processors[:i], p.processors[i+1:]...)
			log.Printf("[PROCESSOR] Unregistered processor: %s", name)
			return true
		}
	}
	return false
}

// ListProcessors returns the names of all registered processors
func (p *Processor) ListProcessors() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	names := make([]string, len(p.processors))
	for i, proc := range p.processors {
		names[i] = proc.Name()
	}
	return names
}

// Process handles a webhook event by running it through all applicable processors
func (p *Processor) Process(event *WebhookEvent) {
	log.Printf("[PROCESSOR] Processing event ID: %d, Monitor: %s, Status: %s",
		event.ID, event.Payload.MonitorName, event.Payload.AlertStatus)

	// Get active configurations
	configs, err := p.storage.GetActiveConfigs()
	if err != nil {
		log.Printf("[PROCESSOR] Error getting configs: %v", err)
		p.storage.UpdateEventStatus(event.ID, "failed", nil, err.Error())
		return
	}

	// Use a default empty config if none exist
	if len(configs) == 0 {
		configs = []WebhookConfig{{}}
	}

	var allResults []ProcessorResult
	var allForwardedTo []string

	p.mu.RLock()
	processors := make([]WebhookProcessor, len(p.processors))
	copy(processors, p.processors)
	p.mu.RUnlock()

	// Process each configuration
	for _, config := range configs {
		configCopy := config // Avoid closure issues

		// Run each processor
		for _, proc := range processors {
			if !proc.CanProcess(event, &configCopy) {
				continue
			}

			log.Printf("[PROCESSOR] Running %s for event %d", proc.Name(), event.ID)

			// Execute processor (some may run async internally)
			result := proc.Process(event, &configCopy)
			allResults = append(allResults, result)

			if result.Success {
				log.Printf("[PROCESSOR] %s succeeded: %s", proc.Name(), result.Message)
			} else {
				log.Printf("[PROCESSOR] %s failed: %s", proc.Name(), result.Error)
			}

			// Collect forwarded URLs
			allForwardedTo = append(allForwardedTo, result.ForwardedTo...)
		}
	}

	// Determine final status
	status := "processed"
	var errorMsg string

	hasError := false
	for _, r := range allResults {
		if !r.Success {
			hasError = true
			if errorMsg != "" {
				errorMsg += "; "
			}
			errorMsg += r.ProcessorName + ": " + r.Error
		}
	}

	if hasError && len(allResults) > 0 {
		// Check if all failed
		allFailed := true
		for _, r := range allResults {
			if r.Success {
				allFailed = false
				break
			}
		}
		if allFailed {
			status = "failed"
		}
	}

	p.storage.UpdateEventStatus(event.ID, status, allForwardedTo, errorMsg)
}

// ProcessPending reprocesses all pending webhook events
func (p *Processor) ProcessPending() error {
	events, _, err := p.storage.GetRecentEvents(100, 0)
	if err != nil {
		return err
	}

	count := 0
	for _, event := range events {
		if event.Status == "pending" {
			eventCopy := event
			go p.Process(&eventCopy)
			count++
		}
	}

	log.Printf("[PROCESSOR] Reprocessing %d pending events", count)
	return nil
}
