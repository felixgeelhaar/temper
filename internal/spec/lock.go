package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// GenerateLock creates a SpecLock from a ProductSpec
// Uses SHA256 for hashing (can be swapped to blake3 with lukechampine.com/blake3)
func GenerateLock(spec *domain.ProductSpec) (*domain.SpecLock, error) {
	lock := &domain.SpecLock{
		Version:  spec.Version,
		Features: make(map[string]domain.LockedFeature),
		LockedAt: time.Now(),
	}

	// Generate hash for the entire spec
	specHash, err := hashSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("hash spec: %w", err)
	}
	lock.SpecHash = specHash

	// Generate hash for each feature
	for _, feat := range spec.Features {
		featHash, err := hashFeature(&feat)
		if err != nil {
			return nil, fmt.Errorf("hash feature %s: %w", feat.ID, err)
		}

		locked := domain.LockedFeature{
			Hash: featHash,
		}

		if feat.API != nil {
			locked.APIPath = feat.API.Path
		}

		lock.Features[feat.ID] = locked
	}

	return lock, nil
}

// VerifyLock checks if a spec matches its lock
func VerifyLock(spec *domain.ProductSpec, lock *domain.SpecLock) (bool, []string) {
	var drifts []string

	// Check version
	if spec.Version != lock.Version {
		drifts = append(drifts, fmt.Sprintf("version changed: %s -> %s", lock.Version, spec.Version))
	}

	// Check spec hash
	currentHash, err := hashSpec(spec)
	if err == nil && currentHash != lock.SpecHash {
		drifts = append(drifts, "spec content has changed")
	}

	// Check each locked feature
	specFeatures := make(map[string]*domain.Feature)
	for i := range spec.Features {
		specFeatures[spec.Features[i].ID] = &spec.Features[i]
	}

	// Check for modified or removed features
	for id, locked := range lock.Features {
		feat, exists := specFeatures[id]
		if !exists {
			drifts = append(drifts, fmt.Sprintf("feature removed: %s", id))
			continue
		}

		currentHash, err := hashFeature(feat)
		if err == nil && currentHash != locked.Hash {
			drifts = append(drifts, fmt.Sprintf("feature modified: %s", id))
		}
	}

	// Check for new features
	for id := range specFeatures {
		if _, exists := lock.Features[id]; !exists {
			drifts = append(drifts, fmt.Sprintf("feature added: %s", id))
		}
	}

	return len(drifts) == 0, drifts
}

// CalculateDrift returns detailed drift information between spec and lock
func CalculateDrift(spec *domain.ProductSpec, lock *domain.SpecLock) *DriftReport {
	report := &DriftReport{
		HasDrift:         false,
		AddedFeatures:    []string{},
		RemovedFeatures:  []string{},
		ModifiedFeatures: []string{},
		VersionChanged:   spec.Version != lock.Version,
	}

	if report.VersionChanged {
		report.OldVersion = lock.Version
		report.NewVersion = spec.Version
		report.HasDrift = true
	}

	// Build feature maps
	specFeatures := make(map[string]*domain.Feature)
	for i := range spec.Features {
		specFeatures[spec.Features[i].ID] = &spec.Features[i]
	}

	// Check locked features
	for id, locked := range lock.Features {
		feat, exists := specFeatures[id]
		if !exists {
			report.RemovedFeatures = append(report.RemovedFeatures, id)
			report.HasDrift = true
			continue
		}

		currentHash, _ := hashFeature(feat)
		if currentHash != locked.Hash {
			report.ModifiedFeatures = append(report.ModifiedFeatures, id)
			report.HasDrift = true
		}
	}

	// Check for new features
	for id := range specFeatures {
		if _, exists := lock.Features[id]; !exists {
			report.AddedFeatures = append(report.AddedFeatures, id)
			report.HasDrift = true
		}
	}

	// Sort for consistent output
	sort.Strings(report.AddedFeatures)
	sort.Strings(report.RemovedFeatures)
	sort.Strings(report.ModifiedFeatures)

	return report
}

// DriftReport contains detailed drift information
type DriftReport struct {
	HasDrift         bool     `json:"has_drift"`
	VersionChanged   bool     `json:"version_changed"`
	OldVersion       string   `json:"old_version,omitempty"`
	NewVersion       string   `json:"new_version,omitempty"`
	AddedFeatures    []string `json:"added_features"`
	RemovedFeatures  []string `json:"removed_features"`
	ModifiedFeatures []string `json:"modified_features"`
}

// hashSpec creates a canonical hash of the entire spec
func hashSpec(spec *domain.ProductSpec) (string, error) {
	// Create a canonical representation
	canonical := struct {
		Name               string
		Version            string
		Goals              []string
		Features           []domain.Feature
		NonFunctional      domain.NonFunctionalReqs
		AcceptanceCriteria []domain.AcceptanceCriterion
		Milestones         []domain.Milestone
	}{
		Name:               spec.Name,
		Version:            spec.Version,
		Goals:              spec.Goals,
		Features:           spec.Features,
		NonFunctional:      spec.NonFunctional,
		AcceptanceCriteria: spec.AcceptanceCriteria,
		Milestones:         spec.Milestones,
	}

	data, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// hashFeature creates a canonical hash of a feature
func hashFeature(feat *domain.Feature) (string, error) {
	data, err := json.Marshal(feat)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}
