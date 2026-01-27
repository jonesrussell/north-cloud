package worker

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/queue"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// WorkerState represents the current state of a worker.
type WorkerState int32

const (
	// WorkerStateIdle means the worker is waiting for work.
	WorkerStateIdle WorkerState = iota

	// WorkerStateBusy means the worker is processing a job.
	WorkerStateBusy

	// WorkerStateStopping means the worker is shutting down.
	WorkerStateStopping

	// WorkerStateStopped means the worker has stopped.
	WorkerStateStopped

	// stuckThresholdMultiplier is used to calculate stuck threshold from job timeout.
	stuckThresholdMultiplier = 2

	// percentageMultiplier converts ratio to percentage.
	percentageMultiplier = 100
)

// String returns the string representation of a worker state.
func (s WorkerState) String() string {
	switch s {
	case WorkerStateIdle:
		return "idle"
	case WorkerStateBusy:
		return "busy"
	case WorkerStateStopping:
		return "stopping"
	case WorkerStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// JobHandler is a function that processes a job.
type JobHandler func(ctx context.Context, job *domain.Job) error

// Worker represents an individual worker in the pool.
type Worker struct {
	id         int
	state      atomic.Int32
	handler    JobHandler
	jobTimeout time.Duration
	logger     infralogger.Logger

	// Stats
	jobsProcessed atomic.Int64
	jobsSucceeded atomic.Int64
	jobsFailed    atomic.Int64
	lastJobAt     atomic.Int64
	lastError     atomic.Value

	// Current job tracking
	currentJob   atomic.Value
	jobStartedAt atomic.Int64
}

// NewWorker creates a new worker.
func NewWorker(id int, handler JobHandler, jobTimeout time.Duration, logger infralogger.Logger) *Worker {
	w := &Worker{
		id:         id,
		handler:    handler,
		jobTimeout: jobTimeout,
		logger:     logger,
	}
	w.state.Store(int32(WorkerStateIdle))
	return w
}

// ID returns the worker ID.
func (w *Worker) ID() int {
	return w.id
}

// State returns the current worker state.
func (w *Worker) State() WorkerState {
	return WorkerState(w.state.Load())
}

// IsIdle returns true if the worker is idle.
func (w *Worker) IsIdle() bool {
	return w.State() == WorkerStateIdle
}

// IsBusy returns true if the worker is busy.
func (w *Worker) IsBusy() bool {
	return w.State() == WorkerStateBusy
}

// Process processes a job from the queue.
func (w *Worker) Process(ctx context.Context, consumedJob *queue.ConsumedJob) error {
	if consumedJob == nil || consumedJob.Job == nil {
		return fmt.Errorf("worker %d: job cannot be nil", w.id)
	}

	// Mark as busy
	if !w.state.CompareAndSwap(int32(WorkerStateIdle), int32(WorkerStateBusy)) {
		return fmt.Errorf("worker %d: not idle, current state: %s", w.id, w.State())
	}

	// Track current job
	w.currentJob.Store(consumedJob.Job)
	w.jobStartedAt.Store(time.Now().UnixNano())

	defer func() {
		w.currentJob.Store((*domain.Job)(nil))
		w.jobStartedAt.Store(0)
		w.state.Store(int32(WorkerStateIdle))
	}()

	// Create timeout context
	jobCtx, cancel := context.WithTimeout(ctx, w.jobTimeout)
	defer cancel()

	// Process the job
	w.logger.Info("worker processing job",
		infralogger.Int("worker_id", w.id),
		infralogger.String("job_id", consumedJob.Job.ID),
	)

	startTime := time.Now()
	err := w.handler(jobCtx, consumedJob.Job)
	duration := time.Since(startTime)

	// Update stats
	w.jobsProcessed.Add(1)
	w.lastJobAt.Store(time.Now().UnixNano())

	if err != nil {
		w.jobsFailed.Add(1)
		w.lastError.Store(err)
		w.logger.Error("worker job failed",
			infralogger.Int("worker_id", w.id),
			infralogger.String("job_id", consumedJob.Job.ID),
			infralogger.Duration("duration", duration),
			infralogger.String("error", err.Error()),
		)
		return fmt.Errorf("worker %d: job %s failed: %w", w.id, consumedJob.Job.ID, err)
	}

	w.jobsSucceeded.Add(1)
	w.logger.Info("worker job completed",
		infralogger.Int("worker_id", w.id),
		infralogger.String("job_id", consumedJob.Job.ID),
		infralogger.Duration("duration", duration),
	)

	return nil
}

// Stop signals the worker to stop.
func (w *Worker) Stop() {
	w.state.Store(int32(WorkerStateStopping))
}

// Stats returns the worker's statistics.
func (w *Worker) Stats() WorkerStats {
	var lastErr error
	if v := w.lastError.Load(); v != nil {
		lastErr, _ = v.(error)
	}

	var currentJobID string
	if v := w.currentJob.Load(); v != nil {
		if job, ok := v.(*domain.Job); ok && job != nil {
			currentJobID = job.ID
		}
	}

	var lastJobTime time.Time
	if ts := w.lastJobAt.Load(); ts > 0 {
		lastJobTime = time.Unix(0, ts)
	}

	var jobStartTime time.Time
	if ts := w.jobStartedAt.Load(); ts > 0 {
		jobStartTime = time.Unix(0, ts)
	}

	return WorkerStats{
		ID:            w.id,
		State:         w.State(),
		JobsProcessed: w.jobsProcessed.Load(),
		JobsSucceeded: w.jobsSucceeded.Load(),
		JobsFailed:    w.jobsFailed.Load(),
		LastJobAt:     lastJobTime,
		LastError:     lastErr,
		CurrentJobID:  currentJobID,
		JobStartedAt:  jobStartTime,
	}
}

// WorkerStats holds statistics for a worker.
type WorkerStats struct {
	ID            int
	State         WorkerState
	JobsProcessed int64
	JobsSucceeded int64
	JobsFailed    int64
	LastJobAt     time.Time
	LastError     error
	CurrentJobID  string
	JobStartedAt  time.Time
}

// SuccessRate returns the success rate as a percentage.
func (s WorkerStats) SuccessRate() float64 {
	if s.JobsProcessed == 0 {
		return 0
	}
	return float64(s.JobsSucceeded) / float64(s.JobsProcessed) * percentageMultiplier
}

// IsHealthy returns true if the worker is considered healthy.
func (s WorkerStats) IsHealthy() bool {
	// Worker is healthy if not stopped and not stuck on a job
	if s.State == WorkerStateStopped {
		return false
	}
	// Check if worker is stuck (busy for more than job timeout)
	if s.State == WorkerStateBusy && !s.JobStartedAt.IsZero() {
		stuckThreshold := stuckThresholdMultiplier * time.Hour
		if time.Since(s.JobStartedAt) > stuckThreshold {
			return false
		}
	}
	return true
}
