package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/google/uuid"
)

var (
	ErrSessionNotFound   = errors.New("session not found")
	ErrExerciseNotFound  = errors.New("exercise not found")
	ErrCooldownActive    = errors.New("intervention cooldown active")
	ErrSessionNotActive  = errors.New("session is not active")
)

// Service manages pairing sessions
type Service struct {
	store    *Store
	loader   *exercise.Loader
	executor runner.Executor
}

// NewService creates a new session service
func NewService(store *Store, loader *exercise.Loader, executor runner.Executor) *Service {
	return &Service{
		store:    store,
		loader:   loader,
		executor: executor,
	}
}

// CreateRequest contains data for creating a session
type CreateRequest struct {
	ExerciseID string
	Policy     *domain.LearningPolicy
}

// Create starts a new pairing session
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Session, error) {
	// Parse exercise ID (pack/category/slug)
	parts := splitExerciseID(req.ExerciseID)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid exercise ID format: %s", req.ExerciseID)
	}

	packID := parts[0]
	slug := joinPath(parts[1:]...)

	// Load exercise
	ex, err := s.loader.LoadExercise(packID, slug)
	if err != nil {
		return nil, ErrExerciseNotFound
	}

	// Combine starter and test code
	code := make(map[string]string)
	for k, v := range ex.StarterCode {
		code[k] = v
	}
	for k, v := range ex.TestCode {
		code[k] = v
	}

	// Use provided policy or default
	policy := domain.DefaultPolicy()
	if req.Policy != nil {
		policy = *req.Policy
	}

	// Create session
	session := NewSession(req.ExerciseID, code, policy)

	// Persist
	if err := s.store.Save(session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return session, nil
}

// Get retrieves a session by ID
func (s *Service) Get(ctx context.Context, id string) (*Session, error) {
	session, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}
	return session, nil
}

// Delete removes a session
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.store.Delete(id)
}

// List returns all active sessions
func (s *Service) List(ctx context.Context) ([]*Session, error) {
	return s.store.ListActive()
}

// UpdateCode updates the code in a session
func (s *Service) UpdateCode(ctx context.Context, id string, code map[string]string) (*Session, error) {
	session, err := s.store.Get(id)
	if err != nil {
		return nil, ErrSessionNotFound
	}

	if session.Status != StatusActive {
		return nil, ErrSessionNotActive
	}

	session.UpdateCode(code)

	if err := s.store.Save(session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	return session, nil
}

// RunRequest contains data for running code
type RunRequest struct {
	Code   map[string]string
	Format bool
	Build  bool
	Test   bool
}

// RunCode executes code in a session
func (s *Service) RunCode(ctx context.Context, sessionID string, req RunRequest) (*Run, error) {
	session, err := s.store.Get(sessionID)
	if err != nil {
		return nil, ErrSessionNotFound
	}

	if session.Status != StatusActive {
		return nil, ErrSessionNotActive
	}

	// Use provided code or session's current code
	code := req.Code
	if code == nil {
		code = session.Code
	}

	// Create run record
	run := &Run{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Code:      code,
		CreatedAt: time.Now(),
	}

	result := &RunResult{}

	// Execute format check
	if req.Format {
		formatResult, err := s.executor.RunFormat(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("format check: %w", err)
		}
		result.FormatOK = formatResult.OK
		result.FormatDiff = formatResult.Diff
	}

	// Execute build check
	if req.Build {
		buildResult, err := s.executor.RunBuild(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("build check: %w", err)
		}
		result.BuildOK = buildResult.OK
		result.BuildOutput = buildResult.Output

		// Skip tests if build failed
		if !buildResult.OK {
			run.Result = result
			session.RecordRun()

			if err := s.store.Save(session); err != nil {
				return nil, fmt.Errorf("save session: %w", err)
			}
			if err := s.store.SaveRun(run); err != nil {
				return nil, fmt.Errorf("save run: %w", err)
			}

			return run, nil
		}
	}

	// Execute tests
	if req.Test {
		testResult, err := s.executor.RunTests(ctx, code, []string{"-v"})
		if err != nil {
			return nil, fmt.Errorf("test run: %w", err)
		}
		result.TestOK = testResult.OK
		result.TestOutput = testResult.Output
		result.Duration = testResult.Duration
	}

	run.Result = result

	// Update session
	session.RecordRun()
	session.UpdateCode(code)

	if err := s.store.Save(session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}
	if err := s.store.SaveRun(run); err != nil {
		return nil, fmt.Errorf("save run: %w", err)
	}

	return run, nil
}

// Complete marks a session as completed
func (s *Service) Complete(ctx context.Context, id string) error {
	session, err := s.store.Get(id)
	if err != nil {
		return ErrSessionNotFound
	}

	session.Complete()

	return s.store.Save(session)
}

// GetRuns returns all runs for a session
func (s *Service) GetRuns(ctx context.Context, sessionID string) ([]*Run, error) {
	ids, err := s.store.ListRuns(sessionID)
	if err != nil {
		return nil, err
	}

	runs := make([]*Run, 0, len(ids))
	for _, id := range ids {
		run, err := s.store.GetRun(sessionID, id)
		if err != nil {
			continue
		}
		runs = append(runs, run)
	}

	return runs, nil
}

// RecordIntervention records an intervention in a session
func (s *Service) RecordIntervention(ctx context.Context, intervention *Intervention) error {
	session, err := s.store.Get(intervention.SessionID)
	if err != nil {
		return ErrSessionNotFound
	}

	// Check cooldown for L3+ interventions
	if intervention.Level >= domain.L3ConstrainedSnippet {
		if !session.CanRequestIntervention(intervention.Level) {
			return ErrCooldownActive
		}
	}

	session.RecordIntervention()

	if err := s.store.Save(session); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return s.store.SaveIntervention(intervention)
}

// GetInterventions returns all interventions for a session
func (s *Service) GetInterventions(ctx context.Context, sessionID string) ([]*Intervention, error) {
	ids, err := s.store.ListInterventions(sessionID)
	if err != nil {
		return nil, err
	}

	interventions := make([]*Intervention, 0, len(ids))
	for _, id := range ids {
		intervention, err := s.store.GetIntervention(sessionID, id)
		if err != nil {
			continue
		}
		interventions = append(interventions, intervention)
	}

	return interventions, nil
}

// Helper functions

func splitExerciseID(id string) []string {
	var parts []string
	current := ""
	for _, c := range id {
		if c == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func joinPath(parts ...string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "/"
		}
		result += p
	}
	return result
}
