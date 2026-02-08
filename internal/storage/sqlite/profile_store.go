package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/profile"
)

// ProfileStore implements profile persistence backed by SQLite.
type ProfileStore struct {
	db *DB
}

// NewProfileStore creates a new SQLite-backed profile store.
func NewProfileStore(db *DB) *ProfileStore {
	return &ProfileStore{db: db}
}

// Save persists a profile (insert or update).
func (s *ProfileStore) Save(p *profile.StoredProfile) error {
	topicSkills, err := json.Marshal(p.TopicSkills)
	if err != nil {
		return fmt.Errorf("marshal topic_skills: %w", err)
	}
	exerciseHistory, err := json.Marshal(p.ExerciseHistory)
	if err != nil {
		return fmt.Errorf("marshal exercise_history: %w", err)
	}
	errorPatterns, err := json.Marshal(p.ErrorPatterns)
	if err != nil {
		return fmt.Errorf("marshal error_patterns: %w", err)
	}
	hintTrend, err := json.Marshal(p.HintDependencyTrend)
	if err != nil {
		return fmt.Errorf("marshal hint_dependency_trend: %w", err)
	}

	now := time.Now()
	_, err = s.db.Exec(`
		INSERT INTO profiles (id, topic_skills, total_exercises, total_sessions,
			completed_sessions, total_runs, hint_requests, avg_time_to_green_ms,
			exercise_history, error_patterns, hint_dependency_trend,
			created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			topic_skills=excluded.topic_skills,
			total_exercises=excluded.total_exercises,
			total_sessions=excluded.total_sessions,
			completed_sessions=excluded.completed_sessions,
			total_runs=excluded.total_runs,
			hint_requests=excluded.hint_requests,
			avg_time_to_green_ms=excluded.avg_time_to_green_ms,
			exercise_history=excluded.exercise_history,
			error_patterns=excluded.error_patterns,
			hint_dependency_trend=excluded.hint_dependency_trend,
			updated_at=excluded.updated_at`,
		p.ID, string(topicSkills), p.TotalExercises, p.TotalSessions,
		p.CompletedSessions, p.TotalRuns, p.HintRequests, p.AvgTimeToGreenMs,
		string(exerciseHistory), string(errorPatterns), string(hintTrend),
		p.CreatedAt, now,
	)
	if err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	p.UpdatedAt = now
	return nil
}

// Get retrieves a profile by ID.
func (s *ProfileStore) Get(id string) (*profile.StoredProfile, error) {
	row := s.db.QueryRow(`
		SELECT id, topic_skills, total_exercises, total_sessions,
			completed_sessions, total_runs, hint_requests, avg_time_to_green_ms,
			exercise_history, error_patterns, hint_dependency_trend,
			created_at, updated_at
		FROM profiles WHERE id = ?`, id)

	return scanProfile(row)
}

// GetDefault retrieves or creates the default profile.
func (s *ProfileStore) GetDefault() (*profile.StoredProfile, error) {
	p, err := s.Get("default")
	if err != nil {
		if errors.Is(err, profile.ErrNotFound) {
			p = &profile.StoredProfile{
				ID:                  "default",
				TopicSkills:         make(map[string]profile.StoredSkill),
				ErrorPatterns:       make(map[string]int),
				ExerciseHistory:     []profile.ExerciseAttempt{},
				HintDependencyTrend: []profile.HintDependencyPoint{},
				CreatedAt:           time.Now(),
				UpdatedAt:           time.Now(),
			}
			if err := s.Save(p); err != nil {
				return nil, err
			}
			return p, nil
		}
		return nil, err
	}
	return p, nil
}

// Delete removes a profile.
func (s *ProfileStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM profiles WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return profile.ErrNotFound
	}
	return nil
}

// List returns all profile IDs.
func (s *ProfileStore) List() ([]string, error) {
	rows, err := s.db.Query("SELECT id FROM profiles ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan profile id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Exists checks if a profile exists.
func (s *ProfileStore) Exists(id string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM profiles WHERE id = ?", id).Scan(&count)
	return count > 0
}

func scanProfile(row *sql.Row) (*profile.StoredProfile, error) {
	var p profile.StoredProfile
	var topicSkillsJSON, exerciseHistoryJSON, errorPatternsJSON, hintTrendJSON string

	err := row.Scan(
		&p.ID, &topicSkillsJSON, &p.TotalExercises, &p.TotalSessions,
		&p.CompletedSessions, &p.TotalRuns, &p.HintRequests, &p.AvgTimeToGreenMs,
		&exerciseHistoryJSON, &errorPatternsJSON, &hintTrendJSON,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, profile.ErrNotFound
		}
		return nil, fmt.Errorf("scan profile: %w", err)
	}

	if err := json.Unmarshal([]byte(topicSkillsJSON), &p.TopicSkills); err != nil {
		return nil, fmt.Errorf("unmarshal topic_skills: %w", err)
	}
	if err := json.Unmarshal([]byte(exerciseHistoryJSON), &p.ExerciseHistory); err != nil {
		return nil, fmt.Errorf("unmarshal exercise_history: %w", err)
	}
	if err := json.Unmarshal([]byte(errorPatternsJSON), &p.ErrorPatterns); err != nil {
		return nil, fmt.Errorf("unmarshal error_patterns: %w", err)
	}
	if err := json.Unmarshal([]byte(hintTrendJSON), &p.HintDependencyTrend); err != nil {
		return nil, fmt.Errorf("unmarshal hint_dependency_trend: %w", err)
	}

	return &p, nil
}
