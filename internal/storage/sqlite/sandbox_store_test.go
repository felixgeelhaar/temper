package sqlite

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/sandbox"
	"github.com/felixgeelhaar/temper/internal/session"
)

func createTestSession(t *testing.T, db *DB, id string) {
	t.Helper()
	sessStore := NewSessionStore(db)
	sess := &session.Session{
		ID:        id,
		Status:    session.StatusActive,
		Intent:    session.IntentTraining,
		Code:      map[string]string{"main.go": "package main"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if err := sessStore.Save(sess); err != nil {
		t.Fatalf("create test session: %v", err)
	}
}

func TestSandboxStore_Save_Get(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)
	createTestSession(t, db, "sess-1")

	now := time.Now()
	sb := &sandbox.Sandbox{
		ID:          "sb-1",
		SessionID:   "sess-1",
		ContainerID: "abc123def456",
		Language:    "go",
		Image:       "golang:1.23-alpine",
		Status:      sandbox.StatusReady,
		MemoryMB:    256,
		CPULimit:    0.5,
		NetworkOff:  true,
		ExpiresAt:   now.Add(30 * time.Minute),
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := store.Save(sb); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := store.Get("sb-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if loaded.ID != "sb-1" {
		t.Errorf("ID = %q; want %q", loaded.ID, "sb-1")
	}
	if loaded.SessionID != "sess-1" {
		t.Errorf("SessionID = %q; want %q", loaded.SessionID, "sess-1")
	}
	if loaded.ContainerID != "abc123def456" {
		t.Errorf("ContainerID = %q; want %q", loaded.ContainerID, "abc123def456")
	}
	if loaded.Status != sandbox.StatusReady {
		t.Errorf("Status = %q; want %q", loaded.Status, sandbox.StatusReady)
	}
	if loaded.MemoryMB != 256 {
		t.Errorf("MemoryMB = %d; want 256", loaded.MemoryMB)
	}
	if !loaded.NetworkOff {
		t.Error("NetworkOff = false; want true")
	}
}

func TestSandboxStore_Get_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)

	_, err := store.Get("nonexistent")
	if err != sandbox.ErrSandboxNotFound {
		t.Errorf("Get() error = %v; want ErrSandboxNotFound", err)
	}
}

func TestSandboxStore_GetBySession(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)
	createTestSession(t, db, "sess-2")

	now := time.Now()
	sb := &sandbox.Sandbox{
		ID:          "sb-2",
		SessionID:   "sess-2",
		ContainerID: "xyz789",
		Language:    "go",
		Image:       "golang:1.23-alpine",
		Status:      sandbox.StatusReady,
		MemoryMB:    256,
		CPULimit:    0.5,
		ExpiresAt:   now.Add(30 * time.Minute),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store.Save(sb)

	loaded, err := store.GetBySession("sess-2")
	if err != nil {
		t.Fatalf("GetBySession() error = %v", err)
	}
	if loaded.ID != "sb-2" {
		t.Errorf("ID = %q; want %q", loaded.ID, "sb-2")
	}
}

func TestSandboxStore_GetBySession_NotFound(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)

	_, err := store.GetBySession("no-such-session")
	if err != sandbox.ErrSandboxNotFound {
		t.Errorf("GetBySession() error = %v; want ErrSandboxNotFound", err)
	}
}

func TestSandboxStore_Delete(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)
	createTestSession(t, db, "sess-3")

	now := time.Now()
	sb := &sandbox.Sandbox{
		ID:        "sb-del",
		SessionID: "sess-3",
		Language:  "go",
		Image:     "golang:1.23-alpine",
		Status:    sandbox.StatusReady,
		ExpiresAt: now.Add(30 * time.Minute),
		CreatedAt: now,
		UpdatedAt: now,
	}
	store.Save(sb)

	if err := store.Delete("sb-del"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := store.Get("sb-del")
	if err != sandbox.ErrSandboxNotFound {
		t.Error("Get() should return ErrSandboxNotFound after delete")
	}
}

func TestSandboxStore_ListActive(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)
	createTestSession(t, db, "sess-4")
	createTestSession(t, db, "sess-5")

	now := time.Now()

	// Active sandbox
	store.Save(&sandbox.Sandbox{
		ID: "sb-active", SessionID: "sess-4", Language: "go", Image: "golang:1.23-alpine",
		Status: sandbox.StatusReady, ExpiresAt: now.Add(30 * time.Minute),
		CreatedAt: now, UpdatedAt: now,
	})

	// Destroyed sandbox (should not appear)
	store.Save(&sandbox.Sandbox{
		ID: "sb-destroyed", SessionID: "sess-5", Language: "go", Image: "golang:1.23-alpine",
		Status: sandbox.StatusDestroyed, ExpiresAt: now.Add(30 * time.Minute),
		CreatedAt: now, UpdatedAt: now,
	})

	active, err := store.ListActive()
	if err != nil {
		t.Fatalf("ListActive() error = %v", err)
	}
	if len(active) != 1 {
		t.Errorf("ListActive() returned %d; want 1", len(active))
	}
}

func TestSandboxStore_ListExpired(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)
	createTestSession(t, db, "sess-6")
	createTestSession(t, db, "sess-7")

	now := time.Now()

	// Expired sandbox (expires_at in the past)
	store.Save(&sandbox.Sandbox{
		ID: "sb-expired", SessionID: "sess-6", Language: "go", Image: "golang:1.23-alpine",
		Status: sandbox.StatusReady, ExpiresAt: now.Add(-1 * time.Hour),
		CreatedAt: now, UpdatedAt: now,
	})

	// Not expired
	store.Save(&sandbox.Sandbox{
		ID: "sb-fresh", SessionID: "sess-7", Language: "go", Image: "golang:1.23-alpine",
		Status: sandbox.StatusReady, ExpiresAt: now.Add(30 * time.Minute),
		CreatedAt: now, UpdatedAt: now,
	})

	expired, err := store.ListExpired()
	if err != nil {
		t.Fatalf("ListExpired() error = %v", err)
	}
	if len(expired) != 1 {
		t.Errorf("ListExpired() returned %d; want 1", len(expired))
	}
	if expired[0].ID != "sb-expired" {
		t.Errorf("expired sandbox ID = %q; want %q", expired[0].ID, "sb-expired")
	}
}

func TestSandboxStore_Update(t *testing.T) {
	db := openTestDB(t)
	store := NewSandboxStore(db)
	createTestSession(t, db, "sess-8")

	now := time.Now()
	sb := &sandbox.Sandbox{
		ID:          "sb-update",
		SessionID:   "sess-8",
		ContainerID: "old-container",
		Language:    "go",
		Image:       "golang:1.23-alpine",
		Status:      sandbox.StatusCreating,
		MemoryMB:    256,
		CPULimit:    0.5,
		ExpiresAt:   now.Add(30 * time.Minute),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	store.Save(sb)

	// Update to ready with new container ID
	sb.ContainerID = "new-container"
	sb.Status = sandbox.StatusReady
	sb.UpdatedAt = time.Now()
	if err := store.Save(sb); err != nil {
		t.Fatalf("Save(update) error = %v", err)
	}

	loaded, err := store.Get("sb-update")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.ContainerID != "new-container" {
		t.Errorf("ContainerID = %q; want %q", loaded.ContainerID, "new-container")
	}
	if loaded.Status != sandbox.StatusReady {
		t.Errorf("Status = %q; want %q", loaded.Status, sandbox.StatusReady)
	}
}

func TestSandboxStore_CascadeDelete(t *testing.T) {
	db := openTestDB(t)
	sandboxStore := NewSandboxStore(db)
	sessionStore := NewSessionStore(db)

	createTestSession(t, db, "sess-cascade")

	now := time.Now()
	sandboxStore.Save(&sandbox.Sandbox{
		ID: "sb-cascade", SessionID: "sess-cascade", Language: "go", Image: "golang:1.23-alpine",
		Status: sandbox.StatusReady, ExpiresAt: now.Add(30 * time.Minute),
		CreatedAt: now, UpdatedAt: now,
	})

	// Delete the session — sandbox should cascade
	if err := sessionStore.Delete("sess-cascade"); err != nil {
		t.Fatalf("Delete session error = %v", err)
	}

	_, err := sandboxStore.Get("sb-cascade")
	if err != sandbox.ErrSandboxNotFound {
		t.Errorf("Get() after cascade = %v; want ErrSandboxNotFound", err)
	}
}
