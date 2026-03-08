package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Status tracks the outcome of the most recent ingestion cycle.
// It is safe for concurrent access.
type Status struct {
	LastRun     time.Time `json:"last_run"`
	LastSuccess time.Time `json:"last_success"`
	Fetched     int       `json:"fetched"`
	Indexed     int       `json:"indexed"`
	Failed      int       `json:"failed"`
	DurationMs  int64     `json:"duration_ms"`
	mu          sync.RWMutex
}

// Update records the results of a completed ingestion cycle.
// fatalErr indicates whether the cycle encountered a fatal error (fetch or
// ES failure). Parse-level failures are expected in large CSV feeds and do
// not prevent LastSuccess from being updated.
func (s *Status) Update(fetched, indexed, failed int, duration time.Duration, fatalErr bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.LastRun = time.Now()
	s.Fetched = fetched
	s.Indexed = indexed
	s.Failed = failed
	s.DurationMs = duration.Milliseconds()

	if !fatalErr {
		s.LastSuccess = s.LastRun
	}
}

// Snapshot returns a point-in-time copy of the status under a read lock.
func (s *Status) Snapshot() Status {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Status{
		LastRun:     s.LastRun,
		LastSuccess: s.LastSuccess,
		Fetched:     s.Fetched,
		Indexed:     s.Indexed,
		Failed:      s.Failed,
		DurationMs:  s.DurationMs,
	}
}

// NewServer builds an HTTP server with health endpoints and a /api/v1/status route.
func NewServer(serviceName string, port int, version string, debug bool, log logger.Logger, status *Status) *infragin.Server {
	return infragin.NewServerBuilder(serviceName, port).
		WithLogger(log).
		WithDebug(debug).
		WithVersion(version).
		WithRoutes(func(router *gin.Engine) {
			router.GET("/api/v1/status", func(c *gin.Context) {
				c.JSON(http.StatusOK, status.Snapshot())
			})
		}).
		Build()
}
