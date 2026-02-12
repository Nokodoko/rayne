# agentic_instructions.md

## Purpose
Webhook ingestion, storage, and processing pipeline. Receives Datadog webhooks, stores events in PostgreSQL, and routes them through a tiered processing system (fast processors in parallel, then bounded agent analysis).

## Technology
Go, net/http, database/sql, encoding/json, sync, context, lib/pq

## Contents
- `handler.go` -- HTTP handlers for webhook CRUD operations and dispatcher stats
- `types.go` -- WebhookProcessor interface, WebhookPayload, WebhookEvent, WebhookConfig, ProcessorResult structs
- `storage.go` -- PostgreSQL storage (webhook_events, webhook_configs tables) with auto-migration
- `dispatcher.go` -- Worker pool with bounded concurrency, backpressure queue, graceful shutdown
- `orchestrator.go` -- ProcessorOrchestrator with tiered execution (Tier 1: fast parallel, Tier 2: agent analysis)
- `processor.go` -- Legacy Processor with sequential Register/Unregister/Process pattern
- `downtime.go` -- DowntimeService for creating Datadog API v2 downtimes after monitor recovery
- `processors/` -- Subdirectory containing WebhookProcessor implementations

## Key Functions
- `NewHandler(storage, dispatcher) *Handler` -- Creates webhook handler with dispatcher
- `NewHandlerWithAccounts(storage, dispatcher, accounts) *Handler` -- Creates handler with multi-account support
- `(h *Handler) ReceiveWebhook(w, r) (int, any)` -- Ingests webhook, stores event, submits to dispatcher
- `(h *Handler) GetWebhookEvents(w, r) (int, any)` -- Paginated event retrieval
- `(h *Handler) CreateWebhook(w, r) (int, any)` -- Creates webhook in Datadog via API
- `NewDispatcher(orchestrator, config) *Dispatcher` -- Creates worker pool dispatcher
- `(d *Dispatcher) Submit(ctx, event) error` -- Queues event with backpressure
- `(d *Dispatcher) Shutdown()` -- Graceful shutdown with 30s timeout
- `NewProcessorOrchestrator(storage, agentOrch) *ProcessorOrchestrator` -- Creates tiered orchestrator
- `(o *ProcessorOrchestrator) Process(ctx, event) OrchestratorResult` -- Tiered processing: fast parallel then agent
- `(s *Storage) InitTables() error` -- Creates webhook_events and webhook_configs tables with indexes
- `(s *Storage) StoreEventWithAccount(payload, accountID, accountName) (*WebhookEvent, error)` -- Stores event with account
- `(d *DowntimeService) CreateForMonitor(monitorID, scope, duration) error` -- Creates Datadog downtime

## Data Types
- `WebhookProcessor` -- interface: Name(), CanProcess(event, config), Process(event, config) ProcessorResult
- `WebhookPayload` -- struct: 30+ fields including AlertID, AlertTitle, AlertStatus, MonitorID, Tags, custom fields (ALERT_STATE, APPLICATION_TEAM, etc.)
- `WebhookEvent` -- struct: ID, Payload, ReceivedAt, ProcessedAt, Status, ForwardedTo, Error, AccountID, AccountName
- `WebhookConfig` -- struct: ID, Name, URL, UseCustomPayload, ForwardURLs, AutoDowntime, NotifyEnabled, Active
- `ProcessorResult` -- struct: ProcessorName, Success, Message, Error, ForwardedTo
- `Dispatcher` -- struct: workQueue chan, workers, orchestrator, metrics (processedCount, errorCount, droppedCount)
- `DispatcherStats` -- struct: QueueSize, QueueCapacity, ActiveWorkers, TotalWorkers, ProcessedCount, ErrorCount, DroppedCount
- `OrchestratorResult` -- struct: ProcessedBy, Errors, AgentResult
- `AccountResolver` -- interface: ResolveAccount(orgID, accountName), GetDefault()

## Logging
Uses `log.Printf` with prefixes: `[WEBHOOK]`, `[DISPATCHER]`, `[ORCHESTRATOR]`, `[PROCESSOR]`

## CRUD Entry Points
- **Create**: Add new WebhookProcessor implementation in `processors/` subdirectory, register via `orchestrator.RegisterFastProcessor()`
- **Read**: Import `webhooks.NewHandler()`, call `handler.GetWebhookEvents()` etc.
- **Update**: Modify dispatcher config (workers, queue size), orchestrator tiers, storage queries
- **Delete**: Unregister processors via `Processor.Unregister(name)`

## Style Guide
- Handler signature convention: `func(w http.ResponseWriter, r *http.Request) (int, any)`
- Fan-out/fan-in pattern for parallel processing with `sync.WaitGroup` and channels
- Atomic counters for metrics: `sync/atomic.AddInt64`
- Context propagation: `context.Background()` for async processing (not request context)
- Representative snippet:

```go
func (d *Dispatcher) Submit(ctx context.Context, event *WebhookEvent) error {
	job := &WebhookJob{
		Event: event,
		Ctx:   ctx,
	}

	select {
	case d.workQueue <- job:
		log.Printf("[DISPATCHER] Job queued for event %d (queue: %d/%d)",
			event.ID, len(d.workQueue), cap(d.workQueue))
		return nil
	default:
		atomic.AddInt64(&d.droppedCount, 1)
		log.Printf("[DISPATCHER] Queue full, dropping event %d", event.ID)
		return ErrQueueFull
	}
}
```
