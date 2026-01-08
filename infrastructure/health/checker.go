// Package health provides standardized health check functionality for microservices.
package health

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a service
type Status string

const (
	// StatusHealthy means the service is healthy
	StatusHealthy Status = "healthy"
	// StatusUnhealthy means the service is unhealthy
	StatusUnhealthy Status = "unhealthy"
	// StatusDegraded means the service is degraded but functional
	StatusDegraded Status = "degraded"
)

// Check represents a health check
type Check interface {
	// Name returns the name of the health check
	Name() string
	// Check performs the health check and returns an error if unhealthy
	Check(ctx context.Context) error
}

// CheckFunc is a function that implements the Check interface
type CheckFunc func(ctx context.Context) error

// Name returns the name of the check function
func (f CheckFunc) Name() string {
	return "check"
}

// Check executes the check function
func (f CheckFunc) Check(ctx context.Context) error {
	return f(ctx)
}

// Checker manages health checks for a service
type Checker struct {
	mu     sync.RWMutex
	checks map[string]Check
}

// NewChecker creates a new health checker
func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]Check),
	}
}

// Register registers a health check
func (c *Checker) Register(check Check) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[check.Name()] = check
}

// RegisterFunc registers a health check function
func (c *Checker) RegisterFunc(name string, fn func(ctx context.Context) error) {
	c.Register(CheckFunc(fn))
	// Update the name for the function check
	if cf, ok := c.checks["check"].(CheckFunc); ok {
		delete(c.checks, "check")
		c.checks[name] = cf
	}
}

// Check performs all registered health checks
func (c *Checker) Check(ctx context.Context) (Status, map[string]string) {
	c.mu.RLock()
	checks := make([]Check, 0, len(c.checks))
	for _, check := range c.checks {
		checks = append(checks, check)
	}
	c.mu.RUnlock()

	results := make(map[string]string)
	allHealthy := true
	anyDegraded := false

	for _, check := range checks {
		err := check.Check(ctx)
		if err != nil {
			results[check.Name()] = fmt.Sprintf("error: %v", err)
			allHealthy = false
		} else {
			results[check.Name()] = "ok"
		}
	}

	if !allHealthy {
		return StatusUnhealthy, results
	}
	if anyDegraded {
		return StatusDegraded, results
	}
	return StatusHealthy, results
}

// HTTPHandler returns an HTTP handler for health check endpoints
func (c *Checker) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		status, results := c.Check(ctx)

		response := map[string]any{
			"status":    status,
			"checks":    results,
			"timestamp": time.Now().Format(time.RFC3339),
		}

		w.Header().Set("Content-Type", "application/json")

		statusCode := http.StatusOK
		switch status {
		case StatusUnhealthy:
			statusCode = http.StatusServiceUnavailable
		case StatusDegraded:
			statusCode = http.StatusOK // Still 200, but status indicates degraded
		}

		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(response)
	}
}

// LivenessHandler returns a simple liveness check handler (always returns healthy)
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	}
}

// ReadinessHandler returns a readiness check handler using the checker
func ReadinessHandler(checker *Checker) http.HandlerFunc {
	return checker.HTTPHandler()
}
