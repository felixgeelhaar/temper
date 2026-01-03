package session

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestNewStore(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}
}

func TestStore_Save_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test-exercise", map[string]string{"main.go": "package main"}, domain.DefaultPolicy())

	// Save
	if err := store.Save(session); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Get
	loaded, err := store.Get(session.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != session.ID {
		t.Errorf("ID = %q; want %q", loaded.ID, session.ID)
	}
	if loaded.ExerciseID != session.ExerciseID {
		t.Errorf("ExerciseID = %q; want %q", loaded.ExerciseID, session.ExerciseID)
	}
	if loaded.Code["main.go"] != "package main" {
		t.Errorf("Code[main.go] = %q; want %q", loaded.Code["main.go"], "package main")
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	_, err := store.Get("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Get() error = %v; want ErrNotFound", err)
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(session)

	// Delete
	if err := store.Delete(session.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err := store.Get(session.ID)
	if err != ErrNotFound {
		t.Error("Get() should return ErrNotFound after delete")
	}
}

func TestStore_Delete_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	err := store.Delete("nonexistent")
	if err != ErrNotFound {
		t.Errorf("Delete() error = %v; want ErrNotFound", err)
	}
}

func TestStore_List(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	// Save multiple sessions
	s1 := NewSession("ex1", map[string]string{}, domain.DefaultPolicy())
	s2 := NewSession("ex2", map[string]string{}, domain.DefaultPolicy())
	store.Save(s1)
	store.Save(s2)

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("List() returned %d items; want 2", len(ids))
	}
}

func TestStore_ListActive(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	// Save active and completed sessions
	active := NewSession("ex1", map[string]string{}, domain.DefaultPolicy())
	completed := NewSession("ex2", map[string]string{}, domain.DefaultPolicy())
	completed.Complete()

	store.Save(active)
	store.Save(completed)

	sessions, err := store.ListActive()
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("ListActive() returned %d items; want 1", len(sessions))
	}
	if sessions[0].ID != active.ID {
		t.Errorf("ListActive() returned wrong session")
	}
}

func TestStore_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test", map[string]string{}, domain.DefaultPolicy())

	// Before save
	if store.Exists(session.ID) {
		t.Error("Exists() should return false before save")
	}

	// After save
	store.Save(session)
	if !store.Exists(session.ID) {
		t.Error("Exists() should return true after save")
	}
}

func TestStore_SaveRun_GetRun(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(session)

	run := &Run{
		ID:        "run-1",
		SessionID: session.ID,
		Result: &RunResult{
			FormatOK: true,
			BuildOK:  true,
			TestOK:   true,
		},
	}

	// Save run
	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	// Get run
	loaded, err := store.GetRun(session.ID, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}

	if loaded.ID != run.ID {
		t.Errorf("Run ID = %q; want %q", loaded.ID, run.ID)
	}
	if loaded.SessionID != session.ID {
		t.Errorf("Run SessionID = %q; want %q", loaded.SessionID, session.ID)
	}
}

func TestStore_GetRun_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	_, err := store.GetRun("session", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("GetRun() error = %v; want ErrNotFound", err)
	}
}

func TestStore_ListRuns(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(session)

	// Save multiple runs
	for _, id := range []string{"run-1", "run-2"} {
		run := &Run{ID: id, SessionID: session.ID}
		store.SaveRun(run)
	}

	ids, err := store.ListRuns(session.ID)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("ListRuns() returned %d items; want 2", len(ids))
	}
}

func TestStore_SaveIntervention_GetIntervention(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(session)

	intervention := &Intervention{
		ID:        "int-1",
		SessionID: session.ID,
		Level:     domain.L1CategoryHint,
		Content:   "Try using a loop",
	}

	// Save
	if err := store.SaveIntervention(intervention); err != nil {
		t.Fatalf("SaveIntervention() error = %v", err)
	}

	// Get
	loaded, err := store.GetIntervention(session.ID, intervention.ID)
	if err != nil {
		t.Fatalf("GetIntervention() error = %v", err)
	}

	if loaded.ID != intervention.ID {
		t.Errorf("Intervention ID = %q; want %q", loaded.ID, intervention.ID)
	}
	if loaded.Content != intervention.Content {
		t.Errorf("Intervention Content = %q; want %q", loaded.Content, intervention.Content)
	}
}

func TestStore_GetIntervention_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	_, err := store.GetIntervention("session", "nonexistent")
	if err != ErrNotFound {
		t.Errorf("GetIntervention() error = %v; want ErrNotFound", err)
	}
}

func TestStore_ListInterventions(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	session := NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(session)

	// Save multiple interventions
	for _, id := range []string{"int-1", "int-2", "int-3"} {
		intervention := &Intervention{ID: id, SessionID: session.ID}
		store.SaveIntervention(intervention)
	}

	ids, err := store.ListInterventions(session.ID)
	if err != nil {
		t.Fatalf("ListInterventions() error = %v", err)
	}

	if len(ids) != 3 {
		t.Errorf("ListInterventions() returned %d items; want 3", len(ids))
	}
}
