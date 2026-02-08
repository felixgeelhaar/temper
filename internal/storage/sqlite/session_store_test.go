package sqlite

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
)

func TestSessionStore_Save_Get(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("go-v1/basics/hello-world", map[string]string{"main.go": "package main"}, domain.DefaultPolicy())

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != sess.ID {
		t.Errorf("ID = %q; want %q", loaded.ID, sess.ID)
	}
	if loaded.ExerciseID != sess.ExerciseID {
		t.Errorf("ExerciseID = %q; want %q", loaded.ExerciseID, sess.ExerciseID)
	}
	if loaded.Code["main.go"] != "package main" {
		t.Errorf("Code[main.go] = %q; want %q", loaded.Code["main.go"], "package main")
	}
	if loaded.Status != session.StatusActive {
		t.Errorf("Status = %q; want %q", loaded.Status, session.StatusActive)
	}
	if loaded.Intent != session.IntentTraining {
		t.Errorf("Intent = %q; want %q", loaded.Intent, session.IntentTraining)
	}
	if loaded.Policy.MaxLevel != domain.L3ConstrainedSnippet {
		t.Errorf("Policy.MaxLevel = %d; want %d", loaded.Policy.MaxLevel, domain.L3ConstrainedSnippet)
	}
}

func TestSessionStore_Get_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	_, err := store.Get("nonexistent")
	if err != session.ErrNotFound {
		t.Errorf("Get() error = %v; want ErrNotFound", err)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(sess)

	if err := store.Delete(sess.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get(sess.ID)
	if err != session.ErrNotFound {
		t.Error("Get() should return ErrNotFound after delete")
	}
}

func TestSessionStore_Delete_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	err := store.Delete("nonexistent")
	if err != session.ErrNotFound {
		t.Errorf("Delete() error = %v; want ErrNotFound", err)
	}
}

func TestSessionStore_List(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	s1 := session.NewSession("ex1", map[string]string{}, domain.DefaultPolicy())
	s2 := session.NewSession("ex2", map[string]string{}, domain.DefaultPolicy())
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

func TestSessionStore_ListActive(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	active := session.NewSession("ex1", map[string]string{}, domain.DefaultPolicy())
	completed := session.NewSession("ex2", map[string]string{}, domain.DefaultPolicy())
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

func TestSessionStore_Exists(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())

	if store.Exists(sess.ID) {
		t.Error("Exists() should return false before save")
	}

	store.Save(sess)

	if !store.Exists(sess.ID) {
		t.Error("Exists() should return true after save")
	}
}

func TestSessionStore_SaveRun_GetRun(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(sess)

	run := &session.Run{
		ID:        "run-1",
		SessionID: sess.ID,
		Code:      map[string]string{"main.go": "package main"},
		Result: &session.RunResult{
			FormatOK: true,
			BuildOK:  true,
			TestOK:   true,
			Duration: 100 * time.Millisecond,
		},
		CreatedAt: time.Now(),
	}

	if err := store.SaveRun(run); err != nil {
		t.Fatalf("SaveRun() error = %v", err)
	}

	loaded, err := store.GetRun(sess.ID, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}

	if loaded.ID != run.ID {
		t.Errorf("Run ID = %q; want %q", loaded.ID, run.ID)
	}
	if loaded.SessionID != sess.ID {
		t.Errorf("Run SessionID = %q; want %q", loaded.SessionID, sess.ID)
	}
	if loaded.Result == nil {
		t.Fatal("Run Result is nil")
	}
	if !loaded.Result.TestOK {
		t.Error("Run Result.TestOK should be true")
	}
}

func TestSessionStore_GetRun_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	_, err := store.GetRun("session", "nonexistent")
	if err != session.ErrNotFound {
		t.Errorf("GetRun() error = %v; want ErrNotFound", err)
	}
}

func TestSessionStore_ListRuns(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(sess)

	for _, id := range []string{"run-1", "run-2"} {
		run := &session.Run{ID: id, SessionID: sess.ID, Code: map[string]string{}, CreatedAt: time.Now()}
		store.SaveRun(run)
	}

	ids, err := store.ListRuns(sess.ID)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("ListRuns() returned %d items; want 2", len(ids))
	}
}

func TestSessionStore_SaveIntervention_GetIntervention(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(sess)

	intervention := &session.Intervention{
		ID:        "int-1",
		SessionID: sess.ID,
		Intent:    domain.IntentHint,
		Level:     domain.L1CategoryHint,
		Type:      domain.TypeHint,
		Content:   "Try using a loop",
		CreatedAt: time.Now(),
	}

	if err := store.SaveIntervention(intervention); err != nil {
		t.Fatalf("SaveIntervention() error = %v", err)
	}

	loaded, err := store.GetIntervention(sess.ID, intervention.ID)
	if err != nil {
		t.Fatalf("GetIntervention() error = %v", err)
	}

	if loaded.ID != intervention.ID {
		t.Errorf("Intervention ID = %q; want %q", loaded.ID, intervention.ID)
	}
	if loaded.Content != intervention.Content {
		t.Errorf("Intervention Content = %q; want %q", loaded.Content, intervention.Content)
	}
	if loaded.Level != domain.L1CategoryHint {
		t.Errorf("Intervention Level = %d; want %d", loaded.Level, domain.L1CategoryHint)
	}
}

func TestSessionStore_GetIntervention_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	_, err := store.GetIntervention("session", "nonexistent")
	if err != session.ErrNotFound {
		t.Errorf("GetIntervention() error = %v; want ErrNotFound", err)
	}
}

func TestSessionStore_ListInterventions(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(sess)

	for _, id := range []string{"int-1", "int-2", "int-3"} {
		intervention := &session.Intervention{
			ID:        id,
			SessionID: sess.ID,
			Intent:    domain.IntentHint,
			Level:     domain.L0Clarify,
			Type:      domain.TypeQuestion,
			Content:   "test",
			CreatedAt: time.Now(),
		}
		store.SaveIntervention(intervention)
	}

	ids, err := store.ListInterventions(sess.ID)
	if err != nil {
		t.Fatalf("ListInterventions() error = %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("ListInterventions() returned %d items; want 3", len(ids))
	}
}

func TestSessionStore_CascadeDelete(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{}, domain.DefaultPolicy())
	store.Save(sess)

	// Add a run and an intervention
	run := &session.Run{ID: "run-1", SessionID: sess.ID, Code: map[string]string{}, CreatedAt: time.Now()}
	store.SaveRun(run)

	intervention := &session.Intervention{
		ID: "int-1", SessionID: sess.ID,
		Intent: domain.IntentHint, Level: domain.L0Clarify, Type: domain.TypeQuestion,
		Content: "test", CreatedAt: time.Now(),
	}
	store.SaveIntervention(intervention)

	// Delete session — should cascade
	if err := store.Delete(sess.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify runs are gone
	_, err := store.GetRun(sess.ID, "run-1")
	if err != session.ErrNotFound {
		t.Error("GetRun() should return ErrNotFound after cascade delete")
	}

	// Verify interventions are gone
	_, err = store.GetIntervention(sess.ID, "int-1")
	if err != session.ErrNotFound {
		t.Error("GetIntervention() should return ErrNotFound after cascade delete")
	}
}

func TestSessionStore_Update(t *testing.T) {
	db := openTestDB(t)
	store := NewSessionStore(db)

	sess := session.NewSession("test", map[string]string{"main.go": "v1"}, domain.DefaultPolicy())
	store.Save(sess)

	// Update
	sess.Code["main.go"] = "v2"
	sess.RunCount = 5
	sess.RecordRun()

	if err := store.Save(sess); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	loaded, err := store.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.Code["main.go"] != "v2" {
		t.Errorf("Code[main.go] = %q; want %q", loaded.Code["main.go"], "v2")
	}
	if loaded.RunCount != 6 {
		t.Errorf("RunCount = %d; want 6", loaded.RunCount)
	}
}
