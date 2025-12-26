package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// Config holds runner configuration
type Config struct {
	PoolSize    int
	Timeout     time.Duration
	MemoryMB    int
	CPULimit    float64
	BaseImage   string
}

// DefaultConfig returns default runner configuration
func DefaultConfig() Config {
	return Config{
		PoolSize:  3,
		Timeout:   30 * time.Second,
		MemoryMB:  256,
		CPULimit:  0.5,
		BaseImage: "temper-runner-sandbox:latest",
	}
}

// Service handles code execution
type Service struct {
	config   Config
	executor Executor
	parser   *Parser

	mu       sync.Mutex
	running  map[uuid.UUID]*runState
}

type runState struct {
	run      *domain.Run
	cancel   context.CancelFunc
	doneCh   chan struct{}
}

// NewService creates a new runner service
func NewService(cfg Config, executor Executor) *Service {
	return &Service{
		config:   cfg,
		executor: executor,
		parser:   NewParser(),
		running:  make(map[uuid.UUID]*runState),
	}
}

// ExecuteRequest contains data for executing code
type ExecuteRequest struct {
	RunID      uuid.UUID
	UserID     uuid.UUID
	ArtifactID uuid.UUID
	ExerciseID *string
	Code       map[string]string
	Recipe     domain.CheckRecipe
}

// Execute runs code and returns the output
func (s *Service) Execute(ctx context.Context, req ExecuteRequest) (*domain.RunOutput, error) {
	// Create timeout context
	timeout := s.config.Timeout
	if req.Recipe.Timeout > 0 {
		timeout = time.Duration(req.Recipe.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Track running state
	state := &runState{
		run: &domain.Run{
			ID:         req.RunID,
			ArtifactID: req.ArtifactID,
			UserID:     req.UserID,
			ExerciseID: req.ExerciseID,
			Status:     domain.RunStatusRunning,
			Recipe:     req.Recipe,
		},
		cancel: cancel,
		doneCh: make(chan struct{}),
	}

	s.mu.Lock()
	s.running[req.RunID] = state
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, req.RunID)
		s.mu.Unlock()
		close(state.doneCh)
	}()

	output := &domain.RunOutput{}

	// Run checks based on recipe
	if req.Recipe.Format {
		formatResult, err := s.executor.RunFormat(ctx, req.Code)
		if err != nil {
			return nil, fmt.Errorf("format check: %w", err)
		}
		output.FormatOK = formatResult.OK
		output.FormatDiff = formatResult.Diff
	}

	if req.Recipe.Build {
		buildResult, err := s.executor.RunBuild(ctx, req.Code)
		if err != nil {
			return nil, fmt.Errorf("build check: %w", err)
		}
		output.BuildOK = buildResult.OK
		output.BuildOutput = buildResult.Output
		output.BuildErrors = s.parser.ParseBuildErrors(buildResult.Output)
	}

	// Only run tests if build succeeded
	if req.Recipe.Test && output.BuildOK {
		testResult, err := s.executor.RunTests(ctx, req.Code, req.Recipe.TestFlags)
		if err != nil {
			return nil, fmt.Errorf("test check: %w", err)
		}
		output.TestOK = testResult.OK
		output.TestOutput = testResult.Output
		output.TestResults = s.parser.ParseTestOutput(testResult.Output)

		for _, tr := range output.TestResults {
			if tr.Passed {
				output.TestsPassed++
			} else {
				output.TestsFailed++
			}
		}
		output.Duration = testResult.Duration
		output.Logs = testResult.Output
	}

	return output, nil
}

// Cancel cancels a running execution
func (s *Service) Cancel(runID uuid.UUID) error {
	s.mu.Lock()
	state, ok := s.running[runID]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	state.cancel()
	return nil
}

// IsRunning checks if a run is currently executing
func (s *Service) IsRunning(runID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.running[runID]
	return ok
}

// Wait waits for a run to complete
func (s *Service) Wait(ctx context.Context, runID uuid.UUID) error {
	s.mu.Lock()
	state, ok := s.running[runID]
	s.mu.Unlock()

	if !ok {
		return nil // Already completed
	}

	select {
	case <-state.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
