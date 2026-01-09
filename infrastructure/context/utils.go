// Package context provides shared context utilities for the North Cloud microservices.
package context

import (
	"context"
	"time"
)

const (
	// DefaultTimeout is the default timeout for context operations
	DefaultTimeout = 30 * time.Second

	// DefaultShutdownTimeout is the default timeout for graceful shutdown
	DefaultShutdownTimeout = 10 * time.Second

	// DefaultPingTimeout is the default timeout for ping/health check operations
	DefaultPingTimeout = 5 * time.Second
)

// WithDefaultTimeout creates a context with a default timeout from background context.
func WithDefaultTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultTimeout)
}

// WithTimeout creates a context with the specified timeout from background context.
func WithTimeout(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

// WithShutdownTimeout creates a context with default shutdown timeout.
func WithShutdownTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultShutdownTimeout)
}

// WithPingTimeout creates a context with default ping timeout.
func WithPingTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), DefaultPingTimeout)
}
