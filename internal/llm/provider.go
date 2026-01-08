package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrProviderNotFound  = errors.New("provider not found")
	ErrNoDefaultProvider = errors.New("no default provider configured")
)

// Provider defines the interface for LLM providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// Generate performs a completion request
	Generate(ctx context.Context, req *Request) (*Response, error)

	// GenerateStream performs a streaming completion request
	GenerateStream(ctx context.Context, req *Request) (<-chan StreamChunk, error)

	// SupportsStreaming returns whether the provider supports streaming
	SupportsStreaming() bool
}

// Request represents an LLM request
type Request struct {
	Model       string
	Messages    []Message
	MaxTokens   int
	Temperature float64
	StopSeqs    []string
	System      string // System prompt (some providers handle this separately)
}

// Message represents a chat message
type Message struct {
	Role    Role
	Content string
}

// Role represents the role of a message sender
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Response represents an LLM response
type Response struct {
	Content      string
	FinishReason string
	Usage        Usage
}

// Usage tracks token usage
type Usage struct {
	InputTokens  int
	OutputTokens int
}

// StreamChunk represents a streaming chunk
type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

// Registry manages LLM providers
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
	defaultP  string
}

// NewRegistry creates a new provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *Registry) Register(name string, p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = p
}

// SetDefault sets the default provider
func (r *Registry) SetDefault(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.providers[name]; !ok {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	r.defaultP = name
	return nil
}

// Get retrieves a provider by name
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	return p, nil
}

// Default returns the default provider
// If default is "auto" or not found, returns the first available provider
func (r *Registry) Default() (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try explicit default first (unless it's "auto")
	if r.defaultP != "" && r.defaultP != "auto" {
		if p, ok := r.providers[r.defaultP]; ok {
			return p, nil
		}
	}

	// Auto-select: return first available provider
	for _, p := range r.providers {
		return p, nil
	}

	return nil, ErrNoDefaultProvider
}

// List returns all registered provider names
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// DefaultName returns the name of the default provider
func (r *Registry) DefaultName() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.defaultP
}
