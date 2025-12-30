package profile

import (
	"context"
	"sort"
	"time"
)

// AnalyticsOverview provides aggregate statistics
type AnalyticsOverview struct {
	TotalSessions       int         `json:"total_sessions"`
	CompletedSessions   int         `json:"completed_sessions"`
	TotalRuns           int         `json:"total_runs"`
	TotalHints          int         `json:"total_hints"`
	TotalExercises      int         `json:"total_exercises"`
	HintDependency      float64     `json:"hint_dependency"`
	AvgTimeToGreen      string      `json:"avg_time_to_green"`
	CompletionRate      float64     `json:"completion_rate"`
	MostPracticedTopics []TopicStat `json:"most_practiced_topics"`
}

// TopicStat represents statistics for a single topic
type TopicStat struct {
	Topic    string  `json:"topic"`
	Attempts int     `json:"attempts"`
	Level    float64 `json:"level"`
	Trend    string  `json:"trend"` // "improving", "stable", "declining", "new"
}

// SkillBreakdown provides detailed skill analytics
type SkillBreakdown struct {
	Skills      map[string]SkillAnalytics `json:"skills"`
	Progression []ProgressPoint           `json:"progression"`
}

// SkillAnalytics provides detailed analytics for a skill
type SkillAnalytics struct {
	Topic      string    `json:"topic"`
	Level      float64   `json:"level"`
	Attempts   int       `json:"attempts"`
	LastSeen   time.Time `json:"last_seen"`
	Trend      string    `json:"trend"`
	Confidence float64   `json:"confidence"`
}

// ProgressPoint tracks skill progression over time
type ProgressPoint struct {
	Date         string  `json:"date"`
	AvgSkill     float64 `json:"avg_skill"`
	TopicsActive int     `json:"topics_active"`
}

// ErrorPattern represents a common error with count
type ErrorPattern struct {
	Pattern  string `json:"pattern"`
	Count    int    `json:"count"`
	Category string `json:"category"`
}

// GetOverview returns aggregate analytics
func (s *Service) GetOverview(ctx context.Context) (*AnalyticsOverview, error) {
	profile, err := s.store.GetDefault()
	if err != nil {
		return nil, err
	}

	overview := &AnalyticsOverview{
		TotalSessions:     profile.TotalSessions,
		CompletedSessions: profile.CompletedSessions,
		TotalRuns:         profile.TotalRuns,
		TotalHints:        profile.HintRequests,
		TotalExercises:    profile.TotalExercises,
	}

	// Calculate hint dependency
	if profile.TotalRuns > 0 {
		overview.HintDependency = min(1.0, float64(profile.HintRequests)/float64(profile.TotalRuns))
	}

	// Calculate completion rate
	if profile.TotalSessions > 0 {
		overview.CompletionRate = float64(profile.CompletedSessions) / float64(profile.TotalSessions)
	}

	// Format average time to green
	if profile.AvgTimeToGreenMs > 0 {
		d := time.Duration(profile.AvgTimeToGreenMs) * time.Millisecond
		overview.AvgTimeToGreen = formatDuration(d)
	} else {
		overview.AvgTimeToGreen = "N/A"
	}

	// Get most practiced topics
	overview.MostPracticedTopics = s.getTopTopics(profile, 5)

	return overview, nil
}

// GetSkillBreakdown returns detailed skill analytics
func (s *Service) GetSkillBreakdown(ctx context.Context) (*SkillBreakdown, error) {
	profile, err := s.store.GetDefault()
	if err != nil {
		return nil, err
	}

	breakdown := &SkillBreakdown{
		Skills:      make(map[string]SkillAnalytics),
		Progression: []ProgressPoint{},
	}

	// Convert stored skills to analytics
	for topic, skill := range profile.TopicSkills {
		breakdown.Skills[topic] = SkillAnalytics{
			Topic:      topic,
			Level:      skill.Level,
			Attempts:   skill.Attempts,
			LastSeen:   skill.LastSeen,
			Trend:      determineTrend(skill),
			Confidence: skill.Confidence,
		}
	}

	// Build progression from exercise history
	breakdown.Progression = buildProgression(profile)

	return breakdown, nil
}

// GetErrorPatterns returns common error patterns
func (s *Service) GetErrorPatterns(ctx context.Context) ([]ErrorPattern, error) {
	profile, err := s.store.GetDefault()
	if err != nil {
		return nil, err
	}

	var patterns []ErrorPattern
	for pattern, count := range profile.ErrorPatterns {
		patterns = append(patterns, ErrorPattern{
			Pattern:  pattern,
			Count:    count,
			Category: CategorizeError(pattern),
		})
	}

	// Sort by count descending
	sort.Slice(patterns, func(i, j int) bool {
		return patterns[i].Count > patterns[j].Count
	})

	// Return top 20
	if len(patterns) > 20 {
		patterns = patterns[:20]
	}

	return patterns, nil
}

// GetHintTrend returns the hint dependency trend
func (s *Service) GetHintTrend(ctx context.Context) ([]HintDependencyPoint, error) {
	profile, err := s.store.GetDefault()
	if err != nil {
		return nil, err
	}

	return profile.HintDependencyTrend, nil
}

// getTopTopics returns the most practiced topics
func (s *Service) getTopTopics(profile *StoredProfile, n int) []TopicStat {
	var topics []TopicStat

	for topic, skill := range profile.TopicSkills {
		topics = append(topics, TopicStat{
			Topic:    topic,
			Attempts: skill.Attempts,
			Level:    skill.Level,
			Trend:    determineTrend(skill),
		})
	}

	// Sort by attempts descending
	sort.Slice(topics, func(i, j int) bool {
		return topics[i].Attempts > topics[j].Attempts
	})

	if len(topics) > n {
		topics = topics[:n]
	}

	return topics
}

// determineTrend determines the trend for a skill
func determineTrend(skill StoredSkill) string {
	if skill.Attempts <= 2 {
		return "new"
	}

	// Check recency
	daysSinceLastSeen := time.Since(skill.LastSeen).Hours() / 24
	if daysSinceLastSeen > 14 {
		return "inactive"
	}

	// Simple heuristic based on level
	if skill.Level > 0.7 {
		return "improving"
	} else if skill.Level > 0.3 {
		return "stable"
	} else if skill.Attempts > 5 {
		return "struggling"
	}

	return "learning"
}

// buildProgression builds a progression timeline from exercise history
func buildProgression(profile *StoredProfile) []ProgressPoint {
	if len(profile.ExerciseHistory) == 0 {
		return []ProgressPoint{}
	}

	// Group exercises by day
	dayGroups := make(map[string][]ExerciseAttempt)
	for _, attempt := range profile.ExerciseHistory {
		day := attempt.StartedAt.Format("2006-01-02")
		dayGroups[day] = append(dayGroups[day], attempt)
	}

	// Calculate stats per day
	var points []ProgressPoint
	days := make([]string, 0, len(dayGroups))
	for day := range dayGroups {
		days = append(days, day)
	}
	sort.Strings(days)

	// Track running skill average
	topicLevels := make(map[string]float64)

	for _, day := range days {
		attempts := dayGroups[day]

		// Update topic levels based on attempts
		for _, attempt := range attempts {
			topic := ExtractTopic(attempt.ExerciseID)
			if attempt.Success {
				topicLevels[topic] = min(1.0, topicLevels[topic]+0.05)
			}
		}

		// Calculate average skill
		var totalSkill float64
		for _, level := range topicLevels {
			totalSkill += level
		}
		avgSkill := 0.0
		if len(topicLevels) > 0 {
			avgSkill = totalSkill / float64(len(topicLevels))
		}

		points = append(points, ProgressPoint{
			Date:         day,
			AvgSkill:     avgSkill,
			TopicsActive: len(topicLevels),
		})
	}

	// Keep last 30 days
	if len(points) > 30 {
		points = points[len(points)-30:]
	}

	return points
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		if s > 0 {
			return (time.Duration(m)*time.Minute + time.Duration(s)*time.Second).String()
		}
		return (time.Duration(m) * time.Minute).String()
	}
	return d.Round(time.Minute).String()
}
