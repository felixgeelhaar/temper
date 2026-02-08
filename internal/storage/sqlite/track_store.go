package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// TrackStore implements track persistence backed by SQLite.
type TrackStore struct {
	db *DB
}

// NewTrackStore creates a new SQLite-backed track store.
func NewTrackStore(db *DB) *TrackStore {
	return &TrackStore{db: db}
}

// Save persists a track (insert or update).
func (s *TrackStore) Save(track *domain.Track) error {
	autoProgress, err := json.Marshal(track.AutoProgress)
	if err != nil {
		return fmt.Errorf("marshal auto_progress: %w", err)
	}

	now := time.Now()
	if track.CreatedAt.IsZero() {
		track.CreatedAt = now
	}
	track.UpdatedAt = now

	_, err = s.db.Exec(`
		INSERT INTO tracks (id, name, description, preset, max_level, cooldown_seconds,
			patching_enabled, auto_progress, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, description=excluded.description,
			preset=excluded.preset, max_level=excluded.max_level,
			cooldown_seconds=excluded.cooldown_seconds,
			patching_enabled=excluded.patching_enabled,
			auto_progress=excluded.auto_progress,
			updated_at=excluded.updated_at`,
		track.ID, track.Name, track.Description, track.Preset,
		int(track.MaxLevel), track.CooldownSeconds,
		boolToInt(track.PatchingEnabled), string(autoProgress),
		track.CreatedAt, track.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upsert track: %w", err)
	}
	return nil
}

// Get retrieves a track by ID.
func (s *TrackStore) Get(id string) (*domain.Track, error) {
	row := s.db.QueryRow(`
		SELECT id, name, description, preset, max_level, cooldown_seconds,
			patching_enabled, auto_progress, created_at, updated_at
		FROM tracks WHERE id = ?`, id)
	return scanTrack(row)
}

// Delete removes a track by ID.
func (s *TrackStore) Delete(id string) error {
	result, err := s.db.Exec("DELETE FROM tracks WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete track: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrTrackNotFound
	}
	return nil
}

// List returns all tracks.
func (s *TrackStore) List() ([]*domain.Track, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, preset, max_level, cooldown_seconds,
			patching_enabled, auto_progress, created_at, updated_at
		FROM tracks ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list tracks: %w", err)
	}
	defer rows.Close()

	var tracks []*domain.Track
	for rows.Next() {
		track, err := scanTrackRow(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

// ListByPreset returns tracks matching a preset.
func (s *TrackStore) ListByPreset(preset string) ([]*domain.Track, error) {
	rows, err := s.db.Query(`
		SELECT id, name, description, preset, max_level, cooldown_seconds,
			patching_enabled, auto_progress, created_at, updated_at
		FROM tracks WHERE preset = ? ORDER BY created_at`, preset)
	if err != nil {
		return nil, fmt.Errorf("list tracks by preset: %w", err)
	}
	defer rows.Close()

	var tracks []*domain.Track
	for rows.Next() {
		track, err := scanTrackRow(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, track)
	}
	return tracks, rows.Err()
}

// Exists checks if a track exists.
func (s *TrackStore) Exists(id string) bool {
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM tracks WHERE id = ?", id).Scan(&count)
	return count > 0
}

// ErrTrackNotFound is returned when a track is not found.
var ErrTrackNotFound = errors.New("track not found")

// scanTrack scans a single track from a *sql.Row.
func scanTrack(row *sql.Row) (*domain.Track, error) {
	var track domain.Track
	var maxLevel int
	var patchingEnabled int
	var autoProgressJSON string

	err := row.Scan(
		&track.ID, &track.Name, &track.Description, &track.Preset,
		&maxLevel, &track.CooldownSeconds,
		&patchingEnabled, &autoProgressJSON,
		&track.CreatedAt, &track.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTrackNotFound
		}
		return nil, fmt.Errorf("scan track: %w", err)
	}

	track.MaxLevel = domain.InterventionLevel(maxLevel)
	track.PatchingEnabled = patchingEnabled != 0

	if err := json.Unmarshal([]byte(autoProgressJSON), &track.AutoProgress); err != nil {
		return nil, fmt.Errorf("unmarshal auto_progress: %w", err)
	}

	return &track, nil
}

// scanTrackRow scans a track from *sql.Rows.
func scanTrackRow(rows *sql.Rows) (*domain.Track, error) {
	var track domain.Track
	var maxLevel int
	var patchingEnabled int
	var autoProgressJSON string

	err := rows.Scan(
		&track.ID, &track.Name, &track.Description, &track.Preset,
		&maxLevel, &track.CooldownSeconds,
		&patchingEnabled, &autoProgressJSON,
		&track.CreatedAt, &track.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan track row: %w", err)
	}

	track.MaxLevel = domain.InterventionLevel(maxLevel)
	track.PatchingEnabled = patchingEnabled != 0

	if err := json.Unmarshal([]byte(autoProgressJSON), &track.AutoProgress); err != nil {
		return nil, fmt.Errorf("unmarshal auto_progress: %w", err)
	}

	return &track, nil
}

// boolToInt converts a bool to SQLite integer.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
