package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// cmdExercise manages exercises
func cmdExercise(args []string) error {
	if len(args) < 1 {
		fmt.Println(`Exercise commands:

  temper exercise list              List all exercise packs
  temper exercise info <pack/slug>  Show exercise details`)
		return nil
	}

	switch args[0] {
	case "list":
		return cmdExerciseList()
	case "info":
		if len(args) < 2 {
			return fmt.Errorf("exercise ID required (e.g., go-v1/hello-world)")
		}
		return cmdExerciseInfo(args[1])
	default:
		return fmt.Errorf("unknown exercise command: %s", args[0])
	}
}

func cmdExerciseList() error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	resp, err := http.Get(daemonAddr + "/v1/exercises")
	if err != nil {
		return fmt.Errorf("get exercises: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Packs []struct {
			ID            string `json:"id"`
			Name          string `json:"name"`
			Description   string `json:"description"`
			Language      string `json:"language"`
			ExerciseCount int    `json:"exercise_count"`
		} `json:"packs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("Available Exercise Packs:")
	for _, pack := range result.Packs {
		fmt.Printf("  %s (%s)\n", pack.Name, pack.ID)
		fmt.Printf("    %s\n", pack.Description)
		fmt.Printf("    Language: %s | Exercises: %d\n\n", pack.Language, pack.ExerciseCount)
	}

	fmt.Println("Use 'temper exercise info <pack>/<slug>' for details")
	return nil
}

func cmdExerciseInfo(id string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	parts := strings.Split(id, "/")
	if len(parts) < 2 {
		return fmt.Errorf("exercise ID must be in format: pack/category/slug (e.g., go-v1/basics/hello-world)")
	}

	// Build URL: /v1/exercises/pack/category/slug
	url := fmt.Sprintf("%s/v1/exercises/%s", daemonAddr, id)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get exercise: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("exercise not found: %s", id)
	}

	var exercise struct {
		ID          string   `json:"ID"`
		Title       string   `json:"Title"`
		Description string   `json:"Description"`
		Difficulty  string   `json:"Difficulty"`
		Tags        []string `json:"Tags"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&exercise); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("Exercise: %s\n\n", exercise.Title)
	fmt.Printf("ID:         %s\n", exercise.ID)
	fmt.Printf("Difficulty: %s\n", exercise.Difficulty)
	fmt.Printf("Tags:       %s\n", strings.Join(exercise.Tags, ", "))
	fmt.Printf("\nDescription:\n%s\n", exercise.Description)

	return nil
}
