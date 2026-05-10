// Package eval provides a golden-set evaluation harness for the pairing
// system. Each Case declares an (intent, level, code, profile) input and
// the expected behavior of the LLM response. Used to detect regressions
// in selector logic and prompt restraint when models or prompts change.
package eval

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// Case is one row in the golden evaluation set.
type Case struct {
	ID          string `yaml:"id"`
	Description string `yaml:"description"`

	// Input
	Intent       string            `yaml:"intent"`        // hint|review|stuck|next|explain
	Level        int               `yaml:"level"`         // expected level after selector
	ExerciseID   string            `yaml:"exercise_id"`   // for context
	Language     string            `yaml:"language"`      // go|python|...
	Code         map[string]string `yaml:"code"`
	BuildErrors  []string          `yaml:"build_errors,omitempty"`
	TestsPassed  int               `yaml:"tests_passed,omitempty"`
	TestsFailed  int               `yaml:"tests_failed,omitempty"`

	Profile *ProfileSnapshot `yaml:"profile,omitempty"`

	// Expectations
	Expect Expectations `yaml:"expect"`
}

// ProfileSnapshot captures the subset of LearningProfile that affects
// selector behavior.
type ProfileSnapshot struct {
	HintDependency float64            `yaml:"hint_dependency"`
	TopicSkills    map[string]float64 `yaml:"topic_skills,omitempty"`
	TotalRuns      int                `yaml:"total_runs"`
}

// Expectations declares what a passing response looks like. Most cases
// only set ClampPasses=true and rely on the ClampValidator. ForbiddenSubstrings
// and RequiredSubstrings allow case-specific assertions (e.g. "must mention
// the fmt package").
type Expectations struct {
	ClampPasses         bool     `yaml:"clamp_passes"`
	MustContainQuestion bool     `yaml:"must_contain_question,omitempty"`
	ForbiddenSubstrings []string `yaml:"forbidden_substrings,omitempty"`
	RequiredSubstrings  []string `yaml:"required_substrings,omitempty"`
}

// LoadCases reads every *.yaml file in dir and returns the parsed cases.
func LoadCases(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read eval dir: %w", err)
	}

	var cases []Case
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var c Case
		if err := yaml.Unmarshal(data, &c); err != nil {
			return nil, fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		if c.ID == "" {
			c.ID = e.Name()
		}
		cases = append(cases, c)
	}
	return cases, nil
}

// IntentDomain converts the YAML intent string to a domain.Intent.
func (c *Case) IntentDomain() domain.Intent {
	switch c.Intent {
	case "hint":
		return domain.IntentHint
	case "review":
		return domain.IntentReview
	case "stuck":
		return domain.IntentStuck
	case "next":
		return domain.IntentNext
	case "explain":
		return domain.IntentExplain
	default:
		return domain.IntentHint
	}
}

// LevelDomain returns the expected intervention level.
func (c *Case) LevelDomain() domain.InterventionLevel {
	return domain.InterventionLevel(c.Level)
}
