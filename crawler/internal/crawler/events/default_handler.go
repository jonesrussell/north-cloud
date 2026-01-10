package events

import (
	"context"

	infralogger "github.com/north-cloud/infrastructure/logger"
)

// DefaultHandler provides a basic implementation of EventHandler that logs events.
type DefaultHandler struct {
	logger infralogger.Logger
}

// NewDefaultHandler creates a new DefaultHandler instance.
func NewDefaultHandler(log infralogger.Logger) EventHandler {
	return &DefaultHandler{
		logger: log,
	}
}

// HandleError logs error events.
func (h *DefaultHandler) HandleError(ctx context.Context, err error) error {
	h.logger.Error("Error occurred",
		infralogger.Error(err),
		infralogger.String("component", "crawler"),
	)
	return nil
}

// HandleStart logs start events.
func (h *DefaultHandler) HandleStart(ctx context.Context) error {
	h.logger.Info("Crawler started",
		infralogger.String("component", "crawler"),
	)
	return nil
}

// HandleStop logs stop events.
func (h *DefaultHandler) HandleStop(ctx context.Context) error {
	h.logger.Info("Crawler stopped",
		infralogger.String("component", "crawler"),
	)
	return nil
}
