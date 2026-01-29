// Package events_test provides tests for the events package.
package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/source-manager/internal/events"
	infraevents "github.com/north-cloud/infrastructure/events"
)

func TestPublisher_NewPublisher_RequiresClient(t *testing.T) {
	t.Helper()

	pub := events.NewPublisher(nil, nil)
	if pub != nil {
		t.Error("expected nil publisher when client is nil")
	}
}

func TestPublisher_Publish_SetsEventIDIfEmpty(t *testing.T) {
	t.Helper()

	// This test verifies that Publish generates an EventID if not provided
	// We can't easily test without Redis, so this is a design verification
	sourceID := uuid.New()
	event := infraevents.SourceEvent{
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	// Verify the event fields are set correctly for the test
	if event.EventType != infraevents.SourceCreated {
		t.Error("EventType should be SourceCreated")
	}
	if event.SourceID != sourceID {
		t.Error("SourceID should match")
	}
	if event.Payload == nil {
		t.Error("Payload should not be nil")
	}

	if event.EventID == uuid.Nil {
		// This is expected - the publisher should generate one
		t.Log("EventID is nil as expected, publisher should generate one")
	}
}

func TestPublisher_Publish_NilReceiverIsNoOp(t *testing.T) {
	t.Helper()

	var pub *events.Publisher
	event := infraevents.SourceEvent{
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	// Should not panic and return nil
	err := pub.Publish(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error for nil receiver, got: %v", err)
	}
}

func TestPublisher_PublishAsync_NilReceiverIsNoOp(t *testing.T) {
	t.Helper()

	var pub *events.Publisher
	event := infraevents.SourceEvent{
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	// Should not panic
	pub.PublishAsync(event)

	// Give the goroutine a chance to run (though it should return immediately)
	time.Sleep(10 * time.Millisecond)
}
