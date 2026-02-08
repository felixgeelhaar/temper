package sqlite

import (
	"encoding/json"
	"fmt"
	"time"
)

// AnalyticsEvent represents a recorded analytics event.
type AnalyticsEvent struct {
	ID        int64     `json:"id"`
	EventType string    `json:"event_type"`
	SessionID string    `json:"session_id,omitempty"`
	Data      string    `json:"data"`
	CreatedAt time.Time `json:"created_at"`
}

// AnalyticsStore provides analytics event recording backed by SQLite.
type AnalyticsStore struct {
	db *DB
}

// NewAnalyticsStore creates a new SQLite-backed analytics store.
func NewAnalyticsStore(db *DB) *AnalyticsStore {
	return &AnalyticsStore{db: db}
}

// Record stores an analytics event.
func (s *AnalyticsStore) Record(eventType, sessionID string, data interface{}) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal analytics data: %w", err)
	}

	var sessID *string
	if sessionID != "" {
		sessID = &sessionID
	}

	_, err = s.db.Exec(
		"INSERT INTO analytics_events (event_type, session_id, data) VALUES (?, ?, ?)",
		eventType, sessID, string(payload),
	)
	if err != nil {
		return fmt.Errorf("insert analytics event: %w", err)
	}
	return nil
}

// Query returns analytics events matching the given type, optionally filtered by session and time range.
func (s *AnalyticsStore) Query(eventType string, sessionID string, since, until time.Time) ([]AnalyticsEvent, error) {
	query := "SELECT id, event_type, session_id, data, created_at FROM analytics_events WHERE event_type = ?"
	args := []interface{}{eventType}

	if sessionID != "" {
		query += " AND session_id = ?"
		args = append(args, sessionID)
	}
	if !since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, since)
	}
	if !until.IsZero() {
		query += " AND created_at <= ?"
		args = append(args, until)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query analytics: %w", err)
	}
	defer rows.Close()

	var events []AnalyticsEvent
	for rows.Next() {
		var e AnalyticsEvent
		var sessID *string
		if err := rows.Scan(&e.ID, &e.EventType, &sessID, &e.Data, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan analytics event: %w", err)
		}
		if sessID != nil {
			e.SessionID = *sessID
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// Count returns the number of events matching the given type.
func (s *AnalyticsStore) Count(eventType string) (int, error) {
	var count int
	err := s.db.QueryRow(
		"SELECT COUNT(*) FROM analytics_events WHERE event_type = ?", eventType,
	).Scan(&count)
	return count, err
}

// Prune deletes analytics events older than the given duration.
func (s *AnalyticsStore) Prune(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	result, err := s.db.Exec("DELETE FROM analytics_events WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune analytics: %w", err)
	}
	return result.RowsAffected()
}
