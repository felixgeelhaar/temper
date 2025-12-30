package queue

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// Producer publishes run jobs to the queue
type Producer struct {
	conn *Connection
}

// NewProducer creates a new queue producer
func NewProducer(conn *Connection) *Producer {
	return &Producer{conn: conn}
}

// PublishRunJob publishes a code execution job to the queue
func (p *Producer) PublishRunJob(ctx context.Context, job *RunJob) error {
	if job.ID == uuid.Nil {
		job.ID = uuid.New()
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now()
	}

	if err := p.conn.PublishJSON(ctx, RunQueueName, job); err != nil {
		return fmt.Errorf("failed to publish run job: %w", err)
	}

	slog.Info("published run job",
		"job_id", job.ID,
		"user_id", job.UserID,
		"workspace_id", job.WorkspaceID,
		"exercise_id", job.ExerciseID,
	)

	return nil
}

// PublishResult publishes a run result to the results queue
func (p *Producer) PublishResult(ctx context.Context, result *RunResult) error {
	if result.CompletedAt.IsZero() {
		result.CompletedAt = time.Now()
	}

	if err := p.conn.PublishJSON(ctx, ResultQueueName, result); err != nil {
		return fmt.Errorf("failed to publish run result: %w", err)
	}

	slog.Info("published run result",
		"job_id", result.JobID,
		"status", result.Status,
		"duration", result.Duration,
	)

	return nil
}

// CreateRunJob creates a new run job with the given parameters
func CreateRunJob(
	userID, workspaceID uuid.UUID,
	exerciseID string,
	code map[string]string,
	recipe RunRecipe,
) *RunJob {
	return &RunJob{
		ID:          uuid.New(),
		UserID:      userID,
		WorkspaceID: workspaceID,
		ExerciseID:  exerciseID,
		Code:        code,
		Recipe:      recipe,
		CreatedAt:   time.Now(),
	}
}
