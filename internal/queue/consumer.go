package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// JobHandler processes run jobs
type JobHandler func(ctx context.Context, job *RunJob) (*RunResult, error)

// Consumer consumes run jobs from the queue
type Consumer struct {
	conn       *Connection
	handler    JobHandler
	producer   *Producer
	workers    int
	prefetch   int
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// ConsumerConfig holds consumer configuration
type ConsumerConfig struct {
	Workers  int // Number of concurrent workers
	Prefetch int // Prefetch count per worker
}

// DefaultConsumerConfig returns sensible defaults
func DefaultConsumerConfig() ConsumerConfig {
	return ConsumerConfig{
		Workers:  3,
		Prefetch: 1, // Process one at a time per worker for fairness
	}
}

// NewConsumer creates a new queue consumer
func NewConsumer(conn *Connection, handler JobHandler, cfg ConsumerConfig) *Consumer {
	if cfg.Workers <= 0 {
		cfg.Workers = 3
	}
	if cfg.Prefetch <= 0 {
		cfg.Prefetch = 1
	}

	return &Consumer{
		conn:     conn,
		handler:  handler,
		producer: NewProducer(conn),
		workers:  cfg.Workers,
		prefetch: cfg.Prefetch,
	}
}

// Start begins consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	ctx, c.cancelFunc = context.WithCancel(ctx)

	ch := c.conn.Channel()

	// Set QoS (prefetch)
	if err := ch.Qos(c.prefetch, 0, false); err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}

	// Start consuming
	msgs, err := ch.Consume(
		RunQueueName,
		"",    // consumer tag (auto-generated)
		false, // auto-ack (manual ack for reliability)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consuming: %w", err)
	}

	slog.Info("starting run queue consumer", "workers", c.workers, "prefetch", c.prefetch)

	// Start worker goroutines
	for i := 0; i < c.workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i, msgs)
	}

	return nil
}

// worker processes messages from the queue
func (c *Consumer) worker(ctx context.Context, id int, msgs <-chan amqp.Delivery) {
	defer c.wg.Done()

	slog.Info("worker started", "worker_id", id)

	for {
		select {
		case <-ctx.Done():
			slog.Info("worker stopping", "worker_id", id)
			return

		case msg, ok := <-msgs:
			if !ok {
				slog.Info("message channel closed", "worker_id", id)
				return
			}

			c.processMessage(ctx, id, msg)
		}
	}
}

// processMessage handles a single message
func (c *Consumer) processMessage(ctx context.Context, workerID int, msg amqp.Delivery) {
	start := time.Now()

	// Parse job
	var job RunJob
	if err := json.Unmarshal(msg.Body, &job); err != nil {
		slog.Error("failed to unmarshal job",
			"worker_id", workerID,
			"error", err,
		)
		// Reject without requeue for malformed messages
		_ = msg.Reject(false)
		return
	}

	slog.Info("processing run job",
		"worker_id", workerID,
		"job_id", job.ID,
		"user_id", job.UserID,
		"exercise_id", job.ExerciseID,
	)

	// Create context with timeout from recipe
	timeout := time.Duration(job.Recipe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	jobCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Process job
	result, err := c.handler(jobCtx, &job)
	duration := time.Since(start)

	if err != nil {
		slog.Error("job processing failed",
			"worker_id", workerID,
			"job_id", job.ID,
			"error", err,
			"duration", duration,
		)

		// Create error result
		result = &RunResult{
			JobID:       job.ID,
			Status:      "failed",
			Error:       err.Error(),
			Duration:    duration,
			CompletedAt: time.Now(),
		}

		// Check if it's a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.Error = "execution timed out"
		}
	} else {
		result.JobID = job.ID
		result.Duration = duration
		result.CompletedAt = time.Now()
		if result.Status == "" {
			result.Status = "completed"
		}

		slog.Info("job completed",
			"worker_id", workerID,
			"job_id", job.ID,
			"status", result.Status,
			"duration", duration,
		)
	}

	// Publish result
	if err := c.producer.PublishResult(ctx, result); err != nil {
		slog.Error("failed to publish result",
			"worker_id", workerID,
			"job_id", job.ID,
			"error", err,
		)
	}

	// Acknowledge message
	if err := msg.Ack(false); err != nil {
		slog.Error("failed to ack message",
			"worker_id", workerID,
			"job_id", job.ID,
			"error", err,
		)
	}
}

// Stop gracefully stops the consumer
func (c *Consumer) Stop() {
	if c.cancelFunc != nil {
		c.cancelFunc()
	}
	c.wg.Wait()
	slog.Info("consumer stopped")
}

// ResultConsumer consumes run results (for API server to stream back to clients)
type ResultConsumer struct {
	conn       *Connection
	handlers   map[string]ResultHandler
	handlersMu sync.RWMutex
	cancelFunc context.CancelFunc
	wg         sync.WaitGroup
}

// ResultHandler handles a run result for a specific job
type ResultHandler func(result *RunResult)

// NewResultConsumer creates a result consumer
func NewResultConsumer(conn *Connection) *ResultConsumer {
	return &ResultConsumer{
		conn:     conn,
		handlers: make(map[string]ResultHandler),
	}
}

// Subscribe registers a handler for results of a specific job
func (rc *ResultConsumer) Subscribe(jobID string, handler ResultHandler) {
	rc.handlersMu.Lock()
	defer rc.handlersMu.Unlock()
	rc.handlers[jobID] = handler
}

// Unsubscribe removes a handler
func (rc *ResultConsumer) Unsubscribe(jobID string) {
	rc.handlersMu.Lock()
	defer rc.handlersMu.Unlock()
	delete(rc.handlers, jobID)
}

// Start begins consuming results
func (rc *ResultConsumer) Start(ctx context.Context) error {
	ctx, rc.cancelFunc = context.WithCancel(ctx)

	ch := rc.conn.Channel()

	msgs, err := ch.Consume(
		ResultQueueName,
		"",    // consumer tag
		true,  // auto-ack (results are fire-and-forget)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return fmt.Errorf("failed to start result consumer: %w", err)
	}

	rc.wg.Add(1)
	go rc.consume(ctx, msgs)

	return nil
}

func (rc *ResultConsumer) consume(ctx context.Context, msgs <-chan amqp.Delivery) {
	defer rc.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}

			var result RunResult
			if err := json.Unmarshal(msg.Body, &result); err != nil {
				slog.Error("failed to unmarshal result", "error", err)
				continue
			}

			// Find handler
			rc.handlersMu.RLock()
			handler, ok := rc.handlers[result.JobID.String()]
			rc.handlersMu.RUnlock()

			if ok {
				handler(&result)
			}
		}
	}
}

// Stop stops the result consumer
func (rc *ResultConsumer) Stop() {
	if rc.cancelFunc != nil {
		rc.cancelFunc()
	}
	rc.wg.Wait()
}
