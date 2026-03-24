package events

import (
	"context"

	infraevents "github.com/jonesrussell/north-cloud/infrastructure/events"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// GenerateConsumerID exposes generateConsumerID for testing.
var GenerateConsumerID = generateConsumerID

// IsGroupExistsError exposes isGroupExistsError for testing.
var IsGroupExistsError = isGroupExistsError

// TestableConsumer wraps Consumer for testing without a real Redis connection.
type TestableConsumer struct {
	*Consumer
}

// NewTestableConsumer creates a Consumer with no Redis client, suitable for unit tests.
func NewTestableConsumer(log infralogger.Logger) *TestableConsumer {
	return &TestableConsumer{
		Consumer: &Consumer{
			consumerID: "test-consumer",
			handler:    NewNoOpHandler(log),
			log:        log,
			shutdownCh: make(chan struct{}),
		},
	}
}

// NewTestableConsumerWithHandler creates a Consumer with a custom handler for testing.
func NewTestableConsumerWithHandler(handler EventHandler, log infralogger.Logger) *TestableConsumer {
	return &TestableConsumer{
		Consumer: &Consumer{
			consumerID: "test-consumer",
			handler:    handler,
			log:        log,
			shutdownCh: make(chan struct{}),
		},
	}
}

// ConvertPayload exposes the Consumer's convertPayload method for testing.
func (tc *TestableConsumer) ConvertPayload(event *infraevents.SourceEvent) error {
	return tc.convertPayload(event)
}

// DispatchEvent dispatches an event to the handler without Redis ACK.
// This exercises the same switch/case routing as processMessage.
func (tc *TestableConsumer) DispatchEvent(ctx context.Context, event infraevents.SourceEvent) error {
	var err error
	switch event.EventType {
	case infraevents.SourceCreated:
		err = tc.handler.HandleSourceCreated(ctx, event)
	case infraevents.SourceUpdated:
		err = tc.handler.HandleSourceUpdated(ctx, event)
	case infraevents.SourceDeleted:
		err = tc.handler.HandleSourceDeleted(ctx, event)
	case infraevents.SourceEnabled:
		err = tc.handler.HandleSourceEnabled(ctx, event)
	case infraevents.SourceDisabled:
		err = tc.handler.HandleSourceDisabled(ctx, event)
	default:
		if tc.log != nil {
			tc.log.Warn("Unknown event type",
				infralogger.String("event_type", string(event.EventType)),
			)
		}
	}
	return err
}

// Stop exposes the Consumer's Stop method.
func (tc *TestableConsumer) Stop() {
	tc.Consumer.Stop()
}
