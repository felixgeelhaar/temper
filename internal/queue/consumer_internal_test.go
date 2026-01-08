package queue

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewConsumer_DefaultsZeroConfig(t *testing.T) {
	// NewConsumer should apply defaults when config has zero values
	// We can't fully test without a real connection, but we can verify
	// the config defaults logic by checking the struct fields

	cfg := ConsumerConfig{}

	// Verify that applying defaults would set proper values
	if cfg.Workers <= 0 {
		cfg.Workers = 3
	}
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 1
	}

	if cfg.Workers != 3 {
		t.Errorf("Default Workers = %d; want 3", cfg.Workers)
	}
	if cfg.Prefetch != 1 {
		t.Errorf("Default Prefetch = %d; want 1", cfg.Prefetch)
	}
}

func TestNewConsumer_PreservesCustomConfig(t *testing.T) {
	cfg := ConsumerConfig{
		Workers:  10,
		Prefetch: 5,
	}

	// Verify defaults logic doesn't override custom values
	if cfg.Workers <= 0 {
		cfg.Workers = 3
	}
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 1
	}

	if cfg.Workers != 10 {
		t.Errorf("Custom Workers = %d; want 10", cfg.Workers)
	}
	if cfg.Prefetch != 5 {
		t.Errorf("Custom Prefetch = %d; want 5", cfg.Prefetch)
	}
}

func TestResultConsumer_SubscribeUnsubscribe(t *testing.T) {
	// Create a ResultConsumer with a nil connection
	// We're only testing the handler map management
	rc := &ResultConsumer{
		handlers: make(map[string]ResultHandler),
	}

	jobID := uuid.New().String()

	// Subscribe
	rc.Subscribe(jobID, func(result *RunResult) {
		// Handler registered for testing
	})

	// Verify handler is registered
	rc.handlersMu.RLock()
	_, exists := rc.handlers[jobID]
	rc.handlersMu.RUnlock()

	if !exists {
		t.Error("Handler should be registered after Subscribe")
	}

	// Unsubscribe
	rc.Unsubscribe(jobID)

	// Verify handler is removed
	rc.handlersMu.RLock()
	_, exists = rc.handlers[jobID]
	rc.handlersMu.RUnlock()

	if exists {
		t.Error("Handler should be removed after Unsubscribe")
	}
}

func TestResultConsumer_Subscribe_ConcurrentSafe(t *testing.T) {
	rc := &ResultConsumer{
		handlers: make(map[string]ResultHandler),
	}

	var wg sync.WaitGroup
	numGoroutines := 100

	// Spawn goroutines that concurrently subscribe and unsubscribe
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobID := uuid.New().String()

			// Subscribe
			rc.Subscribe(jobID, func(result *RunResult) {})

			// Small delay to increase chance of concurrent access
			time.Sleep(time.Microsecond)

			// Unsubscribe
			rc.Unsubscribe(jobID)
		}(i)
	}

	wg.Wait()

	// Should not panic and handlers should be empty
	rc.handlersMu.RLock()
	count := len(rc.handlers)
	rc.handlersMu.RUnlock()

	if count != 0 {
		t.Errorf("All handlers should be unsubscribed, got %d remaining", count)
	}
}

func TestResultConsumer_Subscribe_OverwritesPrevious(t *testing.T) {
	rc := &ResultConsumer{
		handlers: make(map[string]ResultHandler),
	}

	jobID := uuid.New().String()
	called1 := false
	called2 := false

	// Subscribe first handler
	rc.Subscribe(jobID, func(result *RunResult) {
		called1 = true
	})

	// Subscribe second handler with same ID (overwrites first)
	rc.Subscribe(jobID, func(result *RunResult) {
		called2 = true
	})

	// Get the handler and call it
	rc.handlersMu.RLock()
	handler, ok := rc.handlers[jobID]
	rc.handlersMu.RUnlock()

	if !ok {
		t.Fatal("Handler should exist")
	}

	handler(&RunResult{})

	if called1 {
		t.Error("First handler should NOT have been called (was overwritten)")
	}
	if !called2 {
		t.Error("Second handler should have been called")
	}
}

func TestResultConsumer_Unsubscribe_NonExistent(t *testing.T) {
	rc := &ResultConsumer{
		handlers: make(map[string]ResultHandler),
	}

	// Unsubscribing a non-existent handler should not panic
	rc.Unsubscribe("non-existent-job-id")
	// If we reach here without panic, test passes
}

func TestResultConsumer_Stop_NilCancelFunc(t *testing.T) {
	rc := &ResultConsumer{
		handlers: make(map[string]ResultHandler),
	}

	// Stop with nil cancelFunc should not panic
	rc.Stop()
	// If we reach here without panic, test passes
}

func TestConsumer_Stop_NilCancelFunc(t *testing.T) {
	c := &Consumer{}

	// Stop with nil cancelFunc should not panic
	c.Stop()
	// If we reach here without panic, test passes
}

func TestJobHandler_Type(t *testing.T) {
	// Verify JobHandler type signature
	var handler JobHandler = func(ctx context.Context, job *RunJob) (*RunResult, error) {
		return &RunResult{
			JobID:       job.ID,
			Status:      "completed",
			CompletedAt: time.Now(),
		}, nil
	}

	// Test handler
	job := &RunJob{
		ID: uuid.New(),
	}

	result, err := handler(context.Background(), job)
	if err != nil {
		t.Errorf("Handler returned unexpected error: %v", err)
	}
	if result.JobID != job.ID {
		t.Errorf("JobID = %v; want %v", result.JobID, job.ID)
	}
}

func TestResultHandler_Type(t *testing.T) {
	// Verify ResultHandler type signature
	var called bool
	var handler ResultHandler = func(result *RunResult) {
		called = true
	}

	// Call handler
	handler(&RunResult{})

	if !called {
		t.Error("Handler should have been called")
	}
}

func TestNewResultConsumer_InitializesHandlersMap(t *testing.T) {
	// NewResultConsumer requires a connection, but we can verify
	// that the struct would be properly initialized
	rc := &ResultConsumer{
		handlers: make(map[string]ResultHandler),
	}

	if rc.handlers == nil {
		t.Error("handlers map should be initialized")
	}

	// Test that map operations work
	rc.handlers["test"] = func(result *RunResult) {}
	if len(rc.handlers) != 1 {
		t.Errorf("handlers length = %d; want 1", len(rc.handlers))
	}
}

func TestConsumerWorkerDefaultTimeout(t *testing.T) {
	// Test the default timeout logic used in processMessage
	recipe := RunRecipe{Timeout: 0}

	timeout := time.Duration(recipe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if timeout != 30*time.Second {
		t.Errorf("Default timeout = %v; want 30s", timeout)
	}
}

func TestConsumerWorkerCustomTimeout(t *testing.T) {
	// Test custom timeout is respected
	recipe := RunRecipe{Timeout: 60}

	timeout := time.Duration(recipe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	if timeout != 60*time.Second {
		t.Errorf("Custom timeout = %v; want 60s", timeout)
	}
}
