package domain

import "time"

// ProductSpec represents a Specular specification for feature work
type ProductSpec struct {
	Name               string               `yaml:"name" json:"name"`
	Version            string               `yaml:"version" json:"version"`
	Goals              []string             `yaml:"goals" json:"goals"`
	Features           []Feature            `yaml:"features" json:"features"`
	NonFunctional      NonFunctionalReqs    `yaml:"non_functional" json:"non_functional"`
	AcceptanceCriteria []AcceptanceCriterion `yaml:"acceptance_criteria" json:"acceptance_criteria"`
	Milestones         []Milestone          `yaml:"milestones" json:"milestones"`
	FilePath           string               `yaml:"-" json:"file_path"`
	CreatedAt          time.Time            `yaml:"-" json:"created_at"`
	UpdatedAt          time.Time            `yaml:"-" json:"updated_at"`
}

// Feature represents a distinct capability in the spec
type Feature struct {
	ID              string   `yaml:"id" json:"id"`
	Title           string   `yaml:"title" json:"title"`
	Description     string   `yaml:"description" json:"description"`
	Priority        Priority `yaml:"priority" json:"priority"`
	API             *APISpec `yaml:"api,omitempty" json:"api,omitempty"`
	SuccessCriteria []string `yaml:"success_criteria" json:"success_criteria"`
}

// Priority represents feature importance
type Priority string

const (
	PriorityHigh   Priority = "high"
	PriorityMedium Priority = "medium"
	PriorityLow    Priority = "low"
)

// APISpec defines an API endpoint
type APISpec struct {
	Method   string `yaml:"method" json:"method"`
	Path     string `yaml:"path" json:"path"`
	Request  string `yaml:"request,omitempty" json:"request,omitempty"`
	Response string `yaml:"response,omitempty" json:"response,omitempty"`
}

// NonFunctionalReqs defines quality attributes
type NonFunctionalReqs struct {
	Performance  []string `yaml:"performance" json:"performance"`
	Security     []string `yaml:"security" json:"security"`
	Scalability  []string `yaml:"scalability" json:"scalability"`
	Availability []string `yaml:"availability,omitempty" json:"availability,omitempty"`
}

// AcceptanceCriterion represents a verifiable acceptance condition
type AcceptanceCriterion struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
	Satisfied   bool   `yaml:"satisfied" json:"satisfied"`
	Evidence    string `yaml:"evidence,omitempty" json:"evidence,omitempty"`
}

// Milestone represents a delivery checkpoint
type Milestone struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Features    []string `yaml:"features" json:"features"`
	Target      string   `yaml:"target" json:"target"`
	Description string   `yaml:"description" json:"description"`
}

// SpecValidation contains validation results
type SpecValidation struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

// SpecProgress represents completion status
type SpecProgress struct {
	TotalCriteria     int     `json:"total_criteria"`
	SatisfiedCriteria int     `json:"satisfied_criteria"`
	PercentComplete   float64 `json:"percent_complete"`
	PendingCriteria   []AcceptanceCriterion `json:"pending_criteria"`
}

// SpecLock represents a canonical hashed snapshot for drift detection
type SpecLock struct {
	Version  string                   `json:"version"`
	SpecHash string                   `json:"spec_hash"`
	Features map[string]LockedFeature `json:"features"`
	LockedAt time.Time                `json:"locked_at"`
}

// LockedFeature contains the hash and metadata for a locked feature
type LockedFeature struct {
	Hash     string `json:"hash"`
	APIPath  string `json:"api_path,omitempty"`
	TestFile string `json:"test_file,omitempty"`
}

// GetProgress calculates completion progress for the spec
func (s *ProductSpec) GetProgress() SpecProgress {
	total := len(s.AcceptanceCriteria)
	satisfied := 0
	var pending []AcceptanceCriterion

	for _, ac := range s.AcceptanceCriteria {
		if ac.Satisfied {
			satisfied++
		} else {
			pending = append(pending, ac)
		}
	}

	var percent float64
	if total > 0 {
		percent = float64(satisfied) / float64(total) * 100
	}

	return SpecProgress{
		TotalCriteria:     total,
		SatisfiedCriteria: satisfied,
		PercentComplete:   percent,
		PendingCriteria:   pending,
	}
}

// GetCriterion finds an acceptance criterion by ID
func (s *ProductSpec) GetCriterion(id string) *AcceptanceCriterion {
	for i := range s.AcceptanceCriteria {
		if s.AcceptanceCriteria[i].ID == id {
			return &s.AcceptanceCriteria[i]
		}
	}
	return nil
}

// GetFeature finds a feature by ID
func (s *ProductSpec) GetFeature(id string) *Feature {
	for i := range s.Features {
		if s.Features[i].ID == id {
			return &s.Features[i]
		}
	}
	return nil
}

// IsComplete returns true if all acceptance criteria are satisfied
func (s *ProductSpec) IsComplete() bool {
	for _, ac := range s.AcceptanceCriteria {
		if !ac.Satisfied {
			return false
		}
	}
	return len(s.AcceptanceCriteria) > 0
}
