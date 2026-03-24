package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/events"
	infraevents "github.com/jonesrussell/north-cloud/infrastructure/events"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestConvertPayload_NilPayload(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())
	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   nil,
	}

	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Errorf("expected nil error for nil payload, got %v", err)
	}
}

func TestConvertPayload_SourceCreated(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())

	// Simulate what JSON unmarshaling produces: map[string]any
	payload := map[string]any{
		"name":       "Test Source",
		"url":        "https://example.com",
		"rate_limit": float64(10),
		"max_depth":  float64(3),
		"enabled":    true,
		"priority":   "normal",
	}

	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   payload,
	}

	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	typed, ok := event.Payload.(infraevents.SourceCreatedPayload)
	if !ok {
		t.Fatalf("expected SourceCreatedPayload, got %T", event.Payload)
	}
	if typed.Name != "Test Source" {
		t.Errorf("expected Name 'Test Source', got %q", typed.Name)
	}
	if typed.URL != "https://example.com" {
		t.Errorf("expected URL 'https://example.com', got %q", typed.URL)
	}
}

func TestConvertPayload_SourceUpdated(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())

	payload := map[string]any{
		"changed_fields": []any{"name", "url"},
		"previous":       map[string]any{"name": "old"},
		"current":        map[string]any{"name": "new"},
	}

	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceUpdated,
		SourceID:  uuid.New(),
		Payload:   payload,
	}

	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	typed, ok := event.Payload.(infraevents.SourceUpdatedPayload)
	if !ok {
		t.Fatalf("expected SourceUpdatedPayload, got %T", event.Payload)
	}
	if len(typed.ChangedFields) != 2 {
		t.Errorf("expected 2 changed fields, got %d", len(typed.ChangedFields))
	}
}

func TestConvertPayload_SourceDeleted(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())

	payload := map[string]any{
		"name":            "Deleted Source",
		"deletion_reason": "test cleanup",
	}

	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceDeleted,
		SourceID:  uuid.New(),
		Payload:   payload,
	}

	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	typed, ok := event.Payload.(infraevents.SourceDeletedPayload)
	if !ok {
		t.Fatalf("expected SourceDeletedPayload, got %T", event.Payload)
	}
	if typed.Name != "Deleted Source" {
		t.Errorf("expected Name 'Deleted Source', got %q", typed.Name)
	}
	if typed.DeletionReason != "test cleanup" {
		t.Errorf("expected DeletionReason 'test cleanup', got %q", typed.DeletionReason)
	}
}

func TestConvertPayload_SourceEnabled(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())

	payload := map[string]any{
		"reason":     "test",
		"toggled_by": "user",
	}

	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceEnabled,
		SourceID:  uuid.New(),
		Payload:   payload,
	}

	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	typed, ok := event.Payload.(infraevents.SourceTogglePayload)
	if !ok {
		t.Fatalf("expected SourceTogglePayload, got %T", event.Payload)
	}
	if typed.Reason != "test" {
		t.Errorf("expected Reason 'test', got %q", typed.Reason)
	}
}

func TestConvertPayload_SourceDisabled(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())

	payload := map[string]any{
		"reason":     "maintenance",
		"toggled_by": "system",
	}

	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceDisabled,
		SourceID:  uuid.New(),
		Payload:   payload,
	}

	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	typed, ok := event.Payload.(infraevents.SourceTogglePayload)
	if !ok {
		t.Fatalf("expected SourceTogglePayload, got %T", event.Payload)
	}
	if typed.ToggledBy != "system" {
		t.Errorf("expected ToggledBy 'system', got %q", typed.ToggledBy)
	}
}

func TestConvertPayload_UnknownEventType(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())

	payload := map[string]any{"foo": "bar"}
	event := &infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.EventType("UNKNOWN_EVENT"),
		SourceID:  uuid.New(),
		Payload:   payload,
	}

	// Unknown event types should pass through without conversion error
	err := consumer.ConvertPayload(event)
	if err != nil {
		t.Errorf("expected no error for unknown event type, got %v", err)
	}
}

func TestDispatchEvent_SourceCreated(t *testing.T) {
	t.Parallel()

	handler := &MockHandler{}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	sourceID := uuid.New()
	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  sourceID,
		Payload:   infraevents.SourceCreatedPayload{Name: "Test", URL: "https://example.com"},
	}

	err := consumer.DispatchEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(handler.CreatedEvents) != 1 {
		t.Fatalf("expected 1 created event, got %d", len(handler.CreatedEvents))
	}
	if handler.CreatedEvents[0].SourceID != sourceID {
		t.Errorf("expected source ID %s, got %s", sourceID, handler.CreatedEvents[0].SourceID)
	}
}

func TestDispatchEvent_SourceUpdated(t *testing.T) {
	t.Parallel()

	handler := &MockHandler{}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceUpdated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceUpdatedPayload{ChangedFields: []string{"name"}},
	}

	err := consumer.DispatchEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(handler.UpdatedEvents) != 1 {
		t.Fatalf("expected 1 updated event, got %d", len(handler.UpdatedEvents))
	}
}

func TestDispatchEvent_SourceDeleted(t *testing.T) {
	t.Parallel()

	handler := &MockHandler{}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceDeleted,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceDeletedPayload{Name: "Deleted", DeletionReason: "test"},
	}

	err := consumer.DispatchEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(handler.DeletedEvents) != 1 {
		t.Fatalf("expected 1 deleted event, got %d", len(handler.DeletedEvents))
	}
}

func TestDispatchEvent_SourceEnabled(t *testing.T) {
	t.Parallel()

	handler := &MockHandler{}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceEnabled,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceTogglePayload{Reason: "test", ToggledBy: "user"},
	}

	err := consumer.DispatchEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(handler.EnabledEvents) != 1 {
		t.Fatalf("expected 1 enabled event, got %d", len(handler.EnabledEvents))
	}
}

func TestDispatchEvent_SourceDisabled(t *testing.T) {
	t.Parallel()

	handler := &MockHandler{}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceDisabled,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceTogglePayload{Reason: "maint", ToggledBy: "system"},
	}

	err := consumer.DispatchEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(handler.DisabledEvents) != 1 {
		t.Fatalf("expected 1 disabled event, got %d", len(handler.DisabledEvents))
	}
}

func TestDispatchEvent_UnknownEventType_NoError(t *testing.T) {
	t.Parallel()

	handler := &MockHandler{}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.EventType("UNKNOWN_EVENT"),
		SourceID:  uuid.New(),
	}

	// Unknown events are logged but do not return an error
	err := consumer.DispatchEvent(context.Background(), event)
	if err != nil {
		t.Errorf("expected nil error for unknown event, got %v", err)
	}

	total := len(handler.CreatedEvents) + len(handler.UpdatedEvents) +
		len(handler.DeletedEvents) + len(handler.EnabledEvents) + len(handler.DisabledEvents)
	if total != 0 {
		t.Errorf("expected no handler calls for unknown event, got %d", total)
	}
}

func TestDispatchEvent_HandlerError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("handler failed")
	handler := &ErrorHandler{err: expectedErr}
	consumer := events.NewTestableConsumerWithHandler(handler, infralogger.NewNop())

	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
		Payload:   infraevents.SourceCreatedPayload{Name: "Test"},
	}

	err := consumer.DispatchEvent(context.Background(), event)
	if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestConsumer_Stop_DoesNotPanic(t *testing.T) {
	t.Parallel()

	consumer := events.NewTestableConsumer(infralogger.NewNop())
	// Should not panic on stop
	consumer.Stop()
}

func TestNewNoOpHandler_WithLogger(t *testing.T) {
	t.Parallel()

	log := infralogger.NewNop()
	handler := events.NewNoOpHandler(log)

	ctx := context.Background()
	event := infraevents.SourceEvent{
		EventID:   uuid.New(),
		EventType: infraevents.SourceCreated,
		SourceID:  uuid.New(),
	}

	// All methods should return nil with a logger set
	if err := handler.HandleSourceCreated(ctx, event); err != nil {
		t.Errorf("HandleSourceCreated: expected nil, got %v", err)
	}
	if err := handler.HandleSourceUpdated(ctx, event); err != nil {
		t.Errorf("HandleSourceUpdated: expected nil, got %v", err)
	}
	if err := handler.HandleSourceDeleted(ctx, event); err != nil {
		t.Errorf("HandleSourceDeleted: expected nil, got %v", err)
	}
	if err := handler.HandleSourceEnabled(ctx, event); err != nil {
		t.Errorf("HandleSourceEnabled: expected nil, got %v", err)
	}
	if err := handler.HandleSourceDisabled(ctx, event); err != nil {
		t.Errorf("HandleSourceDisabled: expected nil, got %v", err)
	}
}

// ErrorHandler is a mock handler that returns errors.
type ErrorHandler struct {
	err error
}

func (h *ErrorHandler) HandleSourceCreated(_ context.Context, _ infraevents.SourceEvent) error {
	return h.err
}

func (h *ErrorHandler) HandleSourceUpdated(_ context.Context, _ infraevents.SourceEvent) error {
	return h.err
}

func (h *ErrorHandler) HandleSourceDeleted(_ context.Context, _ infraevents.SourceEvent) error {
	return h.err
}

func (h *ErrorHandler) HandleSourceEnabled(_ context.Context, _ infraevents.SourceEvent) error {
	return h.err
}

func (h *ErrorHandler) HandleSourceDisabled(_ context.Context, _ infraevents.SourceEvent) error {
	return h.err
}
