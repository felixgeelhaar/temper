package pairing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/google/uuid"
)

// Service handles pairing engine operations
type Service struct {
	llmRegistry     *llm.Registry
	defaultProvider string
	selector        *Selector
	prompter        *Prompter
	clampValidator  *ClampValidator
}

// NewService creates a new pairing service
func NewService(llmRegistry *llm.Registry, defaultProvider string) *Service {
	return &Service{
		llmRegistry:     llmRegistry,
		defaultProvider: defaultProvider,
		selector:        NewSelector(),
		prompter:        NewPrompter(),
		clampValidator:  NewClampValidator(),
	}
}

// InterventionRequest contains data for requesting an intervention
type InterventionRequest struct {
	SessionID     uuid.UUID
	UserID        uuid.UUID
	Intent        domain.Intent
	Context       InterventionContext
	Policy        domain.LearningPolicy
	RunID         *uuid.UUID
	ExplicitLevel domain.InterventionLevel // Explicit level request (for escalation)
	Justification string                   // Required justification for L4/L5 escalation
}

// InterventionContext is defined in context.go with spec support

// Intervene generates an intervention based on the request
func (s *Service) Intervene(ctx context.Context, req InterventionRequest) (*domain.Intervention, error) {
	var level domain.InterventionLevel

	// Use explicit level if provided (for escalation requests)
	if req.ExplicitLevel > 0 {
		level = req.ExplicitLevel
	} else {
		// Select intervention level based on context
		level = s.selector.SelectLevel(req.Intent, req.Context, req.Policy)
		// Clamp level based on policy (only for non-explicit requests)
		level = req.Policy.ClampLevel(level)
	}

	// Select intervention type
	interventionType := s.selector.SelectType(req.Intent, level)

	// Build prompt for LLM
	prompt := s.prompter.BuildPrompt(PromptRequest{
		Intent:         req.Intent,
		Level:          level,
		Type:           interventionType,
		Exercise:       req.Context.Exercise,
		Code:           req.Context.Code,
		Output:         req.Context.RunOutput,
		Profile:        req.Context.Profile,
		Spec:           req.Context.Spec,
		FocusCriterion: req.Context.FocusCriterion,
	})

	// Get LLM provider
	provider, err := s.llmRegistry.Default()
	if err != nil {
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	systemPrompt := s.prompter.SystemPrompt(level)

	// Generate intervention content
	llmResp, err := provider.Generate(ctx, &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System:      systemPrompt,
		MaxTokens:   1024,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generate intervention: %w", err)
	}

	content, rationale := s.enforceClamp(ctx, provider, level, prompt, systemPrompt, llmResp.Content)

	// Build intervention
	intervention := &domain.Intervention{
		ID:          uuid.New(),
		SessionID:   req.SessionID,
		UserID:      req.UserID,
		RunID:       req.RunID,
		Intent:      req.Intent,
		Level:       level,
		Type:        interventionType,
		Content:     content,
		Targets:     s.extractTargets(req.Context),
		Rationale:   fmt.Sprintf("Selected L%d based on intent=%s, profile signals%s", level, req.Intent, rationale),
		RequestedAt: time.Now(),
		DeliveredAt: time.Now(),
	}

	return intervention, nil
}

// enforceClamp validates LLM output against the level clamp. On violation,
// retries once with a tightening directive. If the retry also violates,
// sanitizes the output and annotates the rationale.
func (s *Service) enforceClamp(
	ctx context.Context,
	provider llm.Provider,
	level domain.InterventionLevel,
	userPrompt, systemPrompt, initial string,
) (content, rationaleSuffix string) {
	if s.clampValidator == nil {
		return initial, ""
	}

	if err := s.clampValidator.Validate(level, initial); err == nil {
		return initial, ""
	} else {
		// Retry once with stricter system prompt.
		violation := &ClampViolation{}
		reason := "unspecified"
		if errors.As(err, &violation) {
			reason = violation.Reason
		}

		retryResp, retryErr := provider.Generate(ctx, &llm.Request{
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: userPrompt},
			},
			System:      systemPrompt + s.clampValidator.TighteningDirective(level, reason),
			MaxTokens:   1024,
			Temperature: 0.5,
		})
		if retryErr == nil {
			if validateErr := s.clampValidator.Validate(level, retryResp.Content); validateErr == nil {
				return retryResp.Content, "; clamp retry succeeded"
			} else {
				// Retry also violated. Sanitize as last resort.
				return s.clampValidator.Sanitize(level, retryResp.Content),
					"; clamp violated twice — output sanitized"
			}
		}

		// Retry failed (network etc) — sanitize the original.
		return s.clampValidator.Sanitize(level, initial), "; clamp violated, retry failed — sanitized"
	}
}

// IntervenStream generates an intervention with streaming response
func (s *Service) IntervenStream(ctx context.Context, req InterventionRequest) (<-chan StreamChunk, error) {
	level := s.selector.SelectLevel(req.Intent, req.Context, req.Policy)
	level = req.Policy.ClampLevel(level)
	interventionType := s.selector.SelectType(req.Intent, level)

	prompt := s.prompter.BuildPrompt(PromptRequest{
		Intent:         req.Intent,
		Level:          level,
		Type:           interventionType,
		Exercise:       req.Context.Exercise,
		Code:           req.Context.Code,
		Output:         req.Context.RunOutput,
		Profile:        req.Context.Profile,
		Spec:           req.Context.Spec,
		FocusCriterion: req.Context.FocusCriterion,
	})

	provider, err := s.llmRegistry.Default()
	if err != nil {
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	llmStream, err := provider.GenerateStream(ctx, &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System:      s.prompter.SystemPrompt(level),
		MaxTokens:   1024,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generate stream: %w", err)
	}

	outCh := make(chan StreamChunk, 100)

	go func() {
		defer close(outCh)

		// Send metadata first
		outCh <- StreamChunk{
			Type: "metadata",
			Metadata: &InterventionMetadata{
				Level: level,
				Type:  interventionType,
			},
		}

		// Stream content
		for chunk := range llmStream {
			if chunk.Error != nil {
				outCh <- StreamChunk{Type: "error", Error: chunk.Error}
				return
			}
			if chunk.Done {
				outCh <- StreamChunk{Type: "done"}
				return
			}
			outCh <- StreamChunk{Type: "content", Content: chunk.Content}
		}
	}()

	return outCh, nil
}

// StreamChunk represents a streaming chunk
type StreamChunk struct {
	Type     string
	Content  string
	Metadata *InterventionMetadata
	Error    error
}

// InterventionMetadata contains intervention metadata
type InterventionMetadata struct {
	Level domain.InterventionLevel
	Type  domain.InterventionType
}

func (s *Service) extractTargets(ctx InterventionContext) []domain.Target {
	if ctx.CurrentFile == "" {
		return nil
	}

	return []domain.Target{
		{
			File:      ctx.CurrentFile,
			StartLine: ctx.CursorLine,
			EndLine:   ctx.CursorLine,
		},
	}
}

// SuggestForSection generates suggestions for a spec section based on project docs
func (s *Service) SuggestForSection(ctx context.Context, authCtx AuthoringContext) ([]domain.AuthoringSuggestion, error) {
	if !authCtx.HasDocuments() {
		return nil, fmt.Errorf("no documents available for authoring")
	}

	// Build prompt for suggestions
	prompt := s.prompter.BuildAuthoringPrompt(authCtx)

	// Get LLM provider
	provider, err := s.llmRegistry.Default()
	if err != nil {
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	// Generate suggestions
	llmResp, err := provider.Generate(ctx, &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System:      s.prompter.AuthoringSystemPrompt(authCtx.Section),
		MaxTokens:   2048,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generate suggestions: %w", err)
	}

	// Parse suggestions from response
	suggestions := s.prompter.ParseSuggestions(llmResp.Content, authCtx.Section)

	return suggestions, nil
}

// AuthoringHint generates a hint for spec authoring based on a question
func (s *Service) AuthoringHint(ctx context.Context, authCtx AuthoringContext) (*domain.Intervention, error) {
	// Build prompt for hint
	prompt := s.prompter.BuildAuthoringHintPrompt(authCtx)

	// Get LLM provider
	provider, err := s.llmRegistry.Default()
	if err != nil {
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	// Generate hint
	llmResp, err := provider.Generate(ctx, &llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System:      s.prompter.AuthoringSystemPrompt(authCtx.Section),
		MaxTokens:   1024,
		Temperature: 0.7,
	})
	if err != nil {
		return nil, fmt.Errorf("generate hint: %w", err)
	}

	return &domain.Intervention{
		ID:          uuid.New(),
		Intent:      domain.IntentExplain,
		Level:       domain.L3ConstrainedSnippet,
		Type:        domain.TypeExplain,
		Content:     llmResp.Content,
		RequestedAt: time.Now(),
		DeliveredAt: time.Now(),
	}, nil
}
