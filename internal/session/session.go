package session

import (
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// SessionIntent represents the type of session (Training, Greenfield, Feature Guidance)
type SessionIntent string

const (
	IntentTraining        SessionIntent = "training"
	IntentGreenfield      SessionIntent = "greenfield"
	IntentFeatureGuidance SessionIntent = "feature_guidance"
)

// Session represents an active pairing session
type Session struct {
	ID         string            `json:"id"`
	ExerciseID string            `json:"exercise_id,omitempty"`
	Code       map[string]string `json:"code"`
	Policy     domain.LearningPolicy `json:"policy"`
	Status     Status            `json:"status"`

	// Session intent and spec (for feature guidance)
	Intent   SessionIntent `json:"intent"`
	SpecPath string        `json:"spec_path,omitempty"`

	// Statistics
	RunCount         int       `json:"run_count"`
	HintCount        int       `json:"hint_count"`
	LastRunAt        *time.Time `json:"last_run_at,omitempty"`
	LastInterventionAt *time.Time `json:"last_intervention_at,omitempty"`

	// Timestamps
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Status represents the session state
type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
	StatusAbandoned Status = "abandoned"
)

// Run represents a code execution within a session
type Run struct {
	ID        string           `json:"id"`
	SessionID string           `json:"session_id"`
	Code      map[string]string `json:"code"`
	Result    *RunResult       `json:"result,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

// RunResult contains the outcome of a run
type RunResult struct {
	FormatOK   bool          `json:"format_ok"`
	FormatDiff string        `json:"format_diff,omitempty"`
	BuildOK    bool          `json:"build_ok"`
	BuildOutput string       `json:"build_output,omitempty"`
	TestOK     bool          `json:"test_ok"`
	TestOutput string        `json:"test_output,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// Intervention represents an AI intervention within a session
type Intervention struct {
	ID        string                  `json:"id"`
	SessionID string                  `json:"session_id"`
	RunID     *string                 `json:"run_id,omitempty"`
	Intent    domain.Intent           `json:"intent"`
	Level     domain.InterventionLevel `json:"level"`
	Type      domain.InterventionType `json:"type"`
	Content   string                  `json:"content"`
	CreatedAt time.Time               `json:"created_at"`
}

// NewSession creates a new session for an exercise (training intent)
func NewSession(exerciseID string, code map[string]string, policy domain.LearningPolicy) *Session {
	now := time.Now()
	return &Session{
		ID:         uuid.New().String(),
		ExerciseID: exerciseID,
		Code:       code,
		Policy:     policy,
		Status:     StatusActive,
		Intent:     IntentTraining,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// NewFeatureSession creates a new session for feature guidance with a spec
func NewFeatureSession(specPath string, code map[string]string, policy domain.LearningPolicy) *Session {
	now := time.Now()
	return &Session{
		ID:        uuid.New().String(),
		Code:      code,
		Policy:    policy,
		Status:    StatusActive,
		Intent:    IntentFeatureGuidance,
		SpecPath:  specPath,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewGreenfieldSession creates a new session for greenfield projects
func NewGreenfieldSession(code map[string]string, policy domain.LearningPolicy) *Session {
	now := time.Now()
	return &Session{
		ID:        uuid.New().String(),
		Code:      code,
		Policy:    policy,
		Status:    StatusActive,
		Intent:    IntentGreenfield,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// UpdateCode updates the session's code
func (s *Session) UpdateCode(code map[string]string) {
	s.Code = code
	s.UpdatedAt = time.Now()
}

// RecordRun records that a run was executed
func (s *Session) RecordRun() {
	now := time.Now()
	s.RunCount++
	s.LastRunAt = &now
	s.UpdatedAt = now
}

// RecordIntervention records that an intervention was delivered
func (s *Session) RecordIntervention() {
	now := time.Now()
	s.HintCount++
	s.LastInterventionAt = &now
	s.UpdatedAt = now
}

// Complete marks the session as completed
func (s *Session) Complete() {
	s.Status = StatusCompleted
	s.UpdatedAt = time.Now()
}

// Abandon marks the session as abandoned
func (s *Session) Abandon() {
	s.Status = StatusAbandoned
	s.UpdatedAt = time.Now()
}

// CanRequestIntervention checks if an intervention can be requested based on cooldown
func (s *Session) CanRequestIntervention(level domain.InterventionLevel) bool {
	// Always allow L0-L2
	if level <= domain.L2LocationConcept {
		return true
	}

	// Check cooldown for L3+
	if s.LastInterventionAt == nil {
		return true
	}

	cooldown := time.Duration(s.Policy.CooldownSeconds) * time.Second
	return time.Since(*s.LastInterventionAt) >= cooldown
}

// CooldownRemaining returns the remaining cooldown time
func (s *Session) CooldownRemaining() time.Duration {
	if s.LastInterventionAt == nil {
		return 0
	}

	cooldown := time.Duration(s.Policy.CooldownSeconds) * time.Second
	elapsed := time.Since(*s.LastInterventionAt)

	if elapsed >= cooldown {
		return 0
	}

	return cooldown - elapsed
}
