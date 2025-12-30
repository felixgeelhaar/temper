package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Queue names
const (
	RunQueueName    = "temper.runs"
	ResultQueueName = "temper.results"
)

// RunJob represents a code execution job to be processed
type RunJob struct {
	ID          uuid.UUID         `json:"id"`
	UserID      uuid.UUID         `json:"user_id"`
	WorkspaceID uuid.UUID         `json:"workspace_id"`
	ExerciseID  string            `json:"exercise_id"`
	Code        map[string]string `json:"code"`
	Recipe      RunRecipe         `json:"recipe"`
	CreatedAt   time.Time         `json:"created_at"`
}

// RunRecipe defines what checks to run
type RunRecipe struct {
	Format  bool `json:"format"`
	Build   bool `json:"build"`
	Test    bool `json:"test"`
	Timeout int  `json:"timeout"` // seconds
}

// RunResult represents the result of a code execution
type RunResult struct {
	JobID        uuid.UUID     `json:"job_id"`
	Status       string        `json:"status"` // completed, failed, timeout
	FormatResult *FormatOutput `json:"format_result,omitempty"`
	BuildResult  *BuildOutput  `json:"build_result,omitempty"`
	TestResult   *TestOutput   `json:"test_result,omitempty"`
	Error        string        `json:"error,omitempty"`
	Duration     time.Duration `json:"duration"`
	CompletedAt  time.Time     `json:"completed_at"`
}

// FormatOutput contains gofmt results
type FormatOutput struct {
	OK   bool   `json:"ok"`
	Diff string `json:"diff,omitempty"`
}

// BuildOutput contains go build results
type BuildOutput struct {
	OK     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
}

// TestOutput contains go test results
type TestOutput struct {
	OK       bool          `json:"ok"`
	Output   string        `json:"output"`
	Duration time.Duration `json:"duration"`
}

// Connection manages the RabbitMQ connection with automatic reconnection
type Connection struct {
	url        string
	conn       *amqp.Connection
	channel    *amqp.Channel
	mu         sync.RWMutex
	closed     bool
	reconnects int
}

// NewConnection creates a new RabbitMQ connection
func NewConnection(url string) (*Connection, error) {
	c := &Connection{
		url: url,
	}

	if err := c.connect(); err != nil {
		return nil, err
	}

	return c, nil
}

// connect establishes connection and channel
func (c *Connection) connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	c.conn, err = amqp.Dial(c.url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	c.channel, err = c.conn.Channel()
	if err != nil {
		c.conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}

	// Declare queues
	if err := c.declareQueues(); err != nil {
		c.channel.Close()
		c.conn.Close()
		return err
	}

	// Set up reconnection on close
	go c.handleReconnect()

	slog.Info("connected to RabbitMQ", "url", sanitizeURL(c.url))
	return nil
}

// declareQueues creates the necessary queues
func (c *Connection) declareQueues() error {
	// Run jobs queue - durable for reliability
	_, err := c.channel.QueueDeclare(
		RunQueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-message-ttl": int32(300000), // 5 minute TTL
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare run queue: %w", err)
	}

	// Results queue
	_, err = c.channel.QueueDeclare(
		ResultQueueName,
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-message-ttl": int32(60000), // 1 minute TTL for results
		},
	)
	if err != nil {
		return fmt.Errorf("failed to declare results queue: %w", err)
	}

	return nil
}

// handleReconnect listens for connection close and attempts to reconnect
func (c *Connection) handleReconnect() {
	notifyClose := c.conn.NotifyClose(make(chan *amqp.Error, 1))

	for {
		select {
		case err := <-notifyClose:
			if err == nil {
				return // Normal close
			}

			c.mu.Lock()
			if c.closed {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()

			slog.Warn("RabbitMQ connection closed, attempting to reconnect",
				"error", err,
				"reconnects", c.reconnects,
			)

			// Exponential backoff
			for i := 0; i < 10; i++ {
				c.reconnects++
				backoff := time.Duration(1<<i) * time.Second
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				time.Sleep(backoff)

				if err := c.connect(); err != nil {
					slog.Error("reconnection failed", "error", err, "attempt", i+1)
					continue
				}

				slog.Info("reconnected to RabbitMQ", "attempts", i+1)
				return
			}

			slog.Error("failed to reconnect to RabbitMQ after 10 attempts")
			return
		}
	}
}

// Channel returns the current channel (thread-safe)
func (c *Connection) Channel() *amqp.Channel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.channel
}

// Close closes the connection
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.closed = true

	if c.channel != nil {
		c.channel.Close()
	}
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected checks if the connection is active
func (c *Connection) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.conn != nil && !c.conn.IsClosed()
}

// PublishJSON publishes a JSON message to a queue
func (c *Connection) PublishJSON(ctx context.Context, queue string, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	c.mu.RLock()
	ch := c.channel
	c.mu.RUnlock()

	return ch.PublishWithContext(
		ctx,
		"",    // exchange
		queue, // routing key
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// sanitizeURL removes password from URL for logging
func sanitizeURL(url string) string {
	// Simple sanitization - just show host
	if len(url) > 20 {
		return url[:20] + "..."
	}
	return url
}
