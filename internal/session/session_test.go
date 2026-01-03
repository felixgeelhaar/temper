package session

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestNewSession(t *testing.T) {
	code := map[string]string{"main.go": "package main"}
	policy := domain.DefaultPolicy()

	s := NewSession("go-v1/basics/hello-world", code, policy)

	if s.ID == "" {
		t.Error("NewSession() should generate an ID")
	}
	if s.ExerciseID != "go-v1/basics/hello-world" {
		t.Errorf("ExerciseID = %q; want %q", s.ExerciseID, "go-v1/basics/hello-world")
	}
	if s.Status != StatusActive {
		t.Errorf("Status = %q; want %q", s.Status, StatusActive)
	}
	if s.Intent != IntentTraining {
		t.Errorf("Intent = %q; want %q", s.Intent, IntentTraining)
	}
	if s.RunCount != 0 {
		t.Errorf("RunCount = %d; want 0", s.RunCount)
	}
	if s.HintCount != 0 {
		t.Errorf("HintCount = %d; want 0", s.HintCount)
	}
}

func TestNewFeatureSession(t *testing.T) {
	code := map[string]string{"main.go": "package main"}
	policy := domain.DefaultPolicy()

	s := NewFeatureSession("/path/to/spec.yaml", code, policy)

	if s.Intent != IntentFeatureGuidance {
		t.Errorf("Intent = %q; want %q", s.Intent, IntentFeatureGuidance)
	}
	if s.SpecPath != "/path/to/spec.yaml" {
		t.Errorf("SpecPath = %q; want %q", s.SpecPath, "/path/to/spec.yaml")
	}
	if s.ExerciseID != "" {
		t.Errorf("ExerciseID = %q; want empty", s.ExerciseID)
	}
}

func TestNewGreenfieldSession(t *testing.T) {
	code := map[string]string{}
	policy := domain.DefaultPolicy()

	s := NewGreenfieldSession(code, policy)

	if s.Intent != IntentGreenfield {
		t.Errorf("Intent = %q; want %q", s.Intent, IntentGreenfield)
	}
}

func TestSession_UpdateCode(t *testing.T) {
	s := NewSession("test", map[string]string{"a.go": "old"}, domain.DefaultPolicy())
	originalUpdated := s.UpdatedAt

	time.Sleep(1 * time.Millisecond)
	s.UpdateCode(map[string]string{"a.go": "new"})

	if s.Code["a.go"] != "new" {
		t.Errorf("Code[a.go] = %q; want %q", s.Code["a.go"], "new")
	}
	if !s.UpdatedAt.After(originalUpdated) {
		t.Error("UpdatedAt should be updated")
	}
}

func TestSession_RecordRun(t *testing.T) {
	s := NewSession("test", map[string]string{}, domain.DefaultPolicy())

	s.RecordRun()

	if s.RunCount != 1 {
		t.Errorf("RunCount = %d; want 1", s.RunCount)
	}
	if s.LastRunAt == nil {
		t.Error("LastRunAt should be set")
	}

	s.RecordRun()
	if s.RunCount != 2 {
		t.Errorf("RunCount = %d; want 2", s.RunCount)
	}
}

func TestSession_RecordIntervention(t *testing.T) {
	s := NewSession("test", map[string]string{}, domain.DefaultPolicy())

	s.RecordIntervention()

	if s.HintCount != 1 {
		t.Errorf("HintCount = %d; want 1", s.HintCount)
	}
	if s.LastInterventionAt == nil {
		t.Error("LastInterventionAt should be set")
	}
}

func TestSession_Complete(t *testing.T) {
	s := NewSession("test", map[string]string{}, domain.DefaultPolicy())

	s.Complete()

	if s.Status != StatusCompleted {
		t.Errorf("Status = %q; want %q", s.Status, StatusCompleted)
	}
}

func TestSession_Abandon(t *testing.T) {
	s := NewSession("test", map[string]string{}, domain.DefaultPolicy())

	s.Abandon()

	if s.Status != StatusAbandoned {
		t.Errorf("Status = %q; want %q", s.Status, StatusAbandoned)
	}
}

func TestSession_CanRequestIntervention(t *testing.T) {
	policy := domain.LearningPolicy{
		MaxLevel:        domain.L3ConstrainedSnippet,
		CooldownSeconds: 1, // 1 second for testing
	}
	s := NewSession("test", map[string]string{}, policy)

	// L0-L2 should always be allowed
	if !s.CanRequestIntervention(domain.L0Clarify) {
		t.Error("L0 should always be allowed")
	}
	if !s.CanRequestIntervention(domain.L2LocationConcept) {
		t.Error("L2 should always be allowed")
	}

	// L3 should be allowed before any intervention
	if !s.CanRequestIntervention(domain.L3ConstrainedSnippet) {
		t.Error("L3 should be allowed before any intervention")
	}

	// Record an intervention
	s.RecordIntervention()

	// L3 should be blocked during cooldown
	if s.CanRequestIntervention(domain.L3ConstrainedSnippet) {
		t.Error("L3 should be blocked during cooldown")
	}

	// Wait for cooldown
	time.Sleep(1100 * time.Millisecond)

	// L3 should be allowed after cooldown
	if !s.CanRequestIntervention(domain.L3ConstrainedSnippet) {
		t.Error("L3 should be allowed after cooldown")
	}
}

func TestSession_CooldownRemaining(t *testing.T) {
	policy := domain.LearningPolicy{
		CooldownSeconds: 60,
	}
	s := NewSession("test", map[string]string{}, policy)

	// No intervention yet
	if s.CooldownRemaining() != 0 {
		t.Error("CooldownRemaining should be 0 before any intervention")
	}

	// Record intervention
	s.RecordIntervention()

	remaining := s.CooldownRemaining()
	if remaining <= 0 || remaining > 60*time.Second {
		t.Errorf("CooldownRemaining = %v; want between 0 and 60s", remaining)
	}
}
