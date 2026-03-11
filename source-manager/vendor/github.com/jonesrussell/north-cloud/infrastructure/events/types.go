// Package events provides shared event types for source lifecycle communication
// between source-manager and crawler via Redis Streams.
package events

import (
	"time"

	"github.com/google/uuid"
)

// StreamName is the Redis stream for source events.
const StreamName = "source-events"

// ConsumerGroup is the consumer group for crawler workers.
const ConsumerGroup = "crawler-workers"

// EventType represents the type of source event.
type EventType string

const (
	// SourceCreated indicates a new source was created.
	SourceCreated EventType = "SOURCE_CREATED"
	// SourceUpdated indicates an existing source was modified.
	SourceUpdated EventType = "SOURCE_UPDATED"
	// SourceDeleted indicates a source was deleted.
	SourceDeleted EventType = "SOURCE_DELETED"
	// SourceEnabled indicates a source was enabled.
	SourceEnabled EventType = "SOURCE_ENABLED"
	// SourceDisabled indicates a source was disabled.
	SourceDisabled EventType = "SOURCE_DISABLED"
)

// Priority levels for sources.
const (
	PriorityLow      = "low"
	PriorityNormal   = "normal"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

// SourceEvent is the envelope for all source-related events.
type SourceEvent struct {
	EventID   uuid.UUID `json:"event_id"`
	EventType EventType `json:"event_type"`
	SourceID  uuid.UUID `json:"source_id"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

// SourceCreatedPayload contains data for SOURCE_CREATED events.
type SourceCreatedPayload struct {
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	RateLimit int            `json:"rate_limit"`
	MaxDepth  int            `json:"max_depth"`
	Enabled   bool           `json:"enabled"`
	Selectors map[string]any `json:"selectors,omitempty"`
	Priority  string         `json:"priority"`
}

// SourceUpdatedPayload contains data for SOURCE_UPDATED events.
type SourceUpdatedPayload struct {
	ChangedFields []string       `json:"changed_fields"`
	Previous      map[string]any `json:"previous"`
	Current       map[string]any `json:"current"`
}

// SourceDeletedPayload contains data for SOURCE_DELETED events.
type SourceDeletedPayload struct {
	Name           string `json:"name"`
	DeletionReason string `json:"deletion_reason"`
}

// SourceTogglePayload contains data for SOURCE_ENABLED/DISABLED events.
type SourceTogglePayload struct {
	Reason    string `json:"reason"`
	ToggledBy string `json:"toggled_by"` // "user" or "system"
}
