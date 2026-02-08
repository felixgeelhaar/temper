package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
)

// SessionStore implements session persistence backed by SQLite.
type SessionStore struct {
	db *DB
}

// NewSessionStore creates a new SQLite-backed session store.
func NewSessionStore(db *DB) *SessionStore {
	return &SessionStore{db: db}
}

// Save persists a session (insert or update).
func (s *SessionStore) Save(sess *session.Session) error {
	code, err := json.Marshal(sess.Code)
	if err != nil {
		return fmt.Errorf("marshal code: %w", err)
	}
	policy, err := json.Marshal(sess.Policy)
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}
	authoringDocs, err := json.Marshal(sess.AuthoringDocs)
	if err != nil {
		return fmt.Errorf("marshal authoring_docs: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO sessions (id, exercise_id, intent, spec_path, status, code, policy,
			authoring_docs, authoring_section,
			run_count, hint_count, last_run_at, last_intervention_at,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			exercise_id=excluded.exercise_id, intent=excluded.intent,
			spec_path=excluded.spec_path, status=excluded.status,
			code=excluded.code, policy=excluded.policy,
			authoring_docs=excluded.authoring_docs, authoring_section=excluded.authoring_section,
			run_count=excluded.run_count, hint_count=excluded.hint_count,
			last_run_at=excluded.last_run_at, last_intervention_at=excluded.last_intervention_at,
			updated_at=excluded.updated_at`,
		sess.ID, sess.ExerciseID, string(sess.Intent), sess.SpecPath,
		string(sess.Status), string(code), string(policy),
		string(authoringDocs), sess.AuthoringSection,
		sess.RunCount, sess.HintCount,
		nullTime(sess.LastRunAt), nullTime(sess.LastInterventionAt),
		sess.CreatedAt, sess.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert session: %w", err)
	}
	return nil
}

// Get retrieves a session by ID.
func (s *SessionStore) Get(id string) (*session.Session, error) {
	row := s.db.QueryRow(`
		SELECT id, exercise_id, intent, spec_path, status, code, policy,
			authoring_docs, authoring_section,
			run_count, hint_count, last_run_at, last_intervention_at,
			created_at, updated_at
		FROM sessions WHERE id = ?`, id)
	return scanSession(row)
}

// Delete removes a session and its cascaded runs/interventions.
func (s *SessionStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return session.ErrNotFound
	}
	return nil
}

// List returns all session IDs.
func (s *SessionStore) List() ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM sessions ORDER BY created_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan session id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ListActive returns all active sessions.
func (s *SessionStore) ListActive() ([]*session.Session, error) {
	rows, err := s.db.Query(`
		SELECT id, exercise_id, intent, spec_path, status, code, policy,
			authoring_docs, authoring_section,
			run_count, hint_count, last_run_at, last_intervention_at,
			created_at, updated_at
		FROM sessions WHERE status = 'active' ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*session.Session
	for rows.Next() {
		sess, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

// Exists checks if a session exists.
func (s *SessionStore) Exists(id string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", id).Scan(&count)
	return count > 0
}

// SaveRun persists a run within a session.
func (s *SessionStore) SaveRun(run *session.Run) error {
	code, err := json.Marshal(run.Code)
	if err != nil {
		return fmt.Errorf("marshal run code: %w", err)
	}

	var result []byte
	if run.Result != nil {
		result, err = json.Marshal(run.Result)
		if err != nil {
			return fmt.Errorf("marshal run result: %w", err)
		}
	}

	_, err = s.db.Exec(`
		INSERT INTO runs (id, session_id, code, result, created_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			code=excluded.code, result=excluded.result`,
		run.ID, run.SessionID, string(code), nullString(result), run.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert run: %w", err)
	}
	return nil
}

// GetRun retrieves a run by ID.
func (s *SessionStore) GetRun(sessionID, runID string) (*session.Run, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, code, result, created_at
		FROM runs WHERE id = ? AND session_id = ?`, runID, sessionID)

	var run session.Run
	var codeJSON, resultJSON string
	var resultNull sql.NullString

	if err := row.Scan(&run.ID, &run.SessionID, &codeJSON, &resultNull, &run.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrNotFound
		}
		return nil, fmt.Errorf("scan run: %w", err)
	}

	if err := json.Unmarshal([]byte(codeJSON), &run.Code); err != nil {
		return nil, fmt.Errorf("unmarshal run code: %w", err)
	}
	if resultNull.Valid {
		resultJSON = resultNull.String
		run.Result = &session.RunResult{}
		if err := json.Unmarshal([]byte(resultJSON), run.Result); err != nil {
			return nil, fmt.Errorf("unmarshal run result: %w", err)
		}
	}
	return &run, nil
}

// ListRuns returns all run IDs for a session.
func (s *SessionStore) ListRuns(sessionID string) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM runs WHERE session_id = ? ORDER BY created_at", sessionID)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan run id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// SaveIntervention persists an intervention within a session.
func (s *SessionStore) SaveIntervention(intervention *session.Intervention) error {
	var runID *string
	if intervention.RunID != nil {
		runID = intervention.RunID
	}

	_, err := s.db.Exec(`
		INSERT INTO interventions (id, session_id, run_id, intent, level, type, content, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content=excluded.content`,
		intervention.ID, intervention.SessionID, runID,
		string(intervention.Intent), int(intervention.Level),
		string(intervention.Type), intervention.Content, intervention.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert intervention: %w", err)
	}
	return nil
}

// GetIntervention retrieves an intervention by ID.
func (s *SessionStore) GetIntervention(sessionID, interventionID string) (*session.Intervention, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, run_id, intent, level, type, content, created_at
		FROM interventions WHERE id = ? AND session_id = ?`, interventionID, sessionID)

	var intervention session.Intervention
	var runID sql.NullString
	var level int

	if err := row.Scan(
		&intervention.ID, &intervention.SessionID, &runID,
		&intervention.Intent, &level,
		&intervention.Type, &intervention.Content, &intervention.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrNotFound
		}
		return nil, fmt.Errorf("scan intervention: %w", err)
	}

	intervention.Level = domain.InterventionLevel(level)
	if runID.Valid {
		intervention.RunID = &runID.String
	}
	return &intervention, nil
}

// ListInterventions returns all intervention IDs for a session.
func (s *SessionStore) ListInterventions(sessionID string) ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM interventions WHERE session_id = ? ORDER BY created_at", sessionID)
	if err != nil {
		return nil, fmt.Errorf("list interventions: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan intervention id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// scanSession scans a single session from a *sql.Row.
func scanSession(row *sql.Row) (*session.Session, error) {
	var sess session.Session
	var codeJSON, policyJSON, authoringDocsJSON string
	var intentStr, statusStr string
	var lastRunAt, lastInterventionAt sql.NullTime

	err := row.Scan(
		&sess.ID, &sess.ExerciseID, &intentStr, &sess.SpecPath,
		&statusStr, &codeJSON, &policyJSON,
		&authoringDocsJSON, &sess.AuthoringSection,
		&sess.RunCount, &sess.HintCount, &lastRunAt, &lastInterventionAt,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrNotFound
		}
		return nil, fmt.Errorf("scan session: %w", err)
	}

	sess.Intent = session.SessionIntent(intentStr)
	sess.Status = session.Status(statusStr)

	if err := json.Unmarshal([]byte(codeJSON), &sess.Code); err != nil {
		return nil, fmt.Errorf("unmarshal code: %w", err)
	}
	if err := json.Unmarshal([]byte(policyJSON), &sess.Policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}
	if err := json.Unmarshal([]byte(authoringDocsJSON), &sess.AuthoringDocs); err != nil {
		return nil, fmt.Errorf("unmarshal authoring_docs: %w", err)
	}

	if lastRunAt.Valid {
		sess.LastRunAt = &lastRunAt.Time
	}
	if lastInterventionAt.Valid {
		sess.LastInterventionAt = &lastInterventionAt.Time
	}

	return &sess, nil
}

// scanSessionRow scans a session from *sql.Rows (for list queries).
func scanSessionRow(rows *sql.Rows) (*session.Session, error) {
	var sess session.Session
	var codeJSON, policyJSON, authoringDocsJSON string
	var intentStr, statusStr string
	var lastRunAt, lastInterventionAt sql.NullTime

	err := rows.Scan(
		&sess.ID, &sess.ExerciseID, &intentStr, &sess.SpecPath,
		&statusStr, &codeJSON, &policyJSON,
		&authoringDocsJSON, &sess.AuthoringSection,
		&sess.RunCount, &sess.HintCount, &lastRunAt, &lastInterventionAt,
		&sess.CreatedAt, &sess.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan session row: %w", err)
	}

	sess.Intent = session.SessionIntent(intentStr)
	sess.Status = session.Status(statusStr)

	if err := json.Unmarshal([]byte(codeJSON), &sess.Code); err != nil {
		return nil, fmt.Errorf("unmarshal code: %w", err)
	}
	if err := json.Unmarshal([]byte(policyJSON), &sess.Policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}
	if err := json.Unmarshal([]byte(authoringDocsJSON), &sess.AuthoringDocs); err != nil {
		return nil, fmt.Errorf("unmarshal authoring_docs: %w", err)
	}

	if lastRunAt.Valid {
		sess.LastRunAt = &lastRunAt.Time
	}
	if lastInterventionAt.Valid {
		sess.LastInterventionAt = &lastInterventionAt.Time
	}

	return &sess, nil
}

// nullTime converts a *time.Time to sql.NullTime for storage.
func nullTime(t *time.Time) sql.NullTime {
	if t == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *t, Valid: true}
}

// nullString converts a byte slice to a *string for nullable TEXT columns.
func nullString(b []byte) *string {
	if b == nil {
		return nil
	}
	s := string(b)
	return &s
}
