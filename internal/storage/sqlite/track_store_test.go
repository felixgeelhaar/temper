package sqlite

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
)

func TestTrackStore_BuiltinSeeds(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	tracks, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(tracks) != 4 {
		t.Fatalf("List() returned %d tracks; want 4 (seeded built-ins)", len(tracks))
	}

	// Verify beginner track
	beginner, err := store.Get("beginner")
	if err != nil {
		t.Fatalf("Get(beginner) error = %v", err)
	}
	if beginner.MaxLevel != domain.L4PartialSolution {
		t.Errorf("beginner.MaxLevel = %d; want %d", beginner.MaxLevel, domain.L4PartialSolution)
	}
	if beginner.CooldownSeconds != 30 {
		t.Errorf("beginner.CooldownSeconds = %d; want 30", beginner.CooldownSeconds)
	}
	if !beginner.PatchingEnabled {
		t.Error("beginner.PatchingEnabled = false; want true")
	}
	if !beginner.AutoProgress.Enabled {
		t.Error("beginner.AutoProgress.Enabled = false; want true")
	}
	if beginner.AutoProgress.PromoteAfterStreak != 5 {
		t.Errorf("beginner.AutoProgress.PromoteAfterStreak = %d; want 5", beginner.AutoProgress.PromoteAfterStreak)
	}
}

func TestTrackStore_Save_Get(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	track := &domain.Track{
		ID:              "custom-test",
		Name:            "Custom Test",
		Description:     "A custom test track.",
		Preset:          "custom",
		MaxLevel:        domain.L3ConstrainedSnippet,
		CooldownSeconds: 45,
		PatchingEnabled: true,
		AutoProgress: domain.AutoProgressRules{
			Enabled:             true,
			PromoteAfterStreak:  10,
			DemoteAfterFailures: 4,
			MinSkillForPromote:  0.6,
		},
	}

	if err := store.Save(track); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Get("custom-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != "custom-test" {
		t.Errorf("ID = %q; want %q", loaded.ID, "custom-test")
	}
	if loaded.Name != "Custom Test" {
		t.Errorf("Name = %q; want %q", loaded.Name, "Custom Test")
	}
	if loaded.MaxLevel != domain.L3ConstrainedSnippet {
		t.Errorf("MaxLevel = %d; want %d", loaded.MaxLevel, domain.L3ConstrainedSnippet)
	}
	if loaded.CooldownSeconds != 45 {
		t.Errorf("CooldownSeconds = %d; want 45", loaded.CooldownSeconds)
	}
	if !loaded.PatchingEnabled {
		t.Error("PatchingEnabled = false; want true")
	}
	if loaded.AutoProgress.PromoteAfterStreak != 10 {
		t.Errorf("AutoProgress.PromoteAfterStreak = %d; want 10", loaded.AutoProgress.PromoteAfterStreak)
	}
	if loaded.AutoProgress.MinSkillForPromote != 0.6 {
		t.Errorf("AutoProgress.MinSkillForPromote = %f; want 0.6", loaded.AutoProgress.MinSkillForPromote)
	}
}

func TestTrackStore_Get_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	_, err := store.Get("nonexistent")
	if err != ErrTrackNotFound {
		t.Errorf("Get() error = %v; want ErrTrackNotFound", err)
	}
}

func TestTrackStore_Delete(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	track := &domain.Track{
		ID:     "to-delete",
		Name:   "Delete Me",
		Preset: "custom",
	}
	store.Save(track)

	if err := store.Delete("to-delete"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get("to-delete")
	if err != ErrTrackNotFound {
		t.Error("Get() should return ErrTrackNotFound after delete")
	}
}

func TestTrackStore_Delete_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	err := store.Delete("nonexistent")
	if err != ErrTrackNotFound {
		t.Errorf("Delete() error = %v; want ErrTrackNotFound", err)
	}
}

func TestTrackStore_List(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	// Should include 4 built-in + any custom
	store.Save(&domain.Track{ID: "custom-1", Name: "C1", Preset: "custom"})
	store.Save(&domain.Track{ID: "custom-2", Name: "C2", Preset: "custom"})

	tracks, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(tracks) != 6 { // 4 built-in + 2 custom
		t.Errorf("List() returned %d items; want 6", len(tracks))
	}
}

func TestTrackStore_ListByPreset(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	store.Save(&domain.Track{ID: "custom-a", Name: "A", Preset: "custom"})
	store.Save(&domain.Track{ID: "custom-b", Name: "B", Preset: "custom"})

	tracks, err := store.ListByPreset("custom")
	if err != nil {
		t.Fatalf("ListByPreset() error = %v", err)
	}
	if len(tracks) != 2 {
		t.Errorf("ListByPreset(custom) returned %d items; want 2", len(tracks))
	}

	// Built-in presets
	begTracks, err := store.ListByPreset("beginner")
	if err != nil {
		t.Fatalf("ListByPreset(beginner) error = %v", err)
	}
	if len(begTracks) != 1 {
		t.Errorf("ListByPreset(beginner) returned %d items; want 1", len(begTracks))
	}
}

func TestTrackStore_Exists(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	if !store.Exists("beginner") {
		t.Error("Exists(beginner) = false; want true (seeded)")
	}
	if store.Exists("nonexistent") {
		t.Error("Exists(nonexistent) = true; want false")
	}
}

func TestTrackStore_Update(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	track := &domain.Track{
		ID:              "update-test",
		Name:            "Original",
		Preset:          "custom",
		MaxLevel:        domain.L2LocationConcept,
		CooldownSeconds: 60,
	}
	store.Save(track)

	// Update
	track.Name = "Updated"
	track.MaxLevel = domain.L4PartialSolution
	track.CooldownSeconds = 30

	if err := store.Save(track); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	loaded, err := store.Get("update-test")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Name != "Updated" {
		t.Errorf("Name = %q; want %q", loaded.Name, "Updated")
	}
	if loaded.MaxLevel != domain.L4PartialSolution {
		t.Errorf("MaxLevel = %d; want %d", loaded.MaxLevel, domain.L4PartialSolution)
	}
	if loaded.CooldownSeconds != 30 {
		t.Errorf("CooldownSeconds = %d; want 30", loaded.CooldownSeconds)
	}
}

func TestTrackStore_ToPolicy(t *testing.T) {
	db := openTestDB(t)
	store := NewTrackStore(db)

	track, err := store.Get("standard")
	if err != nil {
		t.Fatalf("Get(standard) error = %v", err)
	}

	policy := track.ToPolicy()
	if policy.MaxLevel != domain.L3ConstrainedSnippet {
		t.Errorf("policy.MaxLevel = %d; want %d", policy.MaxLevel, domain.L3ConstrainedSnippet)
	}
	if policy.CooldownSeconds != 60 {
		t.Errorf("policy.CooldownSeconds = %d; want 60", policy.CooldownSeconds)
	}
	if policy.Track != "standard" {
		t.Errorf("policy.Track = %q; want %q", policy.Track, "standard")
	}
}
