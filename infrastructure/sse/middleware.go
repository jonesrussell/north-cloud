package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SSE header constants.
const (
	headerContentType              = "Content-Type"
	headerCacheControl             = "Cache-Control"
	headerConnection               = "Connection"
	headerXAccelBuffering          = "X-Accel-Buffering"
	headerAccessControlAllowOrigin = "Access-Control-Allow-Origin"

	sseContentType = "text/event-stream"
)

// Handler creates a Gin handler for SSE endpoints.
// The handler sets appropriate SSE headers, subscribes to the broker,
// and streams events to the client until disconnection.
func Handler(broker Broker, logger infralogger.Logger, opts ...ClientOption) gin.HandlerFunc {
	return func(c *gin.Context) {
		setSSEHeaders(c.Writer)
		c.Writer.Flush()

		eventChan, cleanup := broker.Subscribe(c.Request.Context(), opts...)
		defer cleanup()

		if !checkSubscriptionValid(eventChan, c, logger) {
			return
		}

		if err := sendConnectionEvent(c.Writer); err != nil {
			logger.Error("Failed to write connection event", infralogger.Error(err))
			return
		}

		logger.Debug("SSE client connected",
			infralogger.String("remote_addr", c.ClientIP()),
		)

		streamEvents(c, eventChan, logger)
	}
}

// setSSEHeaders sets the standard SSE headers on a Gin response writer.
func setSSEHeaders(w gin.ResponseWriter) {
	w.Header().Set(headerContentType, sseContentType)
	w.Header().Set(headerCacheControl, "no-cache")
	w.Header().Set(headerConnection, "keep-alive")
	w.Header().Set(headerXAccelBuffering, "no")
	w.Header().Set(headerAccessControlAllowOrigin, "*")
}

// checkSubscriptionValid checks if the subscription was successful.
func checkSubscriptionValid(eventChan <-chan Event, c *gin.Context, logger infralogger.Logger) bool {
	select {
	case _, ok := <-eventChan:
		if !ok {
			logger.Warn("SSE subscription rejected (max clients reached)")
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "too many connections"})
			return false
		}
	default:
		// Channel is open, proceed
	}
	return true
}

// sendConnectionEvent sends the initial connection event.
func sendConnectionEvent(w gin.ResponseWriter) error {
	connectedEvent := Event{
		Type: eventTypeConnected,
		Data: map[string]any{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"message":   "SSE connection established",
		},
	}
	return writeEvent(w, connectedEvent)
}

// streamEvents handles the main event streaming loop.
func streamEvents(c *gin.Context, eventChan <-chan Event, logger infralogger.Logger) {
	ticker := time.NewTicker(DefaultHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-eventChan:
			if !handleEventReceived(c.Writer, event, ok, logger) {
				return
			}
		case <-ticker.C:
			if err := writeHeartbeat(c.Writer); err != nil {
				logger.Debug("SSE heartbeat failed (client disconnected)")
				return
			}
		case <-c.Request.Context().Done():
			logger.Debug("SSE client request context cancelled")
			return
		}
	}
}

// handleEventReceived processes a received event and returns false if streaming should stop.
func handleEventReceived(w gin.ResponseWriter, event Event, ok bool, logger infralogger.Logger) bool {
	if !ok {
		logger.Debug("SSE event channel closed")
		return false
	}

	if err := writeEvent(w, event); err != nil {
		logger.Debug("SSE write failed (client likely disconnected)",
			infralogger.Error(err),
			infralogger.String("event_type", event.Type),
		)
		return false
	}

	return true
}

// flusher interface for response writers that support flushing.
type flusher interface {
	Flush()
}

// writeEventToWriter writes an SSE event to any io.Writer.
// This is the core implementation used by both writeEvent and WriteEventDirect.
func writeEventToWriter(w interface{ Write([]byte) (int, error) }, event Event) error {
	if event.Type != "" {
		if _, writeErr := fmt.Fprintf(w, "event: %s\n", event.Type); writeErr != nil {
			return fmt.Errorf("write event type: %w", writeErr)
		}
	}

	if event.ID != "" {
		if _, writeErr := fmt.Fprintf(w, "id: %s\n", event.ID); writeErr != nil {
			return fmt.Errorf("write event id: %w", writeErr)
		}
	}

	if event.Retry > 0 {
		if _, writeErr := fmt.Fprintf(w, "retry: %d\n", event.Retry); writeErr != nil {
			return fmt.Errorf("write retry: %w", writeErr)
		}
	}

	dataJSON, marshalErr := json.Marshal(event.Data)
	if marshalErr != nil {
		return fmt.Errorf("marshal event data: %w", marshalErr)
	}

	if _, writeErr := fmt.Fprintf(w, "data: %s\n\n", dataJSON); writeErr != nil {
		return fmt.Errorf("write event data: %w", writeErr)
	}

	return nil
}

// writeEvent writes an SSE event to the response writer.
func writeEvent(w gin.ResponseWriter, event Event) error {
	if err := writeEventToWriter(w, event); err != nil {
		return err
	}
	w.Flush()
	return nil
}

// writeHeartbeat writes an SSE comment to keep the connection alive.
func writeHeartbeat(w gin.ResponseWriter) error {
	if _, writeErr := fmt.Fprintf(w, ": heartbeat %s\n\n", time.Now().UTC().Format(time.RFC3339)); writeErr != nil {
		return fmt.Errorf("write heartbeat: %w", writeErr)
	}
	w.Flush()
	return nil
}

// WriteEventDirect writes an SSE event directly to an http.ResponseWriter.
// This is useful for custom SSE handlers that don't use the broker.
func WriteEventDirect(w http.ResponseWriter, event Event) error {
	if err := writeEventToWriter(w, event); err != nil {
		return err
	}

	if f, ok := w.(flusher); ok {
		f.Flush()
	}

	return nil
}

// SetSSEHeaders sets the standard SSE headers on a response writer.
// This is useful for custom SSE handlers.
func SetSSEHeaders(w http.ResponseWriter) {
	w.Header().Set(headerContentType, sseContentType)
	w.Header().Set(headerCacheControl, "no-cache")
	w.Header().Set(headerConnection, "keep-alive")
	w.Header().Set(headerXAccelBuffering, "no")
	w.Header().Set(headerAccessControlAllowOrigin, "*")
}
