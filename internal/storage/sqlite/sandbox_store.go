package sqlite

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/sandbox"
)

// SandboxStore implements sandbox persistence backed by SQLite.
type SandboxStore struct {
	db *DB
}

// NewSandboxStore creates a new SQLite-backed sandbox store.
func NewSandboxStore(db *DB) *SandboxStore {
	return &SandboxStore{db: db}
}

// Save persists a sandbox (insert or update).
func (s *SandboxStore) Save(sb *sandbox.Sandbox) error {
	_, err := s.db.Exec(`
		INSERT INTO sandboxes (id, session_id, container_id, language, image, status,
			memory_mb, cpu_limit, network_off, last_exec_at, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			container_id=excluded.container_id, status=excluded.status,
			last_exec_at=excluded.last_exec_at, expires_at=excluded.expires_at,
			updated_at=excluded.updated_at`,
		sb.ID, sb.SessionID, sb.ContainerID, sb.Language, sb.Image,
		string(sb.Status), sb.MemoryMB, sb.CPULimit, boolToInt(sb.NetworkOff),
		nullTime(sb.LastExecAt), sb.ExpiresAt, sb.CreatedAt, sb.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert sandbox: %w", err)
	}
	return nil
}

// Get retrieves a sandbox by ID.
func (s *SandboxStore) Get(id string) (*sandbox.Sandbox, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, container_id, language, image, status,
			memory_mb, cpu_limit, network_off, last_exec_at, expires_at,
			created_at, updated_at
		FROM sandboxes WHERE id = ?`, id)
	return scanSandbox(row)
}

// GetBySession retrieves the active sandbox for a session.
func (s *SandboxStore) GetBySession(sessionID string) (*sandbox.Sandbox, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, container_id, language, image, status,
			memory_mb, cpu_limit, network_off, last_exec_at, expires_at,
			created_at, updated_at
		FROM sandboxes WHERE session_id = ? AND status NOT IN ('destroyed')
		ORDER BY created_at DESC LIMIT 1`, sessionID)
	return scanSandbox(row)
}

// Delete removes a sandbox by ID.
func (s *SandboxStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM sandboxes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete sandbox: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return sandbox.ErrSandboxNotFound
	}
	return nil
}

// ListActive returns all non-destroyed sandboxes.
func (s *SandboxStore) ListActive() ([]*sandbox.Sandbox, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, container_id, language, image, status,
			memory_mb, cpu_limit, network_off, last_exec_at, expires_at,
			created_at, updated_at
		FROM sandboxes WHERE status NOT IN ('destroyed')
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list active sandboxes: %w", err)
	}
	defer rows.Close()

	var sandboxes []*sandbox.Sandbox
	for rows.Next() {
		sb, err := scanSandboxRow(rows)
		if err != nil {
			return nil, err
		}
		sandboxes = append(sandboxes, sb)
	}
	return sandboxes, rows.Err()
}

// ListExpired returns all active sandboxes past their expiry time.
func (s *SandboxStore) ListExpired() ([]*sandbox.Sandbox, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, container_id, language, image, status,
			memory_mb, cpu_limit, network_off, last_exec_at, expires_at,
			created_at, updated_at
		FROM sandboxes WHERE status NOT IN ('destroyed') AND expires_at < ?
		ORDER BY expires_at`, time.Now())
	if err != nil {
		return nil, fmt.Errorf("list expired sandboxes: %w", err)
	}
	defer rows.Close()

	var sandboxes []*sandbox.Sandbox
	for rows.Next() {
		sb, err := scanSandboxRow(rows)
		if err != nil {
			return nil, err
		}
		sandboxes = append(sandboxes, sb)
	}
	return sandboxes, rows.Err()
}

func scanSandbox(row *sql.Row) (*sandbox.Sandbox, error) {
	var sb sandbox.Sandbox
	var status string
	var networkOff int
	var lastExecAt sql.NullTime

	err := row.Scan(
		&sb.ID, &sb.SessionID, &sb.ContainerID, &sb.Language, &sb.Image,
		&status, &sb.MemoryMB, &sb.CPULimit, &networkOff,
		&lastExecAt, &sb.ExpiresAt, &sb.CreatedAt, &sb.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sandbox.ErrSandboxNotFound
		}
		return nil, fmt.Errorf("scan sandbox: %w", err)
	}

	sb.Status = sandbox.Status(status)
	sb.NetworkOff = networkOff != 0
	if lastExecAt.Valid {
		sb.LastExecAt = &lastExecAt.Time
	}

	return &sb, nil
}

func scanSandboxRow(rows *sql.Rows) (*sandbox.Sandbox, error) {
	var sb sandbox.Sandbox
	var status string
	var networkOff int
	var lastExecAt sql.NullTime

	err := rows.Scan(
		&sb.ID, &sb.SessionID, &sb.ContainerID, &sb.Language, &sb.Image,
		&status, &sb.MemoryMB, &sb.CPULimit, &networkOff,
		&lastExecAt, &sb.ExpiresAt, &sb.CreatedAt, &sb.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan sandbox row: %w", err)
	}

	sb.Status = sandbox.Status(status)
	sb.NetworkOff = networkOff != 0
	if lastExecAt.Valid {
		sb.LastExecAt = &lastExecAt.Time
	}

	return &sb, nil
}

// Ensure SandboxStore implements sandbox.Store.
var _ sandbox.Store = (*SandboxStore)(nil)
