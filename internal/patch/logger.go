package patch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/google/uuid"
)

// LogEntry represents a single patch action in the audit log
type LogEntry struct {
	ID             string             `json:"id"`
	Timestamp      time.Time          `json:"timestamp"`
	Action         LogAction          `json:"action"`
	SessionID      string             `json:"session_id"`
	PatchID        string             `json:"patch_id"`
	InterventionID string             `json:"intervention_id"`
	File           string             `json:"file"`
	Description    string             `json:"description"`
	Diff           string             `json:"diff,omitempty"`
	LinesAdded     int                `json:"lines_added"`
	LinesRemoved   int                `json:"lines_removed"`
	Status         domain.PatchStatus `json:"status"`
}

// LogAction represents the type of action taken on a patch
type LogAction string

const (
	LogActionCreated   LogAction = "created"
	LogActionPreviewed LogAction = "previewed"
	LogActionApplied   LogAction = "applied"
	LogActionRejected  LogAction = "rejected"
	LogActionExpired   LogAction = "expired"
)

// Logger manages the patch audit log
type Logger struct {
	logPath string
	mu      sync.Mutex
	entries []LogEntry
}

// NewLogger creates a new patch logger
func NewLogger(logDir string) (*Logger, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}

	logPath := filepath.Join(logDir, "patches.log")
	l := &Logger{
		logPath: logPath,
		entries: make([]LogEntry, 0),
	}

	// Load existing entries
	if err := l.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	return l, nil
}

// Log records a patch action
func (l *Logger) Log(action LogAction, patch *domain.Patch) error {
	entry := LogEntry{
		ID:             uuid.New().String(),
		Timestamp:      time.Now(),
		Action:         action,
		SessionID:      patch.SessionID.String(),
		PatchID:        patch.ID.String(),
		InterventionID: patch.InterventionID.String(),
		File:           patch.File,
		Description:    patch.Description,
		Status:         patch.Status,
		LinesAdded:     countLines(patch.Diff, "+"),
		LinesRemoved:   countLines(patch.Diff, "-"),
	}

	// Only include diff for applied patches (for potential rollback)
	if action == LogActionApplied {
		entry.Diff = patch.Diff
	}

	l.mu.Lock()
	l.entries = append(l.entries, entry)
	err := l.save()
	l.mu.Unlock()

	return err
}

// GetEntries returns all log entries
func (l *Logger) GetEntries() []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Return a copy
	entries := make([]LogEntry, len(l.entries))
	copy(entries, l.entries)
	return entries
}

// GetRecentEntries returns the most recent n entries
func (l *Logger) GetRecentEntries(n int) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 || n > len(l.entries) {
		n = len(l.entries)
	}

	// Get last n entries in reverse order (most recent first)
	entries := make([]LogEntry, n)
	for i := 0; i < n; i++ {
		entries[i] = l.entries[len(l.entries)-1-i]
	}
	return entries
}

// GetSessionEntries returns all entries for a specific session
func (l *Logger) GetSessionEntries(sessionID string) []LogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()

	var entries []LogEntry
	for _, e := range l.entries {
		if e.SessionID == sessionID {
			entries = append(entries, e)
		}
	}
	return entries
}

// GetStats returns summary statistics
func (l *Logger) GetStats() LogStats {
	l.mu.Lock()
	defer l.mu.Unlock()

	stats := LogStats{
		TotalPatches: 0,
		Applied:      0,
		Rejected:     0,
		Expired:      0,
		TotalAdded:   0,
		TotalRemoved: 0,
	}

	// Track unique patches
	patchActions := make(map[string]LogAction)
	for _, e := range l.entries {
		// Track final status per patch
		if e.Action == LogActionApplied || e.Action == LogActionRejected || e.Action == LogActionExpired {
			patchActions[e.PatchID] = e.Action
		}
		if e.Action == LogActionCreated {
			if _, exists := patchActions[e.PatchID]; !exists {
				patchActions[e.PatchID] = e.Action
			}
		}

		// Count lines only for applied patches
		if e.Action == LogActionApplied {
			stats.TotalAdded += e.LinesAdded
			stats.TotalRemoved += e.LinesRemoved
		}
	}

	stats.TotalPatches = len(patchActions)
	for _, action := range patchActions {
		switch action {
		case LogActionApplied:
			stats.Applied++
		case LogActionRejected:
			stats.Rejected++
		case LogActionExpired:
			stats.Expired++
		}
	}

	return stats
}

// LogStats contains summary statistics
type LogStats struct {
	TotalPatches int `json:"total_patches"`
	Applied      int `json:"applied"`
	Rejected     int `json:"rejected"`
	Expired      int `json:"expired"`
	TotalAdded   int `json:"total_lines_added"`
	TotalRemoved int `json:"total_lines_removed"`
}

func (l *Logger) load() error {
	data, err := os.ReadFile(l.logPath)
	if err != nil {
		return err
	}

	// Parse JSONL format (one JSON object per line)
	var entries []LogEntry
	decoder := json.NewDecoder(newLineReader(data))
	for decoder.More() {
		var entry LogEntry
		if err := decoder.Decode(&entry); err != nil {
			continue // Skip malformed entries
		}
		entries = append(entries, entry)
	}

	l.entries = entries
	return nil
}

func (l *Logger) save() error {
	// Append the last entry in JSONL format
	if len(l.entries) == 0 {
		return nil
	}

	entry := l.entries[len(l.entries)-1]
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(l.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(data, '\n'))
	return err
}

func countLines(diff, prefix string) int {
	count := 0
	for i := 0; i < len(diff); i++ {
		if i == 0 || diff[i-1] == '\n' {
			// Check if line starts with prefix but not with prefix repeated (e.g., --- or +++)
			if len(diff) > i && string(diff[i]) == prefix {
				if len(diff) > i+1 && string(diff[i+1]) == prefix {
					continue // Skip --- or +++
				}
				count++
			}
		}
	}
	return count
}

// lineReader wraps bytes to allow reading line by line
type lineReader struct {
	data []byte
	pos  int
}

func newLineReader(data []byte) *lineReader {
	return &lineReader{data: data}
}

func (r *lineReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, os.ErrClosed
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
