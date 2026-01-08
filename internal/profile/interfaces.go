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
