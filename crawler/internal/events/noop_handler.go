// Package events provides event handling for source lifecycle events.
package events

import (
	"context"

	infraevents "github.com/north-cloud/infrastructure/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// NoOpHandler logs events but takes no action.
// Used during Phase 1 to verify event flow without affecting job management.
type NoOpHandler struct {
	log infralogger.Logger
}

// NewNoOpHandler creates a new no-op handler.
func NewNoOpHandler(log infralogger.Logger) *NoOpHandler {
	return &NoOpHandler{log: log}
}

// HandleSourceCreated logs the event and returns nil.
func (h *NoOpHandler) HandleSourceCreated(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_CREATED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

// HandleSourceUpdated logs the event and returns nil.
func (h *NoOpHandler) HandleSourceUpdated(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_UPDATED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

// HandleSourceDeleted logs the event and returns nil.
func (h *NoOpHandler) HandleSourceDeleted(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_DELETED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

// HandleSourceEnabled logs the event and returns nil.
func (h *NoOpHandler) HandleSourceEnabled(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_ENABLED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}

// HandleSourceDisabled logs the event and returns nil.
func (h *NoOpHandler) HandleSourceDisabled(ctx context.Context, event infraevents.SourceEvent) error {
	if h.log != nil {
		h.log.Info("[NOOP] SOURCE_DISABLED received",
			infralogger.String("source_id", event.SourceID.String()),
		)
	}
	return nil
}
