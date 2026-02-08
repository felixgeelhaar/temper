package sqlite

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/profile"
)

func TestProfileStore_Save_Get(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	p := &profile.StoredProfile{
		ID: "test-profile",
		TopicSkills: map[string]profile.StoredSkill{
			"go/basics": {Level: 0.5, Attempts: 10, LastSeen: time.Now(), Confidence: 0.8},
		},
		TotalExercises:    5,
		TotalSessions:     10,
		CompletedSessions: 7,
		TotalRuns:         50,
		HintRequests:      15,
		AvgTimeToGreenMs:  30000,
		ExerciseHistory: []profile.ExerciseAttempt{
			{ExerciseID: "go-v1/basics/hello-world", SessionID: "s1", StartedAt: time.Now(), Success: true},
		},
		ErrorPatterns:       map[string]int{"undefined variable": 5, "type mismatch": 3},
		HintDependencyTrend: []profile.HintDependencyPoint{{Timestamp: time.Now(), Dependency: 0.3, RunWindow: 10}},
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := store.Save(p); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Get("test-profile")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != p.ID {
		t.Errorf("ID = %q; want %q", loaded.ID, p.ID)
	}
	if loaded.TotalExercises != 5 {
		t.Errorf("TotalExercises = %d; want 5", loaded.TotalExercises)
	}
	if loaded.TotalRuns != 50 {
		t.Errorf("TotalRuns = %d; want 50", loaded.TotalRuns)
	}

	skill, ok := loaded.TopicSkills["go/basics"]
	if !ok {
		t.Fatal("TopicSkills[go/basics] not found")
	}
	if skill.Level != 0.5 {
		t.Errorf("TopicSkills[go/basics].Level = %f; want 0.5", skill.Level)
	}

	if len(loaded.ExerciseHistory) != 1 {
		t.Errorf("ExerciseHistory length = %d; want 1", len(loaded.ExerciseHistory))
	}
	if loaded.ErrorPatterns["undefined variable"] != 5 {
		t.Errorf("ErrorPatterns[undefined variable] = %d; want 5", loaded.ErrorPatterns["undefined variable"])
	}
}

func TestProfileStore_Get_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	_, err := store.Get("nonexistent")
	if err != profile.ErrNotFound {
		t.Errorf("Get() error = %v; want ErrNotFound", err)
	}
}

func TestProfileStore_GetDefault(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	// First call creates the default profile
	p, err := store.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}
	if p.ID != "default" {
		t.Errorf("ID = %q; want %q", p.ID, "default")
	}

	// Second call returns the same profile
	p2, err := store.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault() second call error = %v", err)
	}
	if p2.ID != "default" {
		t.Errorf("ID = %q; want %q", p2.ID, "default")
	}
}

func TestProfileStore_Delete(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	p := &profile.StoredProfile{
		ID:                  "to-delete",
		TopicSkills:         map[string]profile.StoredSkill{},
		ErrorPatterns:       map[string]int{},
		ExerciseHistory:     []profile.ExerciseAttempt{},
		HintDependencyTrend: []profile.HintDependencyPoint{},
		CreatedAt:           time.Now(),
	}
	store.Save(p)

	if err := store.Delete("to-delete"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get("to-delete")
	if err != profile.ErrNotFound {
		t.Error("Get() should return ErrNotFound after delete")
	}
}

func TestProfileStore_Delete_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	err := store.Delete("nonexistent")
	if err != profile.ErrNotFound {
		t.Errorf("Delete() error = %v; want ErrNotFound", err)
	}
}

func TestProfileStore_List(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	for _, id := range []string{"p1", "p2", "p3"} {
		store.Save(&profile.StoredProfile{
			ID:                  id,
			TopicSkills:         map[string]profile.StoredSkill{},
			ErrorPatterns:       map[string]int{},
			ExerciseHistory:     []profile.ExerciseAttempt{},
			HintDependencyTrend: []profile.HintDependencyPoint{},
			CreatedAt:           time.Now(),
		})
	}

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("List() returned %d items; want 3", len(ids))
	}
}

func TestProfileStore_Exists(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	if store.Exists("not-there") {
		t.Error("Exists() should return false for missing profile")
	}

	store.Save(&profile.StoredProfile{
		ID:                  "exists",
		TopicSkills:         map[string]profile.StoredSkill{},
		ErrorPatterns:       map[string]int{},
		ExerciseHistory:     []profile.ExerciseAttempt{},
		HintDependencyTrend: []profile.HintDependencyPoint{},
		CreatedAt:           time.Now(),
	})

	if !store.Exists("exists") {
		t.Error("Exists() should return true after save")
	}
}

func TestProfileStore_Update(t *testing.T) {
	db := openTestDB(t)
	store := NewProfileStore(db)

	p := &profile.StoredProfile{
		ID:                  "update-test",
		TopicSkills:         map[string]profile.StoredSkill{},
		TotalRuns:           10,
		ErrorPatterns:       map[string]int{},
		ExerciseHistory:     []profile.ExerciseAttempt{},
		HintDependencyTrend: []profile.HintDependencyPoint{},
		CreatedAt:           time.Now(),
	}
	store.Save(p)

	// Update
	p.TotalRuns = 20
	p.TopicSkills["go/testing"] = profile.StoredSkill{Level: 0.7, Attempts: 5}

	if err := store.Save(p); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	loaded, err := store.Get("update-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.TotalRuns != 20 {
		t.Errorf("TotalRuns = %d; want 20", loaded.TotalRuns)
	}
	if loaded.TopicSkills["go/testing"].Level != 0.7 {
		t.Errorf("TopicSkills[go/testing].Level = %f; want 0.7", loaded.TopicSkills["go/testing"].Level)
	}
}
