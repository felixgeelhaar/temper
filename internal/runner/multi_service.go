package runner

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/risk"
	"github.com/google/uuid"
)

// MultiLanguageService handles code execution for multiple languages
type MultiLanguageService struct {
	config       Config
	registry     *ExecutorRegistry
	parser       *Parser
	riskDetector *risk.Detector

	mu      sync.Mutex
	running map[uuid.UUID]*runState
}

// NewMultiLanguageService creates a new multi-language runner service
func NewMultiLanguageService(cfg Config) *MultiLanguageService {
	registry := NewExecutorRegistry()

	// Register all language executors
	registry.Register(NewGoExecutor(false))
	registry.Register(NewPythonExecutor())
	registry.Register(NewTypeScriptExecutor())
	registry.Register(NewRustExecutor())

	return &MultiLanguageService{
		config:       cfg,
		registry:     registry,
		parser:       NewParser(),
		riskDetector: risk.NewDetector(),
		running:      make(map[uuid.UUID]*runState),
	}
}

// MultiExecuteRequest contains data for executing code with language specification
type MultiExecuteRequest struct {
	RunID      uuid.UUID
	UserID     uuid.UUID
	ArtifactID uuid.UUID
	ExerciseID *string
	Language   Language
	Code       map[string]string
	Recipe     domain.CheckRecipe
}

// Execute runs code using the appropriate language executor
func (s *MultiLanguageService) Execute(ctx context.Context, req MultiExecuteRequest) (*domain.RunOutput, error) {
	// Get executor for language
	executor, err := s.registry.Get(req.Language)
	if err != nil {
		return nil, fmt.Errorf("get executor: %w", err)
	}

	// Create timeout context
	timeout := s.config.Timeout
	if req.Recipe.Timeout > 0 {
		timeout = time.Duration(req.Recipe.Timeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Track running state
	state := &runState{
		run: &domain.Run{
			ID:         req.RunID,
			ArtifactID: req.ArtifactID,
			UserID:     req.UserID,
			ExerciseID: req.ExerciseID,
			Status:     domain.RunStatusRunning,
			Recipe:     req.Recipe,
		},
		cancel: cancel,
		doneCh: make(chan struct{}),
	}

	s.mu.Lock()
	s.running[req.RunID] = state
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, req.RunID)
		s.mu.Unlock()
		close(state.doneCh)
	}()

	output := &domain.RunOutput{}

	// Run checks based on recipe
	if req.Recipe.Format {
		formatResult, err := executor.Format(ctx, req.Code)
		if err != nil {
			return nil, fmt.Errorf("format check: %w", err)
		}
		output.FormatOK = formatResult.OK
		output.FormatDiff = formatResult.Diff
	}

	if req.Recipe.Build {
		buildResult, err := executor.Build(ctx, req.Code)
		if err != nil {
			return nil, fmt.Errorf("build check: %w", err)
		}
		output.BuildOK = buildResult.OK
		output.BuildOutput = buildResult.Output
		output.BuildErrors = s.parseBuildErrors(req.Language, buildResult.Output)
	}

	// Only run tests if build succeeded (or if no build step)
	if req.Recipe.Test && (output.BuildOK || !req.Recipe.Build) {
		testResult, err := executor.Test(ctx, req.Code, req.Recipe.TestFlags)
		if err != nil {
			return nil, fmt.Errorf("test check: %w", err)
		}
		output.TestOK = testResult.OK
		output.TestOutput = testResult.Output
		output.TestResults = s.parseTestOutput(req.Language, testResult.Output)

		for _, tr := range output.TestResults {
			if tr.Passed {
				output.TestsPassed++
			} else {
				output.TestsFailed++
			}
		}
		output.Duration = testResult.Duration
		output.Logs = testResult.Output
	}

	// Run risk detection on the code
	output.Risks = s.riskDetector.Analyze(req.Code)

	return output, nil
}

// parseBuildErrors parses build errors based on language
func (s *MultiLanguageService) parseBuildErrors(lang Language, output string) []domain.Diagnostic {
	switch lang {
	case LanguageGo:
		return s.parser.ParseBuildErrors(output)
	case LanguagePython:
		return s.parsePythonBuildErrors(output)
	case LanguageTypeScript:
		return s.parseTypeScriptBuildErrors(output)
	case LanguageRust:
		return s.parseRustBuildErrors(output)
	default:
		return nil
	}
}

// parseTestOutput parses test output based on language
func (s *MultiLanguageService) parseTestOutput(lang Language, output string) []domain.TestResult {
	switch lang {
	case LanguageGo:
		return s.parser.ParseTestOutput(output)
	case LanguagePython:
		return s.parsePythonTestOutput(output)
	case LanguageTypeScript:
		return s.parseTypeScriptTestOutput(output)
	case LanguageRust:
		return s.parseRustTestOutput(output)
	default:
		return nil
	}
}

// Language-specific parsers (simplified implementations)

func (s *MultiLanguageService) parsePythonBuildErrors(output string) []domain.Diagnostic {
	// Parse Python syntax errors
	// Format: File "filename.py", line N
	var errors []domain.Diagnostic
	// Simple parsing - can be enhanced
	if output != "" {
		errors = append(errors, domain.Diagnostic{
			File:     "unknown",
			Severity: "error",
			Message:  output,
		})
	}
	return errors
}

func (s *MultiLanguageService) parsePythonTestOutput(output string) []domain.TestResult {
	// Parse pytest output - simplified
	var results []domain.TestResult
	// Basic implementation - enhance with actual pytest JSON parsing
	return results
}

func (s *MultiLanguageService) parseTypeScriptBuildErrors(output string) []domain.Diagnostic {
	// Parse TypeScript compiler errors
	var errors []domain.Diagnostic
	if output != "" {
		errors = append(errors, domain.Diagnostic{
			File:     "unknown",
			Severity: "error",
			Message:  output,
		})
	}
	return errors
}

func (s *MultiLanguageService) parseTypeScriptTestOutput(output string) []domain.TestResult {
	// Parse vitest output - simplified
	var results []domain.TestResult
	return results
}

func (s *MultiLanguageService) parseRustBuildErrors(output string) []domain.Diagnostic {
	// Parse Rust compiler errors
	var errors []domain.Diagnostic
	if output != "" {
		errors = append(errors, domain.Diagnostic{
			File:     "unknown",
			Severity: "error",
			Message:  output,
		})
	}
	return errors
}

func (s *MultiLanguageService) parseRustTestOutput(output string) []domain.TestResult {
	// Parse cargo test output - simplified
	var results []domain.TestResult
	return results
}

// FormatCode formats code using the appropriate language formatter
func (s *MultiLanguageService) FormatCode(ctx context.Context, lang Language, code map[string]string) (map[string]string, error) {
	executor, err := s.registry.Get(lang)
	if err != nil {
		return nil, err
	}
	return executor.FormatFix(ctx, code)
}

// Cancel cancels a running execution
func (s *MultiLanguageService) Cancel(runID uuid.UUID) error {
	s.mu.Lock()
	state, ok := s.running[runID]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	state.cancel()
	return nil
}

// IsRunning checks if a run is currently executing
func (s *MultiLanguageService) IsRunning(runID uuid.UUID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.running[runID]
	return ok
}

// Wait waits for a run to complete
func (s *MultiLanguageService) Wait(ctx context.Context, runID uuid.UUID) error {
	s.mu.Lock()
	state, ok := s.running[runID]
	s.mu.Unlock()

	if !ok {
		return nil // Already completed
	}

	select {
	case <-state.doneCh:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// SupportedLanguages returns all supported languages
func (s *MultiLanguageService) SupportedLanguages() []Language {
	return s.registry.SupportedLanguages()
}

// Registry returns the executor registry
func (s *MultiLanguageService) Registry() *ExecutorRegistry {
	return s.registry
}
