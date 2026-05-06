package domain

import "time"

// EventType describes the kind of state transition that produced the event.
type EventType string

const (
	EventCreated   EventType = "created"
	EventUpdated   EventType = "updated"
	EventRescinded EventType = "rescinded"
)

// LifecycleEvent is the Redis pub/sub payload published on every alert state
// transition. It matches contracts/lifecycle-event.schema.json.
// Consumers MAY use the embedded Payload directly; for strong consistency they
// SHOULD re-read the canonical ES record.
type LifecycleEvent struct {
	EventType EventType `json:"event_type"`
	EventAt   time.Time `json:"event_at"`
	AlertID   string    `json:"alert_id"`
	Category  Category  `json:"category"`
	Severity  Severity  `json:"severity"`
	Scope     []string  `json:"scope"`
	Payload   Alert     `json:"payload"`
}

// NewLifecycleEvent constructs a LifecycleEvent from an alert, stamping EventAt
// to the current UTC time and copying convenience fields so consumers can route
// without re-fetching the full alert from ES.
func NewLifecycleEvent(eventType EventType, alert Alert) LifecycleEvent {
	return LifecycleEvent{
		EventType: eventType,
		EventAt:   time.Now().UTC(),
		AlertID:   alert.ID,
		Category:  alert.Category,
		Severity:  alert.Severity,
		Scope:     alert.Scope,
		Payload:   alert,
	}
}
