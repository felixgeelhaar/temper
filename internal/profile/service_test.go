package profile

import (
	"context"
	"testing"
	"time"
)

func setupService(t *testing.T) *Service {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := NewStore(tmpDir)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return NewService(store)
}

func TestNewService(t *testing.T) {
	service := setupService(t)
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_GetProfile(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	profile, err := service.GetProfile(ctx)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}

	if profile.ID != "default" {
		t.Errorf("ID = %q; want %q", profile.ID, "default")
	}
}

func TestService_OnSessionStart(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sess := SessionInfo{
		ID:         "session-1",
		ExerciseID: "go-v1/basics/hello",
		CreatedAt:  time.Now(),
	}

	err := service.OnSessionStart(ctx, sess)
	if err != nil {
		t.Fatalf("OnSessionStart() error = %v", err)
	}

	profile, _ := service.GetProfile(ctx)
	if profile.TotalSessions != 1 {
		t.Errorf("TotalSessions = %d; want 1", profile.TotalSessions)
	}
	if len(profile.ExerciseHistory) != 1 {
		t.Errorf("ExerciseHistory length = %d; want 1", len(profile.ExerciseHistory))
	}
}

func TestService_OnSessionStart_BoundsHistory(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// Add 110 sessions
	for i := 0; i < 110; i++ {
		sess := SessionInfo{
			ID:         "session-" + string(rune('0'+i)),
			ExerciseID: "go-v1/basics/hello",
			CreatedAt:  time.Now(),
		}
		service.OnSessionStart(ctx, sess)
	}

	profile, _ := service.GetProfile(ctx)
	if len(profile.ExerciseHistory) > 100 {
		t.Errorf("ExerciseHistory length = %d; should be bounded to 100", len(profile.ExerciseHistory))
	}
}

func TestService_OnSessionComplete(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// Start a session first
	sess := SessionInfo{
		ID:         "session-1",
		ExerciseID: "go-v1/basics/hello",
		CreatedAt:  time.Now(),
	}
	service.OnSessionStart(ctx, sess)

	// Complete it
	sess.Status = "completed"
	sess.RunCount = 3
	sess.HintCount = 1

	err := service.OnSessionComplete(ctx, sess)
	if err != nil {
		t.Fatalf("OnSessionComplete() error = %v", err)
	}

	profile, _ := service.GetProfile(ctx)
	if profile.CompletedSessions != 1 {
		t.Errorf("CompletedSessions = %d; want 1", profile.CompletedSessions)
	}
	if profile.TotalExercises != 1 {
		t.Errorf("TotalExercises = %d; want 1", profile.TotalExercises)
	}

	// Check topic skill was updated
	topic := ExtractTopic(sess.ExerciseID)
	skill, ok := profile.TopicSkills[topic]
	if !ok {
		t.Fatal("Topic skill not created")
	}
	if skill.Attempts != 1 {
		t.Errorf("Skill attempts = %d; want 1", skill.Attempts)
	}
	if skill.Level == 0 {
		t.Error("Skill level should be > 0 after completion")
	}
}

func TestService_OnSessionComplete_Abandoned(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sess := SessionInfo{
		ID:         "session-1",
		ExerciseID: "go-v1/basics/hello",
		Status:     "abandoned",
		CreatedAt:  time.Now(),
	}
	service.OnSessionStart(ctx, sess)
	service.OnSessionComplete(ctx, sess)

	profile, _ := service.GetProfile(ctx)
	if profile.CompletedSessions != 0 {
		t.Errorf("CompletedSessions = %d; want 0", profile.CompletedSessions)
	}
}

func TestService_OnRunComplete(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sess := SessionInfo{
		ID:         "session-1",
		ExerciseID: "go-v1/basics/hello",
	}

	run := RunInfo{
		Success:  true,
		Duration: 5 * time.Second,
	}

	err := service.OnRunComplete(ctx, sess, run)
	if err != nil {
		t.Fatalf("OnRunComplete() error = %v", err)
	}

	profile, _ := service.GetProfile(ctx)
	if profile.TotalRuns != 1 {
		t.Errorf("TotalRuns = %d; want 1", profile.TotalRuns)
	}
	if profile.AvgTimeToGreenMs == 0 {
		t.Error("AvgTimeToGreenMs should be set")
	}
}

func TestService_OnRunComplete_WithErrors(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sess := SessionInfo{
		ID:         "session-1",
		ExerciseID: "go-v1/basics/hello",
	}

	run := RunInfo{
		Success:     false,
		BuildOutput: "undefined: someVariable",
		TestOutput:  "",
	}

	service.OnRunComplete(ctx, sess, run)

	profile, _ := service.GetProfile(ctx)
	if len(profile.ErrorPatterns) == 0 {
		t.Error("ErrorPatterns should be recorded on failed run")
	}
}

func TestService_OnHintDelivered(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sess := SessionInfo{
		ID:         "session-1",
		ExerciseID: "go-v1/basics/hello",
	}

	err := service.OnHintDelivered(ctx, sess)
	if err != nil {
		t.Fatalf("OnHintDelivered() error = %v", err)
	}

	profile, _ := service.GetProfile(ctx)
	if profile.HintRequests != 1 {
		t.Errorf("HintRequests = %d; want 1", profile.HintRequests)
	}
}

func TestService_RebuildFromSessions(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	sessions := []SessionInfo{
		{ID: "s1", ExerciseID: "go-v1/basics/hello", Status: "completed", RunCount: 2, HintCount: 1, CreatedAt: time.Now()},
		{ID: "s2", ExerciseID: "go-v1/basics/variables", Status: "completed", RunCount: 3, HintCount: 0, CreatedAt: time.Now()},
		{ID: "s3", ExerciseID: "python-v1/basics/hello", Status: "abandoned", RunCount: 1, HintCount: 2, CreatedAt: time.Now()},
	}

	runs := map[string][]RunInfo{
		"s1": {
			{Success: true},
			{Success: true},
		},
		"s2": {
			{Success: false, BuildOutput: "undefined: x"},
			{Success: false, BuildOutput: "undefined: y"},
			{Success: true},
		},
	}

	err := service.RebuildFromSessions(ctx, sessions, runs)
	if err != nil {
		t.Fatalf("RebuildFromSessions() error = %v", err)
	}

	profile, _ := service.GetProfile(ctx)
	if profile.TotalSessions != 3 {
		t.Errorf("TotalSessions = %d; want 3", profile.TotalSessions)
	}
	if profile.CompletedSessions != 2 {
		t.Errorf("CompletedSessions = %d; want 2", profile.CompletedSessions)
	}
	if profile.TotalRuns != 5 {
		t.Errorf("TotalRuns = %d; want 5", profile.TotalRuns)
	}
	if profile.HintRequests != 3 {
		t.Errorf("HintRequests = %d; want 3", profile.HintRequests)
	}
	if len(profile.TopicSkills) == 0 {
		t.Error("TopicSkills should be populated")
	}
	if len(profile.ExerciseHistory) != 3 {
		t.Errorf("ExerciseHistory length = %d; want 3", len(profile.ExerciseHistory))
	}
}

func TestService_UpdateHintTrend(t *testing.T) {
	service := setupService(t)
	ctx := context.Background()

	// updateHintTrend is called in OnSessionComplete, and only adds point when TotalRuns % 10 == 0
	sess := SessionInfo{ID: "s1", ExerciseID: "go-v1/basics/hello", Status: "completed", CreatedAt: time.Now()}
	service.OnSessionStart(ctx, sess)

	run := RunInfo{Success: true, Duration: time.Second}

	// Add 10 runs
	for i := 0; i < 10; i++ {
		service.OnRunComplete(ctx, sess, run)
	}

	// Complete session to trigger updateHintTrend
	service.OnSessionComplete(ctx, sess)

	profile, _ := service.GetProfile(ctx)
	if len(profile.HintDependencyTrend) == 0 {
		t.Error("HintDependencyTrend should have entries after 10 runs and session complete")
	}
}
