package events_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/events"
	infraevents "github.com/north-cloud/infrastructure/events"
)

// MockHandler implements events.EventHandler for testing.
type MockHandler struct {
	CreatedEvents  []infraevents.SourceEvent
	UpdatedEvents  []infraevents.SourceEvent
	DeletedEvents  []infraevents.SourceEvent
	EnabledEvents  []infraevents.SourceEvent
	DisabledEvents []infraevents.SourceEvent
}

func (m *MockHandler) HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error {
	m.CreatedEvents = append(m.CreatedEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error {
	m.UpdatedEvents = append(m.UpdatedEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error {
	m.DeletedEvents = append(m.DeletedEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error {
	m.EnabledEvents = append(m.EnabledEvents, event)
	return nil
}

func (m *MockHandler) HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error {
	m.DisabledEvents = append(m.DisabledEvents, event)
	return nil
}

func TestNewConsumer_RequiresClient(t *testing.T) {
	t.Helper()

	consumer := events.NewConsumer(nil, "test", &MockHandler{}, nil)
	if consumer != nil {
		t.Error("expected nil consumer when client is nil")
	}
}

func TestNewConsumer_GeneratesConsumerIDWhenEmpty(t *testing.T) {
	t.Helper()

	// We can't test with a real client without Redis, but we can test the ID generation
	id := events.GenerateConsumerID()
	if id == "" {
		t.Error("expected non-empty consumer ID")
	}

	// Consumer ID should be in format "crawler-{8-char-uuid}"
	const expectedMinLength = 16 // "crawler-" (8) + uuid prefix (8)
	if len(id) < expectedMinLength {
		t.Errorf("consumer ID too short: got %d, want at least %d", len(id), expectedMinLength)
	}
}

func TestConsumer_GeneratesConsumerID(t *testing.T) {
	t.Helper()

	// Verify that consumer ID generation works
	id := events.GenerateConsumerID()
	if id == "" {
		t.Error("expected non-empty consumer ID")
	}

	const minIDLength = 10
	if len(id) < minIDLength {
		t.Error("consumer ID too short")
	}
}

func TestGenerateConsumerID_UniqueIDs(t *testing.T) {
	t.Helper()

	// Generate multiple IDs and ensure they're unique
	const idCount = 100
	ids := make(map[string]bool, idCount)
	for range idCount {
		id := events.GenerateConsumerID()
		if ids[id] {
			t.Errorf("duplicate consumer ID generated: %s", id)
		}
		ids[id] = true
	}
}

func TestIsGroupExistsError(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "group exists error",
			err:      &busyGroupError{},
			expected: true,
		},
		{
			name:     "other error",
			err:      context.Canceled,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := events.IsGroupExistsError(tt.err)
			if result != tt.expected {
				t.Errorf("IsGroupExistsError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

// busyGroupError simulates the Redis BUSYGROUP error.
type busyGroupError struct{}

func (e *busyGroupError) Error() string {
	return "BUSYGROUP Consumer Group name already exists"
}
