package session

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/felixgeelhaar/temper/internal/runner"
	"github.com/felixgeelhaar/temper/internal/spec"
	"github.com/google/uuid"
)

var (
	ErrSessionNotFound   = errors.New("session not found")
	ErrExerciseNotFound  = errors.New("exercise not found")
	ErrCooldownActive    = errors.New("intervention cooldown active")
	ErrSessionNotActive  = errors.New("session is not active")
	ErrSpecRequired      = errors.New("spec path required for feature guidance intent")
	ErrSpecInvalid       = errors.New("spec validation failed")
)

// Service manages pairing sessions
type Service struct {
	store          *Store
	loader         *exercise.Loader
	executor       runner.Executor
	profileService *profile.Service // Optional: tracks learning progress
	specService    *spec.Service    // Optional: spec management for feature guidance
}

// NewService creates a new session service
func NewService(store *Store, loader *exercise.Loader, executor runner.Executor) *Service {
	return &Service{
		store:    store,
		loader:   loader,
		executor: executor,
	}
}

// SetProfileService sets the profile service for tracking learning progress
func (s *Service) SetProfileService(ps *profile.Service) {
	s.profileService = ps
}

// SetSpecService sets the spec service for feature guidance sessions
func (s *Service) SetSpecService(ss *spec.Service) {
	s.specService = ss
}

// CreateRequest contains data for creating a session
type CreateRequest struct {
	ExerciseID string                // For training intent
	SpecPath   string                // For feature guidance intent
	Intent     SessionIntent         // Explicit intent (optional, inferred if empty)
	Code       map[string]string     // Initial code (for greenfield/feature)
	Policy     *domain.LearningPolicy
}

// Create starts a new pairing session
func (s *Service) Create(ctx context.Context, req CreateRequest) (*Session, error) {
	// Infer intent if not explicitly provided
	intent := s.inferIntent(req)

	// Use provided policy or default
	policy := domain.DefaultPolicy()
	if req.Policy != nil {
		policy = *req.Policy
	}

	var session *Session

	switch intent {
	case IntentTraining:
		// Training intent requires an exercise
		if req.ExerciseID == "" {
			return nil, fmt.Errorf("exercise ID required for training intent")
		}
		sess, err := s.createTrainingSession(ctx, req.ExerciseID, policy)
		if err != nil {
			return nil, err
		}
		session = sess

	case IntentFeatureGuidance:
		// Feature guidance requires a valid spec
		if req.SpecPath == "" {
			return nil, ErrSpecRequired
		}
		sess, err := s.createFeatureSession(ctx, req.SpecPath, req.Code, policy)
		if err != nil {
			return nil, err
		}
		session = sess

	case IntentGreenfield:
		// Greenfield creates a fresh session
		session = NewGreenfieldSession(req.Code, policy)

	default:
		return nil, fmt.Errorf("unknown intent: %s", intent)
	}

	// Persist
	if err := s.store.Save(session); err != nil {
		return nil, fmt.Errorf("save session: %w", err)
	}

	// Notify profile service of session start
	if s.profileService != nil {
		if err := s.profileService.OnSessionStart(ctx, profile.SessionInfo{
			ID:         session.ID,
			ExerciseID: session.ExerciseID,
			Status:     string(session.Status),
			CreatedAt:  session.CreatedAt,
		}); err != nil {
			slog.Warn("failed to record session start in profile", "error", err)
		}
	}

	return session, nil
}

// inferIntent determines session intent from request context
func (s *Service) inferIntent(req CreateRequest) SessionIntent {
	// If explicitly set, use that
	if req.Intent != "" {
		return req.Intent
	}

	// Infer based on what's provided
	if req.ExerciseID != "" {
		return IntentTraining
	}
	if req.SpecPath != "" {
		return IntentFeatureGuidance
	}
	return IntentGreenfield
}

// createTrainingSession creates a session for an exercise
func (s *Service) createTrainingSession(ctx context.Context, exerciseID string, policy domain.LearningPolicy) (*Session, error) {
	// Parse exercise ID (pack/category/slug)
	parts := splitExerciseID(exerciseID)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid exercise ID format: %s", exerciseID)
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

	return NewSession(exerciseID, code, policy), nil
}

// createFeatureSession creates a session for feature guidance with spec
func (s *Service) createFeatureSession(ctx context.Context, specPath string, code map[string]string, policy domain.LearningPolicy) (*Session, error) {
	// Validate spec if spec service is available
	if s.specService != nil {
		validation, err := s.specService.Validate(ctx, specPath)
		if err != nil {
			return nil, fmt.Errorf("load spec: %w", err)
		}
		if !validation.Valid {
			return nil, fmt.Errorf("%w: %v", ErrSpecInvalid, validation.Errors)
		}
	}

	if code == nil {
		code = make(map[string]string)
	}

	return NewFeatureSession(specPath, code, policy), nil
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

	// Notify profile service of run completion
	if s.profileService != nil {
		if err := s.profileService.OnRunComplete(ctx, profile.SessionInfo{
			ID:         session.ID,
			ExerciseID: session.ExerciseID,
			RunCount:   session.RunCount,
			HintCount:  session.HintCount,
			Status:     string(session.Status),
			CreatedAt:  session.CreatedAt,
		}, profile.RunInfo{
			Success:     result.TestOK && result.BuildOK,
			BuildOutput: result.BuildOutput,
			TestOutput:  result.TestOutput,
			Duration:    result.Duration,
		}); err != nil {
			slog.Warn("failed to record run in profile", "error", err)
		}
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

	if err := s.store.Save(session); err != nil {
		return err
	}

	// Notify profile service of session completion
	if s.profileService != nil {
		if err := s.profileService.OnSessionComplete(ctx, profile.SessionInfo{
			ID:         session.ID,
			ExerciseID: session.ExerciseID,
			RunCount:   session.RunCount,
			HintCount:  session.HintCount,
			Status:     string(session.Status),
			CreatedAt:  session.CreatedAt,
		}); err != nil {
			slog.Warn("failed to record session completion in profile", "error", err)
		}
	}

	return nil
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

	if err := s.store.SaveIntervention(intervention); err != nil {
		return err
	}

	// Notify profile service of hint delivery
	if s.profileService != nil {
		if err := s.profileService.OnHintDelivered(ctx, profile.SessionInfo{
			ID:         session.ID,
			ExerciseID: session.ExerciseID,
			RunCount:   session.RunCount,
			HintCount:  session.HintCount,
			Status:     string(session.Status),
			CreatedAt:  session.CreatedAt,
		}); err != nil {
			slog.Warn("failed to record hint in profile", "error", err)
		}
	}

	return nil
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
