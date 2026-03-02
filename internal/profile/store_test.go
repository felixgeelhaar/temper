package profile

import "testing"

func TestStore_Save_Get_DefaultLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	profile, err := store.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault() error = %v", err)
	}

	profile.TotalSessions = 2
	if err := store.Save(profile); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Get(profile.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.TotalSessions != 2 {
		t.Errorf("TotalSessions = %d; want 2", loaded.TotalSessions)
	}
}

func TestStore_Get_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	_, err := store.Get("missing")
	if err != ErrNotFound {
		t.Errorf("Get() error = %v; want ErrNotFound", err)
	}
}

func TestStore_Delete_List_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, _ := NewStore(tmpDir)

	profile, _ := store.GetDefault()

	if !store.Exists(profile.ID) {
		t.Error("Exists() should return true for default profile")
	}

	ids, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("List() = %d; want 1", len(ids))
	}

	if err := store.Delete(profile.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if store.Exists(profile.ID) {
		t.Error("Exists() should return false after delete")
	}
}
