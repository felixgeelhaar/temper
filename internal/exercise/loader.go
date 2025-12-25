package exercise

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/felixgeelhaar/temper/internal/domain"
	"gopkg.in/yaml.v3"
)

// PackFile represents the YAML structure for an exercise pack
type PackFile struct {
	ID              string   `yaml:"id"`
	Name            string   `yaml:"name"`
	Version         string   `yaml:"version"`
	Description     string   `yaml:"description"`
	Language        string   `yaml:"language"`
	DifficultyRange []string `yaml:"difficulty_range"`
	DefaultPolicy   struct {
		MaxLevel        int    `yaml:"max_level"`
		PatchingEnabled bool   `yaml:"patching_enabled"`
		CooldownSeconds int    `yaml:"cooldown_seconds"`
		Track           string `yaml:"track"`
	} `yaml:"default_policy"`
	Exercises []string `yaml:"exercises"`
}

// ExerciseFile represents the YAML structure for an exercise
type ExerciseFile struct {
	ID            string            `yaml:"id"`
	Title         string            `yaml:"title"`
	Description   string            `yaml:"description"`
	Difficulty    string            `yaml:"difficulty"`
	Tags          []string          `yaml:"tags"`
	Prerequisites []string          `yaml:"prerequisites"`
	Starter       map[string]string `yaml:"starter"`
	Tests         map[string]string `yaml:"tests"`
	CheckRecipe   struct {
		Format    bool     `yaml:"format"`
		Build     bool     `yaml:"build"`
		Test      bool     `yaml:"test"`
		TestFlags []string `yaml:"test_flags"`
		Timeout   int      `yaml:"timeout"`
	} `yaml:"check_recipe"`
	Rubric struct {
		Criteria []struct {
			ID          string   `yaml:"id"`
			Name        string   `yaml:"name"`
			Description string   `yaml:"description"`
			Weight      float64  `yaml:"weight"`
			Signals     []string `yaml:"signals"`
		} `yaml:"criteria"`
	} `yaml:"rubric"`
	Hints struct {
		L0 []string `yaml:"L0"`
		L1 []string `yaml:"L1"`
		L2 []string `yaml:"L2"`
		L3 []string `yaml:"L3"`
	} `yaml:"hints"`
	Solution map[string]string `yaml:"solution"`
}

// Loader handles loading exercises from YAML files
type Loader struct {
	basePath string
}

// NewLoader creates a new exercise loader
func NewLoader(basePath string) *Loader {
	return &Loader{basePath: basePath}
}

// LoadPack loads an exercise pack from a directory
func (l *Loader) LoadPack(packID string) (*domain.ExercisePack, error) {
	packPath := filepath.Join(l.basePath, packID, "pack.yaml")

	data, err := os.ReadFile(packPath)
	if err != nil {
		return nil, fmt.Errorf("read pack file: %w", err)
	}

	var packFile PackFile
	if err := yaml.Unmarshal(data, &packFile); err != nil {
		return nil, fmt.Errorf("parse pack file: %w", err)
	}

	pack := &domain.ExercisePack{
		ID:          packFile.ID,
		Name:        packFile.Name,
		Version:     packFile.Version,
		Description: packFile.Description,
		Language:    packFile.Language,
		DefaultPolicy: domain.LearningPolicy{
			MaxLevel:        domain.InterventionLevel(packFile.DefaultPolicy.MaxLevel),
			PatchingEnabled: packFile.DefaultPolicy.PatchingEnabled,
			CooldownSeconds: packFile.DefaultPolicy.CooldownSeconds,
			Track:           packFile.DefaultPolicy.Track,
		},
		ExerciseIDs: make([]string, len(packFile.Exercises)),
	}

	for i, ex := range packFile.Exercises {
		pack.ExerciseIDs[i] = fmt.Sprintf("%s/%s", packID, ex)
	}

	return pack, nil
}

// LoadExercise loads a single exercise from a YAML file
func (l *Loader) LoadExercise(packID, slug string) (*domain.Exercise, error) {
	// Build path: basePath/packID/category/exercise.yaml
	parts := strings.Split(slug, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid exercise slug: %s", slug)
	}

	exercisePath := filepath.Join(l.basePath, packID, slug+".yaml")

	data, err := os.ReadFile(exercisePath)
	if err != nil {
		return nil, fmt.Errorf("read exercise file: %w", err)
	}

	var exFile ExerciseFile
	if err := yaml.Unmarshal(data, &exFile); err != nil {
		return nil, fmt.Errorf("parse exercise file: %w", err)
	}

	exercise := &domain.Exercise{
		ID:            fmt.Sprintf("%s/%s", packID, slug),
		PackID:        packID,
		Title:         exFile.Title,
		Description:   exFile.Description,
		Difficulty:    domain.Difficulty(exFile.Difficulty),
		StarterCode:   exFile.Starter,
		TestCode:      exFile.Tests,
		Tags:          exFile.Tags,
		Prerequisites: exFile.Prerequisites,
		CheckRecipe: domain.CheckRecipe{
			Format:    exFile.CheckRecipe.Format,
			Build:     exFile.CheckRecipe.Build,
			Test:      exFile.CheckRecipe.Test,
			TestFlags: exFile.CheckRecipe.TestFlags,
			Timeout:   exFile.CheckRecipe.Timeout,
		},
		Hints: domain.HintSet{
			L0: exFile.Hints.L0,
			L1: exFile.Hints.L1,
			L2: exFile.Hints.L2,
			L3: exFile.Hints.L3,
		},
	}

	// Build rubric
	exercise.Rubric.Criteria = make([]domain.RubricCriterion, len(exFile.Rubric.Criteria))
	for i, c := range exFile.Rubric.Criteria {
		exercise.Rubric.Criteria[i] = domain.RubricCriterion{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			Weight:      c.Weight,
			Signals:     c.Signals,
		}
	}

	return exercise, nil
}

// LoadAllPacks loads all exercise packs from the base directory
func (l *Loader) LoadAllPacks() ([]*domain.ExercisePack, error) {
	entries, err := os.ReadDir(l.basePath)
	if err != nil {
		return nil, fmt.Errorf("read exercises directory: %w", err)
	}

	var packs []*domain.ExercisePack
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		packPath := filepath.Join(l.basePath, entry.Name(), "pack.yaml")
		if _, err := os.Stat(packPath); os.IsNotExist(err) {
			continue
		}

		pack, err := l.LoadPack(entry.Name())
		if err != nil {
			return nil, fmt.Errorf("load pack %s: %w", entry.Name(), err)
		}
		packs = append(packs, pack)
	}

	return packs, nil
}

// LoadPackExercises loads all exercises for a pack
func (l *Loader) LoadPackExercises(packID string) ([]*domain.Exercise, error) {
	pack, err := l.LoadPack(packID)
	if err != nil {
		return nil, err
	}

	exercises := make([]*domain.Exercise, 0, len(pack.ExerciseIDs))
	for _, exID := range pack.ExerciseIDs {
		// Extract slug from full ID (packID/category/exercise -> category/exercise)
		slug := strings.TrimPrefix(exID, packID+"/")

		exercise, err := l.LoadExercise(packID, slug)
		if err != nil {
			return nil, fmt.Errorf("load exercise %s: %w", exID, err)
		}
		exercises = append(exercises, exercise)
	}

	return exercises, nil
}
