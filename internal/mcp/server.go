package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	mcp "github.com/felixgeelhaar/mcp-go"
	"github.com/felixgeelhaar/mcp-go/server"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/exercise"
	"github.com/felixgeelhaar/temper/internal/pairing"
	"github.com/felixgeelhaar/temper/internal/session"
	"github.com/google/uuid"
)

// Server wraps the MCP server with Temper functionality
type Server struct {
	mcpServer      *server.Server
	sessionService *session.Service
	pairingService *pairing.Service
	exerciseLoader *exercise.Loader
}

// Config contains configuration for the MCP server
type Config struct {
	SessionService *session.Service
	PairingService *pairing.Service
	ExerciseLoader *exercise.Loader
}

// NewServer creates a new MCP server for Temper
func NewServer(cfg Config) *Server {
	s := &Server{
		sessionService: cfg.SessionService,
		pairingService: cfg.PairingService,
		exerciseLoader: cfg.ExerciseLoader,
	}

	// Create MCP server
	s.mcpServer = server.New(server.Info{
		Name:    "temper",
		Version: "0.1.0",
	}, server.WithInstructions(`
Temper is an adaptive AI pairing tool for learning.
It enforces a Learning Contract that limits intervention depth based on skill level.

Available tools:
- temper_start: Start a learning session with an exercise
- temper_hint: Get a learning-appropriate hint
- temper_review: Get code review feedback
- temper_stuck: Signal being stuck for adaptive help
- temper_run: Run code checks (format, build, test)
- temper_explain: Get concept explanations
- temper_status: Check session status
- temper_stop: End a session

Learning Contract Levels:
- L0: Clarifying questions only
- L1: Category hints (direction to explore)
- L2: Location + concept (practice default max)
- L3: Constrained snippets/outlines
- L4-L5: Gated for teach mode only
`))

	// Register tools
	s.registerTools()

	return s
}

// registerTools registers all Temper MCP tools
func (s *Server) registerTools() {
	// temper_start - Start a session
	s.mcpServer.Tool("temper_start").
		Description("Start a Temper learning session with an exercise").
		Handler(s.handleStart)

	// temper_hint - Get a hint
	s.mcpServer.Tool("temper_hint").
		Description("Get a learning-appropriate hint. Respects the Learning Contract.").
		Handler(s.handleHint)

	// temper_review - Get code review
	s.mcpServer.Tool("temper_review").
		Description("Get code review feedback at an appropriate level.").
		Handler(s.handleReview)

	// temper_stuck - Signal stuck
	s.mcpServer.Tool("temper_stuck").
		Description("Signal being stuck for adaptive help.").
		Handler(s.handleStuck)

	// temper_run - Run checks
	s.mcpServer.Tool("temper_run").
		Description("Run code checks (format, build, test).").
		Handler(s.handleRun)

	// temper_explain - Explain concept
	s.mcpServer.Tool("temper_explain").
		Description("Get an explanation of a concept at an appropriate level.").
		Handler(s.handleExplain)

	// temper_status - Session status
	s.mcpServer.Tool("temper_status").
		Description("Get current session status.").
		Handler(s.handleStatus)

	// temper_stop - End session
	s.mcpServer.Tool("temper_stop").
		Description("End a Temper session.").
		Handler(s.handleStop)
}

// Input/Output types for tools

type StartInput struct {
	ExerciseID string `json:"exercise_id" jsonschema:"description=Exercise ID in format pack/exercise"`
	Track      string `json:"track,omitempty" jsonschema:"description=Learning track: practice or interview-prep,enum=practice,enum=interview-prep"`
}

type StartOutput struct {
	SessionID  string `json:"session_id"`
	ExerciseID string `json:"exercise_id"`
	Track      string `json:"track"`
	Message    string `json:"message"`
}

type InterventionInput struct {
	SessionID string            `json:"session_id" jsonschema:"description=Session ID from temper_start"`
	Code      map[string]string `json:"code,omitempty" jsonschema:"description=Current code files as filename -> content map"`
	Context   string            `json:"context,omitempty" jsonschema:"description=Additional context"`
}

type InterventionOutput struct {
	Level   int    `json:"level"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

type RunInput struct {
	SessionID string            `json:"session_id" jsonschema:"description=Session ID from temper_start"`
	Code      map[string]string `json:"code" jsonschema:"description=Code files as filename -> content map"`
	Format    *bool             `json:"format,omitempty" jsonschema:"description=Run format check (default: true)"`
	Build     *bool             `json:"build,omitempty" jsonschema:"description=Run build check (default: true)"`
	Test      *bool             `json:"test,omitempty" jsonschema:"description=Run tests (default: true)"`
}

type RunOutput struct {
	FormatOK    bool   `json:"format_ok"`
	FormatDiff  string `json:"format_diff,omitempty"`
	BuildOK     bool   `json:"build_ok"`
	BuildOutput string `json:"build_output,omitempty"`
	TestOK      bool   `json:"test_ok"`
	TestOutput  string `json:"test_output,omitempty"`
	Summary     string `json:"summary"`
}

type StatusInput struct {
	SessionID string `json:"session_id" jsonschema:"description=Session ID from temper_start"`
}

type StatusOutput struct {
	SessionID  string `json:"session_id"`
	ExerciseID string `json:"exercise_id"`
	Status     string `json:"status"`
	RunCount   int    `json:"run_count"`
	HintCount  int    `json:"hint_count"`
	Track      string `json:"track"`
	MaxLevel   int    `json:"max_level"`
}

type StopInput struct {
	SessionID string `json:"session_id" jsonschema:"description=Session ID to end"`
}

type StopOutput struct {
	Message string `json:"message"`
}

// Tool handlers

func (s *Server) handleStart(ctx context.Context, input StartInput) (StartOutput, error) {
	track := input.Track
	if track == "" {
		track = "practice"
	}

	var policy *domain.LearningPolicy
	switch track {
	case "interview-prep":
		p := domain.InterviewPrepPolicy()
		policy = &p
	default:
		p := domain.DefaultPolicy()
		policy = &p
	}

	sess, err := s.sessionService.Create(ctx, session.CreateRequest{
		ExerciseID: input.ExerciseID,
		Policy:     policy,
	})
	if err != nil {
		return StartOutput{}, fmt.Errorf("failed to create session: %w", err)
	}

	return StartOutput{
		SessionID:  sess.ID,
		ExerciseID: sess.ExerciseID,
		Track:      track,
		Message:    fmt.Sprintf("Session started. Max intervention level: L%d. Cooldown: %ds", policy.MaxLevel, policy.CooldownSeconds),
	}, nil
}

func (s *Server) handleHint(ctx context.Context, input InterventionInput) (InterventionOutput, error) {
	return s.handleIntervention(ctx, input, domain.IntentHint)
}

func (s *Server) handleReview(ctx context.Context, input InterventionInput) (InterventionOutput, error) {
	return s.handleIntervention(ctx, input, domain.IntentReview)
}

func (s *Server) handleStuck(ctx context.Context, input InterventionInput) (InterventionOutput, error) {
	return s.handleIntervention(ctx, input, domain.IntentStuck)
}

func (s *Server) handleExplain(ctx context.Context, input InterventionInput) (InterventionOutput, error) {
	return s.handleIntervention(ctx, input, domain.IntentExplain)
}

func (s *Server) handleIntervention(ctx context.Context, input InterventionInput, intent domain.Intent) (InterventionOutput, error) {
	// Get session
	sess, err := s.sessionService.Get(ctx, input.SessionID)
	if err != nil {
		return InterventionOutput{}, fmt.Errorf("session not found: %w", err)
	}

	if sess.Status != session.StatusActive {
		return InterventionOutput{}, fmt.Errorf("session is not active")
	}

	// Check cooldown
	if !sess.CanRequestIntervention(domain.L3ConstrainedSnippet) {
		remaining := sess.CooldownRemaining()
		return InterventionOutput{
			Level:   -1,
			Type:    "cooldown",
			Content: fmt.Sprintf("Please wait %.0f seconds before requesting more detailed help.", remaining.Seconds()),
		}, nil
	}

	// Load exercise for context
	var ex *domain.Exercise
	parts := strings.SplitN(sess.ExerciseID, "/", 2)
	if len(parts) >= 2 {
		ex, _ = s.exerciseLoader.LoadExercise(parts[0], parts[1])
	}

	// Use provided code or session's code
	code := sess.Code
	if len(input.Code) > 0 {
		code = input.Code
	}

	// Build intervention request
	pairingReq := pairing.InterventionRequest{
		SessionID: uuid.MustParse(sess.ID),
		UserID:    uuid.Nil,
		Intent:    intent,
		Context: pairing.InterventionContext{
			Exercise: ex,
			Code:     code,
		},
		Policy: sess.Policy,
	}

	// Generate intervention
	intervention, err := s.pairingService.Intervene(ctx, pairingReq)
	if err != nil {
		return InterventionOutput{}, fmt.Errorf("failed to generate intervention: %w", err)
	}

	// Record intervention
	sessionIntervention := &session.Intervention{
		ID:        intervention.ID.String(),
		SessionID: sess.ID,
		Intent:    intervention.Intent,
		Level:     intervention.Level,
		Type:      intervention.Type,
		Content:   intervention.Content,
		CreatedAt: time.Now(),
	}
	// Record intervention - log but don't fail on error
	_ = s.sessionService.RecordIntervention(ctx, sessionIntervention)

	return InterventionOutput{
		Level:   int(intervention.Level),
		Type:    string(intervention.Type),
		Content: intervention.Content,
	}, nil
}

func (s *Server) handleRun(ctx context.Context, input RunInput) (RunOutput, error) {
	// Default to running all checks
	format := true
	build := true
	test := true

	if input.Format != nil {
		format = *input.Format
	}
	if input.Build != nil {
		build = *input.Build
	}
	if input.Test != nil {
		test = *input.Test
	}

	run, err := s.sessionService.RunCode(ctx, input.SessionID, session.RunRequest{
		Code:   input.Code,
		Format: format,
		Build:  build,
		Test:   test,
	})
	if err != nil {
		return RunOutput{}, fmt.Errorf("run failed: %w", err)
	}

	result := run.Result
	output := RunOutput{
		FormatOK: result.FormatOK,
		BuildOK:  result.BuildOK,
		TestOK:   result.TestOK,
	}

	if result.FormatDiff != "" {
		output.FormatDiff = result.FormatDiff
	}
	if result.BuildOutput != "" {
		output.BuildOutput = result.BuildOutput
	}
	if result.TestOutput != "" {
		output.TestOutput = result.TestOutput
	}

	// Build summary
	var summary []string
	if format {
		if result.FormatOK {
			summary = append(summary, "Format: ✓")
		} else {
			summary = append(summary, "Format: ✗")
		}
	}
	if build {
		if result.BuildOK {
			summary = append(summary, "Build: ✓")
		} else {
			summary = append(summary, "Build: ✗")
		}
	}
	if test {
		if result.TestOK {
			summary = append(summary, "Tests: ✓")
		} else {
			summary = append(summary, "Tests: ✗")
		}
	}
	output.Summary = strings.Join(summary, " | ")

	return output, nil
}

func (s *Server) handleStatus(ctx context.Context, input StatusInput) (StatusOutput, error) {
	sess, err := s.sessionService.Get(ctx, input.SessionID)
	if err != nil {
		return StatusOutput{}, fmt.Errorf("session not found: %w", err)
	}

	return StatusOutput{
		SessionID:  sess.ID,
		ExerciseID: sess.ExerciseID,
		Status:     string(sess.Status),
		RunCount:   sess.RunCount,
		HintCount:  sess.HintCount,
		Track:      sess.Policy.Track,
		MaxLevel:   int(sess.Policy.MaxLevel),
	}, nil
}

func (s *Server) handleStop(ctx context.Context, input StopInput) (StopOutput, error) {
	err := s.sessionService.Delete(ctx, input.SessionID)
	if err != nil {
		return StopOutput{}, fmt.Errorf("failed to delete session: %w", err)
	}

	return StopOutput{
		Message: "Session ended successfully",
	}, nil
}

// ServeStdio starts the MCP server on stdio (for Cursor integration)
func (s *Server) ServeStdio(ctx context.Context) error {
	return mcp.ServeStdio(ctx, s.mcpServer)
}

// ServeHTTP starts the MCP server on HTTP (alternative transport)
func (s *Server) ServeHTTP(ctx context.Context, addr string) error {
	return mcp.ServeHTTP(ctx, s.mcpServer, addr)
}

// GetMCPServer returns the underlying MCP server (for testing)
func (s *Server) GetMCPServer() *server.Server {
	return s.mcpServer
}
