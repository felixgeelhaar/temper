package llm

// LLMRegistry defines the interface for LLM provider registry operations
// used by the daemon handlers
type LLMRegistry interface {
	// List returns all registered provider names
	List() []string

	// Default returns the default provider
	Default() (Provider, error)

	// Get retrieves a provider by name
	Get(name string) (Provider, error)

	// SetDefault sets the default provider
	SetDefault(name string) error

	// Register adds a provider to the registry
	Register(name string, p Provider)
}

// Ensure Registry implements LLMRegistry
var _ LLMRegistry = (*Registry)(nil)
