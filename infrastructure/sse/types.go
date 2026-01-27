// Package sse provides Server-Sent Events infrastructure for real-time updates.
package sse

import (
	"context"
	"time"
)

// Event represents a Server-Sent Event.
// Format: event: <Type>\ndata: <JSON payload>\n\n
type Event struct {
	// Type is the event type (e.g., "job:status", "health:status")
	Type string `json:"type"`
	// Data is the JSON payload (must be JSON-serializable)
	Data any `json:"data"`
	// ID is an optional event ID for client-side tracking
	ID string `json:"id,omitempty"`
	// Retry tells the client how long to wait before reconnecting (milliseconds)
	Retry int `json:"retry,omitempty"`
}

// Publisher sends events to the broker.
type Publisher interface {
	// Publish sends an event to all connected clients.
	// Returns error if the broker is not running or the publish buffer is full.
	Publish(ctx context.Context, event Event) error
}

// Subscriber receives events from the broker.
type Subscriber interface {
	// Subscribe returns a channel that receives events.
	// The channel is closed when the subscription ends (client disconnect or broker shutdown).
	Subscribe(ctx context.Context, opts ...ClientOption) (<-chan Event, func())
}

// Broker manages SSE connections and event distribution.
type Broker interface {
	Publisher
	Subscriber
	// Start begins processing events (non-blocking).
	Start(ctx context.Context) error
	// Stop gracefully shuts down the broker.
	Stop() error
	// ClientCount returns the number of connected clients.
	ClientCount() int
}

// EventFilter determines if an event should be sent to a client.
// Return true to send the event, false to skip.
type EventFilter func(event Event) bool

// ClientOptions configures a single SSE client connection.
type ClientOptions struct {
	// Filter is an optional event filter for this client
	Filter EventFilter
	// BufferSize is the event buffer size (default: 100)
	BufferSize int
}

// Event types for crawler events (matches frontend expectations).
const (
	EventTypeJobStatus    = "job:status"
	EventTypeJobProgress  = "job:progress"
	EventTypeJobCompleted = "job:completed"
)

// Event types for health events.
const (
	EventTypeHealthStatus = "health:status"
)

// Event types for metrics events.
const (
	EventTypeMetricsUpdate = "metrics:update"
	EventTypePipelineStage = "pipeline:stage"
)

// Internal event types.
const (
	eventTypeConnected = "connected"
	eventTypeHeartbeat = "heartbeat"
)

// JobStatusData is the payload for job:status events.
type JobStatusData struct {
	JobID     string            `json:"job_id"`
	Status    string            `json:"status"`
	Timestamp string            `json:"timestamp"`
	Details   *JobStatusDetails `json:"details,omitempty"`
}

// JobStatusDetails contains optional details for job status events.
type JobStatusDetails struct {
	ErrorMessage *string `json:"error_message,omitempty"`
	NextRunAt    *string `json:"next_run_at,omitempty"`
}

// JobProgressData is the payload for job:progress events.
type JobProgressData struct {
	JobID           string `json:"job_id"`
	ExecutionID     string `json:"execution_id"`
	ArticlesFound   int    `json:"articles_found"`
	ArticlesIndexed int    `json:"articles_indexed"`
	Timestamp       string `json:"timestamp"`
}

// JobCompletedData is the payload for job:completed events.
type JobCompletedData struct {
	JobID           string  `json:"job_id"`
	ExecutionID     string  `json:"execution_id"`
	Status          string  `json:"status"`
	DurationMs      int64   `json:"duration_ms"`
	ArticlesIndexed int     `json:"articles_indexed"`
	ErrorMessage    *string `json:"error_message,omitempty"`
	Timestamp       string  `json:"timestamp"`
}

// HealthStatusData is the payload for health:status events.
type HealthStatusData struct {
	Service   string  `json:"service"`
	Status    string  `json:"status"` // "healthy", "degraded", "unhealthy"
	Latency   *int64  `json:"latency,omitempty"`
	Details   *string `json:"details,omitempty"`
	Timestamp string  `json:"timestamp"`
}

// MetricsUpdateData is the payload for metrics:update events.
type MetricsUpdateData struct {
	Metric    string  `json:"metric"`
	Value     float64 `json:"value"`
	Timestamp string  `json:"timestamp"`
}

// PipelineStageData is the payload for pipeline:stage events.
type PipelineStageData struct {
	Stage     string `json:"stage"` // "crawled", "classified", "published"
	Count     int    `json:"count"`
	Timestamp string `json:"timestamp"`
}

// NewJobStatusEvent creates a job:status event.
func NewJobStatusEvent(jobID, status string, details *JobStatusDetails) Event {
	return Event{
		Type: EventTypeJobStatus,
		Data: JobStatusData{
			JobID:     jobID,
			Status:    status,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Details:   details,
		},
	}
}

// NewJobProgressEvent creates a job:progress event.
func NewJobProgressEvent(jobID, executionID string, articlesFound, articlesIndexed int) Event {
	return Event{
		Type: EventTypeJobProgress,
		Data: JobProgressData{
			JobID:           jobID,
			ExecutionID:     executionID,
			ArticlesFound:   articlesFound,
			ArticlesIndexed: articlesIndexed,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// NewJobCompletedEvent creates a job:completed event.
func NewJobCompletedEvent(jobID, executionID, status string, durationMs int64, articlesIndexed int, errorMessage *string) Event {
	return Event{
		Type: EventTypeJobCompleted,
		Data: JobCompletedData{
			JobID:           jobID,
			ExecutionID:     executionID,
			Status:          status,
			DurationMs:      durationMs,
			ArticlesIndexed: articlesIndexed,
			ErrorMessage:    errorMessage,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// NewHealthStatusEvent creates a health:status event.
func NewHealthStatusEvent(service, status string, latency *int64, details *string) Event {
	return Event{
		Type: EventTypeHealthStatus,
		Data: HealthStatusData{
			Service:   service,
			Status:    status,
			Latency:   latency,
			Details:   details,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// NewMetricsUpdateEvent creates a metrics:update event.
func NewMetricsUpdateEvent(metric string, value float64) Event {
	return Event{
		Type: EventTypeMetricsUpdate,
		Data: MetricsUpdateData{
			Metric:    metric,
			Value:     value,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}

// NewPipelineStageEvent creates a pipeline:stage event.
func NewPipelineStageEvent(stage string, count int) Event {
	return Event{
		Type: EventTypePipelineStage,
		Data: PipelineStageData{
			Stage:     stage,
			Count:     count,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	}
}
