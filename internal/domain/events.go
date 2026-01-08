package domain

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// -----------------------------------------------------------------------------
// Event Interface and Base Event
// -----------------------------------------------------------------------------

// Event represents a domain event
type Event interface {
	// EventID returns the unique identifier for this event
	EventID() uuid.UUID
	// EventType returns the type name of this event
	EventType() string
	// OccurredAt returns when this event occurred
	OccurredAt() time.Time
	// AggregateID returns the ID of the aggregate that produced this event
	AggregateID() uuid.UUID
	// AggregateType returns the type of aggregate that produced this event
	AggregateType() string
}

// BaseEvent provides common event fields
type BaseEvent struct {
	ID            uuid.UUID `json:"id"`
	Type          string    `json:"type"`
	Timestamp     time.Time `json:"timestamp"`
	AggregateUUID uuid.UUID `json:"aggregate_id"`
	AggregateName string    `json:"aggregate_type"`
}

// NewBaseEvent creates a new BaseEvent
func NewBaseEvent(eventType, aggregateType string, aggregateID uuid.UUID) BaseEvent {
	return BaseEvent{
		ID:            uuid.New(),
		Type:          eventType,
		Timestamp:     time.Now(),
		AggregateUUID: aggregateID,
		AggregateName: aggregateType,
	}
}

func (e BaseEvent) EventID() uuid.UUID     { return e.ID }
func (e BaseEvent) EventType() string      { return e.Type }
func (e BaseEvent) OccurredAt() time.Time  { return e.Timestamp }
func (e BaseEvent) AggregateID() uuid.UUID { return e.AggregateUUID }
func (e BaseEvent) AggregateType() string  { return e.AggregateName }

// -----------------------------------------------------------------------------
// Event Handler and Dispatcher
// -----------------------------------------------------------------------------

// EventHandler processes domain events
type EventHandler func(event Event)

// EventDispatcher manages event subscriptions and publishing
type EventDispatcher struct {
	mu          sync.RWMutex
	handlers    map[string][]EventHandler
	allHandlers []EventHandler // handlers for all events
}

// NewEventDispatcher creates a new event dispatcher
func NewEventDispatcher() *EventDispatcher {
	return &EventDispatcher{
		handlers: make(map[string][]EventHandler),
	}
}

// Subscribe registers a handler for a specific event type
func (d *EventDispatcher) Subscribe(eventType string, handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// SubscribeAll registers a handler for all event types
func (d *EventDispatcher) SubscribeAll(handler EventHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.allHandlers = append(d.allHandlers, handler)
}

// Publish dispatches an event to all registered handlers
func (d *EventDispatcher) Publish(event Event) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Call type-specific handlers
	if handlers, ok := d.handlers[event.EventType()]; ok {
		for _, h := range handlers {
			h(event)
		}
	}

	// Call all-event handlers
	for _, h := range d.allHandlers {
		h(event)
	}
}

// PublishAll dispatches multiple events
func (d *EventDispatcher) PublishAll(events []Event) {
	for _, event := range events {
		d.Publish(event)
	}
}

// -----------------------------------------------------------------------------
// Aggregate Root with Event Support
// -----------------------------------------------------------------------------

// EventRecorder is an interface for aggregates that record events
type EventRecorder interface {
	// RecordedEvents returns events recorded since last clear
	RecordedEvents() []Event
	// ClearEvents clears recorded events (typically after persistence)
	ClearEvents()
}

// AggregateRoot provides base functionality for aggregates with event recording
type AggregateRoot struct {
	events []Event
}

// RecordEvent adds an event to the aggregate's recorded events
func (a *AggregateRoot) RecordEvent(event Event) {
	a.events = append(a.events, event)
}

// RecordedEvents returns all recorded events
func (a *AggregateRoot) RecordedEvents() []Event {
	return a.events
}

// ClearEvents clears recorded events
func (a *AggregateRoot) ClearEvents() {
	a.events = nil
}

// -----------------------------------------------------------------------------
// Session Events
// -----------------------------------------------------------------------------

// SessionStartedEvent is published when a pairing session begins
type SessionStartedEvent struct {
	BaseEvent
	UserID     uuid.UUID `json:"user_id"`
	ExerciseID string    `json:"exercise_id,omitempty"`
	Intent     string    `json:"intent"`
}

// NewSessionStartedEvent creates a new session started event
func NewSessionStartedEvent(sessionID, userID uuid.UUID, exerciseID, intent string) SessionStartedEvent {
	return SessionStartedEvent{
		BaseEvent:  NewBaseEvent("session.started", "Session", sessionID),
		UserID:     userID,
		ExerciseID: exerciseID,
		Intent:     intent,
	}
}

// SessionEndedEvent is published when a pairing session ends
type SessionEndedEvent struct {
	BaseEvent
	UserID        uuid.UUID         `json:"user_id"`
	Duration      time.Duration     `json:"duration"`
	Interventions int               `json:"interventions"`
	FinalLevel    InterventionLevel `json:"final_level"`
}

// NewSessionEndedEvent creates a new session ended event
func NewSessionEndedEvent(sessionID, userID uuid.UUID, duration time.Duration, interventions int, finalLevel InterventionLevel) SessionEndedEvent {
	return SessionEndedEvent{
		BaseEvent:     NewBaseEvent("session.ended", "Session", sessionID),
		UserID:        userID,
		Duration:      duration,
		Interventions: interventions,
		FinalLevel:    finalLevel,
	}
}

// -----------------------------------------------------------------------------
// Intervention Events
// -----------------------------------------------------------------------------

// InterventionRequestedEvent is published when the user requests help
type InterventionRequestedEvent struct {
	BaseEvent
	SessionID uuid.UUID `json:"session_id"`
	UserID    uuid.UUID `json:"user_id"`
	Intent    string    `json:"intent"`
}

// NewInterventionRequestedEvent creates a new intervention requested event
func NewInterventionRequestedEvent(interventionID, sessionID, userID uuid.UUID, intent string) InterventionRequestedEvent {
	return InterventionRequestedEvent{
		BaseEvent: NewBaseEvent("intervention.requested", "Intervention", interventionID),
		SessionID: sessionID,
		UserID:    userID,
		Intent:    intent,
	}
}

// InterventionDeliveredEvent is published when AI provides an intervention
type InterventionDeliveredEvent struct {
	BaseEvent
	SessionID uuid.UUID         `json:"session_id"`
	UserID    uuid.UUID         `json:"user_id"`
	Level     InterventionLevel `json:"level"`
	Escalated bool              `json:"escalated"`
}

// NewInterventionDeliveredEvent creates a new intervention delivered event
func NewInterventionDeliveredEvent(interventionID, sessionID, userID uuid.UUID, level InterventionLevel, escalated bool) InterventionDeliveredEvent {
	return InterventionDeliveredEvent{
		BaseEvent: NewBaseEvent("intervention.delivered", "Intervention", interventionID),
		SessionID: sessionID,
		UserID:    userID,
		Level:     level,
		Escalated: escalated,
	}
}

// -----------------------------------------------------------------------------
// Exercise Events
// -----------------------------------------------------------------------------

// ExerciseStartedEvent is published when a user starts an exercise
type ExerciseStartedEvent struct {
	BaseEvent
	UserID     uuid.UUID `json:"user_id"`
	SessionID  uuid.UUID `json:"session_id"`
	ExerciseID string    `json:"exercise_id"`
}

// NewExerciseStartedEvent creates a new exercise started event
func NewExerciseStartedEvent(artifactID, userID, sessionID uuid.UUID, exerciseID string) ExerciseStartedEvent {
	return ExerciseStartedEvent{
		BaseEvent:  NewBaseEvent("exercise.started", "Artifact", artifactID),
		UserID:     userID,
		SessionID:  sessionID,
		ExerciseID: exerciseID,
	}
}

// ExerciseCompletedEvent is published when a user completes an exercise
type ExerciseCompletedEvent struct {
	BaseEvent
	UserID     uuid.UUID         `json:"user_id"`
	SessionID  uuid.UUID         `json:"session_id"`
	ExerciseID string            `json:"exercise_id"`
	Duration   time.Duration     `json:"duration"`
	HintsUsed  int               `json:"hints_used"`
	MaxLevel   InterventionLevel `json:"max_level"`
}

// NewExerciseCompletedEvent creates a new exercise completed event
func NewExerciseCompletedEvent(artifactID, userID, sessionID uuid.UUID, exerciseID string, duration time.Duration, hintsUsed int, maxLevel InterventionLevel) ExerciseCompletedEvent {
	return ExerciseCompletedEvent{
		BaseEvent:  NewBaseEvent("exercise.completed", "Artifact", artifactID),
		UserID:     userID,
		SessionID:  sessionID,
		ExerciseID: exerciseID,
		Duration:   duration,
		HintsUsed:  hintsUsed,
		MaxLevel:   maxLevel,
	}
}

// -----------------------------------------------------------------------------
// Run Events
// -----------------------------------------------------------------------------

// RunCompletedEvent is published when a code run completes
type RunCompletedEvent struct {
	BaseEvent
	UserID    uuid.UUID `json:"user_id"`
	SessionID uuid.UUID `json:"session_id"`
	Success   bool      `json:"success"`
	TestsPass int       `json:"tests_pass"`
	TestsFail int       `json:"tests_fail"`
}

// NewRunCompletedEvent creates a new run completed event
func NewRunCompletedEvent(runID, userID, sessionID uuid.UUID, success bool, testsPass, testsFail int) RunCompletedEvent {
	return RunCompletedEvent{
		BaseEvent: NewBaseEvent("run.completed", "Run", runID),
		UserID:    userID,
		SessionID: sessionID,
		Success:   success,
		TestsPass: testsPass,
		TestsFail: testsFail,
	}
}

// -----------------------------------------------------------------------------
// Patch Events
// -----------------------------------------------------------------------------

// PatchProposedEvent is published when AI proposes a code patch
type PatchProposedEvent struct {
	BaseEvent
	SessionID uuid.UUID         `json:"session_id"`
	UserID    uuid.UUID         `json:"user_id"`
	File      string            `json:"file"`
	Level     InterventionLevel `json:"level"`
}

// NewPatchProposedEvent creates a new patch proposed event
func NewPatchProposedEvent(patchID, sessionID, userID uuid.UUID, file string, level InterventionLevel) PatchProposedEvent {
	return PatchProposedEvent{
		BaseEvent: NewBaseEvent("patch.proposed", "Patch", patchID),
		SessionID: sessionID,
		UserID:    userID,
		File:      file,
		Level:     level,
	}
}

// PatchAppliedEvent is published when user applies a patch
type PatchAppliedEvent struct {
	BaseEvent
	SessionID uuid.UUID `json:"session_id"`
	UserID    uuid.UUID `json:"user_id"`
	File      string    `json:"file"`
}

// NewPatchAppliedEvent creates a new patch applied event
func NewPatchAppliedEvent(patchID, sessionID, userID uuid.UUID, file string) PatchAppliedEvent {
	return PatchAppliedEvent{
		BaseEvent: NewBaseEvent("patch.applied", "Patch", patchID),
		SessionID: sessionID,
		UserID:    userID,
		File:      file,
	}
}

// PatchRejectedEvent is published when user rejects a patch
type PatchRejectedEvent struct {
	BaseEvent
	SessionID uuid.UUID `json:"session_id"`
	UserID    uuid.UUID `json:"user_id"`
	File      string    `json:"file"`
}

// NewPatchRejectedEvent creates a new patch rejected event
func NewPatchRejectedEvent(patchID, sessionID, userID uuid.UUID, file string) PatchRejectedEvent {
	return PatchRejectedEvent{
		BaseEvent: NewBaseEvent("patch.rejected", "Patch", patchID),
		SessionID: sessionID,
		UserID:    userID,
		File:      file,
	}
}

// -----------------------------------------------------------------------------
// Profile Events
// -----------------------------------------------------------------------------

// SkillUpdatedEvent is published when a user's skill level changes
type SkillUpdatedEvent struct {
	BaseEvent
	Topic    string  `json:"topic"`
	OldLevel float64 `json:"old_level"`
	NewLevel float64 `json:"new_level"`
	Reason   string  `json:"reason"`
}

// NewSkillUpdatedEvent creates a new skill updated event
func NewSkillUpdatedEvent(userID uuid.UUID, topic string, oldLevel, newLevel float64, reason string) SkillUpdatedEvent {
	return SkillUpdatedEvent{
		BaseEvent: NewBaseEvent("skill.updated", "LearningProfile", userID),
		Topic:     topic,
		OldLevel:  oldLevel,
		NewLevel:  newLevel,
		Reason:    reason,
	}
}

// HintDependencyDetectedEvent is published when hint dependency pattern is detected
type HintDependencyDetectedEvent struct {
	BaseEvent
	Topic        string  `json:"topic"`
	Dependency   float64 `json:"dependency"`
	SessionCount int     `json:"session_count"`
}

// NewHintDependencyDetectedEvent creates a new hint dependency detected event
func NewHintDependencyDetectedEvent(userID uuid.UUID, topic string, dependency float64, sessionCount int) HintDependencyDetectedEvent {
	return HintDependencyDetectedEvent{
		BaseEvent:    NewBaseEvent("profile.hint_dependency_detected", "LearningProfile", userID),
		Topic:        topic,
		Dependency:   dependency,
		SessionCount: sessionCount,
	}
}
