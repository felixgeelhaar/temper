package profile

import (
	"context"
)

// ProfileService defines the interface for profile management operations
// used by the daemon handlers
type ProfileService interface {
	// GetProfile returns the default learning profile
	GetProfile(ctx context.Context) (*StoredProfile, error)

	// GetOverview returns aggregate analytics
	GetOverview(ctx context.Context) (*AnalyticsOverview, error)

	// GetSkillBreakdown returns detailed skill analytics
	GetSkillBreakdown(ctx context.Context) (*SkillBreakdown, error)

	// GetErrorPatterns returns common error patterns
	GetErrorPatterns(ctx context.Context) ([]ErrorPattern, error)

	// GetHintTrend returns the hint dependency trend
	GetHintTrend(ctx context.Context) ([]HintDependencyPoint, error)

	// OnSessionStart records the start of a new exercise session
	OnSessionStart(ctx context.Context, sess SessionInfo) error

	// OnSessionComplete updates the profile when a session is completed
	OnSessionComplete(ctx context.Context, sess SessionInfo) error

	// OnRunComplete updates the profile when a code run completes
	OnRunComplete(ctx context.Context, sess SessionInfo, run RunInfo) error

	// OnHintDelivered updates the profile when a hint is delivered
	OnHintDelivered(ctx context.Context, sess SessionInfo) error
}

// Ensure Service implements ProfileService
var _ ProfileService = (*Service)(nil)

// ProfileStore defines the persistence interface for profiles.
// Both the JSON file store and SQLite store implement this.
type ProfileStore interface {
	Save(profile *StoredProfile) error
	Get(id string) (*StoredProfile, error)
	GetDefault() (*StoredProfile, error)
	Delete(id string) error
	List() ([]string, error)
	Exists(id string) bool
}

// Ensure Store (JSON) implements ProfileStore
var _ ProfileStore = (*Store)(nil)
