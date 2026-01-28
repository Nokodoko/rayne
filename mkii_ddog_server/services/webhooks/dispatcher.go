package webhooks

import (
	"context"
	"log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Dispatcher manages a pool of workers to process webhook events
// using bounded concurrency and graceful shutdown support.
type Dispatcher struct {
	workQueue    chan *WebhookJob
	workers      int
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
	orchestrator *ProcessorOrchestrator
	started      bool
	mu           sync.Mutex

	// Metrics
	processedCount int64
	errorCount     int64
	droppedCount   int64
	activeWorkers  int64
}

// WebhookJob represents a unit of work for the dispatcher
type WebhookJob struct {
	Event    *WebhookEvent
	Ctx      context.Context
	ResultCh chan<- JobResult // Optional channel for result delivery
}

// JobResult contains the outcome of processing a webhook job
type JobResult struct {
	EventID     int64
	Success     bool
	ProcessedBy []string
	Errors      []string
	Duration    time.Duration
}

// DispatcherConfig holds configuration for the dispatcher
type DispatcherConfig struct {
	// Workers is the number of worker goroutines
	// Default: GOMAXPROCS
	Workers int

	// QueueSize is the size of the work queue buffer
	// Default: GOMAXPROCS * 2
	QueueSize int
}

// DefaultDispatcherConfig returns sensible defaults based on system resources
func DefaultDispatcherConfig() DispatcherConfig {
	numCPU := runtime.GOMAXPROCS(0)
	return DispatcherConfig{
		Workers:   numCPU,
		QueueSize: numCPU * 2,
	}
}

// NewDispatcher creates a new dispatcher with the given orchestrator
func NewDispatcher(orchestrator *ProcessorOrchestrator, config DispatcherConfig) *Dispatcher {
	if config.Workers <= 0 {
		config.Workers = runtime.GOMAXPROCS(0)
	}
	if config.QueueSize <= 0 {
		config.QueueSize = config.Workers * 2
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Dispatcher{
		workQueue:    make(chan *WebhookJob, config.QueueSize),
		workers:      config.Workers,
		ctx:          ctx,
		cancel:       cancel,
		orchestrator: orchestrator,
	}
}

// Start launches the worker pool
func (d *Dispatcher) Start() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.started {
		log.Println("[DISPATCHER] Already started")
		return
	}

	log.Printf("[DISPATCHER] Starting with %d workers, queue size %d",
		d.workers, cap(d.workQueue))

	for i := 0; i < d.workers; i++ {
		d.wg.Add(1)
		go d.worker(i)
	}

	d.started = true
	log.Println("[DISPATCHER] Workers started")
}

// Submit adds a webhook event to the processing queue.
// Returns an error if the queue is full (backpressure).
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

// SubmitWithResult submits a job and returns a channel for the result
func (d *Dispatcher) SubmitWithResult(ctx context.Context, event *WebhookEvent) (<-chan JobResult, error) {
	resultCh := make(chan JobResult, 1)

	job := &WebhookJob{
		Event:    event,
		Ctx:      ctx,
		ResultCh: resultCh,
	}

	select {
	case d.workQueue <- job:
		log.Printf("[DISPATCHER] Job with result channel queued for event %d", event.ID)
		return resultCh, nil
	default:
		close(resultCh)
		atomic.AddInt64(&d.droppedCount, 1)
		return nil, ErrQueueFull
	}
}

// Shutdown gracefully stops all workers after draining the queue
func (d *Dispatcher) Shutdown() {
	d.mu.Lock()
	if !d.started {
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()

	log.Println("[DISPATCHER] Initiating shutdown...")

	// Signal workers to stop accepting new jobs
	d.cancel()

	// Close the work queue to signal workers to drain and exit
	close(d.workQueue)

	// Wait for all workers to complete
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		log.Println("[DISPATCHER] All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Println("[DISPATCHER] Shutdown timeout, some workers may not have completed")
	}

	d.mu.Lock()
	d.started = false
	d.mu.Unlock()
}

// Stats returns current dispatcher statistics
func (d *Dispatcher) Stats() DispatcherStats {
	return DispatcherStats{
		QueueSize:      len(d.workQueue),
		QueueCapacity:  cap(d.workQueue),
		ActiveWorkers:  int(atomic.LoadInt64(&d.activeWorkers)),
		TotalWorkers:   d.workers,
		ProcessedCount: atomic.LoadInt64(&d.processedCount),
		ErrorCount:     atomic.LoadInt64(&d.errorCount),
		DroppedCount:   atomic.LoadInt64(&d.droppedCount),
	}
}

// DispatcherStats holds dispatcher metrics
type DispatcherStats struct {
	QueueSize      int   `json:"queue_size"`
	QueueCapacity  int   `json:"queue_capacity"`
	ActiveWorkers  int   `json:"active_workers"`
	TotalWorkers   int   `json:"total_workers"`
	ProcessedCount int64 `json:"processed_count"`
	ErrorCount     int64 `json:"error_count"`
	DroppedCount   int64 `json:"dropped_count"`
}

// worker processes jobs from the work queue
func (d *Dispatcher) worker(id int) {
	defer d.wg.Done()

	log.Printf("[DISPATCHER] Worker %d started", id)

	for job := range d.workQueue {
		log.Printf("[DISPATCHER] Worker %d received job for event %d", id, job.Event.ID)
		atomic.AddInt64(&d.activeWorkers, 1)

		result := d.processJob(job)
		log.Printf("[DISPATCHER] Worker %d finished job for event %d, success=%v", id, job.Event.ID, result.Success)

		atomic.AddInt64(&d.activeWorkers, -1)
		atomic.AddInt64(&d.processedCount, 1)

		if !result.Success {
			atomic.AddInt64(&d.errorCount, 1)
		}

		// Send result if channel provided
		if job.ResultCh != nil {
			select {
			case job.ResultCh <- result:
			default:
				log.Printf("[DISPATCHER] Worker %d: result channel full for event %d", id, job.Event.ID)
			}
			close(job.ResultCh)
		}
	}

	log.Printf("[DISPATCHER] Worker %d stopped", id)
}

// processJob handles a single webhook job
func (d *Dispatcher) processJob(job *WebhookJob) JobResult {
	startTime := time.Now()

	result := JobResult{
		EventID:     job.Event.ID,
		Success:     true,
		ProcessedBy: make([]string, 0),
		Errors:      make([]string, 0),
	}

	// Use job context with dispatcher context as fallback
	ctx := job.Ctx
	if ctx == nil {
		ctx = d.ctx
	}

	// Check if context is already cancelled
	select {
	case <-ctx.Done():
		result.Success = false
		result.Errors = append(result.Errors, "context cancelled")
		result.Duration = time.Since(startTime)
		return result
	default:
	}

	// Process through orchestrator
	log.Printf("[DISPATCHER] Worker calling orchestrator.Process for event %d", job.Event.ID)
	processorResult := d.orchestrator.Process(ctx, job.Event)
	log.Printf("[DISPATCHER] Orchestrator returned for event %d: %d errors", job.Event.ID, len(processorResult.Errors))

	result.ProcessedBy = processorResult.ProcessedBy
	result.Errors = processorResult.Errors
	result.Success = len(processorResult.Errors) == 0
	result.Duration = time.Since(startTime)

	log.Printf("[DISPATCHER] Processed event %d: success=%v, processors=%v, duration=%v",
		job.Event.ID, result.Success, result.ProcessedBy, result.Duration)

	return result
}

// ErrQueueFull indicates the work queue is at capacity
var ErrQueueFull = &queueFullError{}

type queueFullError struct{}

func (e *queueFullError) Error() string {
	return "dispatcher queue is full"
}
