// Package events_test provides tests for the events package.
package events_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/events"
	infraevents "github.com/north-cloud/infrastructure/events"
)

func TestNoOpHandler_HandleSourceCreated_LogsAndReturnsNil(t *testing.T) {
	t.Helper()

	handler := events.NewNoOpHandler(nil)

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	err := handler.HandleSourceCreated(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNoOpHandler_HandleSourceUpdated_LogsAndReturnsNil(t *testing.T) {
	t.Helper()

	handler := events.NewNoOpHandler(nil)

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceUpdated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceUpdatedPayload{ChangedFields: []string{"name"}},
	}

	err := handler.HandleSourceUpdated(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNoOpHandler_HandleSourceDeleted_LogsAndReturnsNil(t *testing.T) {
	t.Helper()

	handler := events.NewNoOpHandler(nil)

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceDeleted,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceDeletedPayload{Name: "Test", DeletionReason: "test"},
	}

	err := handler.HandleSourceDeleted(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNoOpHandler_HandleSourceEnabled_LogsAndReturnsNil(t *testing.T) {
	t.Helper()

	handler := events.NewNoOpHandler(nil)

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceEnabled,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceTogglePayload{Reason: "test", ToggledBy: "user"},
	}

	err := handler.HandleSourceEnabled(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNoOpHandler_HandleSourceDisabled_LogsAndReturnsNil(t *testing.T) {
	t.Helper()

	handler := events.NewNoOpHandler(nil)

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceDisabled,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceTogglePayload{Reason: "test", ToggledBy: "system"},
	}

	err := handler.HandleSourceDisabled(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestNoOpHandler_ImplementsEventHandler(t *testing.T) {
	t.Helper()

	var _ events.EventHandler = (*events.NoOpHandler)(nil)
}
