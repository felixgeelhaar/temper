package profile

import (
	"testing"
	"time"
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

	profile := &StoredProfile{
		ID:            "test-profile",
		TopicSkills:   map[string]StoredSkill{},
		ErrorPatterns: map[string]int{},
		CreatedAt:     time.Now(),
	}

	// Save
	if err := store.Save(profile); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Get
	loaded, err := store.Get(profile.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != profile.ID {
		t.Errorf("ID = %q; want %q", loaded.ID, profile.ID)
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

func TestStore_GetDefault_CreatesNew(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	// First call should create
	profile, err := store.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}

	if profile.ID != "default" {
		t.Errorf("ID = %q; want %q", profile.ID, "default")
	}

	// Second call should return same
	profile2, err := store.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault() second call error = %v", err)
	}

	if profile2.ID != profile.ID {
		t.Error("GetDefault() should return same profile on second call")
	}
}

func TestStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	profile := &StoredProfile{ID: "to-delete"}
	store.Save(profile)

	if err := store.Delete(profile.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get(profile.ID)
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

	store.Save(&StoredProfile{ID: "p1"})
	store.Save(&StoredProfile{ID: "p2"})

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if len(ids) != 2 {
		t.Errorf("List() returned %d items; want 2", len(ids))
	}
}

func TestStore_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	profile := &StoredProfile{ID: "test"}

	// Before save
	if store.Exists(profile.ID) {
		t.Error("Exists() should return false before save")
	}

	// After save
	store.Save(profile)
	if !store.Exists(profile.ID) {
		t.Error("Exists() should return true after save")
	}
}

func TestNewID(t *testing.T) {
	id1 := newID()
	id2 := newID()

	if id1 == "" {
		t.Error("newID() returned empty string")
	}
	if id1 == id2 {
		t.Error("newID() should return unique IDs")
	}
}
