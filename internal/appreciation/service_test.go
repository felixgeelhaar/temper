package appreciation

import (
	"testing"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
)

func TestNewService(t *testing.T) {
	s := NewService()
	if s == nil {
		t.Fatal("NewService() returned nil")
	}
	if s.detector == nil {
		t.Error("detector should not be nil")
	}
	if s.generator == nil {
		t.Error("generator should not be nil")
	}
	if s.lastAppreciation == nil {
		t.Error("lastAppreciation map should not be nil")
	}
}

func TestService_CheckSession(t *testing.T) {
	s := NewService()
	userID := uuid.New()

	tests := []struct {
		name    string
		session *session.Session
		output  *domain.RunOutput
		wantMsg bool
	}{
		{
			name: "first try success generates message",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 0,
				RunCount:  1,
				Status:    session.StatusActive,
				CreatedAt: time.Now(),
			},
			output: &domain.RunOutput{
				TestsPassed: 5,
				TestsFailed: 0,
			},
			wantMsg: true,
		},
		{
			name: "no moments generates no message",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 10,
				RunCount:  20,
				Status:    session.StatusActive,
				CreatedAt: time.Now().Add(-60 * time.Minute),
			},
			output:  nil,
			wantMsg: false,
		},
		{
			name: "no hints generates message",
			session: &session.Session{
				ID:        uuid.New().String(),
				HintCount: 0,
				RunCount:  3,
				Status:    session.StatusActive,
				CreatedAt: time.Now(),
			},
			output:  nil,
			wantMsg: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use different user for each test to avoid rate limiting
			testUserID := uuid.New()
			msg := s.CheckSession(testUserID, tt.session, tt.output)

			if tt.wantMsg && msg == nil {
				t.Error("CheckSession() returned nil, want message")
			}
			if !tt.wantMsg && msg != nil {
				t.Errorf("CheckSession() = %v, want nil", msg)
			}
		})
	}

	// Test rate limiting
	t.Run("rate limiting", func(t *testing.T) {
		sess := &session.Session{
			ID:        uuid.New().String(),
			HintCount: 0,
			RunCount:  1,
			Status:    session.StatusActive,
			CreatedAt: time.Now(),
		}

		// First call should generate message
		msg1 := s.CheckSession(userID, sess, nil)
		if msg1 == nil {
			t.Error("first CheckSession() should return message")
		}

		// Immediate second call should be rate limited (low priority)
		msg2 := s.CheckSession(userID, sess, nil)
		if msg2 != nil {
			t.Error("immediate second CheckSession() should be rate limited")
		}
	})
}

func TestService_CheckProgress(t *testing.T) {
	s := NewService()

	tests := []struct {
		name     string
		current  *domain.LearningProfile
		previous *domain.LearningProfile
		wantMsg  bool
	}{
		{
			name:     "nil profiles",
			current:  nil,
			previous: nil,
			wantMsg:  false,
		},
		{
			name: "significant improvement",
			current: &domain.LearningProfile{
				HintRequests: 10,
				TotalRuns:    100,
			},
			previous: &domain.LearningProfile{
				HintRequests: 40,
				TotalRuns:    100,
			},
			wantMsg: true,
		},
		{
			name: "no improvement",
			current: &domain.LearningProfile{
				HintRequests: 50,
				TotalRuns:    100,
			},
			previous: &domain.LearningProfile{
				HintRequests: 50,
				TotalRuns:    100,
			},
			wantMsg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New() // Fresh user for each test
			msg := s.CheckProgress(userID, tt.current, tt.previous)

			if tt.wantMsg && msg == nil {
				t.Error("CheckProgress() returned nil, want message")
			}
			if !tt.wantMsg && msg != nil {
				t.Errorf("CheckProgress() = %v, want nil", msg)
			}
		})
	}
}

func TestService_CheckSkill(t *testing.T) {
	s := NewService()

	tests := []struct {
		name    string
		skill   *Skill
		isFirst bool
		wantMsg bool
	}{
		{
			name:    "nil skill",
			skill:   nil,
			isFirst: false,
			wantMsg: false,
		},
		{
			name: "first in topic",
			skill: &Skill{
				Topic: "functions",
				Level: 1,
			},
			isFirst: true,
			wantMsg: true,
		},
		{
			name: "topic mastery",
			skill: &Skill{
				Topic:              "testing",
				Level:              5,
				ExercisesCompleted: 10,
			},
			isFirst: false,
			wantMsg: true,
		},
		{
			name: "regular progress",
			skill: &Skill{
				Topic: "basics",
				Level: 2,
			},
			isFirst: false,
			wantMsg: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()
			msg := s.CheckSkill(userID, tt.skill, tt.isFirst)

			if tt.wantMsg && msg == nil {
				t.Error("CheckSkill() returned nil, want message")
			}
			if !tt.wantMsg && msg != nil {
				t.Errorf("CheckSkill() = %v, want nil", msg)
			}
		})
	}
}

func TestService_CheckSpec(t *testing.T) {
	s := NewService()

	tests := []struct {
		name      string
		spec      *domain.ProductSpec
		criterion *domain.AcceptanceCriterion
		wantMsg   bool
	}{
		{
			name:      "nil spec",
			spec:      nil,
			criterion: nil,
			wantMsg:   false,
		},
		{
			name: "criterion satisfied",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: true},
					{ID: "AC-002", Satisfied: false},
				},
			},
			criterion: &domain.AcceptanceCriterion{
				ID:        "AC-001",
				Satisfied: true,
			},
			wantMsg: true,
		},
		{
			name: "spec complete",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: true},
				},
			},
			criterion: nil,
			wantMsg:   true,
		},
		{
			name: "spec not complete no criterion",
			spec: &domain.ProductSpec{
				Name: "user-auth",
				AcceptanceCriteria: []domain.AcceptanceCriterion{
					{ID: "AC-001", Satisfied: false},
				},
			},
			criterion: nil,
			wantMsg:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID := uuid.New()
			msg := s.CheckSpec(userID, tt.spec, tt.criterion)

			if tt.wantMsg && msg == nil {
				t.Error("CheckSpec() returned nil, want message")
			}
			if !tt.wantMsg && msg != nil {
				t.Errorf("CheckSpec() = %v, want nil", msg)
			}
		})
	}
}

func TestService_GetMomentPriority(t *testing.T) {
	s := NewService()

	tests := []struct {
		momentType MomentType
		want       int
	}{
		{MomentSpecComplete, 10},
		{MomentTopicMastery, 9},
		{MomentFirstTrySuccess, 8},
		{MomentNoHintsNeeded, 7},
		{MomentReducedDependency, 6},
		{MomentCriterionSatisfied, 5},
		{MomentNoEscalation, 4},
		{MomentMinimalHints, 3},
		{MomentConsistentSuccess, 3},
		{MomentQuickResolution, 2},
		{MomentAllTestsPassing, 1},
		{MomentFirstInTopic, 1},
		{MomentType("unknown"), 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.momentType), func(t *testing.T) {
			got := s.GetMomentPriority(tt.momentType)
			if got != tt.want {
				t.Errorf("GetMomentPriority(%v) = %d, want %d", tt.momentType, got, tt.want)
			}
		})
	}
}

func TestService_MinutesSinceLastAppreciation(t *testing.T) {
	s := NewService()

	// New user should return -1
	newUser := uuid.New()
	mins := s.MinutesSinceLastAppreciation(newUser)
	if mins != -1 {
		t.Errorf("MinutesSinceLastAppreciation(new user) = %d, want -1", mins)
	}

	// User with recent appreciation
	userWithHistory := uuid.New()
	s.lastAppreciation[userWithHistory] = time.Now().Add(-30 * time.Minute)

	mins = s.MinutesSinceLastAppreciation(userWithHistory)
	if mins < 29 || mins > 31 {
		t.Errorf("MinutesSinceLastAppreciation() = %d, want ~30", mins)
	}
}

func TestService_ShouldShow(t *testing.T) {
	s := NewService()

	// New user always shows
	newUser := uuid.New()
	if !s.shouldShow(newUser, MomentMinimalHints) {
		t.Error("shouldShow(new user) = false, want true")
	}

	// User with recent appreciation - high priority shows
	recentUser := uuid.New()
	s.lastAppreciation[recentUser] = time.Now()

	if !s.shouldShow(recentUser, MomentSpecComplete) {
		t.Error("shouldShow(recent user, high priority) = false, want true")
	}

	// User with recent appreciation - low priority does not show
	if s.shouldShow(recentUser, MomentAllTestsPassing) {
		t.Error("shouldShow(recent user, low priority) = true, want false")
	}

	// User with old appreciation - any priority shows
	oldUser := uuid.New()
	s.lastAppreciation[oldUser] = time.Now().Add(-120 * time.Minute)

	if !s.shouldShow(oldUser, MomentAllTestsPassing) {
		t.Error("shouldShow(old user, low priority) = false, want true")
	}
}

func TestService_RecordAppreciation(t *testing.T) {
	s := NewService()
	userID := uuid.New()

	// Before recording
	_, exists := s.lastAppreciation[userID]
	if exists {
		t.Error("user should not exist before recording")
	}

	// Record appreciation
	s.recordAppreciation(userID)

	// After recording
	lastTime, exists := s.lastAppreciation[userID]
	if !exists {
		t.Error("user should exist after recording")
	}

	elapsed := time.Since(lastTime)
	if elapsed > time.Second {
		t.Errorf("lastTime too old: %v ago", elapsed)
	}
}

func TestService_Concurrency(t *testing.T) {
	s := NewService()
	userID := uuid.New()

	// Concurrent reads and writes
	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func() {
			s.recordAppreciation(userID)
			done <- true
		}()

		go func() {
			s.MinutesSinceLastAppreciation(userID)
			done <- true
		}()

		go func() {
			s.shouldShow(userID, MomentFirstTrySuccess)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 30; i++ {
		<-done
	}

	// If we get here without deadlock, the test passes
}
