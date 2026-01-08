package pairing

import (
	"context"

	"github.com/felixgeelhaar/temper/internal/domain"
)

// PairingService defines the interface for pairing engine operations
// used by the daemon handlers
type PairingService interface {
	// Intervene generates an intervention based on the request
	Intervene(ctx context.Context, req InterventionRequest) (*domain.Intervention, error)

	// IntervenStream generates an intervention with streaming response
	IntervenStream(ctx context.Context, req InterventionRequest) (<-chan StreamChunk, error)

	// SuggestForSection generates suggestions for a spec section based on project docs
	SuggestForSection(ctx context.Context, authCtx AuthoringContext) ([]domain.AuthoringSuggestion, error)

	// AuthoringHint generates a hint for spec authoring based on a question
	AuthoringHint(ctx context.Context, authCtx AuthoringContext) (*domain.Intervention, error)
}

// Ensure Service implements PairingService
var _ PairingService = (*Service)(nil)
