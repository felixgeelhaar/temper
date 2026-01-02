package profile

import (
	"errors"
	"time"

	"github.com/felixgeelhaar/temper/internal/storage/local"
)

const (
	collectionProfiles = "profiles"
	defaultProfileID   = "default"
)

var ErrNotFound = errors.New("profile not found")

// StoredProfile is the JSON-serializable profile structure
type StoredProfile struct {
	ID                  string                   `json:"id"`
	TopicSkills         map[string]StoredSkill   `json:"topic_skills"`
	TotalExercises      int                      `json:"total_exercises"`
	TotalSessions       int                      `json:"total_sessions"`
	CompletedSessions   int                      `json:"completed_sessions"`
	TotalRuns           int                      `json:"total_runs"`
	HintRequests        int                      `json:"hint_requests"`
	AvgTimeToGreenMs    int64                    `json:"avg_time_to_green_ms"`
	ExerciseHistory     []ExerciseAttempt        `json:"exercise_history"`
	ErrorPatterns       map[string]int           `json:"error_patterns"`
	HintDependencyTrend []HintDependencyPoint    `json:"hint_dependency_trend"`
	UpdatedAt           time.Time                `json:"updated_at"`
	CreatedAt           time.Time                `json:"created_at"`
}

// StoredSkill is the JSON-serializable skill structure
type StoredSkill struct {
	Level      float64   `json:"level"`
	Attempts   int       `json:"attempts"`
	LastSeen   time.Time `json:"last_seen"`
	Confidence float64   `json:"confidence"`
}

// ExerciseAttempt tracks a single exercise session
type ExerciseAttempt struct {
	ExerciseID       string     `json:"exercise_id"`
	SessionID        string     `json:"session_id"`
	StartedAt        time.Time  `json:"started_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	RunCount         int        `json:"run_count"`
	HintCount        int        `json:"hint_count"`
	TimeToCompleteMs int64      `json:"time_to_complete_ms,omitempty"`
	Success          bool       `json:"success"`
}

// HintDependencyPoint tracks hint dependency over time
type HintDependencyPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	Dependency float64   `json:"dependency"`
	RunWindow  int       `json:"run_window"`
}

// Store handles profile persistence
type Store struct {
	store *local.Store
}

// NewStore creates a new profile store
func NewStore(basePath string) (*Store, error) {
	store, err := local.NewStore(basePath)
	if err != nil {
		return nil, err
	}
	return &Store{store: store}, nil
}

// Save persists a profile
func (s *Store) Save(profile *StoredProfile) error {
	profile.UpdatedAt = time.Now()
	return s.store.Save(collectionProfiles, profile.ID, profile)
}

// Get retrieves a profile by ID
func (s *Store) Get(id string) (*StoredProfile, error) {
	var profile StoredProfile
	if err := s.store.Load(collectionProfiles, id, &profile); err != nil {
		if errors.Is(err, local.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &profile, nil
}

// GetDefault retrieves or creates the default profile
func (s *Store) GetDefault() (*StoredProfile, error) {
	profile, err := s.Get(defaultProfileID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			// Create new default profile
			profile = &StoredProfile{
				ID:              defaultProfileID,
				TopicSkills:     make(map[string]StoredSkill),
				ErrorPatterns:   make(map[string]int),
				ExerciseHistory: []ExerciseAttempt{},
				HintDependencyTrend: []HintDependencyPoint{},
				CreatedAt:       time.Now(),
				UpdatedAt:       time.Now(),
			}
			if err := s.Save(profile); err != nil {
				return nil, err
			}
			return profile, nil
		}
		return nil, err
	}
	return profile, nil
}

// Delete removes a profile
func (s *Store) Delete(id string) error {
	if err := s.store.Delete(collectionProfiles, id); err != nil {
		if errors.Is(err, local.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

// List returns all profile IDs
func (s *Store) List() ([]string, error) {
	return s.store.List(collectionProfiles)
}

// Exists checks if a profile exists
func (s *Store) Exists(id string) bool {
	return s.store.Exists(collectionProfiles, id)
}
