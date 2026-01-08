package queue_test

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/queue"
)

// skipIfNoRabbitMQ skips tests if RabbitMQ is not available
func skipIfNoRabbitMQ(t *testing.T) {
	t.Helper()

	// Check if docker-compose rabbitmq is running
	cmd := exec.Command("docker", "exec", "temper-rabbitmq-1", "rabbitmq-diagnostics", "ping")
	if err := cmd.Run(); err != nil {
		t.Skip("RabbitMQ not available, skipping queue tests")
	}
}

func TestRunJob_Serialization(t *testing.T) {
	job := queue.RunJob{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ExerciseID:  "go-v1/basics/hello-world",
		Code: map[string]string{
			"main.go": `package main

func main() {
	println("hello")
}
`,
		},
		Recipe: queue.RunRecipe{
			Format:  true,
			Build:   true,
			Test:    true,
			Timeout: 30,
		},
		CreatedAt: time.Now(),
	}

	// Verify all fields are set
	if job.ID == uuid.Nil {
		t.Error("Job ID should not be nil")
	}
	if job.ExerciseID == "" {
		t.Error("ExerciseID should not be empty")
	}
	if len(job.Code) == 0 {
		t.Error("Code should not be empty")
	}
}

func TestRunResult_StatusTypes(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{"completed", "completed"},
		{"failed", "failed"},
		{"timeout", "timeout"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := queue.RunResult{
				JobID:       uuid.New(),
				Status:      tc.status,
				CompletedAt: time.Now(),
			}

			if result.Status != tc.status {
				t.Errorf("Status = %q; want %q", result.Status, tc.status)
			}
		})
	}
}

func TestCreateRunJob(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	exerciseID := "go-v1/basics/hello-world"
	code := map[string]string{"main.go": "package main"}
	recipe := queue.RunRecipe{Format: true, Build: true, Test: true}

	job := queue.CreateRunJob(userID, workspaceID, exerciseID, code, recipe)

	if job.ID == uuid.Nil {
		t.Error("Job ID should be generated")
	}
	if job.UserID != userID {
		t.Errorf("UserID = %v; want %v", job.UserID, userID)
	}
	if job.WorkspaceID != workspaceID {
		t.Errorf("WorkspaceID = %v; want %v", job.WorkspaceID, workspaceID)
	}
	if job.ExerciseID != exerciseID {
		t.Errorf("ExerciseID = %q; want %q", job.ExerciseID, exerciseID)
	}
	if job.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestDefaultConsumerConfig(t *testing.T) {
	cfg := queue.DefaultConsumerConfig()

	if cfg.Workers <= 0 {
		t.Error("Workers should be positive")
	}
	if cfg.Prefetch <= 0 {
		t.Error("Prefetch should be positive")
	}
}

func TestDefaultConsumerConfig_SpecificValues(t *testing.T) {
	cfg := queue.DefaultConsumerConfig()

	// Verify specific default values per the implementation
	if cfg.Workers != 3 {
		t.Errorf("Default Workers = %d; want 3", cfg.Workers)
	}
	if cfg.Prefetch != 1 {
		t.Errorf("Default Prefetch = %d; want 1", cfg.Prefetch)
	}
}

func TestCreateRunJob_GeneratesUniqueIDs(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	code := map[string]string{"main.go": "package main"}
	recipe := queue.RunRecipe{}

	// Create multiple jobs and verify unique IDs
	ids := make(map[uuid.UUID]bool)
	for i := 0; i < 10; i++ {
		job := queue.CreateRunJob(userID, workspaceID, "exercise", code, recipe)
		if ids[job.ID] {
			t.Errorf("Duplicate job ID generated: %v", job.ID)
		}
		ids[job.ID] = true
	}
}

func TestCreateRunJob_SetsTimestamp(t *testing.T) {
	before := time.Now()
	job := queue.CreateRunJob(uuid.New(), uuid.New(), "test", nil, queue.RunRecipe{})
	after := time.Now()

	if job.CreatedAt.Before(before) || job.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v; should be between %v and %v", job.CreatedAt, before, after)
	}
}

func TestCreateRunJob_CopiesAllFields(t *testing.T) {
	userID := uuid.New()
	workspaceID := uuid.New()
	exerciseID := "go-v1/test/exercise"
	code := map[string]string{
		"main.go":      "package main",
		"main_test.go": "package main",
	}
	recipe := queue.RunRecipe{
		Format:  true,
		Build:   true,
		Test:    true,
		Timeout: 60,
	}

	job := queue.CreateRunJob(userID, workspaceID, exerciseID, code, recipe)

	if job.UserID != userID {
		t.Errorf("UserID = %v; want %v", job.UserID, userID)
	}
	if job.WorkspaceID != workspaceID {
		t.Errorf("WorkspaceID = %v; want %v", job.WorkspaceID, workspaceID)
	}
	if job.ExerciseID != exerciseID {
		t.Errorf("ExerciseID = %q; want %q", job.ExerciseID, exerciseID)
	}
	if len(job.Code) != len(code) {
		t.Errorf("Code length = %d; want %d", len(job.Code), len(code))
	}
	if job.Recipe.Format != recipe.Format {
		t.Errorf("Recipe.Format = %v; want %v", job.Recipe.Format, recipe.Format)
	}
	if job.Recipe.Build != recipe.Build {
		t.Errorf("Recipe.Build = %v; want %v", job.Recipe.Build, recipe.Build)
	}
	if job.Recipe.Test != recipe.Test {
		t.Errorf("Recipe.Test = %v; want %v", job.Recipe.Test, recipe.Test)
	}
	if job.Recipe.Timeout != recipe.Timeout {
		t.Errorf("Recipe.Timeout = %d; want %d", job.Recipe.Timeout, recipe.Timeout)
	}
}

func TestRunResult_AllFields(t *testing.T) {
	jobID := uuid.New()
	completedAt := time.Now()
	duration := 5 * time.Second

	result := queue.RunResult{
		JobID:  jobID,
		Status: "completed",
		FormatResult: &queue.FormatOutput{
			OK:   true,
			Diff: "",
		},
		BuildResult: &queue.BuildOutput{
			OK:     true,
			Output: "",
		},
		TestResult: &queue.TestOutput{
			OK:       true,
			Output:   "PASS\n",
			Duration: 2 * time.Second,
		},
		Duration:    duration,
		CompletedAt: completedAt,
	}

	if result.JobID != jobID {
		t.Errorf("JobID = %v; want %v", result.JobID, jobID)
	}
	if result.Status != "completed" {
		t.Errorf("Status = %q; want %q", result.Status, "completed")
	}
	if result.FormatResult == nil {
		t.Error("FormatResult should not be nil")
	}
	if result.BuildResult == nil {
		t.Error("BuildResult should not be nil")
	}
	if result.TestResult == nil {
		t.Error("TestResult should not be nil")
	}
	if result.Duration != duration {
		t.Errorf("Duration = %v; want %v", result.Duration, duration)
	}
}

func TestRunResult_ErrorCase(t *testing.T) {
	result := queue.RunResult{
		JobID:       uuid.New(),
		Status:      "failed",
		Error:       "execution error: container crashed",
		Duration:    1 * time.Second,
		CompletedAt: time.Now(),
	}

	if result.Status != "failed" {
		t.Errorf("Status = %q; want %q", result.Status, "failed")
	}
	if result.Error == "" {
		t.Error("Error should not be empty for failed status")
	}
}

func TestRunResult_TimeoutCase(t *testing.T) {
	result := queue.RunResult{
		JobID:       uuid.New(),
		Status:      "timeout",
		Error:       "execution timed out after 30s",
		Duration:    30 * time.Second,
		CompletedAt: time.Now(),
	}

	if result.Status != "timeout" {
		t.Errorf("Status = %q; want %q", result.Status, "timeout")
	}
	if result.Duration != 30*time.Second {
		t.Errorf("Duration = %v; want %v", result.Duration, 30*time.Second)
	}
}

func TestConsumerConfig_ZeroValues(t *testing.T) {
	cfg := queue.ConsumerConfig{}

	if cfg.Workers != 0 {
		t.Errorf("Zero value Workers = %d; want 0", cfg.Workers)
	}
	if cfg.Prefetch != 0 {
		t.Errorf("Zero value Prefetch = %d; want 0", cfg.Prefetch)
	}
}

func TestConsumerConfig_CustomValues(t *testing.T) {
	cfg := queue.ConsumerConfig{
		Workers:  10,
		Prefetch: 5,
	}

	if cfg.Workers != 10 {
		t.Errorf("Workers = %d; want 10", cfg.Workers)
	}
	if cfg.Prefetch != 5 {
		t.Errorf("Prefetch = %d; want 5", cfg.Prefetch)
	}
}

// Integration tests (require RabbitMQ)

func TestConnection_Integration(t *testing.T) {
	skipIfNoRabbitMQ(t)

	conn, err := queue.NewConnection("amqp://temper:temper@localhost:5672/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	if !conn.IsConnected() {
		t.Error("Connection should be active")
	}
}

func TestProducer_PublishJob_Integration(t *testing.T) {
	skipIfNoRabbitMQ(t)

	conn, err := queue.NewConnection("amqp://temper:temper@localhost:5672/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	producer := queue.NewProducer(conn)

	job := &queue.RunJob{
		ID:          uuid.New(),
		UserID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ExerciseID:  "test-exercise",
		Code:        map[string]string{"main.go": "package main"},
		Recipe:      queue.RunRecipe{Format: true},
		CreatedAt:   time.Now(),
	}

	ctx := context.Background()
	if err := producer.PublishRunJob(ctx, job); err != nil {
		t.Fatalf("Failed to publish job: %v", err)
	}
}

func TestConsumer_ProcessJob_Integration(t *testing.T) {
	skipIfNoRabbitMQ(t)

	conn, err := queue.NewConnection("amqp://temper:temper@localhost:5672/")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Generate unique test ID
	testID := uuid.New()
	testExerciseID := "integration-test-" + testID.String()

	// Track processed jobs matching our test
	processed := make(chan *queue.RunJob, 10)

	handler := func(ctx context.Context, job *queue.RunJob) (*queue.RunResult, error) {
		// Only track jobs from this test
		if job.ExerciseID == testExerciseID {
			processed <- job
		}
		return &queue.RunResult{
			JobID:       job.ID,
			Status:      "completed",
			CompletedAt: time.Now(),
		}, nil
	}

	// Start consumer
	consumer := queue.NewConsumer(conn, handler, queue.ConsumerConfig{Workers: 1, Prefetch: 1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("Failed to start consumer: %v", err)
	}

	// Small delay to let consumer start
	time.Sleep(100 * time.Millisecond)

	// Publish a job
	producer := queue.NewProducer(conn)
	testJob := &queue.RunJob{
		ID:          testID,
		UserID:      uuid.New(),
		WorkspaceID: uuid.New(),
		ExerciseID:  testExerciseID,
		Code:        map[string]string{"main.go": "package main"},
		Recipe:      queue.RunRecipe{Format: true},
		CreatedAt:   time.Now(),
	}

	if err := producer.PublishRunJob(ctx, testJob); err != nil {
		t.Fatalf("Failed to publish job: %v", err)
	}

	// Wait for job to be processed
	select {
	case receivedJob := <-processed:
		if receivedJob.ID != testJob.ID {
			t.Errorf("Received job ID = %v; want %v", receivedJob.ID, testJob.ID)
		}
		if receivedJob.ExerciseID != testExerciseID {
			t.Errorf("Received exercise ID = %q; want %q", receivedJob.ExerciseID, testExerciseID)
		}
	case <-time.After(5 * time.Second):
		t.Error("Timeout waiting for job to be processed")
	}

	consumer.Stop()
}
