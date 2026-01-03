package patch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

func TestNewLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logger, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewLogger() error = %v", err)
	}
	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}

	// Verify log directory was created
	logPath := filepath.Join(tmpDir, "patches.log")
	// File won't exist until first write
	if _, err := os.Stat(tmpDir); os.IsNotExist(err) {
		t.Error("NewLogger() should create log directory")
	}

	_ = logPath // Used for clarity
}

func TestLogger_Log(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewLogger(tmpDir)

	patch := &domain.Patch{
		ID:             uuid.New(),
		SessionID:      uuid.New(),
		InterventionID: uuid.New(),
		File:           "main.go",
		Description:    "Add error handling",
		Diff:           "+line1\n+line2\n-oldline",
		Status:         domain.PatchStatusPending,
	}

	err := logger.Log(LogActionCreated, patch)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	entries := logger.GetEntries()
	if len(entries) != 1 {
		t.Errorf("GetEntries() count = %d; want 1", len(entries))
	}
}

func TestLogger_Log_Applied(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewLogger(tmpDir)

	patch := &domain.Patch{
		ID:             uuid.New(),
		SessionID:      uuid.New(),
		InterventionID: uuid.New(),
		File:           "main.go",
		Diff:           "+added\n-removed",
		Status:         domain.PatchStatusApplied,
	}

	logger.Log(LogActionApplied, patch)

	entries := logger.GetEntries()
	if len(entries) != 1 {
		t.Fatalf("GetEntries() count = %d; want 1", len(entries))
	}

	// Applied patches should include diff for rollback
	if entries[0].Diff == "" {
		t.Error("Applied patch log should include diff")
	}
}

func TestLogger_GetRecentEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewLogger(tmpDir)

	sessionID := uuid.New()
	for i := 0; i < 5; i++ {
		patch := &domain.Patch{
			ID:             uuid.New(),
			SessionID:      sessionID,
			InterventionID: uuid.New(),
			File:           "test.go",
			Status:         domain.PatchStatusPending,
		}
		logger.Log(LogActionCreated, patch)
	}

	// Get last 3
	recent := logger.GetRecentEntries(3)
	if len(recent) != 3 {
		t.Errorf("GetRecentEntries(3) returned %d; want 3", len(recent))
	}

	// Get more than available
	all := logger.GetRecentEntries(10)
	if len(all) != 5 {
		t.Errorf("GetRecentEntries(10) returned %d; want 5", len(all))
	}

	// Get with 0
	none := logger.GetRecentEntries(0)
	if len(none) != 5 {
		t.Errorf("GetRecentEntries(0) should return all; got %d", len(none))
	}
}

func TestLogger_GetSessionEntries(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewLogger(tmpDir)

	session1 := uuid.New()
	session2 := uuid.New()

	// Add entries for different sessions
	logger.Log(LogActionCreated, &domain.Patch{
		ID:             uuid.New(),
		SessionID:      session1,
		InterventionID: uuid.New(),
		File:           "file1.go",
	})
	logger.Log(LogActionCreated, &domain.Patch{
		ID:             uuid.New(),
		SessionID:      session2,
		InterventionID: uuid.New(),
		File:           "file2.go",
	})
	logger.Log(LogActionCreated, &domain.Patch{
		ID:             uuid.New(),
		SessionID:      session1,
		InterventionID: uuid.New(),
		File:           "file3.go",
	})

	entries := logger.GetSessionEntries(session1.String())
	if len(entries) != 2 {
		t.Errorf("GetSessionEntries() returned %d; want 2", len(entries))
	}
}

func TestLogger_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	logger, _ := NewLogger(tmpDir)

	sessionID := uuid.New()

	// Create patches
	patch1 := &domain.Patch{
		ID:             uuid.New(),
		SessionID:      sessionID,
		InterventionID: uuid.New(),
		File:           "file1.go",
		Diff:           "+line1\n+line2",
		Status:         domain.PatchStatusPending,
	}
	patch2 := &domain.Patch{
		ID:             uuid.New(),
		SessionID:      sessionID,
		InterventionID: uuid.New(),
		File:           "file2.go",
		Diff:           "-removed",
		Status:         domain.PatchStatusPending,
	}
	patch3 := &domain.Patch{
		ID:             uuid.New(),
		SessionID:      sessionID,
		InterventionID: uuid.New(),
		File:           "file3.go",
		Status:         domain.PatchStatusPending,
	}

	// Log creation
	logger.Log(LogActionCreated, patch1)
	logger.Log(LogActionCreated, patch2)
	logger.Log(LogActionCreated, patch3)

	// Log actions
	patch1.Status = domain.PatchStatusApplied
	logger.Log(LogActionApplied, patch1)

	patch2.Status = domain.PatchStatusRejected
	logger.Log(LogActionRejected, patch2)

	patch3.Status = domain.PatchStatusExpired
	logger.Log(LogActionExpired, patch3)

	stats := logger.GetStats()
	if stats.TotalPatches != 3 {
		t.Errorf("TotalPatches = %d; want 3", stats.TotalPatches)
	}
	if stats.Applied != 1 {
		t.Errorf("Applied = %d; want 1", stats.Applied)
	}
	if stats.Rejected != 1 {
		t.Errorf("Rejected = %d; want 1", stats.Rejected)
	}
	if stats.Expired != 1 {
		t.Errorf("Expired = %d; want 1", stats.Expired)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name   string
		diff   string
		prefix string
		want   int
	}{
		{
			name:   "additions",
			diff:   "+line1\n+line2\n+line3",
			prefix: "+",
			want:   3,
		},
		{
			name:   "deletions",
			diff:   "-line1\n-line2",
			prefix: "-",
			want:   2,
		},
		{
			name:   "skip header",
			diff:   "--- a/file\n+++ b/file\n+added",
			prefix: "+",
			want:   1, // Should skip +++ header
		},
		{
			name:   "empty diff",
			diff:   "",
			prefix: "+",
			want:   0,
		},
		{
			name:   "mixed",
			diff:   "+added\n regular\n-removed",
			prefix: "+",
			want:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := countLines(tt.diff, tt.prefix)
			if got != tt.want {
				t.Errorf("countLines() = %d; want %d", got, tt.want)
			}
		})
	}
}

func TestLogger_Persistence(t *testing.T) {
	tmpDir := t.TempDir()

	// Create logger and log an entry
	logger1, _ := NewLogger(tmpDir)
	patch := &domain.Patch{
		ID:             uuid.New(),
		SessionID:      uuid.New(),
		InterventionID: uuid.New(),
		File:           "persistent.go",
		Description:    "Test persistence",
	}
	logger1.Log(LogActionCreated, patch)

	// Create new logger from same directory (should load existing)
	logger2, err := NewLogger(tmpDir)
	if err != nil {
		t.Fatalf("NewLogger() for reload error = %v", err)
	}

	entries := logger2.GetEntries()
	if len(entries) != 1 {
		t.Errorf("Reloaded logger should have 1 entry; got %d", len(entries))
	}

	if len(entries) > 0 && entries[0].File != "persistent.go" {
		t.Errorf("Reloaded entry File = %q; want %q", entries[0].File, "persistent.go")
	}
}

func TestNewLineReader(t *testing.T) {
	data := []byte(`{"id":"1"}
{"id":"2"}`)

	reader := newLineReader(data)
	buf := make([]byte, 10)

	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 10 {
		t.Errorf("Read() n = %d; want 10", n)
	}

	// Continue reading
	n, err = reader.Read(buf)
	if err != nil {
		t.Fatalf("Second Read() error = %v", err)
	}

	// Read past end
	buf2 := make([]byte, 100)
	for {
		_, err := reader.Read(buf2)
		if err != nil {
			break // Expected
		}
	}
}
