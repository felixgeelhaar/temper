package domain

// Exercise represents a structured learning task
type Exercise struct {
	ID            string // slug: "go-v1/basics/hello-world"
	PackID        string // "go-v1"
	Title         string
	Description   string
	Difficulty    Difficulty
	StarterCode   map[string]string // filename -> content
	TestCode      map[string]string // test files (read-only for user)
	Rubric        Rubric
	CheckRecipe   CheckRecipe
	Tags          []string
	Prerequisites []string // other exercise IDs
	Hints         HintSet  // hints organized by level
}

// Difficulty represents exercise difficulty level
type Difficulty string

const (
	DifficultyBeginner     Difficulty = "beginner"
	DifficultyIntermediate Difficulty = "intermediate"
	DifficultyAdvanced     Difficulty = "advanced"
)

// Rubric defines evaluation criteria for an exercise
type Rubric struct {
	Criteria []RubricCriterion
}

// RubricCriterion represents a single evaluation criterion
type RubricCriterion struct {
	ID          string
	Name        string
	Description string
	Weight      float64
	Signals     []string // what to look for
}

// CheckRecipe defines the checks to run for an exercise
type CheckRecipe struct {
	Format    bool     // run gofmt
	Build     bool     // run go build
	Test      bool     // run go test
	TestFlags []string // e.g., ["-v", "-race"]
	Timeout   int      // seconds
}

// HintSet contains hints organized by intervention level
type HintSet struct {
	L0 []string // clarifying questions
	L1 []string // category hints
	L2 []string // location + concept
	L3 []string // constrained snippets
}

// ExercisePack represents a collection of related exercises
type ExercisePack struct {
	ID            string
	Name          string
	Version       string
	Description   string
	Language      string
	DefaultPolicy LearningPolicy
	ExerciseIDs   []string // ordered list of exercise slugs
}

// GetHintsForLevel returns hints for the specified intervention level
func (e *Exercise) GetHintsForLevel(level InterventionLevel) []string {
	switch level {
	case L0Clarify:
		return e.Hints.L0
	case L1CategoryHint:
		return e.Hints.L1
	case L2LocationConcept:
		return e.Hints.L2
	case L3ConstrainedSnippet:
		return e.Hints.L3
	default:
		return nil
	}
}

// AllFiles returns all files for the exercise (starter + tests)
func (e *Exercise) AllFiles() map[string]string {
	files := make(map[string]string)
	for k, v := range e.StarterCode {
		files[k] = v
	}
	for k, v := range e.TestCode {
		files[k] = v
	}
	return files
}
