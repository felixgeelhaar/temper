package exercise_test

import (
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
)

func setupRegistry(t *testing.T) *exercise.Registry {
	t.Helper()

	loader := exercise.NewLoader("../../exercises")
	registry := exercise.NewRegistry(loader)

	if err := registry.Load(); err != nil {
		t.Fatalf("Failed to load exercises: %v", err)
	}

	return registry
}

func TestRegistry_Load(t *testing.T) {
	registry := setupRegistry(t)

	stats := registry.Stats()
	if stats.PackCount == 0 {
		t.Error("No packs loaded")
	}
	if stats.ExerciseCount == 0 {
		t.Error("No exercises loaded")
	}
}

func TestRegistry_ListPacks(t *testing.T) {
	registry := setupRegistry(t)

	packs := registry.ListPacks()
	if len(packs) == 0 {
		t.Fatal("No packs found")
	}

	// Find go-v1 pack
	var goPack *domain.ExercisePack
	for _, p := range packs {
		if p.ID == "go-v1" {
			goPack = p
			break
		}
	}

	if goPack == nil {
		t.Fatal("go-v1 pack not found")
	}

	if goPack.Name == "" {
		t.Error("Pack name should not be empty")
	}
}

func TestRegistry_ListPackExercises(t *testing.T) {
	registry := setupRegistry(t)

	exercises, err := registry.ListPackExercises("go-v1")
	if err != nil {
		t.Fatalf("ListPackExercises failed: %v", err)
	}

	if len(exercises) == 0 {
		t.Fatal("No exercises found in go-v1 pack")
	}

	// Verify we have expected exercises
	// Exercise IDs are in format: packID/category/slug
	hasHelloWorld := false
	hasVariables := false
	for _, ex := range exercises {
		if ex.ID == "go-v1/basics/hello-world" {
			hasHelloWorld = true
		}
		if ex.ID == "go-v1/basics/variables" {
			hasVariables = true
		}
	}

	if !hasHelloWorld {
		t.Error("go-v1/basics/hello-world exercise not found")
	}
	if !hasVariables {
		t.Error("go-v1/basics/variables exercise not found")
	}
}

func TestRegistry_GetExercise(t *testing.T) {
	registry := setupRegistry(t)

	// Get specific exercise - use full ID: packID/category/slug
	ex, err := registry.GetExercise("go-v1/basics/hello-world")
	if err != nil {
		t.Fatalf("GetExercise failed: %v", err)
	}

	if ex.ID != "go-v1/basics/hello-world" {
		t.Errorf("Exercise ID = %q; want %q", ex.ID, "go-v1/basics/hello-world")
	}
	if ex.Title == "" {
		t.Error("Exercise title should not be empty")
	}
	if ex.Description == "" {
		t.Error("Exercise description should not be empty")
	}

	// Verify check recipe is configured
	if ex.CheckRecipe.Timeout == 0 {
		t.Error("Exercise should have check recipe with timeout")
	}
}

func TestRegistry_GetExercise_NotFound(t *testing.T) {
	registry := setupRegistry(t)

	// Get non-existent exercise
	_, err := registry.GetExercise("nonexistent-exercise")
	if err == nil {
		t.Error("GetExercise should fail for non-existent exercise")
	}
}

func TestRegistry_GetPack(t *testing.T) {
	registry := setupRegistry(t)

	pack, err := registry.GetPack("go-v1")
	if err != nil {
		t.Fatalf("GetPack failed: %v", err)
	}

	if pack.ID != "go-v1" {
		t.Errorf("Pack ID = %q; want %q", pack.ID, "go-v1")
	}
	if pack.Name == "" {
		t.Error("Pack name should not be empty")
	}
}

func TestRegistry_GetPack_NotFound(t *testing.T) {
	registry := setupRegistry(t)

	_, err := registry.GetPack("nonexistent-pack")
	if err == nil {
		t.Error("GetPack should fail for non-existent pack")
	}
}

func TestRegistry_GetExercisesByDifficulty(t *testing.T) {
	registry := setupRegistry(t)

	beginnerExercises := registry.GetExercisesByDifficulty(domain.DifficultyBeginner)
	if len(beginnerExercises) == 0 {
		t.Error("Should have beginner exercises")
	}

	for _, ex := range beginnerExercises {
		if ex.Difficulty != domain.DifficultyBeginner {
			t.Errorf("Exercise %s has difficulty %s; want beginner", ex.ID, ex.Difficulty)
		}
	}
}

func TestRegistry_GetExercisesByTag(t *testing.T) {
	registry := setupRegistry(t)

	basicsExercises := registry.GetExercisesByTag("basics")
	if len(basicsExercises) == 0 {
		t.Error("Should have exercises with 'basics' tag")
	}
}

func TestRegistry_Reload(t *testing.T) {
	registry := setupRegistry(t)

	// Get initial stats
	initialStats := registry.Stats()

	// Reload
	if err := registry.Reload(); err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Verify stats are same after reload
	newStats := registry.Stats()
	if newStats.PackCount != initialStats.PackCount {
		t.Errorf("PackCount after reload = %d; want %d", newStats.PackCount, initialStats.PackCount)
	}
	if newStats.ExerciseCount != initialStats.ExerciseCount {
		t.Errorf("ExerciseCount after reload = %d; want %d", newStats.ExerciseCount, initialStats.ExerciseCount)
	}
}

func TestRegistry_Stats(t *testing.T) {
	registry := setupRegistry(t)

	stats := registry.Stats()

	if stats.PackCount < 1 {
		t.Error("Should have at least 1 pack")
	}
	if stats.ExerciseCount < 4 {
		t.Error("Should have at least 4 exercises (original set)")
	}
	if len(stats.ByDifficulty) == 0 {
		t.Error("ByDifficulty should not be empty")
	}
}
