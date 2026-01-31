// Package events_test provides tests for the events package.
package events_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/events"
	infraevents "github.com/north-cloud/infrastructure/events"
)

func TestNoOpHandler_AllMethods_ReturnNil(t *testing.T) {
	t.Helper()

	handler := events.NewNoOpHandler(nil)
	ctx := context.Background()

	testCases := []struct {
		name    string
		handler func() error
	}{
		{
			name: "HandleSourceCreated",
			handler: func() error {
				return handler.HandleSourceCreated(ctx, infraevents.SourceEvent{
					EventID:   uuid.New(),
					EventType: infraevents.SourceCreated,
					SourceID:  uuid.New(),
					Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
				})
			},
		},
		{
			name: "HandleSourceUpdated",
			handler: func() error {
				return handler.HandleSourceUpdated(ctx, infraevents.SourceEvent{
					EventID:   uuid.New(),
					EventType: infraevents.SourceUpdated,
					SourceID:  uuid.New(),
					Payload:   infraevents.SourceUpdatedPayload{ChangedFields: []string{"name"}},
				})
			},
		},
		{
			name: "HandleSourceDeleted",
			handler: func() error {
				return handler.HandleSourceDeleted(ctx, infraevents.SourceEvent{
					EventID:   uuid.New(),
					EventType: infraevents.SourceDeleted,
					SourceID:  uuid.New(),
					Payload:   infraevents.SourceDeletedPayload{Name: "Test", DeletionReason: "test"},
				})
			},
		},
		{
			name: "HandleSourceEnabled",
			handler: func() error {
				return handler.HandleSourceEnabled(ctx, infraevents.SourceEvent{
					EventID:   uuid.New(),
					EventType: infraevents.SourceEnabled,
					SourceID:  uuid.New(),
					Payload:   infraevents.SourceTogglePayload{Reason: "test", ToggledBy: "user"},
				})
			},
		},
		{
			name: "HandleSourceDisabled",
			handler: func() error {
				return handler.HandleSourceDisabled(ctx, infraevents.SourceEvent{
					EventID:   uuid.New(),
					EventType: infraevents.SourceDisabled,
					SourceID:  uuid.New(),
					Payload:   infraevents.SourceTogglePayload{Reason: "test", ToggledBy: "system"},
				})
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.handler()
			if err != nil {
				t.Errorf("%s: expected nil error, got %v", tc.name, err)
			}
		})
	}
}

func TestNoOpHandler_ImplementsEventHandler(t *testing.T) {
	t.Helper()

	var _ events.EventHandler = (*events.NoOpHandler)(nil)
}
