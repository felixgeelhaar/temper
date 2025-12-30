package exercise

import (
	"fmt"
	"sync"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Registry provides access to exercises and packs
type Registry struct {
	loader    *Loader
	mu        sync.RWMutex
	packs     map[string]*domain.ExercisePack
	exercises map[string]*domain.Exercise
	loaded    bool
}

// NewRegistry creates a new exercise registry
func NewRegistry(loader *Loader) *Registry {
	return &Registry{
		loader:    loader,
		packs:     make(map[string]*domain.ExercisePack),
		exercises: make(map[string]*domain.Exercise),
	}
}

// Load loads all packs and exercises into memory
func (r *Registry) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	packs, err := r.loader.LoadAllPacks()
	if err != nil {
		return fmt.Errorf("load packs: %w", err)
	}

	for _, pack := range packs {
		r.packs[pack.ID] = pack

		exercises, err := r.loader.LoadPackExercises(pack.ID)
		if err != nil {
			return fmt.Errorf("load exercises for pack %s: %w", pack.ID, err)
		}

		for _, ex := range exercises {
			r.exercises[ex.ID] = ex
		}
	}

	r.loaded = true
	return nil
}

// Reload reloads all exercises (useful for development)
func (r *Registry) Reload() error {
	r.mu.Lock()
	r.packs = make(map[string]*domain.ExercisePack)
	r.exercises = make(map[string]*domain.Exercise)
	r.loaded = false
	r.mu.Unlock()

	return r.Load()
}

// GetPack returns a pack by ID
func (r *Registry) GetPack(id string) (*domain.ExercisePack, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pack, ok := r.packs[id]
	if !ok {
		return nil, fmt.Errorf("pack not found: %s", id)
	}
	return pack, nil
}

// GetExercise returns an exercise by ID
func (r *Registry) GetExercise(id string) (*domain.Exercise, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exercise, ok := r.exercises[id]
	if !ok {
		return nil, fmt.Errorf("exercise not found: %s", id)
	}
	return exercise, nil
}

// ListPacks returns all packs
func (r *Registry) ListPacks() []*domain.ExercisePack {
	r.mu.RLock()
	defer r.mu.RUnlock()

	packs := make([]*domain.ExercisePack, 0, len(r.packs))
	for _, pack := range r.packs {
		packs = append(packs, pack)
	}
	return packs
}

// ListExercises returns all exercises
func (r *Registry) ListExercises() []*domain.Exercise {
	r.mu.RLock()
	defer r.mu.RUnlock()

	exercises := make([]*domain.Exercise, 0, len(r.exercises))
	for _, ex := range r.exercises {
		exercises = append(exercises, ex)
	}
	return exercises
}

// ListPackExercises returns all exercises for a pack
func (r *Registry) ListPackExercises(packID string) ([]*domain.Exercise, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	pack, ok := r.packs[packID]
	if !ok {
		return nil, fmt.Errorf("pack not found: %s", packID)
	}

	exercises := make([]*domain.Exercise, 0, len(pack.ExerciseIDs))
	for _, exID := range pack.ExerciseIDs {
		if ex, ok := r.exercises[exID]; ok {
			exercises = append(exercises, ex)
		}
	}
	return exercises, nil
}

// GetExercisesByDifficulty returns exercises filtered by difficulty
func (r *Registry) GetExercisesByDifficulty(difficulty domain.Difficulty) []*domain.Exercise {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var exercises []*domain.Exercise
	for _, ex := range r.exercises {
		if ex.Difficulty == difficulty {
			exercises = append(exercises, ex)
		}
	}
	return exercises
}

// GetNextExercise returns the next exercise in the pack after the given exercise ID
// Returns nil if there is no next exercise (current is the last one)
func (r *Registry) GetNextExercise(currentExerciseID string) (*domain.Exercise, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Find which pack contains this exercise
	var pack *domain.ExercisePack
	for _, p := range r.packs {
		for _, exID := range p.ExerciseIDs {
			if exID == currentExerciseID {
				pack = p
				break
			}
		}
		if pack != nil {
			break
		}
	}

	if pack == nil {
		return nil, fmt.Errorf("exercise not found in any pack: %s", currentExerciseID)
	}

	// Find the current exercise index and return the next one
	for i, exID := range pack.ExerciseIDs {
		if exID == currentExerciseID {
			// Check if there's a next exercise
			if i+1 < len(pack.ExerciseIDs) {
				nextID := pack.ExerciseIDs[i+1]
				if nextEx, ok := r.exercises[nextID]; ok {
					return nextEx, nil
				}
			}
			// No next exercise - this is the last one
			return nil, nil
		}
	}

	return nil, fmt.Errorf("exercise not found: %s", currentExerciseID)
}

// GetExercisesByTag returns exercises that have a specific tag
func (r *Registry) GetExercisesByTag(tag string) []*domain.Exercise {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var exercises []*domain.Exercise
	for _, ex := range r.exercises {
		for _, t := range ex.Tags {
			if t == tag {
				exercises = append(exercises, ex)
				break
			}
		}
	}
	return exercises
}

// Stats returns statistics about loaded exercises
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		PackCount:     len(r.packs),
		ExerciseCount: len(r.exercises),
		ByDifficulty:  make(map[string]int),
	}

	for _, ex := range r.exercises {
		stats.ByDifficulty[string(ex.Difficulty)]++
	}

	return stats
}

// RegistryStats holds statistics about the registry
type RegistryStats struct {
	PackCount     int
	ExerciseCount int
	ByDifficulty  map[string]int
}
