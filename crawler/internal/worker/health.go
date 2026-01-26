package worker

import (
	"context"
	"sync"
	"time"

	infralogger "github.com/north-cloud/infrastructure/logger"
)

// HealthStatus represents the health status of the pool.
type HealthStatus string

const (
	// HealthStatusHealthy means the pool is operating normally.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusDegraded means the pool has some unhealthy workers.
	HealthStatusDegraded HealthStatus = "degraded"

	// HealthStatusUnhealthy means the pool is not functioning properly.
	HealthStatusUnhealthy HealthStatus = "unhealthy"

	// degradedThreshold is the minimum healthy ratio to be considered degraded (vs unhealthy).
	degradedThreshold = 0.5
)

// HealthCheck represents a health check result.
type HealthCheck struct {
	Status           HealthStatus
	Timestamp        time.Time
	PoolState        PoolState
	TotalWorkers     int
	HealthyWorkers   int
	UnhealthyWorkers int
	BusyWorkers      int
	IdleWorkers      int
	Details          []WorkerHealthDetail
}

// WorkerHealthDetail contains health details for a single worker.
type WorkerHealthDetail struct {
	WorkerID     int
	State        WorkerState
	IsHealthy    bool
	CurrentJobID string
	JobDuration  time.Duration
	LastError    string
}

// HealthMonitor monitors the health of the worker pool.
type HealthMonitor struct {
	pool      *Pool
	logger    infralogger.Logger
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
	mu        sync.RWMutex
	lastCheck *HealthCheck
}

// NewHealthMonitor creates a new health monitor.
func NewHealthMonitor(pool *Pool, interval time.Duration, logger infralogger.Logger) *HealthMonitor {
	if interval <= 0 {
		interval = DefaultHealthCheckInterval
	}

	return &HealthMonitor{
		pool:     pool,
		logger:   logger,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the health monitor.
func (m *HealthMonitor) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.run(ctx)
}

// Stop stops the health monitor.
func (m *HealthMonitor) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// Check performs a health check and returns the result.
func (m *HealthMonitor) Check() HealthCheck {
	stats := m.pool.Stats()

	healthyCount := 0
	unhealthyCount := 0
	details := make([]WorkerHealthDetail, len(stats.Workers))

	for i, ws := range stats.Workers {
		isHealthy := ws.IsHealthy()
		if isHealthy {
			healthyCount++
		} else {
			unhealthyCount++
		}

		var lastErr string
		if ws.LastError != nil {
			lastErr = ws.LastError.Error()
		}

		var jobDuration time.Duration
		if ws.State == WorkerStateBusy && !ws.JobStartedAt.IsZero() {
			jobDuration = time.Since(ws.JobStartedAt)
		}

		details[i] = WorkerHealthDetail{
			WorkerID:     ws.ID,
			State:        ws.State,
			IsHealthy:    isHealthy,
			CurrentJobID: ws.CurrentJobID,
			JobDuration:  jobDuration,
			LastError:    lastErr,
		}
	}

	status := m.determineStatus(stats.PoolSize, healthyCount, unhealthyCount)

	check := HealthCheck{
		Status:           status,
		Timestamp:        time.Now(),
		PoolState:        stats.State,
		TotalWorkers:     stats.PoolSize,
		HealthyWorkers:   healthyCount,
		UnhealthyWorkers: unhealthyCount,
		BusyWorkers:      stats.BusyWorkers,
		IdleWorkers:      stats.IdleWorkers,
		Details:          details,
	}

	m.mu.Lock()
	m.lastCheck = &check
	m.mu.Unlock()

	return check
}

// LastCheck returns the most recent health check result.
func (m *HealthMonitor) LastCheck() *HealthCheck {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastCheck
}

// determineStatus determines the overall health status based on worker health.
func (m *HealthMonitor) determineStatus(total, healthy, unhealthy int) HealthStatus {
	if total == 0 {
		return HealthStatusUnhealthy
	}

	healthRatio := float64(healthy) / float64(total)

	// All workers healthy
	if unhealthy == 0 {
		return HealthStatusHealthy
	}

	// More than threshold healthy
	if healthRatio >= degradedThreshold {
		return HealthStatusDegraded
	}

	// Less than 50% healthy
	return HealthStatusUnhealthy
}

// run is the main health monitoring loop.
func (m *HealthMonitor) run(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	// Initial check
	m.performCheck()

	for {
		select {
		case <-ticker.C:
			m.performCheck()
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		}
	}
}

// performCheck performs a health check and logs the result.
func (m *HealthMonitor) performCheck() {
	check := m.Check()

	switch check.Status {
	case HealthStatusHealthy:
		m.logger.Debug("pool health check: healthy",
			infralogger.Int("total_workers", check.TotalWorkers),
			infralogger.Int("busy_workers", check.BusyWorkers),
		)
	case HealthStatusDegraded:
		m.logger.Warn("pool health check: degraded",
			infralogger.Int("healthy_workers", check.HealthyWorkers),
			infralogger.Int("unhealthy_workers", check.UnhealthyWorkers),
		)
	case HealthStatusUnhealthy:
		m.logger.Error("pool health check: unhealthy",
			infralogger.Int("healthy_workers", check.HealthyWorkers),
			infralogger.Int("unhealthy_workers", check.UnhealthyWorkers),
		)
	}
}

// IsHealthy returns true if the pool is healthy or degraded.
func (m *HealthMonitor) IsHealthy() bool {
	check := m.LastCheck()
	if check == nil {
		return false
	}
	return check.Status == HealthStatusHealthy || check.Status == HealthStatusDegraded
}

// String returns a string representation of the health status.
func (s HealthStatus) String() string {
	return string(s)
}
