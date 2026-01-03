package pairing

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/session"
)

func TestInterventionContext_HasSpec(t *testing.T) {
	tests := []struct {
		name string
		ctx  InterventionContext
		want bool
	}{
		{
			name: "has spec",
			ctx:  InterventionContext{Spec: &domain.ProductSpec{}},
			want: true,
		},
		{
			name: "no spec",
			ctx:  InterventionContext{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.HasSpec(); got != tt.want {
				t.Errorf("HasSpec() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestInterventionContext_IsFeatureGuidance(t *testing.T) {
	tests := []struct {
		name string
		ctx  InterventionContext
		want bool
	}{
		{
			name: "feature guidance with spec",
			ctx: InterventionContext{
				SessionIntent: session.IntentFeatureGuidance,
				Spec:          &domain.ProductSpec{},
			},
			want: true,
		},
		{
			name: "feature guidance without spec",
			ctx: InterventionContext{
				SessionIntent: session.IntentFeatureGuidance,
			},
			want: false,
		},
		{
			name: "training intent with spec",
			ctx: InterventionContext{
				SessionIntent: session.IntentTraining,
				Spec:          &domain.ProductSpec{},
			},
			want: false,
		},
		{
			name: "no intent",
			ctx: InterventionContext{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ctx.IsFeatureGuidance(); got != tt.want {
				t.Errorf("IsFeatureGuidance() = %v; want %v", got, tt.want)
			}
		})
	}
}

func TestInterventionContext_GetNextUnsatisfiedCriterion(t *testing.T) {
	tests := []struct {
		name string
		ctx  InterventionContext
		want string // criterion ID or empty
	}{
		{
			name: "no spec",
			ctx:  InterventionContext{},
			want: "",
		},
		{
			name: "all satisfied",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: true},
					},
				},
			},
			want: "",
		},
		{
			name: "first unsatisfied",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: false},
						{ID: "ac-3", Satisfied: false},
					},
				},
			},
			want: "ac-2",
		},
		{
			name: "first is unsatisfied",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: false},
					},
				},
			},
			want: "ac-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.GetNextUnsatisfiedCriterion()
			if tt.want == "" {
				if got != nil {
					t.Errorf("GetNextUnsatisfiedCriterion() = %v; want nil", got.ID)
				}
			} else {
				if got == nil {
					t.Errorf("GetNextUnsatisfiedCriterion() = nil; want %s", tt.want)
				} else if got.ID != tt.want {
					t.Errorf("GetNextUnsatisfiedCriterion().ID = %s; want %s", got.ID, tt.want)
				}
			}
		})
	}
}

func TestInterventionContext_GetCurrentFeature(t *testing.T) {
	tests := []struct {
		name string
		ctx  InterventionContext
		want string // feature ID or empty
	}{
		{
			name: "no spec",
			ctx:  InterventionContext{},
			want: "",
		},
		{
			name: "no focus criterion",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					Features: []domain.Feature{{ID: "f1"}},
				},
			},
			want: "",
		},
		{
			name: "high priority feature first",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					Features: []domain.Feature{
						{ID: "f1", Priority: domain.PriorityLow},
						{ID: "f2", Priority: domain.PriorityHigh},
					},
				},
				FocusCriterion: &domain.AcceptanceCriterion{ID: "ac-1"},
			},
			want: "f2",
		},
		{
			name: "first feature when no high priority",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					Features: []domain.Feature{
						{ID: "f1", Priority: domain.PriorityLow},
						{ID: "f2", Priority: domain.PriorityMedium},
					},
				},
				FocusCriterion: &domain.AcceptanceCriterion{ID: "ac-1"},
			},
			want: "f1",
		},
		{
			name: "no features",
			ctx: InterventionContext{
				Spec:           &domain.ProductSpec{},
				FocusCriterion: &domain.AcceptanceCriterion{ID: "ac-1"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.GetCurrentFeature()
			if tt.want == "" {
				if got != nil {
					t.Errorf("GetCurrentFeature() = %v; want nil", got.ID)
				}
			} else {
				if got == nil {
					t.Errorf("GetCurrentFeature() = nil; want %s", tt.want)
				} else if got.ID != tt.want {
					t.Errorf("GetCurrentFeature().ID = %s; want %s", got.ID, tt.want)
				}
			}
		})
	}
}

func TestInterventionContext_SpecProgress(t *testing.T) {
	tests := []struct {
		name          string
		ctx           InterventionContext
		wantSatisfied int
		wantTotal     int
	}{
		{
			name:          "no spec",
			ctx:           InterventionContext{},
			wantSatisfied: 0,
			wantTotal:     0,
		},
		{
			name: "empty criteria",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{},
			},
			wantSatisfied: 0,
			wantTotal:     0,
		},
		{
			name: "some satisfied",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: false},
						{ID: "ac-3", Satisfied: true},
					},
				},
			},
			wantSatisfied: 2,
			wantTotal:     3,
		},
		{
			name: "all satisfied",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					AcceptanceCriteria: []domain.AcceptanceCriterion{
						{ID: "ac-1", Satisfied: true},
						{ID: "ac-2", Satisfied: true},
					},
				},
			},
			wantSatisfied: 2,
			wantTotal:     2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			satisfied, total := tt.ctx.SpecProgress()
			if satisfied != tt.wantSatisfied {
				t.Errorf("SpecProgress() satisfied = %d; want %d", satisfied, tt.wantSatisfied)
			}
			if total != tt.wantTotal {
				t.Errorf("SpecProgress() total = %d; want %d", total, tt.wantTotal)
			}
		})
	}
}

func TestInterventionContext_CheckScopeDrift(t *testing.T) {
	tests := []struct {
		name    string
		ctx     InterventionContext
		wantNil bool
	}{
		{
			name:    "no spec returns nil",
			ctx:     InterventionContext{},
			wantNil: true,
		},
		{
			name: "with spec returns indicators",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					Features: []domain.Feature{
						{ID: "f1", API: &domain.APISpec{Path: "/api/v1/users"}},
					},
				},
			},
			wantNil: false,
		},
		{
			name: "spec without API features",
			ctx: InterventionContext{
				Spec: &domain.ProductSpec{
					Features: []domain.Feature{
						{ID: "f1", Title: "No API"},
					},
				},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.CheckScopeDrift()
			if tt.wantNil {
				if got != nil {
					t.Errorf("CheckScopeDrift() = %v; want nil", got)
				}
			} else {
				if got == nil {
					t.Error("CheckScopeDrift() = nil; want non-nil")
				}
			}
		})
	}
}

func TestCheckScopeDrift_APICollection(t *testing.T) {
	ctx := InterventionContext{
		Spec: &domain.ProductSpec{
			Features: []domain.Feature{
				{ID: "f1", API: &domain.APISpec{Path: "/api/v1/users"}},
				{ID: "f2", API: &domain.APISpec{Path: "/api/v1/posts"}},
				{ID: "f3"}, // No API
			},
		},
	}

	indicators := ctx.CheckScopeDrift()
	if indicators == nil {
		t.Fatal("CheckScopeDrift() returned nil")
	}

	// Should collect missing spec APIs
	if len(indicators.MissingSpecAPIs) != 2 {
		t.Errorf("MissingSpecAPIs count = %d; want 2", len(indicators.MissingSpecAPIs))
	}
}
