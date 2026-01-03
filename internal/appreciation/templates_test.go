package appreciation

import (
	"testing"
)

func TestNewGenerator(t *testing.T) {
	g := NewGenerator()
	if g == nil {
		t.Fatal("NewGenerator() returned nil")
	}
	if g.templates == nil {
		t.Error("templates map should not be nil")
	}
	if len(g.templates) == 0 {
		t.Error("templates map should not be empty")
	}
}

func TestGenerator_Generate(t *testing.T) {
	g := NewGenerator()

	tests := []struct {
		name       string
		moment     *Moment
		wantNil    bool
		wantType   MomentType
		checkEvid  bool
	}{
		{
			name:    "nil moment returns nil",
			moment:  nil,
			wantNil: true,
		},
		{
			name: "no hints needed moment",
			moment: &Moment{
				Type: MomentNoHintsNeeded,
				Evidence: Evidence{
					HintCount: 0,
					RunCount:  3,
				},
			},
			wantNil:   false,
			wantType:  MomentNoHintsNeeded,
			checkEvid: true,
		},
		{
			name: "minimal hints moment",
			moment: &Moment{
				Type: MomentMinimalHints,
				Evidence: Evidence{
					HintCount: 1,
					RunCount:  5,
				},
			},
			wantNil:   false,
			wantType:  MomentMinimalHints,
			checkEvid: true,
		},
		{
			name: "first try success",
			moment: &Moment{
				Type: MomentFirstTrySuccess,
				Evidence: Evidence{
					RunCount: 1,
				},
			},
			wantNil:   false,
			wantType:  MomentFirstTrySuccess,
			checkEvid: true,
		},
		{
			name: "topic mastery",
			moment: &Moment{
				Type: MomentTopicMastery,
				Evidence: Evidence{
					TopicName:          "functions",
					ExercisesCompleted: 5,
				},
			},
			wantNil:   false,
			wantType:  MomentTopicMastery,
			checkEvid: true,
		},
		{
			name: "spec complete",
			moment: &Moment{
				Type: MomentSpecComplete,
				Evidence: Evidence{
					SpecName: "user-auth",
				},
			},
			wantNil:   false,
			wantType:  MomentSpecComplete,
			checkEvid: true,
		},
		{
			name: "unknown type returns nil",
			moment: &Moment{
				Type: MomentType("unknown"),
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := g.Generate(tt.moment)

			if tt.wantNil {
				if msg != nil {
					t.Errorf("Generate() = %v, want nil", msg)
				}
				return
			}

			if msg == nil {
				t.Fatal("Generate() returned nil, want message")
			}

			if msg.Type != tt.wantType {
				t.Errorf("msg.Type = %v, want %v", msg.Type, tt.wantType)
			}

			if msg.Text == "" {
				t.Error("msg.Text should not be empty")
			}

			if tt.checkEvid && tt.moment != nil {
				if msg.Evidence != tt.moment.Evidence {
					t.Errorf("msg.Evidence = %v, want %v", msg.Evidence, tt.moment.Evidence)
				}
			}
		})
	}
}

func TestGenerator_FormatTemplate(t *testing.T) {
	g := NewGenerator()

	tests := []struct {
		name     string
		template string
		moment   *Moment
		contains string
	}{
		{
			name:     "no hints format",
			template: "Completed in %d runs",
			moment: &Moment{
				Type:     MomentNoHintsNeeded,
				Evidence: Evidence{RunCount: 5},
			},
			contains: "5",
		},
		{
			name:     "single hint format",
			template: "Completed with %s",
			moment: &Moment{
				Type:     MomentMinimalHints,
				Evidence: Evidence{HintCount: 1},
			},
			contains: "just one hint",
		},
		{
			name:     "multiple hints format",
			template: "Completed with %s",
			moment: &Moment{
				Type:     MomentMinimalHints,
				Evidence: Evidence{HintCount: 2},
			},
			contains: "only 2 hints",
		},
		{
			name:     "no escalation format",
			template: "Worked through with %d hints",
			moment: &Moment{
				Type:     MomentNoEscalation,
				Evidence: Evidence{HintCount: 3},
			},
			contains: "3",
		},
		{
			name:     "quick resolution format",
			template: "Resolved in %s",
			moment: &Moment{
				Type:     MomentQuickResolution,
				Evidence: Evidence{Duration: "5 minutes"},
			},
			contains: "5 minutes",
		},
		{
			name:     "reduced dependency format",
			template: "Dependency dropped by %.0f%%",
			moment: &Moment{
				Type:     MomentReducedDependency,
				Evidence: Evidence{ImprovementPercent: 25.0},
			},
			contains: "25%",
		},
		{
			name:     "topic mastery format",
			template: "Skills in %s",
			moment: &Moment{
				Type:     MomentTopicMastery,
				Evidence: Evidence{TopicName: "testing"},
			},
			contains: "testing",
		},
		{
			name:     "first in topic format",
			template: "First exercise in %s",
			moment: &Moment{
				Type:     MomentFirstInTopic,
				Evidence: Evidence{TopicName: "concurrency"},
			},
			contains: "concurrency",
		},
		{
			name:     "criterion satisfied format",
			template: "Criterion %s satisfied",
			moment: &Moment{
				Type:     MomentCriterionSatisfied,
				Evidence: Evidence{CriterionID: "AC-001"},
			},
			contains: "AC-001",
		},
		{
			name:     "spec complete format",
			template: "Spec %s complete",
			moment: &Moment{
				Type:     MomentSpecComplete,
				Evidence: Evidence{SpecName: "user-auth"},
			},
			contains: "user-auth",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := g.formatTemplate(tt.template, tt.moment)
			if !containsString(result, tt.contains) {
				t.Errorf("formatTemplate() = %q, want to contain %q", result, tt.contains)
			}
		})
	}
}

func TestShouldAppreciate(t *testing.T) {
	tests := []struct {
		name           string
		lastMinutes    int
		priority       int
		want           bool
	}{
		{"high priority always shows", 0, 8, true},
		{"high priority after long time", 120, 9, true},
		{"medium priority after 30 mins", 30, 5, true},
		{"medium priority before 30 mins", 20, 5, false},
		{"medium priority at exactly 30 mins", 30, 6, true},
		{"low priority after 60 mins", 60, 3, true},
		{"low priority before 60 mins", 45, 2, false},
		{"low priority at exactly 60 mins", 60, 1, true},
		{"very low priority after hour", 90, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldAppreciate(tt.lastMinutes, tt.priority)
			if got != tt.want {
				t.Errorf("ShouldAppreciate(%d, %d) = %v, want %v", tt.lastMinutes, tt.priority, got, tt.want)
			}
		})
	}
}

func TestDefaultTemplates(t *testing.T) {
	templates := defaultTemplates()

	expectedTypes := []MomentType{
		MomentNoHintsNeeded,
		MomentMinimalHints,
		MomentNoEscalation,
		MomentFirstTrySuccess,
		MomentQuickResolution,
		MomentAllTestsPassing,
		MomentReducedDependency,
		MomentTopicMastery,
		MomentConsistentSuccess,
		MomentFirstInTopic,
		MomentCriterionSatisfied,
		MomentSpecComplete,
	}

	for _, mt := range expectedTypes {
		templates, ok := templates[mt]
		if !ok {
			t.Errorf("missing templates for moment type %v", mt)
			continue
		}
		if len(templates) == 0 {
			t.Errorf("no templates for moment type %v", mt)
		}
	}
}

func TestMessage_Fields(t *testing.T) {
	msg := Message{
		Text: "Great work!",
		Type: MomentFirstTrySuccess,
		Evidence: Evidence{
			RunCount: 1,
		},
	}

	if msg.Text != "Great work!" {
		t.Errorf("Text = %q, want %q", msg.Text, "Great work!")
	}
	if msg.Type != MomentFirstTrySuccess {
		t.Errorf("Type = %v, want %v", msg.Type, MomentFirstTrySuccess)
	}
	if msg.Evidence.RunCount != 1 {
		t.Errorf("Evidence.RunCount = %d, want 1", msg.Evidence.RunCount)
	}
}

// Helper function
func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
