package domain

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBaseEvent(t *testing.T) {
	aggregateID := uuid.New()
	event := NewBaseEvent("test.created", "TestAggregate", aggregateID)

	t.Run("EventID is unique", func(t *testing.T) {
		if event.EventID() == uuid.Nil {
			t.Error("EventID() should not be nil")
		}
	})

	t.Run("EventType", func(t *testing.T) {
		if event.EventType() != "test.created" {
			t.Errorf("EventType() = %q, want test.created", event.EventType())
		}
	})

	t.Run("OccurredAt is set", func(t *testing.T) {
		if event.OccurredAt().IsZero() {
			t.Error("OccurredAt() should not be zero")
		}
		if event.OccurredAt().After(time.Now()) {
			t.Error("OccurredAt() should not be in the future")
		}
	})

	t.Run("AggregateID", func(t *testing.T) {
		if event.AggregateID() != aggregateID {
			t.Errorf("AggregateID() = %v, want %v", event.AggregateID(), aggregateID)
		}
	})

	t.Run("AggregateType", func(t *testing.T) {
		if event.AggregateType() != "TestAggregate" {
			t.Errorf("AggregateType() = %q, want TestAggregate", event.AggregateType())
		}
	})
}

func TestEventDispatcher(t *testing.T) {
	t.Run("Subscribe and Publish", func(t *testing.T) {
		dispatcher := NewEventDispatcher()
		var received Event

		dispatcher.Subscribe("test.event", func(e Event) {
			received = e
		})

		event := NewBaseEvent("test.event", "Test", uuid.New())
		dispatcher.Publish(event)

		if received == nil {
			t.Fatal("Event handler was not called")
		}
		if received.EventType() != "test.event" {
			t.Errorf("Received event type = %q, want test.event", received.EventType())
		}
	})

	t.Run("Multiple handlers for same event type", func(t *testing.T) {
		dispatcher := NewEventDispatcher()
		callCount := 0
		mu := sync.Mutex{}

		for i := 0; i < 3; i++ {
			dispatcher.Subscribe("test.event", func(e Event) {
				mu.Lock()
				callCount++
				mu.Unlock()
			})
		}

		event := NewBaseEvent("test.event", "Test", uuid.New())
		dispatcher.Publish(event)

		if callCount != 3 {
			t.Errorf("Handler call count = %d, want 3", callCount)
		}
	})

	t.Run("SubscribeAll receives all events", func(t *testing.T) {
		dispatcher := NewEventDispatcher()
		var receivedEvents []Event
		mu := sync.Mutex{}

		dispatcher.SubscribeAll(func(e Event) {
			mu.Lock()
			receivedEvents = append(receivedEvents, e)
			mu.Unlock()
		})

		event1 := NewBaseEvent("event.type1", "Test", uuid.New())
		event2 := NewBaseEvent("event.type2", "Test", uuid.New())
		dispatcher.Publish(event1)
		dispatcher.Publish(event2)

		if len(receivedEvents) != 2 {
			t.Errorf("Received events count = %d, want 2", len(receivedEvents))
		}
	})

	t.Run("PublishAll dispatches multiple events", func(t *testing.T) {
		dispatcher := NewEventDispatcher()
		callCount := 0
		mu := sync.Mutex{}

		dispatcher.SubscribeAll(func(e Event) {
			mu.Lock()
			callCount++
			mu.Unlock()
		})

		events := []Event{
			NewBaseEvent("event.1", "Test", uuid.New()),
			NewBaseEvent("event.2", "Test", uuid.New()),
			NewBaseEvent("event.3", "Test", uuid.New()),
		}
		dispatcher.PublishAll(events)

		if callCount != 3 {
			t.Errorf("Handler call count = %d, want 3", callCount)
		}
	})

	t.Run("Unsubscribed events are ignored", func(t *testing.T) {
		dispatcher := NewEventDispatcher()
		called := false

		dispatcher.Subscribe("other.event", func(e Event) {
			called = true
		})

		event := NewBaseEvent("test.event", "Test", uuid.New())
		dispatcher.Publish(event)

		if called {
			t.Error("Handler should not be called for unsubscribed event type")
		}
	})
}

func TestAggregateRoot(t *testing.T) {
	t.Run("RecordEvent and RecordedEvents", func(t *testing.T) {
		root := &AggregateRoot{}
		event := NewBaseEvent("test.event", "Test", uuid.New())

		root.RecordEvent(event)

		events := root.RecordedEvents()
		if len(events) != 1 {
			t.Fatalf("RecordedEvents() len = %d, want 1", len(events))
		}
		if events[0].EventType() != "test.event" {
			t.Errorf("Event type = %q, want test.event", events[0].EventType())
		}
	})

	t.Run("ClearEvents", func(t *testing.T) {
		root := &AggregateRoot{}
		root.RecordEvent(NewBaseEvent("event.1", "Test", uuid.New()))
		root.RecordEvent(NewBaseEvent("event.2", "Test", uuid.New()))

		if len(root.RecordedEvents()) != 2 {
			t.Fatal("Should have 2 events before clear")
		}

		root.ClearEvents()

		if len(root.RecordedEvents()) != 0 {
			t.Errorf("RecordedEvents() len = %d, want 0 after clear", len(root.RecordedEvents()))
		}
	})

	t.Run("Multiple events recorded in order", func(t *testing.T) {
		root := &AggregateRoot{}
		root.RecordEvent(NewBaseEvent("event.first", "Test", uuid.New()))
		root.RecordEvent(NewBaseEvent("event.second", "Test", uuid.New()))
		root.RecordEvent(NewBaseEvent("event.third", "Test", uuid.New()))

		events := root.RecordedEvents()
		if len(events) != 3 {
			t.Fatalf("RecordedEvents() len = %d, want 3", len(events))
		}
		if events[0].EventType() != "event.first" {
			t.Errorf("First event type = %q, want event.first", events[0].EventType())
		}
		if events[2].EventType() != "event.third" {
			t.Errorf("Third event type = %q, want event.third", events[2].EventType())
		}
	})
}

func TestSessionEvents(t *testing.T) {
	sessionID := uuid.New()
	userID := uuid.New()

	t.Run("SessionStartedEvent", func(t *testing.T) {
		event := NewSessionStartedEvent(sessionID, userID, "go-v1/hello", "training")

		if event.EventType() != "session.started" {
			t.Errorf("EventType() = %q, want session.started", event.EventType())
		}
		if event.AggregateType() != "Session" {
			t.Errorf("AggregateType() = %q, want Session", event.AggregateType())
		}
		if event.AggregateID() != sessionID {
			t.Errorf("AggregateID() = %v, want %v", event.AggregateID(), sessionID)
		}
		if event.UserID != userID {
			t.Errorf("UserID = %v, want %v", event.UserID, userID)
		}
		if event.ExerciseID != "go-v1/hello" {
			t.Errorf("ExerciseID = %q, want go-v1/hello", event.ExerciseID)
		}
		if event.Intent != "training" {
			t.Errorf("Intent = %q, want training", event.Intent)
		}
	})

	t.Run("SessionEndedEvent", func(t *testing.T) {
		event := NewSessionEndedEvent(sessionID, userID, 30*time.Minute, 5, L2LocationConcept)

		if event.EventType() != "session.ended" {
			t.Errorf("EventType() = %q, want session.ended", event.EventType())
		}
		if event.Duration != 30*time.Minute {
			t.Errorf("Duration = %v, want 30m", event.Duration)
		}
		if event.Interventions != 5 {
			t.Errorf("Interventions = %d, want 5", event.Interventions)
		}
		if event.FinalLevel != L2LocationConcept {
			t.Errorf("FinalLevel = %v, want L2LocationConcept", event.FinalLevel)
		}
	})
}

func TestInterventionEvents(t *testing.T) {
	interventionID := uuid.New()
	sessionID := uuid.New()
	userID := uuid.New()

	t.Run("InterventionRequestedEvent", func(t *testing.T) {
		event := NewInterventionRequestedEvent(interventionID, sessionID, userID, "hint")

		if event.EventType() != "intervention.requested" {
			t.Errorf("EventType() = %q, want intervention.requested", event.EventType())
		}
		if event.SessionID != sessionID {
			t.Errorf("SessionID = %v, want %v", event.SessionID, sessionID)
		}
		if event.Intent != "hint" {
			t.Errorf("Intent = %q, want hint", event.Intent)
		}
	})

	t.Run("InterventionDeliveredEvent", func(t *testing.T) {
		event := NewInterventionDeliveredEvent(interventionID, sessionID, userID, L3ConstrainedSnippet, true)

		if event.EventType() != "intervention.delivered" {
			t.Errorf("EventType() = %q, want intervention.delivered", event.EventType())
		}
		if event.Level != L3ConstrainedSnippet {
			t.Errorf("Level = %v, want L3ConstrainedSnippet", event.Level)
		}
		if !event.Escalated {
			t.Error("Escalated should be true")
		}
	})
}

func TestExerciseEvents(t *testing.T) {
	artifactID := uuid.New()
	sessionID := uuid.New()
	userID := uuid.New()

	t.Run("ExerciseStartedEvent", func(t *testing.T) {
		event := NewExerciseStartedEvent(artifactID, userID, sessionID, "go-v1/hello")

		if event.EventType() != "exercise.started" {
			t.Errorf("EventType() = %q, want exercise.started", event.EventType())
		}
		if event.AggregateType() != "Artifact" {
			t.Errorf("AggregateType() = %q, want Artifact", event.AggregateType())
		}
		if event.ExerciseID != "go-v1/hello" {
			t.Errorf("ExerciseID = %q, want go-v1/hello", event.ExerciseID)
		}
	})

	t.Run("ExerciseCompletedEvent", func(t *testing.T) {
		event := NewExerciseCompletedEvent(artifactID, userID, sessionID, "go-v1/hello", 15*time.Minute, 3, L2LocationConcept)

		if event.EventType() != "exercise.completed" {
			t.Errorf("EventType() = %q, want exercise.completed", event.EventType())
		}
		if event.Duration != 15*time.Minute {
			t.Errorf("Duration = %v, want 15m", event.Duration)
		}
		if event.HintsUsed != 3 {
			t.Errorf("HintsUsed = %d, want 3", event.HintsUsed)
		}
		if event.MaxLevel != L2LocationConcept {
			t.Errorf("MaxLevel = %v, want L2LocationConcept", event.MaxLevel)
		}
	})
}

func TestRunEvents(t *testing.T) {
	runID := uuid.New()
	sessionID := uuid.New()
	userID := uuid.New()

	t.Run("RunCompletedEvent success", func(t *testing.T) {
		event := NewRunCompletedEvent(runID, userID, sessionID, true, 10, 0)

		if event.EventType() != "run.completed" {
			t.Errorf("EventType() = %q, want run.completed", event.EventType())
		}
		if !event.Success {
			t.Error("Success should be true")
		}
		if event.TestsPass != 10 {
			t.Errorf("TestsPass = %d, want 10", event.TestsPass)
		}
		if event.TestsFail != 0 {
			t.Errorf("TestsFail = %d, want 0", event.TestsFail)
		}
	})

	t.Run("RunCompletedEvent failure", func(t *testing.T) {
		event := NewRunCompletedEvent(runID, userID, sessionID, false, 5, 3)

		if event.Success {
			t.Error("Success should be false")
		}
		if event.TestsFail != 3 {
			t.Errorf("TestsFail = %d, want 3", event.TestsFail)
		}
	})
}

func TestPatchEvents(t *testing.T) {
	patchID := uuid.New()
	sessionID := uuid.New()
	userID := uuid.New()

	t.Run("PatchProposedEvent", func(t *testing.T) {
		event := NewPatchProposedEvent(patchID, sessionID, userID, "main.go", L4PartialSolution)

		if event.EventType() != "patch.proposed" {
			t.Errorf("EventType() = %q, want patch.proposed", event.EventType())
		}
		if event.File != "main.go" {
			t.Errorf("File = %q, want main.go", event.File)
		}
		if event.Level != L4PartialSolution {
			t.Errorf("Level = %v, want L4PartialSolution", event.Level)
		}
	})

	t.Run("PatchAppliedEvent", func(t *testing.T) {
		event := NewPatchAppliedEvent(patchID, sessionID, userID, "main.go")

		if event.EventType() != "patch.applied" {
			t.Errorf("EventType() = %q, want patch.applied", event.EventType())
		}
	})

	t.Run("PatchRejectedEvent", func(t *testing.T) {
		event := NewPatchRejectedEvent(patchID, sessionID, userID, "main.go")

		if event.EventType() != "patch.rejected" {
			t.Errorf("EventType() = %q, want patch.rejected", event.EventType())
		}
	})
}

func TestProfileEvents(t *testing.T) {
	userID := uuid.New()

	t.Run("SkillUpdatedEvent", func(t *testing.T) {
		event := NewSkillUpdatedEvent(userID, "go-basics", 0.5, 0.6, "exercise completed")

		if event.EventType() != "skill.updated" {
			t.Errorf("EventType() = %q, want skill.updated", event.EventType())
		}
		if event.Topic != "go-basics" {
			t.Errorf("Topic = %q, want go-basics", event.Topic)
		}
		if event.OldLevel != 0.5 {
			t.Errorf("OldLevel = %f, want 0.5", event.OldLevel)
		}
		if event.NewLevel != 0.6 {
			t.Errorf("NewLevel = %f, want 0.6", event.NewLevel)
		}
	})

	t.Run("HintDependencyDetectedEvent", func(t *testing.T) {
		event := NewHintDependencyDetectedEvent(userID, "go-concurrency", 0.8, 5)

		if event.EventType() != "profile.hint_dependency_detected" {
			t.Errorf("EventType() = %q, want profile.hint_dependency_detected", event.EventType())
		}
		if event.Dependency != 0.8 {
			t.Errorf("Dependency = %f, want 0.8", event.Dependency)
		}
		if event.SessionCount != 5 {
			t.Errorf("SessionCount = %d, want 5", event.SessionCount)
		}
	})
}
