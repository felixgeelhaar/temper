package profile

import (
	"context"
	"testing"
	"time"
)

func TestService_GetOverview(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	profile, _ := service.GetProfile(ctx)
	profile.TotalSessions = 4
	profile.CompletedSessions = 2
	profile.TotalRuns = 10
	profile.HintRequests = 3
	profile.TotalExercises = 2
	profile.AvgTimeToGreenMs = int64(90 * time.Second / time.Millisecond)
	profile.TopicSkills["go/basics"] = StoredSkill{Attempts: 3, Level: 0.6, LastSeen: time.Now()}
	service.store.Save(profile)

	overview, err := service.GetOverview(ctx)
	if err != nil {
		t.Fatalf("GetOverview() error = %v", err)
	}
	if overview.HintDependency <= 0 {
		t.Errorf("HintDependency = %f; want > 0", overview.HintDependency)
	}
	if overview.CompletionRate != 0.5 {
		t.Errorf("CompletionRate = %f; want 0.5", overview.CompletionRate)
	}
	if overview.AvgTimeToGreen == "N/A" {
		t.Errorf("AvgTimeToGreen = %q; want duration", overview.AvgTimeToGreen)
	}
	if len(overview.MostPracticedTopics) == 0 {
		t.Error("MostPracticedTopics should not be empty")
	}
}

func TestService_GetSkillBreakdown(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	profile, _ := service.GetProfile(ctx)
	profile.TopicSkills["go/basics"] = StoredSkill{Attempts: 5, Level: 0.8, LastSeen: time.Now()}
	profile.ExerciseHistory = []ExerciseAttempt{
		{ExerciseID: "go-v1/basics/hello", StartedAt: time.Now(), Success: true},
	}
	service.store.Save(profile)

	breakdown, err := service.GetSkillBreakdown(ctx)
	if err != nil {
		t.Fatalf("GetSkillBreakdown() error = %v", err)
	}
	if len(breakdown.Skills) == 0 {
		t.Error("Skills should not be empty")
	}
	if len(breakdown.Progression) == 0 {
		t.Error("Progression should not be empty")
	}
}

func TestService_GetErrorPatterns(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	profile, _ := service.GetProfile(ctx)
	profile.ErrorPatterns["syntax error"] = 2
	profile.ErrorPatterns["panic"] = 1
	service.store.Save(profile)

	patterns, err := service.GetErrorPatterns(ctx)
	if err != nil {
		t.Fatalf("GetErrorPatterns() error = %v", err)
	}
	if len(patterns) != 2 {
		t.Errorf("GetErrorPatterns() length = %d; want 2", len(patterns))
	}
	if patterns[0].Count < patterns[1].Count {
		t.Error("GetErrorPatterns() should sort by count desc")
	}
}

func TestService_GetHintTrend(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	profile, _ := service.GetProfile(ctx)
	profile.HintDependencyTrend = []HintDependencyPoint{{Dependency: 0.5, RunWindow: 10}}
	service.store.Save(profile)

	trend, err := service.GetHintTrend(ctx)
	if err != nil {
		t.Fatalf("GetHintTrend() error = %v", err)
	}
	if len(trend) != 1 {
		t.Errorf("GetHintTrend() length = %d; want 1", len(trend))
	}
}

func TestDetermineTrend(t *testing.T) {
	now := time.Now()
	tests := []struct {
		skill StoredSkill
		want  string
	}{
		{StoredSkill{Attempts: 1, Level: 0.2, LastSeen: now}, "new"},
		{StoredSkill{Attempts: 3, Level: 0.8, LastSeen: now}, "improving"},
		{StoredSkill{Attempts: 3, Level: 0.5, LastSeen: now}, "stable"},
		{StoredSkill{Attempts: 6, Level: 0.2, LastSeen: now}, "struggling"},
		{StoredSkill{Attempts: 3, Level: 0.2, LastSeen: now}, "learning"},
		{StoredSkill{Attempts: 3, Level: 0.8, LastSeen: now.Add(-15 * 24 * time.Hour)}, "inactive"},
	}

	for _, tt := range tests {
		if got := determineTrend(tt.skill); got != tt.want {
			t.Errorf("determineTrend() = %q; want %q", got, tt.want)
		}
	}
}

func TestBuildProgression(t *testing.T) {
	profile := &StoredProfile{
		ExerciseHistory: []ExerciseAttempt{
			{ExerciseID: "go-v1/basics/hello", StartedAt: time.Now().Add(-48 * time.Hour), Success: true},
			{ExerciseID: "go-v1/basics/vars", StartedAt: time.Now().Add(-24 * time.Hour), Success: false},
			{ExerciseID: "go-v1/basics/vars", StartedAt: time.Now(), Success: true},
		},
	}

	points := buildProgression(profile)
	if len(points) == 0 {
		t.Error("buildProgression() should return points")
	}
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(30 * time.Second); got == "" {
		t.Error("formatDuration() should format seconds")
	}
	if got := formatDuration(2*time.Minute + 10*time.Second); got == "" {
		t.Error("formatDuration() should format minutes")
	}
	if got := formatDuration(2*time.Hour + 5*time.Minute); got == "" {
		t.Error("formatDuration() should format hours")
	}
}
