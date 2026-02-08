package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// cmdSpec manages product specifications (Specular format)
func cmdSpec(args []string) error {
	if len(args) < 1 {
		fmt.Println(`Spec commands (Specular format):

  temper spec create <name>        Create a new spec scaffold
  temper spec list                 List specs in workspace
  temper spec validate <path>      Validate spec completeness
  temper spec status <path>        Show spec progress
  temper spec lock <path>          Generate SpecLock for drift detection
  temper spec drift <path>         Show drift from locked spec

Examples:
  temper spec create "User Authentication"
  temper spec validate .specs/auth.yaml
  temper spec status .specs/auth.yaml`)
		return nil
	}

	switch args[0] {
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("spec name required (e.g., temper spec create \"User Authentication\")")
		}
		return cmdSpecCreate(args[1])
	case "list":
		return cmdSpecList()
	case "validate":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec validate .specs/auth.yaml)")
		}
		return cmdSpecValidate(args[1])
	case "status":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec status .specs/auth.yaml)")
		}
		return cmdSpecStatus(args[1])
	case "lock":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec lock .specs/auth.yaml)")
		}
		return cmdSpecLock(args[1])
	case "drift":
		if len(args) < 2 {
			return fmt.Errorf("spec path required (e.g., temper spec drift .specs/auth.yaml)")
		}
		return cmdSpecDrift(args[1])
	default:
		return fmt.Errorf("unknown spec command: %s", args[0])
	}
}

func cmdSpecCreate(name string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	body := fmt.Sprintf(`{"name": %q}`, name)
	resp, err := http.Post(daemonAddr+"/v1/specs", "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("create spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errResp)
		return fmt.Errorf("create spec failed: %s", errResp.Error)
	}

	var spec struct {
		Name     string `json:"name"`
		FilePath string `json:"file_path"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("✓ Created spec: %s\n", spec.Name)
	fmt.Printf("  File: .specs/%s\n", spec.FilePath)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Edit the spec file to define your features and acceptance criteria")
	fmt.Println("  2. Run 'temper spec validate .specs/" + spec.FilePath + "' to check completeness")
	fmt.Println("  3. Start a feature guidance session with 'temper session --spec .specs/" + spec.FilePath + "'")

	return nil
}

func cmdSpecList() error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	resp, err := http.Get(daemonAddr + "/v1/specs")
	if err != nil {
		return fmt.Errorf("list specs: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Specs []struct {
			Name     string `json:"name"`
			Version  string `json:"version"`
			FilePath string `json:"file_path"`
			Progress struct {
				Satisfied int     `json:"satisfied"`
				Total     int     `json:"total"`
				Percent   float64 `json:"percent"`
			} `json:"progress"`
		} `json:"specs"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if len(result.Specs) == 0 {
		fmt.Println("No specs found in workspace.")
		fmt.Println("Create one with: temper spec create \"Feature Name\"")
		return nil
	}

	fmt.Println("Product Specifications")
	fmt.Println("======================")
	for _, spec := range result.Specs {
		bar := renderProgressBar(spec.Progress.Percent/100, 20)
		fmt.Printf("\n%s (v%s)\n", spec.Name, spec.Version)
		fmt.Printf("  File:     .specs/%s\n", spec.FilePath)
		fmt.Printf("  Progress: %s %d/%d (%.0f%%)\n",
			bar, spec.Progress.Satisfied, spec.Progress.Total, spec.Progress.Percent)
	}

	return nil
}

func cmdSpecValidate(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	url := fmt.Sprintf("%s/v1/specs/%s/validate", daemonAddr, path)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("validate spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec not found: %s", path)
	}

	var validation struct {
		Valid    bool     `json:"valid"`
		Errors   []string `json:"errors"`
		Warnings []string `json:"warnings"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&validation); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if validation.Valid {
		fmt.Println("✓ Spec is valid")
	} else {
		fmt.Println("✗ Spec validation failed")
	}

	if len(validation.Errors) > 0 {
		fmt.Println("\nErrors:")
		for _, e := range validation.Errors {
			fmt.Printf("  ✗ %s\n", e)
		}
	}

	if len(validation.Warnings) > 0 {
		fmt.Println("\nWarnings:")
		for _, w := range validation.Warnings {
			fmt.Printf("  ⚠ %s\n", w)
		}
	}

	return nil
}

func cmdSpecStatus(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	// Get spec details
	url := fmt.Sprintf("%s/v1/specs/%s", daemonAddr, path)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec not found: %s", path)
	}

	var spec struct {
		Name               string   `json:"name"`
		Version            string   `json:"version"`
		Goals              []string `json:"goals"`
		AcceptanceCriteria []struct {
			ID          string `json:"id"`
			Description string `json:"description"`
			Satisfied   bool   `json:"satisfied"`
			Evidence    string `json:"evidence,omitempty"`
		} `json:"acceptance_criteria"`
		Features []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Priority string `json:"priority"`
		} `json:"features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Printf("%s (v%s)\n", spec.Name, spec.Version)
	fmt.Println(strings.Repeat("=", len(spec.Name)+len(spec.Version)+4))

	// Goals
	fmt.Println("\nGoals:")
	for _, goal := range spec.Goals {
		fmt.Printf("  • %s\n", goal)
	}

	// Features
	fmt.Println("\nFeatures:")
	for _, feat := range spec.Features {
		fmt.Printf("  [%s] %s (%s)\n", feat.ID, feat.Title, feat.Priority)
	}

	// Acceptance Criteria
	fmt.Println("\nAcceptance Criteria:")
	satisfied := 0
	for _, ac := range spec.AcceptanceCriteria {
		status := "⏳"
		if ac.Satisfied {
			status = "✓"
			satisfied++
		}
		fmt.Printf("  %s [%s] %s\n", status, ac.ID, ac.Description)
		if ac.Evidence != "" {
			fmt.Printf("      Evidence: %s\n", ac.Evidence)
		}
	}

	// Progress summary
	total := len(spec.AcceptanceCriteria)
	percent := 0.0
	if total > 0 {
		percent = float64(satisfied) / float64(total) * 100
	}
	bar := renderProgressBar(percent/100, 30)
	fmt.Printf("\nProgress: %s %d/%d (%.0f%%)\n", bar, satisfied, total, percent)

	return nil
}

func cmdSpecLock(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	url := fmt.Sprintf("%s/v1/specs/%s/lock", daemonAddr, path)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("lock spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec not found: %s", path)
	}

	if resp.StatusCode == http.StatusUnprocessableEntity {
		return fmt.Errorf("spec must be valid before locking. Run 'temper spec validate %s' first", path)
	}

	var lock struct {
		Version  string `json:"version"`
		SpecHash string `json:"spec_hash"`
		LockedAt string `json:"locked_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&lock); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	fmt.Println("✓ Spec locked successfully")
	fmt.Printf("  Version:  %s\n", lock.Version)
	fmt.Printf("  Hash:     %s\n", lock.SpecHash[:16]+"...")
	fmt.Printf("  Locked:   %s\n", lock.LockedAt)
	fmt.Println()
	fmt.Println("The lock file has been saved to .specs/spec.lock")
	fmt.Println("Use 'temper spec drift' to detect changes from this baseline.")

	return nil
}

func cmdSpecDrift(path string) error {
	if !isRunning() {
		return fmt.Errorf("daemon not running (run 'temper start' first)")
	}

	url := fmt.Sprintf("%s/v1/specs/%s/drift", daemonAddr, path)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("get drift: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("spec or lock not found: %s", path)
	}

	var drift struct {
		HasDrift         bool     `json:"has_drift"`
		VersionChanged   bool     `json:"version_changed"`
		OldVersion       string   `json:"old_version,omitempty"`
		NewVersion       string   `json:"new_version,omitempty"`
		AddedFeatures    []string `json:"added_features"`
		RemovedFeatures  []string `json:"removed_features"`
		ModifiedFeatures []string `json:"modified_features"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&drift); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if !drift.HasDrift {
		fmt.Println("✓ No drift detected - spec matches lock")
		return nil
	}

	fmt.Println("⚠ Drift detected from locked spec")
	fmt.Println()

	if drift.VersionChanged {
		fmt.Printf("Version: %s → %s\n", drift.OldVersion, drift.NewVersion)
	}

	if len(drift.AddedFeatures) > 0 {
		fmt.Println("\nAdded features:")
		for _, f := range drift.AddedFeatures {
			fmt.Printf("  + %s\n", f)
		}
	}

	if len(drift.RemovedFeatures) > 0 {
		fmt.Println("\nRemoved features:")
		for _, f := range drift.RemovedFeatures {
			fmt.Printf("  - %s\n", f)
		}
	}

	if len(drift.ModifiedFeatures) > 0 {
		fmt.Println("\nModified features:")
		for _, f := range drift.ModifiedFeatures {
			fmt.Printf("  ~ %s\n", f)
		}
	}

	fmt.Println()
	fmt.Println("Run 'temper spec lock' to create a new baseline.")

	return nil
}
