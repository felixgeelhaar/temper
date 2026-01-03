package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/felixgeelhaar/temper/internal/domain"
	"github.com/felixgeelhaar/temper/internal/storage"
)

// EventRepository implements domain.EventRepository using the storage layer
type EventRepository struct {
	queries *storage.Queries
}

// NewEventRepository creates a new EventRepository
func NewEventRepository(queries *storage.Queries) *EventRepository {
	return &EventRepository{queries: queries}
}

// Save persists a domain event
func (r *EventRepository) Save(ctx context.Context, event domain.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	// Extract user ID if the event has one
	var userID uuid.NullUUID
	if u, ok := extractUserIDFromEvent(event); ok {
		userID = uuid.NullUUID{UUID: u, Valid: true}
	}

	_, err = r.queries.CreateEvent(ctx, storage.CreateEventParams{
		UserID:    userID,
		EventType: event.EventType(),
		Payload:   payload,
	})
	return err
}

// SaveAll persists multiple domain events
func (r *EventRepository) SaveAll(ctx context.Context, events []domain.Event) error {
	for _, event := range events {
		if err := r.Save(ctx, event); err != nil {
			return err
		}
	}
	return nil
}

// ListByUser retrieves events for a user
func (r *EventRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Event, error) {
	events, err := r.queries.ListEventsByUser(ctx, storage.ListEventsByUserParams{
		UserID: uuid.NullUUID{UUID: userID, Valid: true},
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.Event, 0, len(events))
	for _, e := range events {
		event, err := mapStorageEventToDomain(e)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, nil
}

// ListByType retrieves events by type
func (r *EventRepository) ListByType(ctx context.Context, eventType string, limit, offset int) ([]domain.Event, error) {
	events, err := r.queries.ListEventsByType(ctx, storage.ListEventsByTypeParams{
		EventType: eventType,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})
	if err != nil {
		return nil, err
	}

	result := make([]domain.Event, 0, len(events))
	for _, e := range events {
		event, err := mapStorageEventToDomain(e)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, nil
}

// extractUserIDFromEvent attempts to extract a user ID from common event types
func extractUserIDFromEvent(event domain.Event) (uuid.UUID, bool) {
	switch e := event.(type) {
	case domain.SessionStartedEvent:
		return e.UserID, true
	case domain.SessionEndedEvent:
		return e.UserID, true
	case domain.InterventionRequestedEvent:
		return e.UserID, true
	case domain.InterventionDeliveredEvent:
		return e.UserID, true
	case domain.ExerciseStartedEvent:
		return e.UserID, true
	case domain.ExerciseCompletedEvent:
		return e.UserID, true
	case domain.RunCompletedEvent:
		return e.UserID, true
	case domain.PatchProposedEvent:
		return e.UserID, true
	case domain.PatchAppliedEvent:
		return e.UserID, true
	case domain.PatchRejectedEvent:
		return e.UserID, true
	default:
		return uuid.Nil, false
	}
}

// mapStorageEventToDomain converts a storage Event to a domain Event
// Note: This returns a generic wrapper since we store events as JSON
func mapStorageEventToDomain(e storage.Event) (domain.Event, error) {
	// Return a base event with the stored data
	// Full event reconstruction would require type-specific unmarshaling
	return &storedEvent{
		id:        e.ID,
		eventType: e.EventType,
		timestamp: e.CreatedAt,
		payload:   e.Payload,
	}, nil
}

// storedEvent is a wrapper for events loaded from storage
type storedEvent struct {
	id        uuid.UUID
	eventType string
	timestamp time.Time
	payload   json.RawMessage
}

func (e *storedEvent) EventID() uuid.UUID     { return e.id }
func (e *storedEvent) EventType() string      { return e.eventType }
func (e *storedEvent) OccurredAt() time.Time  { return e.timestamp }
func (e *storedEvent) AggregateID() uuid.UUID { return uuid.Nil }
func (e *storedEvent) AggregateType() string  { return "" }

// Ensure EventRepository implements domain.EventRepository
var _ domain.EventRepository = (*EventRepository)(nil)
