// Package events_test provides tests for shared event types for source lifecycle
// communication between source-manager and crawler via Redis Streams.
package events_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/north-cloud/infrastructure/events"
)

func TestSourceEvent_MarshalJSON(t *testing.T) {
	t.Helper()

	eventID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440001")
	sourceID := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	timestamp := time.Date(2026, 1, 29, 10, 30, 0, 0, time.UTC)

	event := events.SourceEvent{
		EventID:   eventID,
		EventType: events.SourceCreated,
		SourceID:  sourceID,
		Timestamp: timestamp,
		Payload: events.SourceCreatedPayload{
			Name:      "Example News",
			URL:       "https://example.com",
			RateLimit: 10,
			MaxDepth:  3,
			Enabled:   true,
			Priority:  events.PriorityNormal,
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded events.SourceEvent
	unmarshalErr := json.Unmarshal(data, &decoded)
	if unmarshalErr != nil {
		t.Fatalf("unmarshal failed: %v", unmarshalErr)
	}

	if decoded.EventType != events.SourceCreated {
		t.Errorf("expected event type %s, got %s", events.SourceCreated, decoded.EventType)
	}
	if decoded.SourceID != sourceID {
		t.Errorf("expected source ID %s, got %s", sourceID, decoded.SourceID)
	}
}

func TestEventType_Constants(t *testing.T) {
	t.Helper()

	tests := []struct {
		eventType events.EventType
		expected  string
	}{
		{events.SourceCreated, "SOURCE_CREATED"},
		{events.SourceUpdated, "SOURCE_UPDATED"},
		{events.SourceDeleted, "SOURCE_DELETED"},
		{events.SourceEnabled, "SOURCE_ENABLED"},
		{events.SourceDisabled, "SOURCE_DISABLED"},
	}

	for _, tt := range tests {
		if string(tt.eventType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, tt.eventType)
		}
	}
}

func TestPriority_Constants(t *testing.T) {
	t.Helper()

	if events.PriorityNormal != "normal" {
		t.Errorf("expected 'normal', got %s", events.PriorityNormal)
	}
}
