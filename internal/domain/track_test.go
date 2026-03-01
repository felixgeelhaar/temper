package domain

import "testing"

func TestTrack_Validate(t *testing.T) {
	track := &Track{}
	if err := track.Validate(); err == nil {
		t.Error("Validate() should fail for missing ID and name")
	}

	track = &Track{ID: "t", Name: "Test", MaxLevel: L5FullSolution, CooldownSeconds: 0}
	if err := track.Validate(); err != nil {
		t.Errorf("Validate() error = %v; want nil", err)
	}

	track.MaxLevel = L5FullSolution + 1
	if err := track.Validate(); err == nil {
		t.Error("Validate() should fail for invalid max_level")
	}
}

func TestTrack_ToPolicy(t *testing.T) {
	track := &Track{ID: "standard", MaxLevel: L2LocationConcept, CooldownSeconds: 42, PatchingEnabled: true}
	policy := track.ToPolicy()
	if policy.MaxLevel != L2LocationConcept || policy.CooldownSeconds != 42 || policy.PatchingEnabled != true || policy.Track != "standard" {
		t.Errorf("ToPolicy() = %#v; want matching fields", policy)
	}
}

func TestTrack_MarshalUnmarshal(t *testing.T) {
	track := &Track{ID: "custom", Name: "Custom", MaxLevel: L2LocationConcept, CooldownSeconds: 10}
	data, err := track.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error = %v", err)
	}

	parsed, err := UnmarshalTrackYAML(data)
	if err != nil {
		t.Fatalf("UnmarshalTrackYAML() error = %v", err)
	}
	if parsed.ID != "custom" || parsed.Name != "Custom" {
		t.Errorf("UnmarshalTrackYAML() = %#v; want ID custom and Name Custom", parsed)
	}
}

func TestBuiltinTracks(t *testing.T) {
	tracks := BuiltinTracks()
	if len(tracks) < 3 {
		t.Errorf("BuiltinTracks() = %d; want at least 3", len(tracks))
	}

	foundBeginner := false
	for _, tr := range tracks {
		if tr.ID == "beginner" {
			foundBeginner = true
		}
	}
	if !foundBeginner {
		t.Error("BuiltinTracks() should include beginner track")
	}
}

func TestTrack_ShouldEvaluateAutoProgress(t *testing.T) {
	track := &Track{
		MaxLevel: L2LocationConcept,
		AutoProgress: AutoProgressRules{
			Enabled:             true,
			PromoteAfterStreak:  2,
			DemoteAfterFailures: 2,
			MinSkillForPromote:  0.5,
		},
	}

	newLevel := track.ShouldEvaluateAutoProgress(2, 0, 0.6)
	if newLevel == nil || *newLevel != L1CategoryHint {
		t.Errorf("ShouldEvaluateAutoProgress() promote = %v; want %v", newLevel, L1CategoryHint)
	}

	newLevel = track.ShouldEvaluateAutoProgress(0, 2, 0.2)
	if newLevel == nil || *newLevel != L3ConstrainedSnippet {
		t.Errorf("ShouldEvaluateAutoProgress() demote = %v; want %v", newLevel, L3ConstrainedSnippet)
	}

	track.AutoProgress.Enabled = false
	if track.ShouldEvaluateAutoProgress(10, 10, 1.0) != nil {
		t.Error("ShouldEvaluateAutoProgress() should return nil when disabled")
	}
}
