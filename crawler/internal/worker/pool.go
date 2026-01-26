package worker

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/queue"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// PoolState represents the current state of the pool.
type PoolState int32

const (
	// PoolStateStopped means the pool is not running.
	PoolStateStopped PoolState = iota

	// PoolStateRunning means the pool is actively processing jobs.
	PoolStateRunning

	// PoolStateDraining means the pool is shutting down gracefully.
	PoolStateDraining

	// poolPercentageMultiplier converts ratio to percentage.
	poolPercentageMultiplier = 100
)

// String returns the string representation of a pool state.
func (s PoolState) String() string {
	switch s {
	case PoolStateStopped:
		return "stopped"
	case PoolStateRunning:
		return "running"
	case PoolStateDraining:
		return "draining"
	default:
		return "unknown"
	}
}

// Pool manages a pool of workers for processing jobs.
type Pool struct {
	config  Config
	workers []*Worker
	handler JobHandler
	logger  infralogger.Logger
	state   atomic.Int32
	sem     chan struct{} // Semaphore for bounded concurrency
	wg      sync.WaitGroup
	stopCh  chan struct{}
	mu      sync.RWMutex

	// Stats
	totalJobsProcessed atomic.Int64
	totalJobsSucceeded atomic.Int64
	totalJobsFailed    atomic.Int64
}

// NewPool creates a new worker pool.
func NewPool(cfg Config, handler JobHandler, logger infralogger.Logger) (*Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if handler == nil {
		return nil, errors.New("handler cannot be nil")
	}

	p := &Pool{
		config:  cfg,
		handler: handler,
		logger:  logger,
		workers: make([]*Worker, cfg.PoolSize),
		sem:     make(chan struct{}, cfg.PoolSize),
		stopCh:  make(chan struct{}),
	}

	// Initialize workers
	for i := range cfg.PoolSize {
		p.workers[i] = NewWorker(i, handler, cfg.JobTimeout, logger)
	}

	p.state.Store(int32(PoolStateStopped))

	return p, nil
}

// Start starts the worker pool.
func (p *Pool) Start() error {
	if !p.state.CompareAndSwap(int32(PoolStateStopped), int32(PoolStateRunning)) {
		return errors.New("pool is already running")
	}

	p.logger.Info("worker pool started",
		infralogger.Int("pool_size", p.config.PoolSize),
	)

	return nil
}

// Stop gracefully stops the worker pool.
func (p *Pool) Stop(ctx context.Context) error {
	if !p.state.CompareAndSwap(int32(PoolStateRunning), int32(PoolStateDraining)) {
		return errors.New("pool is not running")
	}

	p.logger.Info("worker pool draining")

	// Signal stop
	close(p.stopCh)

	// Wait for active jobs to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		p.logger.Info("worker pool stopped gracefully")
	case <-ctx.Done():
		p.logger.Warn("worker pool stop timed out")
	case <-time.After(p.config.DrainTimeout):
		p.logger.Warn("worker pool drain timeout exceeded")
	}

	p.state.Store(int32(PoolStateStopped))
	return nil
}

// Submit submits a job for processing.
// Blocks if all workers are busy.
func (p *Pool) Submit(ctx context.Context, job *queue.ConsumedJob) error {
	if p.State() != PoolStateRunning {
		return errors.New("pool is not running")
	}

	// Acquire semaphore (blocks if pool is full)
	select {
	case p.sem <- struct{}{}:
		// Got a slot
	case <-ctx.Done():
		return ctx.Err()
	case <-p.stopCh:
		return errors.New("pool is stopping")
	}

	p.wg.Add(1)

	go func() {
		defer func() {
			<-p.sem // Release semaphore
			p.wg.Done()
		}()

		// Find an idle worker
		worker := p.acquireWorker()
		if worker == nil {
			p.logger.Error("no idle worker available",
				infralogger.String("job_id", job.Job.ID),
			)
			return
		}

		// Process the job
		err := worker.Process(ctx, job)

		// Update pool stats
		p.totalJobsProcessed.Add(1)
		if err != nil {
			p.totalJobsFailed.Add(1)
		} else {
			p.totalJobsSucceeded.Add(1)
		}
	}()

	return nil
}

// TrySubmit attempts to submit a job without blocking.
// Returns false if no worker is available.
func (p *Pool) TrySubmit(ctx context.Context, job *queue.ConsumedJob) (bool, error) {
	if p.State() != PoolStateRunning {
		return false, errors.New("pool is not running")
	}

	// Try to acquire semaphore without blocking
	select {
	case p.sem <- struct{}{}:
		// Got a slot
	default:
		return false, nil // No worker available
	}

	p.wg.Add(1)

	go func() {
		defer func() {
			<-p.sem // Release semaphore
			p.wg.Done()
		}()

		worker := p.acquireWorker()
		if worker == nil {
			return
		}

		err := worker.Process(ctx, job)

		p.totalJobsProcessed.Add(1)
		if err != nil {
			p.totalJobsFailed.Add(1)
		} else {
			p.totalJobsSucceeded.Add(1)
		}
	}()

	return true, nil
}

// acquireWorker finds an idle worker.
func (p *Pool) acquireWorker() *Worker {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for _, w := range p.workers {
		if w.IsIdle() {
			return w
		}
	}
	return nil
}

// State returns the current pool state.
func (p *Pool) State() PoolState {
	return PoolState(p.state.Load())
}

// IsRunning returns true if the pool is running.
func (p *Pool) IsRunning() bool {
	return p.State() == PoolStateRunning
}

// Size returns the pool size.
func (p *Pool) Size() int {
	return p.config.PoolSize
}

// BusyCount returns the number of busy workers.
func (p *Pool) BusyCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()

	count := 0
	for _, w := range p.workers {
		if w.IsBusy() {
			count++
		}
	}
	return count
}

// IdleCount returns the number of idle workers.
func (p *Pool) IdleCount() int {
	return p.Size() - p.BusyCount()
}

// Stats returns pool statistics.
func (p *Pool) Stats() PoolStats {
	workerStats := make([]WorkerStats, len(p.workers))
	for i, w := range p.workers {
		workerStats[i] = w.Stats()
	}

	return PoolStats{
		State:         p.State(),
		PoolSize:      p.config.PoolSize,
		BusyWorkers:   p.BusyCount(),
		IdleWorkers:   p.IdleCount(),
		JobsProcessed: p.totalJobsProcessed.Load(),
		JobsSucceeded: p.totalJobsSucceeded.Load(),
		JobsFailed:    p.totalJobsFailed.Load(),
		Workers:       workerStats,
	}
}

// WorkerStats returns statistics for all workers.
func (p *Pool) WorkerStats() []WorkerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := make([]WorkerStats, len(p.workers))
	for i, w := range p.workers {
		stats[i] = w.Stats()
	}
	return stats
}

// PoolStats holds statistics for the pool.
type PoolStats struct {
	State         PoolState
	PoolSize      int
	BusyWorkers   int
	IdleWorkers   int
	JobsProcessed int64
	JobsSucceeded int64
	JobsFailed    int64
	Workers       []WorkerStats
}

// SuccessRate returns the success rate as a percentage.
func (s PoolStats) SuccessRate() float64 {
	if s.JobsProcessed == 0 {
		return 0
	}
	return float64(s.JobsSucceeded) / float64(s.JobsProcessed) * poolPercentageMultiplier
}

// Utilization returns the pool utilization as a percentage.
func (s PoolStats) Utilization() float64 {
	if s.PoolSize == 0 {
		return 0
	}
	return float64(s.BusyWorkers) / float64(s.PoolSize) * poolPercentageMultiplier
}
