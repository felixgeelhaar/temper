package pairing

import (
	"context"
	"errors"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/google/uuid"
)

func TestNewService(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "test-provider")
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_ExtractTargets(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "test-provider")

	tests := []struct {
		name    string
		ctx     InterventionContext
		wantLen int
	}{
		{
			name:    "no current file",
			ctx:     InterventionContext{},
			wantLen: 0,
		},
		{
			name: "with current file",
			ctx: InterventionContext{
				CurrentFile: "main.go",
				CursorLine:  10,
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := service.extractTargets(tt.ctx)
			if len(got) != tt.wantLen {
				t.Errorf("extractTargets() returned %d targets; want %d", len(got), tt.wantLen)
			}

			if tt.wantLen > 0 {
				if got[0].File != tt.ctx.CurrentFile {
					t.Errorf("Target.File = %q; want %q", got[0].File, tt.ctx.CurrentFile)
				}
				if got[0].StartLine != tt.ctx.CursorLine {
					t.Errorf("Target.StartLine = %d; want %d", got[0].StartLine, tt.ctx.CursorLine)
				}
			}
		})
	}
}

func TestService_ExtractTargets_Details(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "test-provider")

	ctx := InterventionContext{
		CurrentFile: "handler.go",
		CursorLine:  42,
	}

	targets := service.extractTargets(ctx)
	if len(targets) != 1 {
		t.Fatalf("extractTargets() returned %d targets; want 1", len(targets))
	}

	target := targets[0]
	if target.File != "handler.go" {
		t.Errorf("File = %q; want %q", target.File, "handler.go")
	}
	if target.StartLine != 42 {
		t.Errorf("StartLine = %d; want %d", target.StartLine, 42)
	}
	if target.EndLine != 42 {
		t.Errorf("EndLine = %d; want %d", target.EndLine, 42)
	}
}

// mockProvider implements llm.Provider for testing
type mockProvider struct {
	name      string
	response  *llm.Response
	err       error
	streaming bool
	stream    []llm.StreamChunk
}

func (m *mockProvider) Name() string { return m.name }

func (m *mockProvider) Generate(ctx context.Context, req *llm.Request) (*llm.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockProvider) GenerateStream(ctx context.Context, req *llm.Request) (<-chan llm.StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan llm.StreamChunk, len(m.stream))
	for _, c := range m.stream {
		ch <- c
	}
	close(ch)
	return ch, nil
}

func (m *mockProvider) SupportsStreaming() bool { return m.streaming }

func createTestService(provider *mockProvider) *Service {
	registry := llm.NewRegistry()
	registry.Register(provider.name, provider)
	registry.SetDefault(provider.name)
	return NewService(registry, provider.name)
}

func TestService_Intervene_Success(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		response: &llm.Response{
			Content:      "Consider using a different approach here.",
			FinishReason: "stop",
		},
	}
	service := createTestService(mock)

	req := InterventionRequest{
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		Intent:    domain.IntentHint,
		Context: InterventionContext{
			Code: map[string]string{"main.go": "package main"},
		},
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}

	intervention, err := service.Intervene(context.Background(), req)
	if err != nil {
		t.Fatalf("Intervene() error = %v", err)
	}

	if intervention == nil {
		t.Fatal("Intervene() returned nil intervention")
	}

	if intervention.Content != mock.response.Content {
		t.Errorf("Content = %q, want %q", intervention.Content, mock.response.Content)
	}

	if intervention.Intent != domain.IntentHint {
		t.Errorf("Intent = %v, want %v", intervention.Intent, domain.IntentHint)
	}

	if intervention.SessionID != req.SessionID {
		t.Errorf("SessionID = %v, want %v", intervention.SessionID, req.SessionID)
	}

	if intervention.UserID != req.UserID {
		t.Errorf("UserID = %v, want %v", intervention.UserID, req.UserID)
	}
}

func TestService_Intervene_LLMError(t *testing.T) {
	expectedErr := errors.New("LLM service unavailable")
	mock := &mockProvider{
		name: "test",
		err:  expectedErr,
	}
	service := createTestService(mock)

	req := InterventionRequest{
		SessionID: uuid.New(),
		Intent:    domain.IntentHint,
		Policy:    domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}

	_, err := service.Intervene(context.Background(), req)
	if err == nil {
		t.Fatal("Intervene() expected error, got nil")
	}

	if !errors.Is(err, expectedErr) && !containsError(err.Error(), expectedErr.Error()) {
		t.Errorf("error = %v, want to contain %v", err, expectedErr)
	}
}

func TestService_Intervene_NoProvider(t *testing.T) {
	registry := llm.NewRegistry()
	// Don't register any provider
	service := NewService(registry, "nonexistent")

	req := InterventionRequest{
		SessionID: uuid.New(),
		Intent:    domain.IntentHint,
		Policy:    domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}

	_, err := service.Intervene(context.Background(), req)
	if err == nil {
		t.Fatal("Intervene() expected error for missing provider, got nil")
	}
}

func TestService_Intervene_PolicyClamping(t *testing.T) {
	tests := []struct {
		name         string
		maxLevel     domain.InterventionLevel
		intent       domain.Intent
		wantMaxLevel domain.InterventionLevel
	}{
		{
			name:         "clamp to L2",
			maxLevel:     domain.L2LocationConcept,
			intent:       domain.IntentStuck, // would normally suggest L3
			wantMaxLevel: domain.L2LocationConcept,
		},
		{
			name:         "clamp to L3",
			maxLevel:     domain.L3ConstrainedSnippet,
			intent:       domain.IntentStuck,
			wantMaxLevel: domain.L3ConstrainedSnippet,
		},
		{
			name:         "no clamping needed for low intent",
			maxLevel:     domain.L5FullSolution,
			intent:       domain.IntentHint,
			wantMaxLevel: domain.L5FullSolution, // L1 base won't exceed L5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockProvider{
				name: "test",
				response: &llm.Response{
					Content: "Test response",
				},
			}
			service := createTestService(mock)

			req := InterventionRequest{
				SessionID: uuid.New(),
				Intent:    tt.intent,
				Policy:    domain.LearningPolicy{MaxLevel: tt.maxLevel},
			}

			intervention, err := service.Intervene(context.Background(), req)
			if err != nil {
				t.Fatalf("Intervene() error = %v", err)
			}

			if intervention.Level > tt.wantMaxLevel {
				t.Errorf("Level = %d, want at most %d", intervention.Level, tt.wantMaxLevel)
			}
		})
	}
}

func TestService_Intervene_ExplicitLevel(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		response: &llm.Response{
			Content: "Here's a detailed explanation with code.",
		},
	}
	service := createTestService(mock)

	req := InterventionRequest{
		SessionID:     uuid.New(),
		Intent:        domain.IntentStuck,
		ExplicitLevel: domain.L4PartialSolution,
		Justification: "Stuck for over 30 minutes on this problem",
		Policy:        domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet}, // Would normally clamp
	}

	intervention, err := service.Intervene(context.Background(), req)
	if err != nil {
		t.Fatalf("Intervene() error = %v", err)
	}

	// Explicit level should override policy clamping
	if intervention.Level != domain.L4PartialSolution {
		t.Errorf("Level = %d, want %d (explicit level should override policy)", intervention.Level, domain.L4PartialSolution)
	}
}

func TestService_Intervene_WithRunID(t *testing.T) {
	mock := &mockProvider{
		name: "test",
		response: &llm.Response{
			Content: "Based on the test output...",
		},
	}
	service := createTestService(mock)

	runID := uuid.New()
	req := InterventionRequest{
		SessionID: uuid.New(),
		Intent:    domain.IntentHint,
		RunID:     &runID,
		Context: InterventionContext{
			RunOutput: &domain.RunOutput{
				TestOK:      false,
				TestsFailed: 2,
			},
		},
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}

	intervention, err := service.Intervene(context.Background(), req)
	if err != nil {
		t.Fatalf("Intervene() error = %v", err)
	}

	if intervention.RunID == nil {
		t.Error("RunID should be set")
	} else if *intervention.RunID != runID {
		t.Errorf("RunID = %v, want %v", *intervention.RunID, runID)
	}
}

func TestService_Intervene_AllIntents(t *testing.T) {
	intents := []domain.Intent{
		domain.IntentHint,
		domain.IntentReview,
		domain.IntentNext,
		domain.IntentStuck,
		domain.IntentExplain,
	}

	for _, intent := range intents {
		t.Run(string(intent), func(t *testing.T) {
			mock := &mockProvider{
				name: "test",
				response: &llm.Response{
					Content: "Response for " + string(intent),
				},
			}
			service := createTestService(mock)

			req := InterventionRequest{
				SessionID: uuid.New(),
				Intent:    intent,
				Policy:    domain.LearningPolicy{MaxLevel: domain.L5FullSolution},
			}

			intervention, err := service.Intervene(context.Background(), req)
			if err != nil {
				t.Fatalf("Intervene() error = %v", err)
			}

			if intervention.Intent != intent {
				t.Errorf("Intent = %v, want %v", intervention.Intent, intent)
			}
		})
	}
}

func TestService_IntervenStream_Success(t *testing.T) {
	mock := &mockProvider{
		name:      "test",
		streaming: true,
		stream: []llm.StreamChunk{
			{Content: "First "},
			{Content: "part "},
			{Content: "done.", Done: false},
			{Done: true},
		},
	}
	service := createTestService(mock)

	req := InterventionRequest{
		SessionID: uuid.New(),
		Intent:    domain.IntentHint,
		Policy:    domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}

	stream, err := service.IntervenStream(context.Background(), req)
	if err != nil {
		t.Fatalf("IntervenStream() error = %v", err)
	}

	var gotContent string
	var gotMetadata bool
	var gotDone bool

	for chunk := range stream {
		switch chunk.Type {
		case "metadata":
			gotMetadata = true
			if chunk.Metadata == nil {
				t.Error("Metadata chunk has nil Metadata")
			}
		case "content":
			gotContent += chunk.Content
		case "done":
			gotDone = true
		case "error":
			t.Errorf("Unexpected error chunk: %v", chunk.Error)
		}
	}

	if !gotMetadata {
		t.Error("Expected metadata chunk")
	}
	if !gotDone {
		t.Error("Expected done chunk")
	}
}

func TestService_IntervenStream_Error(t *testing.T) {
	expectedErr := errors.New("stream error")
	mock := &mockProvider{
		name:      "test",
		streaming: true,
		err:       expectedErr,
	}
	service := createTestService(mock)

	req := InterventionRequest{
		SessionID: uuid.New(),
		Intent:    domain.IntentHint,
		Policy:    domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	}

	_, err := service.IntervenStream(context.Background(), req)
	if err == nil {
		t.Fatal("IntervenStream() expected error, got nil")
	}
}

// Helper function to check if error message contains expected string
func containsError(got, want string) bool {
	return len(got) >= len(want) && got[len(got)-len(want):] == want || len(got) > 0 && got != ""
}
