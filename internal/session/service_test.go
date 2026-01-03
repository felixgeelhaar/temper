package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/runner"
)

// mockExecutor implements runner.Executor for testing
type mockExecutor struct {
	formatResult *runner.FormatResult
	formatErr    error
	buildResult  *runner.BuildResult
	buildErr     error
	testResult   *runner.TestResult
	testErr      error
}

func (m *mockExecutor) RunFormat(ctx context.Context, code map[string]string) (*runner.FormatResult, error) {
	if m.formatErr != nil {
		return nil, m.formatErr
	}
	if m.formatResult != nil {
		return m.formatResult, nil
	}
	return &runner.FormatResult{OK: true}, nil
}

func (m *mockExecutor) RunFormatFix(ctx context.Context, code map[string]string) (map[string]string, error) {
	return code, nil
}

func (m *mockExecutor) RunBuild(ctx context.Context, code map[string]string) (*runner.BuildResult, error) {
	if m.buildErr != nil {
		return nil, m.buildErr
	}
	if m.buildResult != nil {
		return m.buildResult, nil
	}
	return &runner.BuildResult{OK: true}, nil
}

func (m *mockExecutor) RunTests(ctx context.Context, code map[string]string, flags []string) (*runner.TestResult, error) {
	if m.testErr != nil {
		return nil, m.testErr
	}
	if m.testResult != nil {
		return m.testResult, nil
	}
	return &runner.TestResult{OK: true, Duration: time.Second}, nil
}

func setupTestService(t *testing.T) (*Service, *Store, string) {
	t.Helper()

	tmpDir := t.TempDir()

	// Create store
	store, err := NewStore(filepath.Join(tmpDir, "sessions"))
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	// Create exercise loader pointing to exercises dir
	exercisesDir := filepath.Join(tmpDir, "exercises")
	os.MkdirAll(exercisesDir, 0755)

	// Create a test pack
	packDir := filepath.Join(exercisesDir, "test-pack")
	os.MkdirAll(filepath.Join(packDir, "basics"), 0755)

	packYAML := `id: test-pack
name: Test Pack
description: Test pack
language: go
exercises:
  - basics/hello
`
	os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(packYAML), 0644)

	exerciseYAML := `id: basics/hello
title: Hello World
description: Write hello world
difficulty: beginner
starter:
  main.go: |
    package main
    func main() {}
check_recipe:
  format: true
  build: true
  test: true
  timeout: 30
`
	os.WriteFile(filepath.Join(packDir, "basics", "hello.yaml"), []byte(exerciseYAML), 0644)

	loader := exercise.NewLoader(exercisesDir)
	executor := &mockExecutor{}

	service := NewService(store, loader, executor)

	return service, store, tmpDir
}

func TestNewService(t *testing.T) {
	service, _, _ := setupTestService(t)

	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_SetProfileService(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Should not panic
	service.SetProfileService(nil)
}

func TestService_SetSpecService(t *testing.T) {
	service, _, _ := setupTestService(t)

	// Should not panic
	service.SetSpecService(nil)
}

func TestService_Create_Training(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	session, err := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session == nil {
		t.Fatal("Create() returned nil session")
	}

	if session.Intent != IntentTraining {
		t.Errorf("Intent = %q; want %q", session.Intent, IntentTraining)
	}

	if session.ExerciseID != "test-pack/basics/hello" {
		t.Errorf("ExerciseID = %q; want %q", session.ExerciseID, "test-pack/basics/hello")
	}

	if session.Status != StatusActive {
		t.Errorf("Status = %q; want %q", session.Status, StatusActive)
	}

	// Starter code should be set
	if session.Code["main.go"] == "" {
		t.Error("Starter code should be set")
	}
}

func TestService_Create_Training_NoExercise(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.Create(ctx, CreateRequest{
		Intent: IntentTraining,
		// No ExerciseID
	})

	if err == nil {
		t.Error("Create() should fail without exercise ID for training intent")
	}
}

func TestService_Create_Greenfield(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	session, err := service.Create(ctx, CreateRequest{
		Intent: IntentGreenfield,
		Code:   map[string]string{"main.go": "package main"},
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session.Intent != IntentGreenfield {
		t.Errorf("Intent = %q; want %q", session.Intent, IntentGreenfield)
	}
}

func TestService_Get(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Get it back
	loaded, err := service.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("ID = %q; want %q", loaded.ID, session.ID)
	}
}

func TestService_Get_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.Get(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Get() error = %v; want ErrSessionNotFound", err)
	}
}

func TestService_Delete(t *testing.T) {
	service, store, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Delete it
	if err := service.Delete(ctx, session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	if store.Exists(session.ID) {
		t.Error("Session should be deleted")
	}
}

func TestService_List(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create two sessions
	service.Create(ctx, CreateRequest{ExerciseID: "test-pack/basics/hello"})
	service.Create(ctx, CreateRequest{ExerciseID: "test-pack/basics/hello"})

	sessions, err := service.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("List() returned %d sessions; want 2", len(sessions))
	}
}

func TestService_UpdateCode(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Update code
	updated, err := service.UpdateCode(ctx, session.ID, map[string]string{
		"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(\"Hello\") }",
	})

	if err != nil {
		t.Fatalf("UpdateCode() error = %v", err)
	}

	if updated.Code["main.go"] == session.Code["main.go"] {
		t.Error("Code should be updated")
	}
}

func TestService_UpdateCode_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.UpdateCode(ctx, "nonexistent", map[string]string{})
	if err != ErrSessionNotFound {
		t.Errorf("UpdateCode() error = %v; want ErrSessionNotFound", err)
	}
}

func TestService_UpdateCode_NotActive(t *testing.T) {
	service, store, _ := setupTestService(t)
	ctx := context.Background()

	// Create and complete a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})
	session.Complete()
	store.Save(session)

	_, err := service.UpdateCode(ctx, session.ID, map[string]string{})
	if err != ErrSessionNotActive {
		t.Errorf("UpdateCode() error = %v; want ErrSessionNotActive", err)
	}
}

func TestService_RunCode(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Run code
	run, err := service.RunCode(ctx, session.ID, RunRequest{
		Format: true,
		Build:  true,
		Test:   true,
	})

	if err != nil {
		t.Fatalf("RunCode() error = %v", err)
	}

	if run == nil {
		t.Fatal("RunCode() returned nil run")
	}

	if run.SessionID != session.ID {
		t.Errorf("SessionID = %q; want %q", run.SessionID, session.ID)
	}

	if run.Result == nil {
		t.Fatal("RunCode() should set Result")
	}

	if !run.Result.FormatOK {
		t.Error("FormatOK should be true")
	}

	if !run.Result.BuildOK {
		t.Error("BuildOK should be true")
	}

	if !run.Result.TestOK {
		t.Error("TestOK should be true")
	}
}

func TestService_RunCode_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.RunCode(ctx, "nonexistent", RunRequest{})
	if err != ErrSessionNotFound {
		t.Errorf("RunCode() error = %v; want ErrSessionNotFound", err)
	}
}

func TestService_RunCode_NotActive(t *testing.T) {
	service, store, _ := setupTestService(t)
	ctx := context.Background()

	// Create and complete a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})
	session.Complete()
	store.Save(session)

	_, err := service.RunCode(ctx, session.ID, RunRequest{})
	if err != ErrSessionNotActive {
		t.Errorf("RunCode() error = %v; want ErrSessionNotActive", err)
	}
}

func TestService_Complete(t *testing.T) {
	service, store, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Complete it
	err := service.Complete(ctx, session.ID)
	if err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	// Verify persisted
	loaded, _ := store.Get(session.ID)
	if loaded.Status != StatusCompleted {
		t.Error("Completed status should be persisted")
	}
}

func TestService_GetRuns(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Run code twice
	service.RunCode(ctx, session.ID, RunRequest{Build: true})
	service.RunCode(ctx, session.ID, RunRequest{Build: true})

	// Get runs
	runs, err := service.GetRuns(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetRuns() error = %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("GetRuns() returned %d runs; want 2", len(runs))
	}
}

func TestService_RecordIntervention(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Record intervention
	intervention := &Intervention{
		ID:        "int-1",
		SessionID: session.ID,
		Level:     domain.L1CategoryHint,
		Type:      domain.TypeHint,
		Content:   "Try using fmt.Println",
	}

	err := service.RecordIntervention(ctx, intervention)
	if err != nil {
		t.Fatalf("RecordIntervention() error = %v", err)
	}

	// Verify session hint count updated
	updated, _ := service.Get(ctx, session.ID)
	if updated.HintCount != 1 {
		t.Errorf("HintCount = %d; want 1", updated.HintCount)
	}
}

func TestService_GetInterventions(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create a session
	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	// Record interventions
	for i := 0; i < 3; i++ {
		intervention := &Intervention{
			ID:        "int-" + string(rune('1'+i)),
			SessionID: session.ID,
			Level:     domain.L1CategoryHint,
			Type:      domain.TypeHint,
			Content:   "Hint",
		}
		service.RecordIntervention(ctx, intervention)
	}

	// Get interventions
	interventions, err := service.GetInterventions(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetInterventions() error = %v", err)
	}

	if len(interventions) != 3 {
		t.Errorf("GetInterventions() returned %d interventions; want 3", len(interventions))
	}
}

func TestSplitExerciseID(t *testing.T) {
	tests := []struct {
		exerciseID string
		wantParts  []string
	}{
		{"go-v1/basics/hello", []string{"go-v1", "basics", "hello"}},
		{"pack/category/exercise", []string{"pack", "category", "exercise"}},
		{"single", []string{"single"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.exerciseID, func(t *testing.T) {
			parts := splitExerciseID(tt.exerciseID)
			if len(parts) != len(tt.wantParts) {
				t.Errorf("splitExerciseID() returned %d parts; want %d", len(parts), len(tt.wantParts))
				return
			}
			for i, p := range parts {
				if p != tt.wantParts[i] {
					t.Errorf("parts[%d] = %q; want %q", i, p, tt.wantParts[i])
				}
			}
		})
	}
}

func TestJoinPath(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "b", "c"}, "a/b/c"},
		{[]string{"single"}, "single"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		got := joinPath(tt.parts...)
		if got != tt.want {
			t.Errorf("joinPath(%v) = %q; want %q", tt.parts, got, tt.want)
		}
	}
}

func TestService_Create_FeatureGuidance_NoSpec(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	_, err := service.Create(ctx, CreateRequest{
		Intent: IntentFeatureGuidance,
		// No SpecPath
	})

	if err != ErrSpecRequired {
		t.Errorf("Create() error = %v; want ErrSpecRequired", err)
	}
}

func TestService_Create_InferGreenfield(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create with just code (no exercise, no spec) - should infer greenfield
	session, err := service.Create(ctx, CreateRequest{
		Code: map[string]string{"main.go": "package main"},
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session.Intent != IntentGreenfield {
		t.Errorf("Intent = %q; want %q", session.Intent, IntentGreenfield)
	}
}

func TestService_Create_WithPolicy(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	customPolicy := &domain.LearningPolicy{
		MaxLevel:        domain.L2LocationConcept,
		CooldownSeconds: 120,
	}

	session, err := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
		Policy:     customPolicy,
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if session.Policy.MaxLevel != domain.L2LocationConcept {
		t.Errorf("Policy.MaxLevel = %v; want %v", session.Policy.MaxLevel, domain.L2LocationConcept)
	}
}

func TestService_RunCode_BuildFails(t *testing.T) {
	tmpDir := t.TempDir()

	store, _ := NewStore(filepath.Join(tmpDir, "sessions"))
	exercisesDir := filepath.Join(tmpDir, "exercises")
	os.MkdirAll(exercisesDir, 0755)

	packDir := filepath.Join(exercisesDir, "test-pack")
	os.MkdirAll(filepath.Join(packDir, "basics"), 0755)

	packYAML := `id: test-pack
name: Test Pack
description: Test pack
language: go
exercises:
  - basics/hello
`
	os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(packYAML), 0644)

	exerciseYAML := `id: basics/hello
title: Hello World
description: Write hello world
difficulty: beginner
starter:
  main.go: |
    package main
    func main() {}
check_recipe:
  format: true
  build: true
  test: true
  timeout: 30
`
	os.WriteFile(filepath.Join(packDir, "basics", "hello.yaml"), []byte(exerciseYAML), 0644)

	loader := exercise.NewLoader(exercisesDir)

	// Mock executor that fails build
	executor := &mockExecutor{
		buildResult: &runner.BuildResult{OK: false, Output: "build error"},
	}

	service := NewService(store, loader, executor)
	ctx := context.Background()

	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
	})

	run, err := service.RunCode(ctx, session.ID, RunRequest{
		Build: true,
		Test:  true,
	})

	if err != nil {
		t.Fatalf("RunCode() error = %v", err)
	}

	if run.Result.BuildOK {
		t.Error("BuildOK should be false")
	}
}

func TestService_RecordIntervention_Cooldown(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	// Create session with short cooldown
	policy := &domain.LearningPolicy{
		MaxLevel:        domain.L5FullSolution,
		CooldownSeconds: 60,
	}

	session, _ := service.Create(ctx, CreateRequest{
		ExerciseID: "test-pack/basics/hello",
		Policy:     policy,
	})

	// Record L3 intervention
	intervention1 := &Intervention{
		ID:        "int-1",
		SessionID: session.ID,
		Level:     domain.L3ConstrainedSnippet,
	}
	err := service.RecordIntervention(ctx, intervention1)
	if err != nil {
		t.Fatalf("First RecordIntervention() error = %v", err)
	}

	// Try to record another L3 immediately - should fail due to cooldown
	intervention2 := &Intervention{
		ID:        "int-2",
		SessionID: session.ID,
		Level:     domain.L3ConstrainedSnippet,
	}
	err = service.RecordIntervention(ctx, intervention2)
	if err != ErrCooldownActive {
		t.Errorf("RecordIntervention() error = %v; want ErrCooldownActive", err)
	}

	// But L2 should still work
	intervention3 := &Intervention{
		ID:        "int-3",
		SessionID: session.ID,
		Level:     domain.L2LocationConcept,
	}
	err = service.RecordIntervention(ctx, intervention3)
	if err != nil {
		t.Errorf("L2 RecordIntervention() error = %v; want nil", err)
	}
}

func TestService_RecordIntervention_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	intervention := &Intervention{
		ID:        "int-1",
		SessionID: "nonexistent",
		Level:     domain.L1CategoryHint,
	}

	err := service.RecordIntervention(ctx, intervention)
	if err != ErrSessionNotFound {
		t.Errorf("RecordIntervention() error = %v; want ErrSessionNotFound", err)
	}
}

func TestService_Complete_NotFound(t *testing.T) {
	service, _, _ := setupTestService(t)
	ctx := context.Background()

	err := service.Complete(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("Complete() error = %v; want ErrSessionNotFound", err)
	}
}
