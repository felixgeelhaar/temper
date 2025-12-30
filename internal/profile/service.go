package profile

import (
	"context"
	"log/slog"
	"time"
)

// Service handles profile business logic
type Service struct {
	store *Store
}

// NewService creates a new profile service
func NewService(store *Store) *Service {
	return &Service{store: store}
}

// GetProfile returns the default learning profile
func (s *Service) GetProfile(ctx context.Context) (*StoredProfile, error) {
	return s.store.GetDefault()
}

// SessionInfo contains session data needed for profile updates
type SessionInfo struct {
	ID         string
	ExerciseID string
	RunCount   int
	HintCount  int
	Status     string // "active", "completed", "abandoned"
	CreatedAt  time.Time
}

// RunInfo contains run data needed for profile updates
type RunInfo struct {
	Success     bool
	BuildOutput string
	TestOutput  string
	Duration    time.Duration
}

// OnSessionStart records the start of a new exercise session
func (s *Service) OnSessionStart(ctx context.Context, sess SessionInfo) error {
	profile, err := s.store.GetDefault()
	if err != nil {
		return err
	}

	profile.TotalSessions++

	// Add to exercise history
	attempt := ExerciseAttempt{
		ExerciseID: sess.ExerciseID,
		SessionID:  sess.ID,
		StartedAt:  sess.CreatedAt,
		Success:    false,
	}
	profile.ExerciseHistory = append(profile.ExerciseHistory, attempt)

	// Keep history bounded (last 100 attempts)
	if len(profile.ExerciseHistory) > 100 {
		profile.ExerciseHistory = profile.ExerciseHistory[len(profile.ExerciseHistory)-100:]
	}

	return s.store.Save(profile)
}

// OnSessionComplete updates the profile when a session is completed
func (s *Service) OnSessionComplete(ctx context.Context, sess SessionInfo) error {
	profile, err := s.store.GetDefault()
	if err != nil {
		return err
	}

	// Update completion count
	if sess.Status == "completed" {
		profile.CompletedSessions++
	}

	// Update exercise history entry
	for i := len(profile.ExerciseHistory) - 1; i >= 0; i-- {
		if profile.ExerciseHistory[i].SessionID == sess.ID {
			now := time.Now()
			profile.ExerciseHistory[i].CompletedAt = &now
			profile.ExerciseHistory[i].RunCount = sess.RunCount
			profile.ExerciseHistory[i].HintCount = sess.HintCount
			profile.ExerciseHistory[i].Success = sess.Status == "completed"

			// Calculate time to complete
			elapsed := now.Sub(profile.ExerciseHistory[i].StartedAt)
			profile.ExerciseHistory[i].TimeToCompleteMs = elapsed.Milliseconds()
			break
		}
	}

	// Update topic skill
	topic := ExtractTopic(sess.ExerciseID)
	skill := profile.TopicSkills[topic]
	skill.Attempts++
	skill.LastSeen = time.Now()

	if sess.Status == "completed" {
		// Increase skill on completion
		skill.Level = min(1.0, skill.Level+0.05)
		profile.TotalExercises++
	} else {
		// Slight decrease on abandonment
		skill.Level = max(0.0, skill.Level-0.01)
	}
	profile.TopicSkills[topic] = skill

	// Update hint dependency trend (weekly snapshot)
	s.updateHintTrend(profile)

	return s.store.Save(profile)
}

// OnRunComplete updates the profile when a code run completes
func (s *Service) OnRunComplete(ctx context.Context, sess SessionInfo, run RunInfo) error {
	profile, err := s.store.GetDefault()
	if err != nil {
		return err
	}

	profile.TotalRuns++

	// Update average time to green
	if run.Success && run.Duration > 0 {
		if profile.AvgTimeToGreenMs == 0 {
			profile.AvgTimeToGreenMs = run.Duration.Milliseconds()
		} else {
			// Exponential moving average
			profile.AvgTimeToGreenMs = (profile.AvgTimeToGreenMs*9 + run.Duration.Milliseconds()) / 10
		}
	}

	// Extract and record error patterns
	if !run.Success {
		patterns := ExtractErrorPatterns(run.BuildOutput, run.TestOutput)
		for _, p := range patterns {
			if profile.ErrorPatterns == nil {
				profile.ErrorPatterns = make(map[string]int)
			}
			profile.ErrorPatterns[p]++
		}
	}

	return s.store.Save(profile)
}

// OnHintDelivered updates the profile when a hint is delivered
func (s *Service) OnHintDelivered(ctx context.Context, sess SessionInfo) error {
	profile, err := s.store.GetDefault()
	if err != nil {
		return err
	}

	profile.HintRequests++

	return s.store.Save(profile)
}

// updateHintTrend adds a new data point to the hint dependency trend
func (s *Service) updateHintTrend(profile *StoredProfile) {
	// Only add a new point every ~10 runs
	if profile.TotalRuns%10 != 0 && profile.TotalRuns > 0 {
		return
	}

	dependency := 0.0
	if profile.TotalRuns > 0 {
		dependency = min(1.0, float64(profile.HintRequests)/float64(profile.TotalRuns))
	}

	point := HintDependencyPoint{
		Timestamp:  time.Now(),
		Dependency: dependency,
		RunWindow:  10,
	}

	profile.HintDependencyTrend = append(profile.HintDependencyTrend, point)

	// Keep last 50 data points
	if len(profile.HintDependencyTrend) > 50 {
		profile.HintDependencyTrend = profile.HintDependencyTrend[len(profile.HintDependencyTrend)-50:]
	}
}

// RebuildFromSessions reconstructs the profile from session history
// Useful for migration or data recovery
func (s *Service) RebuildFromSessions(ctx context.Context, sessions []SessionInfo, runs map[string][]RunInfo) error {
	profile := &StoredProfile{
		ID:                  defaultProfileID,
		TopicSkills:         make(map[string]StoredSkill),
		ErrorPatterns:       make(map[string]int),
		ExerciseHistory:     []ExerciseAttempt{},
		HintDependencyTrend: []HintDependencyPoint{},
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	for _, sess := range sessions {
		// Update session counts
		profile.TotalSessions++
		if sess.Status == "completed" {
			profile.CompletedSessions++
			profile.TotalExercises++
		}
		profile.HintRequests += sess.HintCount

		// Update topic skills
		topic := ExtractTopic(sess.ExerciseID)
		skill := profile.TopicSkills[topic]
		skill.Attempts++
		skill.LastSeen = sess.CreatedAt
		if sess.Status == "completed" {
			skill.Level = min(1.0, skill.Level+0.05)
		}
		profile.TopicSkills[topic] = skill

		// Add to history
		attempt := ExerciseAttempt{
			ExerciseID: sess.ExerciseID,
			SessionID:  sess.ID,
			StartedAt:  sess.CreatedAt,
			RunCount:   sess.RunCount,
			HintCount:  sess.HintCount,
			Success:    sess.Status == "completed",
		}
		profile.ExerciseHistory = append(profile.ExerciseHistory, attempt)

		// Process runs
		if sessRuns, ok := runs[sess.ID]; ok {
			for _, run := range sessRuns {
				profile.TotalRuns++
				if !run.Success {
					patterns := ExtractErrorPatterns(run.BuildOutput, run.TestOutput)
					for _, p := range patterns {
						profile.ErrorPatterns[p]++
					}
				}
			}
		}
	}

	// Update hint trend
	s.updateHintTrend(profile)

	slog.Info("profile rebuilt from sessions",
		"sessions", profile.TotalSessions,
		"completed", profile.CompletedSessions,
		"runs", profile.TotalRuns)

	return s.store.Save(profile)
}
