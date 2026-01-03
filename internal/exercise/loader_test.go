package exercise

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader("/test/path")
	if loader == nil {
		t.Fatal("NewLoader returned nil")
	}
	if loader.basePath != "/test/path" {
		t.Errorf("basePath = %q, want %q", loader.basePath, "/test/path")
	}
}

func TestLoader_BasePath(t *testing.T) {
	loader := NewLoader("/exercises")
	if got := loader.BasePath(); got != "/exercises" {
		t.Errorf("BasePath() = %q, want %q", got, "/exercises")
	}
}

func TestLoader_LoadPack(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pack directory
	packDir := filepath.Join(tmpDir, "test-pack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	packYAML := `id: test-pack
name: Test Pack
version: "1.0.0"
description: A test exercise pack
language: go
difficulty_range:
  - beginner
  - intermediate
default_policy:
  max_level: 3
  patching_enabled: true
  cooldown_seconds: 60
  track: practice
exercises:
  - basics/hello
  - basics/variables
`
	if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(packYAML), 0644); err != nil {
		t.Fatalf("failed to write pack.yaml: %v", err)
	}

	loader := NewLoader(tmpDir)

	pack, err := loader.LoadPack("test-pack")
	if err != nil {
		t.Fatalf("LoadPack() error = %v", err)
	}

	if pack.ID != "test-pack" {
		t.Errorf("pack.ID = %q, want %q", pack.ID, "test-pack")
	}
	if pack.Name != "Test Pack" {
		t.Errorf("pack.Name = %q, want %q", pack.Name, "Test Pack")
	}
	if pack.Version != "1.0.0" {
		t.Errorf("pack.Version = %q, want %q", pack.Version, "1.0.0")
	}
	if pack.Language != "go" {
		t.Errorf("pack.Language = %q, want %q", pack.Language, "go")
	}
	if len(pack.ExerciseIDs) != 2 {
		t.Errorf("len(pack.ExerciseIDs) = %d, want 2", len(pack.ExerciseIDs))
	}
	if pack.DefaultPolicy.MaxLevel != 3 {
		t.Errorf("pack.DefaultPolicy.MaxLevel = %d, want 3", pack.DefaultPolicy.MaxLevel)
	}
	if pack.DefaultPolicy.CooldownSeconds != 60 {
		t.Errorf("pack.DefaultPolicy.CooldownSeconds = %d, want 60", pack.DefaultPolicy.CooldownSeconds)
	}
}

func TestLoader_LoadPack_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	_, err := loader.LoadPack("nonexistent")
	if err == nil {
		t.Error("LoadPack() should fail for non-existent pack")
	}
}

func TestLoader_LoadPack_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	packDir := filepath.Join(tmpDir, "invalid-pack")
	if err := os.MkdirAll(packDir, 0755); err != nil {
		t.Fatalf("failed to create pack dir: %v", err)
	}

	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte("invalid: [yaml"), 0644); err != nil {
		t.Fatalf("failed to write pack.yaml: %v", err)
	}

	loader := NewLoader(tmpDir)

	_, err := loader.LoadPack("invalid-pack")
	if err == nil {
		t.Error("LoadPack() should fail for invalid YAML")
	}
}

func TestLoader_LoadExercise(t *testing.T) {
	tmpDir := t.TempDir()

	// Create exercise directory structure
	exDir := filepath.Join(tmpDir, "go-v1", "basics")
	if err := os.MkdirAll(exDir, 0755); err != nil {
		t.Fatalf("failed to create exercise dir: %v", err)
	}

	exerciseYAML := `id: basics/hello
title: Hello World
description: Write a hello world program
difficulty: beginner
tags:
  - basics
  - output
prerequisites:
  - none
starter:
  main.go: |
    package main
    func main() {}
tests:
  main_test.go: |
    package main
    import "testing"
    func TestHello(t *testing.T) {}
check_recipe:
  format: true
  build: true
  test: true
  test_flags:
    - -v
  timeout: 30
rubric:
  criteria:
    - id: compiles
      name: Compiles
      description: Program compiles
      weight: 50
      signals:
        - build_ok
    - id: output
      name: Output
      description: Correct output
      weight: 50
hints:
  L0:
    - What function prints text?
  L1:
    - Use fmt.Println
  L2:
    - The main function is the entry point
  L3:
    - "fmt.Println(\"Hello\")"
solution:
  main.go: |
    package main
    import "fmt"
    func main() { fmt.Println("Hello") }
`
	if err := os.WriteFile(filepath.Join(exDir, "hello.yaml"), []byte(exerciseYAML), 0644); err != nil {
		t.Fatalf("failed to write exercise YAML: %v", err)
	}

	loader := NewLoader(tmpDir)

	ex, err := loader.LoadExercise("go-v1", "basics/hello")
	if err != nil {
		t.Fatalf("LoadExercise() error = %v", err)
	}

	if ex.ID != "go-v1/basics/hello" {
		t.Errorf("ex.ID = %q, want %q", ex.ID, "go-v1/basics/hello")
	}
	if ex.PackID != "go-v1" {
		t.Errorf("ex.PackID = %q, want %q", ex.PackID, "go-v1")
	}
	if ex.Title != "Hello World" {
		t.Errorf("ex.Title = %q, want %q", ex.Title, "Hello World")
	}
	if ex.Difficulty != "beginner" {
		t.Errorf("ex.Difficulty = %q, want %q", ex.Difficulty, "beginner")
	}
	if len(ex.Tags) != 2 {
		t.Errorf("len(ex.Tags) = %d, want 2", len(ex.Tags))
	}
	if len(ex.StarterCode) != 1 {
		t.Errorf("len(ex.StarterCode) = %d, want 1", len(ex.StarterCode))
	}
	if len(ex.TestCode) != 1 {
		t.Errorf("len(ex.TestCode) = %d, want 1", len(ex.TestCode))
	}
	if !ex.CheckRecipe.Format {
		t.Error("ex.CheckRecipe.Format should be true")
	}
	if !ex.CheckRecipe.Build {
		t.Error("ex.CheckRecipe.Build should be true")
	}
	if !ex.CheckRecipe.Test {
		t.Error("ex.CheckRecipe.Test should be true")
	}
	if ex.CheckRecipe.Timeout != 30 {
		t.Errorf("ex.CheckRecipe.Timeout = %d, want 30", ex.CheckRecipe.Timeout)
	}
	if len(ex.Rubric.Criteria) != 2 {
		t.Errorf("len(ex.Rubric.Criteria) = %d, want 2", len(ex.Rubric.Criteria))
	}
	if len(ex.Hints.L0) != 1 {
		t.Errorf("len(ex.Hints.L0) = %d, want 1", len(ex.Hints.L0))
	}
	if len(ex.Hints.L1) != 1 {
		t.Errorf("len(ex.Hints.L1) = %d, want 1", len(ex.Hints.L1))
	}
}

func TestLoader_LoadExercise_InvalidSlug(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	// Slug must have at least 2 parts (category/exercise)
	_, err := loader.LoadExercise("go-v1", "single")
	if err == nil {
		t.Error("LoadExercise() should fail for invalid slug")
	}
}

func TestLoader_LoadExercise_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	_, err := loader.LoadExercise("go-v1", "basics/nonexistent")
	if err == nil {
		t.Error("LoadExercise() should fail for non-existent exercise")
	}
}

func TestLoader_LoadAllPacks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple packs
	for _, packID := range []string{"go-v1", "python-v1"} {
		packDir := filepath.Join(tmpDir, packID)
		if err := os.MkdirAll(packDir, 0755); err != nil {
			t.Fatalf("failed to create pack dir: %v", err)
		}

		packYAML := `id: ` + packID + `
name: Test Pack
description: Test
language: go
exercises: []
`
		if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(packYAML), 0644); err != nil {
			t.Fatalf("failed to write pack.yaml: %v", err)
		}
	}

	// Create a directory without pack.yaml (should be skipped)
	if err := os.MkdirAll(filepath.Join(tmpDir, "not-a-pack"), 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	// Create a file (should be skipped)
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	loader := NewLoader(tmpDir)

	packs, err := loader.LoadAllPacks()
	if err != nil {
		t.Fatalf("LoadAllPacks() error = %v", err)
	}

	if len(packs) != 2 {
		t.Errorf("len(packs) = %d, want 2", len(packs))
	}
}

func TestLoader_LoadAllPacks_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	packs, err := loader.LoadAllPacks()
	if err != nil {
		t.Fatalf("LoadAllPacks() error = %v", err)
	}

	if len(packs) != 0 {
		t.Errorf("len(packs) = %d, want 0", len(packs))
	}
}

func TestLoader_LoadAllPacks_NonExistentDir(t *testing.T) {
	loader := NewLoader("/nonexistent/path")

	_, err := loader.LoadAllPacks()
	if err == nil {
		t.Error("LoadAllPacks() should fail for non-existent directory")
	}
}

func TestLoader_LoadPackExercises(t *testing.T) {
	tmpDir := t.TempDir()

	// Create pack with exercises
	packDir := filepath.Join(tmpDir, "go-v1")
	basicDir := filepath.Join(packDir, "basics")
	if err := os.MkdirAll(basicDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	packYAML := `id: go-v1
name: Go Pack
description: Go exercises
language: go
exercises:
  - basics/hello
  - basics/vars
`
	if err := os.WriteFile(filepath.Join(packDir, "pack.yaml"), []byte(packYAML), 0644); err != nil {
		t.Fatalf("failed to write pack.yaml: %v", err)
	}

	// Create exercise files
	for _, slug := range []string{"hello", "vars"} {
		exerciseYAML := `id: basics/` + slug + `
title: ` + slug + `
description: Test
difficulty: beginner
`
		if err := os.WriteFile(filepath.Join(basicDir, slug+".yaml"), []byte(exerciseYAML), 0644); err != nil {
			t.Fatalf("failed to write exercise YAML: %v", err)
		}
	}

	loader := NewLoader(tmpDir)

	exercises, err := loader.LoadPackExercises("go-v1")
	if err != nil {
		t.Fatalf("LoadPackExercises() error = %v", err)
	}

	if len(exercises) != 2 {
		t.Errorf("len(exercises) = %d, want 2", len(exercises))
	}
}

func TestLoader_LoadPackExercises_PackNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewLoader(tmpDir)

	_, err := loader.LoadPackExercises("nonexistent")
	if err == nil {
		t.Error("LoadPackExercises() should fail for non-existent pack")
	}
}
