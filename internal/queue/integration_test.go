//go:build integration

package queue_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/queue"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/rabbitmq"
)

// setupRabbitMQ creates a RabbitMQ container for testing
func setupRabbitMQ(t *testing.T) (string, func()) {
	ctx := context.Background()

	container, err := rabbitmq.Run(ctx, "rabbitmq:3.12-management")
	if err != nil {
		t.Fatalf("failed to start RabbitMQ container: %v", err)
	}

	amqpURL, err := container.AmqpURL(ctx)
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("failed to get AMQP URL: %v", err)
	}

	cleanup := func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return amqpURL, cleanup
}

func TestIntegration_Connection_ConnectAndClose(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}

	if !conn.IsConnected() {
		t.Error("expected connection to be active")
	}

	if err := conn.Close(); err != nil {
		t.Errorf("failed to close connection: %v", err)
	}
}

func TestIntegration_Connection_InvalidURL(t *testing.T) {
	_, err := queue.NewConnection("amqp://invalid:5672")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestIntegration_Producer_PublishRunJob(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	defer conn.Close()

	producer := queue.NewProducer(conn)

	job := &queue.RunJob{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ExerciseID:  "producer-test",
		Code: map[string]string{
			"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"hello\") }\n",
		},
		Recipe: queue.RunRecipe{
			Format: true,
			Build:  true,
			Test:   true,
		},
		CreatedAt: time.Now(),
	}

	ctx := context.Background()

	if err := producer.PublishRunJob(ctx, job); err != nil {
		t.Fatalf("failed to publish run job: %v", err)
	}

	// Verify by checking queue has a message
	ch := conn.Channel()
	q, err := ch.QueueInspect(queue.RunQueueName)
	if err != nil {
		t.Fatalf("failed to inspect queue: %v", err)
	}

	if q.Messages != 1 {
		t.Errorf("expected 1 message in queue, got %d", q.Messages)
	}
}

func TestIntegration_Producer_PublishResult(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	defer conn.Close()

	producer := queue.NewProducer(conn)

	result := &queue.RunResult{
		JobID:    uuid.New(),
		Status:   "completed",
		Duration: 1500 * time.Millisecond,
		FormatResult: &queue.FormatOutput{
			OK:   true,
			Diff: "",
		},
		BuildResult: &queue.BuildOutput{
			OK: true,
		},
		TestResult: &queue.TestOutput{
			OK:       true,
			Output:   "PASS",
			Duration: 1 * time.Second,
		},
		CompletedAt: time.Now(),
	}

	ctx := context.Background()

	if err := producer.PublishResult(ctx, result); err != nil {
		t.Fatalf("failed to publish result: %v", err)
	}

	// Verify by checking the queue has a message
	ch := conn.Channel()
	q, err := ch.QueueInspect(queue.ResultQueueName)
	if err != nil {
		t.Fatalf("failed to inspect queue: %v", err)
	}

	if q.Messages != 1 {
		t.Errorf("expected 1 message in queue, got %d", q.Messages)
	}
}

func TestIntegration_Consumer_ProcessJobs(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Track received jobs
	var receivedJobs []*queue.RunJob
	var mu sync.Mutex
	receivedCh := make(chan struct{}, 5)

	// Create a handler that captures jobs
	handler := func(ctx context.Context, job *queue.RunJob) (*queue.RunResult, error) {
		mu.Lock()
		receivedJobs = append(receivedJobs, job)
		mu.Unlock()

		receivedCh <- struct{}{}

		return &queue.RunResult{
			JobID:  job.ID,
			Status: "completed",
			FormatResult: &queue.FormatOutput{
				OK: true,
			},
		}, nil
	}

	// Create consumer
	consumer := queue.NewConsumer(conn, handler, queue.ConsumerConfig{
		Workers:  2,
		Prefetch: 1,
	})

	// Start consumer
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("failed to start consumer: %v", err)
	}
	defer consumer.Stop()

	// Publish jobs
	producer := queue.NewProducer(conn)
	jobCount := 3
	sentJobs := make([]*queue.RunJob, jobCount)

	for i := 0; i < jobCount; i++ {
		sentJobs[i] = &queue.RunJob{
			ID:          uuid.New(),
			UserID:      uuid.New(),
			WorkspaceID: uuid.New(),
			ExerciseID:  "consumer-test",
			Code:        map[string]string{"main.go": "package main"},
			Recipe:      queue.RunRecipe{Format: true},
			CreatedAt:   time.Now(),
		}

		if err := producer.PublishRunJob(ctx, sentJobs[i]); err != nil {
			t.Fatalf("failed to publish job %d: %v", i, err)
		}
	}

	// Wait for all jobs to be processed
	for i := 0; i < jobCount; i++ {
		select {
		case <-receivedCh:
			// Job received
		case <-ctx.Done():
			t.Fatalf("timeout waiting for job %d", i)
		}
	}

	// Verify all jobs were received
	mu.Lock()
	if len(receivedJobs) != jobCount {
		t.Errorf("expected %d jobs, got %d", jobCount, len(receivedJobs))
	}
	mu.Unlock()
}

func TestIntegration_Consumer_HandlerError(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processedCh := make(chan struct{}, 1)

	// Create a handler that returns an error
	handler := func(ctx context.Context, job *queue.RunJob) (*queue.RunResult, error) {
		processedCh <- struct{}{}
		return nil, context.DeadlineExceeded
	}

	// Create consumer
	consumer := queue.NewConsumer(conn, handler, queue.DefaultConsumerConfig())

	// Start consumer
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("failed to start consumer: %v", err)
	}
	defer consumer.Stop()

	// Publish a job
	producer := queue.NewProducer(conn)
	job := &queue.RunJob{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ExerciseID:  "error-test",
		Code:        map[string]string{"main.go": "package main"},
		Recipe:      queue.RunRecipe{Format: true},
		CreatedAt:   time.Now(),
	}

	if err := producer.PublishRunJob(ctx, job); err != nil {
		t.Fatalf("failed to publish job: %v", err)
	}

	// Wait for job to be processed
	select {
	case <-processedCh:
		// Job processed (with error)
	case <-ctx.Done():
		t.Fatal("timeout waiting for job processing")
	}

	// Give time for result to be published
	time.Sleep(100 * time.Millisecond)

	// Verify result was published with error
	ch := conn.Channel()
	q, err := ch.QueueInspect(queue.ResultQueueName)
	if err != nil {
		t.Fatalf("failed to inspect result queue: %v", err)
	}

	if q.Messages != 1 {
		t.Errorf("expected 1 result in queue, got %d", q.Messages)
	}
}

func TestIntegration_ResultConsumer_Subscribe(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create result consumer
	resultConsumer := queue.NewResultConsumer(conn)
	if err := resultConsumer.Start(ctx); err != nil {
		t.Fatalf("failed to start result consumer: %v", err)
	}
	defer resultConsumer.Stop()

	// Subscribe to a specific job ID
	jobID := uuid.New()
	receivedCh := make(chan *queue.RunResult, 1)

	resultConsumer.Subscribe(jobID.String(), func(result *queue.RunResult) {
		receivedCh <- result
	})

	// Publish a result for that job
	producer := queue.NewProducer(conn)
	result := &queue.RunResult{
		JobID:  jobID,
		Status: "completed",
		FormatResult: &queue.FormatOutput{
			OK: true,
		},
		Duration:    500 * time.Millisecond,
		CompletedAt: time.Now(),
	}

	if err := producer.PublishResult(ctx, result); err != nil {
		t.Fatalf("failed to publish result: %v", err)
	}

	// Wait for result
	select {
	case received := <-receivedCh:
		if received.JobID != jobID {
			t.Errorf("expected job ID %s, got %s", jobID, received.JobID)
		}
		if received.Status != "completed" {
			t.Errorf("expected status 'completed', got '%s'", received.Status)
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for result")
	}

	// Clean up subscription
	resultConsumer.Unsubscribe(jobID.String())
}

func TestIntegration_Connection_PublishJSON(t *testing.T) {
	amqpURL, cleanup := setupRabbitMQ(t)
	defer cleanup()

	conn, err := queue.NewConnection(amqpURL)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}
	defer conn.Close()

	ctx := context.Background()

	job := queue.RunJob{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ExerciseID:  "test",
		Code:        map[string]string{"main.go": "package main"},
		Recipe:      queue.RunRecipe{Format: true},
		CreatedAt:   time.Now(),
	}

	// Direct publish using PublishJSON
	if err := conn.PublishJSON(ctx, queue.RunQueueName, job); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// Verify
	ch := conn.Channel()
	q, err := ch.QueueInspect(queue.RunQueueName)
	if err != nil {
		t.Fatalf("failed to inspect queue: %v", err)
	}

	if q.Messages != 1 {
		t.Errorf("expected 1 message, got %d", q.Messages)
	}
}

func TestIntegration_CreateRunJob_Helper(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	exerciseID := "test-exercise"
	code := map[string]string{"main.go": "package main"}
	recipe := queue.RunRecipe{Format: true, Build: true}

	job := queue.CreateRunJob(userID, workspaceID, exerciseID, code, recipe)

	if job.UserID != userID {
		t.Errorf("expected user ID %s, got %s", userID, job.UserID)
	}
	if job.WorkspaceID != workspaceID {
		t.Errorf("expected workspace ID %s, got %s", workspaceID, job.WorkspaceID)
	}
	if job.ExerciseID != exerciseID {
		t.Errorf("expected exercise ID %s, got %s", exerciseID, job.ExerciseID)
	}
	if job.ID == uuid.Nil {
		t.Error("expected job ID to be generated")
	}
	if job.CreatedAt.IsZero() {
		t.Error("expected created at to be set")
	}
}
