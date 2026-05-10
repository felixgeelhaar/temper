package pairing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/felixgeelhaar/temper/internal/correlation"
	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/llm"
	"github.com/felixgeelhaar/temper/internal/profile"
	"github.com/google/uuid"
)

// Service handles pairing engine operations
type Service struct {
	llmRegistry     *llm.Registry
	defaultProvider string
	levelModels     map[domain.InterventionLevel]string
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

// SetLevelModels configures per-level model overrides. Empty map disables
// routing — the provider's default model is used for every level.
func (s *Service) SetLevelModels(m map[domain.InterventionLevel]string) {
	s.levelModels = m
}

// modelForLevel returns the configured model for a level, or empty when
// no override is set. Empty signals to the provider that its default
// should be used.
func (s *Service) modelForLevel(level domain.InterventionLevel) string {
	if s.levelModels == nil {
		return ""
	}
	return s.levelModels[level]
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

// exerciseLanguage returns the language slug from a context's exercise,
// or empty when no exercise is attached. Empty triggers the prompter's
// language-agnostic fallback.
func exerciseLanguage(ex *domain.Exercise) string {
	if ex == nil {
		return ""
	}
	return ex.Language
}

// applyPolicyClamp applies the global MaxLevel ceiling plus an optional
// per-topic adjustment based on the learner's profile. Falls back to the
// plain ClampLevel path when no exercise or profile is available so older
// session shapes (no exercise context) keep working.
func applyPolicyClamp(requested domain.InterventionLevel, policy domain.LearningPolicy, ctx InterventionContext) domain.InterventionLevel {
	if ctx.Exercise == nil || ctx.Profile == nil {
		return policy.ClampLevel(requested)
	}
	topic := profile.ExtractTopic(ctx.Exercise.ID)
	skill := ctx.Profile.GetSkillLevel(topic).Level
	return policy.ClampForTopic(requested, skill)
}

// fallbackModelLabel returns a human-readable label for the model that
// will be used. Empty input means the provider default is in effect.
func fallbackModelLabel(m string) string {
	if m == "" {
		return "(provider default)"
	}
	return m
}

// Intervene generates an intervention based on the request
func (s *Service) Intervene(ctx context.Context, req InterventionRequest) (*domain.Intervention, error) {
	var level domain.InterventionLevel

	// Use explicit level if provided (for escalation requests)
	if req.ExplicitLevel > 0 {
		level = req.ExplicitLevel
	} else {
		// Select intervention level based on context
		level = s.selector.SelectLevel(req.Intent, req.Context, req.Policy)
		// Apply policy clamp, including the per-topic adjustment when the
		// learner's exercise + profile expose a topic skill.
		level = applyPolicyClamp(level, req.Policy, req.Context)
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

	systemPrompt := s.prompter.SystemPromptForLanguage(level, exerciseLanguage(req.Context.Exercise))
	systemBlocks := []llm.SystemContentBlock{
		// Stable per (provider, level, language) — cache it. Hint requests
		// within a session reuse the same level system prompt repeatedly.
		{Text: systemPrompt, CacheControl: true},
	}

	chosenModel := s.modelForLevel(level)

	// Get LLM provider. If none is available (no API key, all disabled),
	// fall back to the offline path so the user still gets useful guidance.
	provider, err := s.llmRegistry.Default()
	if err != nil {
		if fallback := s.offlineIntervention(req, level, interventionType, "no LLM provider available"); fallback != nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("get LLM provider: %w", err)
	}

	// Generate intervention content
	llmResp, err := provider.Generate(ctx, &llm.Request{
		Model: chosenModel,
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System:        systemPrompt,
		SystemBlocks:  systemBlocks,
		CorrelationID: correlation.FromContext(ctx),
		MaxTokens:     1024,
		Temperature:   0.7,
	})
	if err != nil {
		// LLM failed (network, circuit breaker open, rate limit, etc.).
		// Serve a YAML hint when one is available rather than fail hard.
		if fallback := s.offlineIntervention(req, level, interventionType, fmt.Sprintf("LLM error: %v", err)); fallback != nil {
			return fallback, nil
		}
		return nil, fmt.Errorf("generate intervention: %w", err)
	}

	content, clampRationale := s.enforceClamp(ctx, provider, level, prompt, systemPrompt, llmResp.Content)

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
		Rationale:   buildRationale(level, req, chosenModel, clampRationale),
		RequestedAt: time.Now(),
		DeliveredAt: time.Now(),
	}

	return intervention, nil
}

// buildRationale composes the rich, user-facing rationale string surfaced
// via `temper hint --why` and editor equivalents. Should answer "why this
// level for me?" with concrete signals: intent, dependency, topic skill,
// run-output, model, and any clamp-retry note.
func buildRationale(
	level domain.InterventionLevel,
	req InterventionRequest,
	chosenModel string,
	clampNote string,
) string {
	parts := []string{
		fmt.Sprintf("Selected L%d for intent=%s", level, req.Intent),
	}

	if req.Context.Profile != nil {
		dep := req.Context.Profile.HintDependency()
		parts = append(parts, fmt.Sprintf("hint dependency %.0f%%", dep*100))

		if req.Context.Exercise != nil {
			topic := profile.ExtractTopic(req.Context.Exercise.ID)
			skill := req.Context.Profile.GetSkillLevel(topic).Level
			parts = append(parts, fmt.Sprintf("topic=%s (skill %.2f)", topic, skill))
		}
	}

	if ex := req.Context.Exercise; ex != nil && ex.Difficulty != "" {
		parts = append(parts, fmt.Sprintf("difficulty=%s", ex.Difficulty))
	}

	if out := req.Context.RunOutput; out != nil {
		switch {
		case out.AllTestsPassed():
			parts = append(parts, "all tests passing")
		case out.HasErrors():
			parts = append(parts, "build errors present")
		case out.TestsFailed > 0:
			parts = append(parts, fmt.Sprintf("%d tests failing", out.TestsFailed))
		}
	}

	parts = append(parts, fmt.Sprintf("policy clamp=L%d", req.Policy.MaxLevel))
	parts = append(parts, fmt.Sprintf("model=%s", fallbackModelLabel(chosenModel)))

	out := strings.Join(parts, "; ")
	if clampNote != "" {
		out += clampNote
	}
	return out
}

// offlineIntervention serves a level-appropriate hint from the exercise's
// YAML when the LLM is unavailable. Returns nil if no hint is available
// for the requested level (caller should propagate the LLM error).
//
// The returned intervention's content is prefixed with "[offline mode]" and
// the rationale explains why the LLM was skipped, so the user is never
// misled about the source of the guidance.
func (s *Service) offlineIntervention(
	req InterventionRequest,
	level domain.InterventionLevel,
	iType domain.InterventionType,
	reason string,
) *domain.Intervention {
	if req.Context.Exercise == nil {
		return nil
	}
	hints := req.Context.Exercise.GetHintsForLevel(level)
	if len(hints) == 0 {
		// Try one level down — better to under-help than fail entirely.
		if level > domain.L0Clarify {
			hints = req.Context.Exercise.GetHintsForLevel(level.Decrement())
		}
	}
	if len(hints) == 0 {
		return nil
	}

	content := "[offline mode] " + hints[0]

	return &domain.Intervention{
		ID:        uuid.New(),
		SessionID: req.SessionID,
		UserID:    req.UserID,
		RunID:     req.RunID,
		Intent:    req.Intent,
		Level:     level,
		Type:      iType,
		Content:   content,
		Targets:   s.extractTargets(req.Context),
		Rationale: fmt.Sprintf("Offline fallback at L%d (intent=%s); reason: %s",
			level, req.Intent, reason),
		RequestedAt: time.Now(),
		DeliveredAt: time.Now(),
	}
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

	streamSystem := s.prompter.SystemPromptForLanguage(level, exerciseLanguage(req.Context.Exercise))
	llmStream, err := provider.GenerateStream(ctx, &llm.Request{
		Model: s.modelForLevel(level),
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		System: streamSystem,
		SystemBlocks: []llm.SystemContentBlock{
			{Text: streamSystem, CacheControl: true},
		},
		CorrelationID: correlation.FromContext(ctx),
		MaxTokens:     1024,
		Temperature:   0.7,
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
		System: s.prompter.AuthoringSystemPrompt(authCtx.Section),
		SystemBlocks: []llm.SystemContentBlock{
			{Text: s.prompter.AuthoringSystemPrompt(authCtx.Section), CacheControl: true},
		},
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
		System: s.prompter.AuthoringSystemPrompt(authCtx.Section),
		SystemBlocks: []llm.SystemContentBlock{
			{Text: s.prompter.AuthoringSystemPrompt(authCtx.Section), CacheControl: true},
		},
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
