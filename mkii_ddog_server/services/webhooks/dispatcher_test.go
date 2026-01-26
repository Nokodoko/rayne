package webhooks

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestDispatcher_Stats(t *testing.T) {
	storage := &Storage{}
	procOrch := NewProcessorOrchestrator(storage, nil)
	config := DispatcherConfig{Workers: 3, QueueSize: 15}
	d := NewDispatcher(procOrch, config)

	stats := d.Stats()

	if stats.TotalWorkers != 3 {
		t.Errorf("Expected 3 workers, got %d", stats.TotalWorkers)
	}
	if stats.QueueCapacity != 15 {
		t.Errorf("Expected capacity 15, got %d", stats.QueueCapacity)
	}
	if stats.ProcessedCount != 0 {
		t.Errorf("Expected 0 processed, got %d", stats.ProcessedCount)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("Expected 0 errors, got %d", stats.ErrorCount)
	}
	if stats.DroppedCount != 0 {
		t.Errorf("Expected 0 dropped, got %d", stats.DroppedCount)
	}
}

func TestDispatcher_DefaultConfig(t *testing.T) {
	config := DefaultDispatcherConfig()

	if config.Workers <= 0 {
		t.Error("Default workers should be > 0")
	}
	if config.QueueSize <= 0 {
		t.Error("Default queue size should be > 0")
	}
	if config.QueueSize < config.Workers {
		t.Error("Queue size should be >= workers")
	}
}

func TestDispatcher_StartAndShutdown(t *testing.T) {
	storage := &Storage{}
	procOrch := NewProcessorOrchestrator(storage, nil)
	config := DispatcherConfig{Workers: 2, QueueSize: 10}
	d := NewDispatcher(procOrch, config)

	d.Start()

	stats := d.Stats()
	if stats.TotalWorkers != 2 {
		t.Errorf("Expected 2 workers, got %d", stats.TotalWorkers)
	}
	if stats.QueueCapacity != 10 {
		t.Errorf("Expected queue capacity 10, got %d", stats.QueueCapacity)
	}

	d.Shutdown()
}

func TestDispatcher_QueueFull(t *testing.T) {
	storage := &Storage{}
	procOrch := NewProcessorOrchestrator(storage, nil)
	config := DispatcherConfig{Workers: 1, QueueSize: 1}
	d := NewDispatcher(procOrch, config)

	// Don't start workers - queue will fill up
	d.mu.Lock()
	d.started = true
	d.mu.Unlock()

	event1 := &WebhookEvent{ID: 1}
	err := d.Submit(context.Background(), event1)
	if err != nil {
		t.Errorf("First submit should succeed: %v", err)
	}

	event2 := &WebhookEvent{ID: 2}
	err = d.Submit(context.Background(), event2)
	if err != ErrQueueFull {
		t.Errorf("Expected ErrQueueFull, got: %v", err)
	}

	stats := d.Stats()
	if stats.DroppedCount != 1 {
		t.Errorf("Expected 1 dropped, got %d", stats.DroppedCount)
	}
}

func TestDispatcher_ConcurrentSubmit(t *testing.T) {
	// This test only verifies submission, not processing (which requires DB)
	storage := &Storage{}
	procOrch := NewProcessorOrchestrator(storage, nil)
	config := DispatcherConfig{Workers: 1, QueueSize: 100}
	d := NewDispatcher(procOrch, config)

	// Only mark as started to allow submits, but don't start workers
	// (workers would panic on nil DB)
	d.mu.Lock()
	d.started = true
	d.mu.Unlock()

	var wg sync.WaitGroup
	submitCount := 50
	var successCount int64

	for i := 0; i < submitCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			event := &WebhookEvent{ID: int64(id)}
			if err := d.Submit(context.Background(), event); err == nil {
				atomic.AddInt64(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != int64(submitCount) {
		t.Errorf("Expected %d successful submits, got %d", submitCount, successCount)
	}

	stats := d.Stats()
	if stats.QueueSize != submitCount {
		t.Errorf("Expected queue size %d, got %d", submitCount, stats.QueueSize)
	}
}

func TestDispatcher_GracefulShutdown(t *testing.T) {
	// Test that shutdown completes even with empty queue
	storage := &Storage{}
	procOrch := NewProcessorOrchestrator(storage, nil)
	config := DispatcherConfig{Workers: 2, QueueSize: 20}
	d := NewDispatcher(procOrch, config)
	d.Start()

	// Don't submit any events - just test clean shutdown
	done := make(chan struct{})
	go func() {
		d.Shutdown()
		close(done)
	}()

	select {
	case <-done:
		// Good
	case <-time.After(5 * time.Second):
		t.Error("Shutdown timeout")
	}
}

func TestNewDispatcher(t *testing.T) {
	storage := &Storage{}
	procOrch := NewProcessorOrchestrator(storage, nil)

	tests := []struct {
		name     string
		config   DispatcherConfig
		wantWork int
		wantQ    int
	}{
		{"default config", DispatcherConfig{}, DefaultDispatcherConfig().Workers, DefaultDispatcherConfig().QueueSize},
		{"custom config", DispatcherConfig{Workers: 5, QueueSize: 20}, 5, 20},
		{"zero workers defaults", DispatcherConfig{Workers: 0, QueueSize: 10}, DefaultDispatcherConfig().Workers, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDispatcher(procOrch, tt.config)
			stats := d.Stats()

			if stats.TotalWorkers != tt.wantWork {
				t.Errorf("Workers: got %d, want %d", stats.TotalWorkers, tt.wantWork)
			}
			if stats.QueueCapacity != tt.wantQ {
				t.Errorf("QueueCapacity: got %d, want %d", stats.QueueCapacity, tt.wantQ)
			}
		})
	}
}

func TestErrQueueFull(t *testing.T) {
	err := ErrQueueFull
	if err.Error() != "dispatcher queue is full" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}
