package domain

import (
	"testing"
)

func TestExercise_GetHintsForLevel(t *testing.T) {
	exercise := &Exercise{
		ID:    "test-exercise",
		Title: "Test Exercise",
		Hints: HintSet{
			L0: []string{"What problem are you solving?"},
			L1: []string{"Think about string operations"},
			L2: []string{"Look at line 10, consider fmt package"},
			L3: []string{"Use fmt.Sprintf to format the string"},
		},
	}

	tests := []struct {
		name     string
		level    InterventionLevel
		wantLen  int
		wantHint string
	}{
		{"L0 clarify", L0Clarify, 1, "What problem are you solving?"},
		{"L1 category", L1CategoryHint, 1, "Think about string operations"},
		{"L2 location", L2LocationConcept, 1, "Look at line 10, consider fmt package"},
		{"L3 snippet", L3ConstrainedSnippet, 1, "Use fmt.Sprintf to format the string"},
		{"L4 partial (no hints)", L4PartialSolution, 0, ""},
		{"L5 full (no hints)", L5FullSolution, 0, ""},
		{"unknown level", InterventionLevel(99), 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := exercise.GetHintsForLevel(tt.level)
			if len(hints) != tt.wantLen {
				t.Errorf("GetHintsForLevel(%d) len = %d, want %d", tt.level, len(hints), tt.wantLen)
			}
			if tt.wantLen > 0 && hints[0] != tt.wantHint {
				t.Errorf("GetHintsForLevel(%d)[0] = %q, want %q", tt.level, hints[0], tt.wantHint)
			}
		})
	}
}

func TestExercise_GetHintsForLevel_EmptyHints(t *testing.T) {
	exercise := &Exercise{
		ID:    "test-exercise",
		Hints: HintSet{},
	}

	levels := []InterventionLevel{L0Clarify, L1CategoryHint, L2LocationConcept, L3ConstrainedSnippet}
	for _, level := range levels {
		hints := exercise.GetHintsForLevel(level)
		if hints != nil && len(hints) > 0 {
			t.Errorf("GetHintsForLevel(%d) = %v, want empty", level, hints)
		}
	}
}

func TestExercise_AllFiles(t *testing.T) {
	t.Run("both starter and test files", func(t *testing.T) {
		exercise := &Exercise{
			ID: "test",
			StarterCode: map[string]string{
				"main.go": "package main",
			},
			TestCode: map[string]string{
				"main_test.go": "package main",
			},
		}

		files := exercise.AllFiles()
		if len(files) != 2 {
			t.Errorf("AllFiles() len = %d, want 2", len(files))
		}
		if files["main.go"] != "package main" {
			t.Error("AllFiles() should include starter code")
		}
		if files["main_test.go"] != "package main" {
			t.Error("AllFiles() should include test code")
		}
	})

	t.Run("only starter files", func(t *testing.T) {
		exercise := &Exercise{
			ID: "test",
			StarterCode: map[string]string{
				"main.go": "package main",
			},
		}

		files := exercise.AllFiles()
		if len(files) != 1 {
			t.Errorf("AllFiles() len = %d, want 1", len(files))
		}
	})

	t.Run("only test files", func(t *testing.T) {
		exercise := &Exercise{
			ID: "test",
			TestCode: map[string]string{
				"main_test.go": "package main",
			},
		}

		files := exercise.AllFiles()
		if len(files) != 1 {
			t.Errorf("AllFiles() len = %d, want 1", len(files))
		}
	})

	t.Run("no files", func(t *testing.T) {
		exercise := &Exercise{ID: "test"}

		files := exercise.AllFiles()
		if len(files) != 0 {
			t.Errorf("AllFiles() len = %d, want 0", len(files))
		}
	})

	t.Run("overlapping filenames (test overwrites starter)", func(t *testing.T) {
		exercise := &Exercise{
			ID: "test",
			StarterCode: map[string]string{
				"shared.go": "starter content",
			},
			TestCode: map[string]string{
				"shared.go": "test content",
			},
		}

		files := exercise.AllFiles()
		// Test code overwrites starter code when same filename
		if files["shared.go"] != "test content" {
			t.Errorf("AllFiles()[shared.go] = %q, want test content (test overwrites starter)", files["shared.go"])
		}
	})
}

func TestExercise_Struct(t *testing.T) {
	exercise := &Exercise{
		ID:          "go-v1/basics/hello",
		PackID:      "go-v1",
		Title:       "Hello World",
		Description: "Learn to print hello world",
		Difficulty:  DifficultyBeginner,
		StarterCode: map[string]string{"main.go": "package main"},
		TestCode:    map[string]string{"main_test.go": "package main"},
		Rubric: Rubric{
			Criteria: []RubricCriterion{
				{ID: "c1", Name: "Output", Weight: 1.0},
			},
		},
		CheckRecipe: CheckRecipe{
			Format:    true,
			Build:     true,
			Test:      true,
			TestFlags: []string{"-v"},
			Timeout:   30,
		},
		Tags:          []string{"basics", "hello"},
		Prerequisites: []string{},
		Hints:         HintSet{L1: []string{"hint"}},
	}

	if exercise.ID != "go-v1/basics/hello" {
		t.Errorf("ID = %q, want go-v1/basics/hello", exercise.ID)
	}
	if exercise.Difficulty != DifficultyBeginner {
		t.Errorf("Difficulty = %q, want beginner", exercise.Difficulty)
	}
	if !exercise.CheckRecipe.Format {
		t.Error("CheckRecipe.Format should be true")
	}
	if len(exercise.Rubric.Criteria) != 1 {
		t.Errorf("Rubric.Criteria len = %d, want 1", len(exercise.Rubric.Criteria))
	}
}

func TestDifficulty_Constants(t *testing.T) {
	if DifficultyBeginner != "beginner" {
		t.Errorf("DifficultyBeginner = %q, want beginner", DifficultyBeginner)
	}
	if DifficultyIntermediate != "intermediate" {
		t.Errorf("DifficultyIntermediate = %q, want intermediate", DifficultyIntermediate)
	}
	if DifficultyAdvanced != "advanced" {
		t.Errorf("DifficultyAdvanced = %q, want advanced", DifficultyAdvanced)
	}
}

func TestExercisePack_Struct(t *testing.T) {
	pack := &ExercisePack{
		ID:            "go-v1",
		Name:          "Go Fundamentals",
		Version:       "1.0.0",
		Description:   "Learn Go basics",
		Language:      "go",
		DefaultPolicy: DefaultPolicy(),
		ExerciseIDs:   []string{"go-v1/hello", "go-v1/variables"},
	}

	if pack.ID != "go-v1" {
		t.Errorf("ID = %q, want go-v1", pack.ID)
	}
	if len(pack.ExerciseIDs) != 2 {
		t.Errorf("ExerciseIDs len = %d, want 2", len(pack.ExerciseIDs))
	}
}
