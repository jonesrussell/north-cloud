// Package metrics provides metrics collection and reporting functionality.
package metrics

import (
	"time"
)

// Metrics holds the processing metrics.
type Metrics struct {
	// ProcessedCount is the number of items processed.
	ProcessedCount int64
	// ErrorCount is the number of processing errors.
	ErrorCount int64
	// LastProcessedTime is the time of the last successful processing.
	LastProcessedTime time.Time
	// ProcessingDuration is the total time spent processing.
	ProcessingDuration time.Duration
	// StartTime is when the metrics collection began.
	StartTime time.Time
	// CurrentSource is the current source being processed.
	CurrentSource string
	// SuccessfulRequests is the number of successful HTTP requests.
	SuccessfulRequests int64
	// FailedRequests is the number of failed HTTP requests.
	FailedRequests int64
	// RateLimitedRequests is the number of rate-limited requests.
	RateLimitedRequests int64
}

// NewMetrics creates a new Metrics instance with default values.
func NewMetrics() *Metrics {
	return &Metrics{
		ProcessedCount:      0,
		ErrorCount:          0,
		LastProcessedTime:   time.Time{},
		ProcessingDuration:  0,
		StartTime:           time.Now(),
		CurrentSource:       "",
		SuccessfulRequests:  0,
		FailedRequests:      0,
		RateLimitedRequests: 0,
	}
}
