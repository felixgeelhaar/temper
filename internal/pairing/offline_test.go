package pairing

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/google/uuid"
)

// failingProvider implements llm.Provider but always returns an error,
// simulating a circuit-breaker-open state or network outage.
type failingProvider struct {
	err error
}

func (p *failingProvider) Name() string                     { return "failing" }
func (p *failingProvider) SupportsStreaming() bool          { return false }
func (p *failingProvider) Generate(_ context.Context, _ *llm.Request) (*llm.Response, error) {
	return nil, p.err
}
func (p *failingProvider) GenerateStream(_ context.Context, _ *llm.Request) (<-chan llm.StreamChunk, error) {
	return nil, p.err
}

func TestService_OfflineFallback_NoProvider(t *testing.T) {
	registry := llm.NewRegistry()
	service := NewService(registry, "")

	exercise := &domain.Exercise{
		ID:    "go-v1/basics/hello-world",
		Title: "Hello World",
		Hints: domain.HintSet{
			L1: []string{"Look at the fmt package."},
		},
	}

	got, err := service.Intervene(context.Background(), InterventionRequest{
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		Intent:    domain.IntentHint,
		Context: InterventionContext{
			Exercise: exercise,
		},
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	})
	if err != nil {
		t.Fatalf("expected offline fallback, got error: %v", err)
	}
	if !strings.HasPrefix(got.Content, "[offline mode]") {
		t.Errorf("offline content must be marked, got: %q", got.Content)
	}
	if !strings.Contains(got.Content, "fmt package") {
		t.Errorf("offline content should serve YAML hint, got: %q", got.Content)
	}
	if !strings.Contains(got.Rationale, "Offline fallback") {
		t.Errorf("rationale should explain offline mode, got: %q", got.Rationale)
	}
}

func TestService_OfflineFallback_LLMError(t *testing.T) {
	registry := llm.NewRegistry()
	registry.Register("failing", &failingProvider{err: errors.New("circuit breaker open")})
	if err := registry.SetDefault("failing"); err != nil {
		t.Fatal(err)
	}
	service := NewService(registry, "failing")

	exercise := &domain.Exercise{
		ID:    "go-v1/basics/hello-world",
		Title: "Hello World",
		Hints: domain.HintSet{
			L2: []string{"Use fmt.Sprintf with %s placeholder."},
		},
	}

	got, err := service.Intervene(context.Background(), InterventionRequest{
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		Intent:    domain.IntentStuck,
		Context: InterventionContext{
			Exercise: exercise,
		},
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	})
	if err != nil {
		t.Fatalf("expected offline fallback after LLM error, got: %v", err)
	}
	if !strings.HasPrefix(got.Content, "[offline mode]") {
		t.Errorf("offline content must be marked, got: %q", got.Content)
	}
	if !strings.Contains(got.Rationale, "circuit breaker open") {
		t.Errorf("rationale should mention LLM error, got: %q", got.Rationale)
	}
}

func TestService_OfflineFallback_NoHintAvailable_ReturnsError(t *testing.T) {
	// When neither LLM nor exercise YAML hint is available, the original
	// error must propagate; offline fallback should not silently mislead.
	registry := llm.NewRegistry()
	service := NewService(registry, "")

	_, err := service.Intervene(context.Background(), InterventionRequest{
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		Intent:    domain.IntentHint,
		Context: InterventionContext{
			Exercise: &domain.Exercise{ID: "test/empty"}, // no Hints
		},
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	})
	if err == nil {
		t.Fatal("expected error when no hint is available")
	}
	if !strings.Contains(err.Error(), "LLM provider") {
		t.Errorf("error should mention LLM provider, got: %v", err)
	}
}

func TestService_OfflineFallback_FallsBackToLowerLevel(t *testing.T) {
	// Selector lands at L2 but only L1 hints exist — should still serve.
	registry := llm.NewRegistry()
	service := NewService(registry, "")

	exercise := &domain.Exercise{
		ID:    "test/partial-hints",
		Title: "Partial",
		Hints: domain.HintSet{
			L1: []string{"Think about formatting."},
		},
	}

	got, err := service.Intervene(context.Background(), InterventionRequest{
		SessionID: uuid.New(),
		UserID:    uuid.New(),
		Intent:    domain.IntentStuck, // base level L2
		Context: InterventionContext{
			Exercise: exercise,
		},
		Policy: domain.LearningPolicy{MaxLevel: domain.L3ConstrainedSnippet},
	})
	if err != nil {
		t.Fatalf("expected fallback to lower level, got error: %v", err)
	}
	if !strings.Contains(got.Content, "Think about formatting") {
		t.Errorf("should fall back to L1 hint when L2 absent, got: %q", got.Content)
	}
}
