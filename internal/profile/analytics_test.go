package profile

import (
	"context"
	"testing"
	"time"
)

func TestService_GetOverview(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// Add some data
	sess := SessionInfo{ID: "s1", ExerciseID: "go-v1/basics/hello", Status: "completed", CreatedAt: time.Now()}
	service.OnSessionStart(ctx, sess)
	service.OnSessionComplete(ctx, sess)

	run := RunInfo{Success: true, Duration: 30 * time.Second}
	service.OnRunComplete(ctx, sess, run)

	overview, err := service.GetOverview(ctx)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}

	if overview.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d; want 1", overview.TotalSessions)
	}
	if overview.CompletedSessions != 1 {
		t.Errorf("CompletedSessions = %d; want 1", overview.CompletedSessions)
	}
	if overview.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d; want 1", overview.TotalRuns)
	}
	if overview.CompletionRate != 1.0 {
		t.Errorf("CompletionRate = %f; want 1.0", overview.CompletionRate)
	}
	if overview.AvgTimeToGreen == "N/A" {
		t.Error("AvgTimeToGreen should be set")
	}
}

func TestService_GetOverview_Empty(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	overview, err := service.GetOverview(ctx)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}

	if overview.TotalSessions != 0 {
		t.Errorf("TotalSessions = %d; want 0", overview.TotalSessions)
	}
	if overview.AvgTimeToGreen != "N/A" {
		t.Errorf("AvgTimeToGreen = %s; want N/A", overview.AvgTimeToGreen)
	}
}

func TestService_GetSkillBreakdown(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// Add sessions for multiple topics
	for i, topic := range []string{"go-v1/basics/hello", "python-v1/testing/pytest"} {
		sess := SessionInfo{
			ID:         "session-" + string(rune('1'+i)),
			ExerciseID: topic,
			Status:     "completed",
			CreatedAt:  time.Now(),
		}
		service.OnSessionStart(ctx, sess)
		service.OnSessionComplete(ctx, sess)
	}

	breakdown, err := service.GetSkillBreakdown(ctx)
	if err != nil {
		t.Fatalf("GetSkillBreakdown() error = %v", err)
	}

	if len(breakdown.Skills) != 2 {
		t.Errorf("Skills count = %d; want 2", len(breakdown.Skills))
	}

	if len(breakdown.Progression) == 0 {
		t.Error("Progression should have entries")
	}
}

func TestService_GetErrorPatterns(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sess := SessionInfo{ID: "s1", ExerciseID: "go-v1/basics/hello"}
	for i := 0; i < 5; i++ {
		service.OnRunComplete(ctx, sess, RunInfo{
			Success:     false,
			BuildOutput: "undefined: someVariable",
		})
	}
	for i := 0; i < 3; i++ {
		service.OnRunComplete(ctx, sess, RunInfo{
			Success:    false,
			TestOutput: "panic: runtime error",
		})
	}

	patterns, err := service.GetErrorPatterns(ctx)
	if err != nil {
		t.Fatalf("GetErrorPatterns() error = %v", err)
	}

	if len(patterns) == 0 {
		t.Error("Should have error patterns")
	}

	// Check sorting (highest count first)
	if len(patterns) >= 2 {
		if patterns[0].Count < patterns[1].Count {
			t.Error("Patterns should be sorted by count descending")
		}
	}
}

func TestService_GetHintTrend(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// updateHintTrend is called in OnSessionComplete, and only adds point when TotalRuns % 10 == 0
	// So we need to get TotalRuns to 10, then call OnSessionComplete
	sess := SessionInfo{ID: "s1", ExerciseID: "go-v1/basics/hello", Status: "completed", CreatedAt: time.Now()}
	service.OnSessionStart(ctx, sess)

	// Add 10 runs to get TotalRuns = 10
	for i := 0; i < 10; i++ {
		service.OnRunComplete(ctx, sess, RunInfo{Success: true, Duration: time.Second})
	}

	// Now complete session which calls updateHintTrend
	service.OnSessionComplete(ctx, sess)

	trend, err := service.GetHintTrend(ctx)
	if err != nil {
		t.Fatalf("GetHintTrend() error = %v", err)
	}

	if len(trend) == 0 {
		t.Error("Trend should have entries after 10 runs and session complete")
	}
}

func TestDetermineTrend(t *testing.T) {
	tests := []struct {
		name  string
		skill StoredSkill
		want  string
	}{
		{
			name:  "new skill",
			skill: StoredSkill{Attempts: 1, Level: 0.1, LastSeen: time.Now()},
			want:  "new",
		},
		{
			name:  "improving skill",
			skill: StoredSkill{Attempts: 10, Level: 0.8, LastSeen: time.Now()},
			want:  "improving",
		},
		{
			name:  "stable skill",
			skill: StoredSkill{Attempts: 10, Level: 0.5, LastSeen: time.Now()},
			want:  "stable",
		},
		{
			name:  "struggling skill",
			skill: StoredSkill{Attempts: 10, Level: 0.2, LastSeen: time.Now()},
			want:  "struggling",
		},
		{
			name:  "learning skill",
			skill: StoredSkill{Attempts: 3, Level: 0.2, LastSeen: time.Now()},
			want:  "learning",
		},
		{
			name:  "inactive skill",
			skill: StoredSkill{Attempts: 10, Level: 0.5, LastSeen: time.Now().Add(-30 * 24 * time.Hour)},
			want:  "inactive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := determineTrend(tt.skill)
			if got != tt.want {
				t.Errorf("determineTrend() = %q; want %q", got, tt.want)
			}
		})
	}
}

func TestBuildProgression(t *testing.T) {
	// Empty profile
	emptyProfile := &StoredProfile{
		ExerciseHistory: []ExerciseAttempt{},
	}
	progression := buildProgression(emptyProfile)
	if len(progression) != 0 {
		t.Errorf("Empty profile progression length = %d; want 0", len(progression))
	}

	// Profile with data
	profile := &StoredProfile{
		ExerciseHistory: []ExerciseAttempt{
			{ExerciseID: "go-v1/basics/hello", Success: true, StartedAt: time.Now().Add(-24 * time.Hour)},
			{ExerciseID: "go-v1/basics/variables", Success: true, StartedAt: time.Now().Add(-24 * time.Hour)},
			{ExerciseID: "go-v1/interfaces/stringer", Success: false, StartedAt: time.Now()},
		},
	}
	progression = buildProgression(profile)
	if len(progression) == 0 {
		t.Error("Progression should have entries")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{5 * time.Minute, "5m0s"},
		{65 * time.Minute, "1h5m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.duration.String(), func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %q; want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestGetTopTopics(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// Add sessions for different topics with varying attempts
	topics := []struct {
		exerciseID string
		attempts   int
	}{
		{"go-v1/basics/hello", 10},
		{"go-v1/interfaces/stringer", 5},
		{"python-v1/testing/pytest", 8},
		{"go-v1/concurrency/channels", 3},
	}

	for _, topic := range topics {
		for i := 0; i < topic.attempts; i++ {
			sess := SessionInfo{
				ID:         newID(),
				ExerciseID: topic.exerciseID,
				Status:     "completed",
				CreatedAt:  time.Now(),
			}
			service.OnSessionStart(ctx, sess)
			service.OnSessionComplete(ctx, sess)
		}
	}

	overview, _ := service.GetOverview(ctx)
	topTopics := overview.MostPracticedTopics

	if len(topTopics) == 0 {
		t.Error("Should have top topics")
	}

	// Check that topics are sorted by attempts
	if len(topTopics) >= 2 {
		if topTopics[0].Attempts < topTopics[1].Attempts {
			t.Error("Topics should be sorted by attempts descending")
		}
	}
}
