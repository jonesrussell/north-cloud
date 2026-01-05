// Package scheduler provides interval-based job scheduling with distributed locking.
package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
)

const (
	defaultCheckInterval   = 10 * time.Second
	defaultLockDuration    = 5 * time.Minute
	defaultMetricsInterval = 30 * time.Second
	hoursPerDay            = 24
	exponentialBackoffBase = 2
)

// JobExecution represents an active job execution with its context.
type JobExecution struct {
	Job       *domain.Job
	Execution *domain.JobExecution
	Context   context.Context
	Cancel    context.CancelFunc
	StartTime time.Time
}

// IntervalScheduler replaces the cron-based scheduler with interval-based scheduling.
type IntervalScheduler struct {
	logger        logger.Interface
	repo          database.JobRepositoryInterface
	executionRepo database.ExecutionRepositoryInterface
	crawler       crawler.Interface

	// Scheduler control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Job execution
	activeJobs   map[string]*JobExecution
	activeJobsMu sync.RWMutex

	// Configuration
	checkInterval          time.Duration
	lockDuration           time.Duration
	metricsInterval        time.Duration
	staleLockCheckInterval time.Duration

	// Metrics
	metrics *SchedulerMetrics
}

// NewIntervalScheduler creates a new interval-based scheduler.
func NewIntervalScheduler(
	log logger.Interface,
	repo database.JobRepositoryInterface,
	executionRepo database.ExecutionRepositoryInterface,
	crawlerInstance crawler.Interface,
	opts ...SchedulerOption,
) *IntervalScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &IntervalScheduler{
		logger:                 log,
		repo:                   repo,
		executionRepo:          executionRepo,
		crawler:                crawlerInstance,
		ctx:                    ctx,
		cancel:                 cancel,
		activeJobs:             make(map[string]*JobExecution),
		checkInterval:          defaultCheckInterval,
		lockDuration:           defaultLockDuration,
		metricsInterval:        defaultMetricsInterval,
		staleLockCheckInterval: 1 * time.Minute,
		metrics:                &SchedulerMetrics{},
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the interval scheduler.
func (s *IntervalScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting interval scheduler",
		"check_interval", s.checkInterval,
		"lock_duration", s.lockDuration,
		"metrics_interval", s.metricsInterval)

	// Start job poller
	s.wg.Add(1)
	go s.pollJobs()

	// Start metrics collector
	s.wg.Add(1)
	go s.collectMetrics()

	// Start stale lock cleaner
	s.wg.Add(1)
	go s.cleanStaleLocks()

	s.logger.Info("Interval scheduler started successfully")
	return nil
}

// Stop stops the interval scheduler gracefully.
func (s *IntervalScheduler) Stop() error {
	s.logger.Info("Stopping interval scheduler")

	// Cancel context to stop all goroutines
	s.cancel()

	// Cancel all active jobs
	s.cancelAllActiveJobs()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.logger.Info("Interval scheduler stopped")
	return nil
}

// pollJobs continuously checks for jobs that need to be executed.
func (s *IntervalScheduler) pollJobs() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	s.logger.Info("Job poller started", "interval", s.checkInterval)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Job poller stopping")
			return
		case <-ticker.C:
			s.checkAndExecuteJobs()
		}
	}
}

// checkAndExecuteJobs finds jobs ready to run and executes them.
func (s *IntervalScheduler) checkAndExecuteJobs() {
	s.metrics.UpdateLastCheck()

	// Get jobs ready to run
	jobs, err := s.repo.GetJobsReadyToRun(s.ctx)
	if err != nil {
		s.logger.Error("Failed to get jobs ready to run", "error", err)
		return
	}

	if len(jobs) > 0 {
		s.logger.Debug("Found jobs ready to run", "count", len(jobs))
	}

	for _, job := range jobs {
		// Try to acquire lock
		acquired, lockErr := s.acquireJobLock(job)
		if lockErr != nil {
			s.logger.Error("Failed to acquire lock", "job_id", job.ID, "error", lockErr)
			continue
		}

		if !acquired {
			s.logger.Debug("Job already locked by another instance", "job_id", job.ID)
			continue
		}

		// Execute job
		s.executeJob(job)
	}
}

// acquireJobLock attempts to acquire a distributed lock for a job.
func (s *IntervalScheduler) acquireJobLock(job *domain.Job) (bool, error) {
	lockToken := uuid.New()
	now := time.Now()

	acquired, err := s.repo.AcquireLock(s.ctx, job.ID, lockToken, now, s.lockDuration)
	if err != nil {
		return false, fmt.Errorf("lock acquisition failed: %w", err)
	}

	if acquired {
		s.logger.Debug("Lock acquired", "job_id", job.ID, "lock_token", lockToken)
		// Update job with lock token for tracking
		job.LockToken = new(string)
		*job.LockToken = lockToken.String()
		job.LockAcquiredAt = &now
	}

	return acquired, nil
}

// executeJob executes a single job.
func (s *IntervalScheduler) executeJob(job *domain.Job) {
	// Check if already running
	s.activeJobsMu.RLock()
	if _, exists := s.activeJobs[job.ID]; exists {
		s.logger.Warn("Job already running", "job_id", job.ID)
		s.activeJobsMu.RUnlock()
		return
	}
	s.activeJobsMu.RUnlock()

	// Create execution record
	execution := &domain.JobExecution{
		ID:              uuid.New().String(),
		JobID:           job.ID,
		ExecutionNumber: s.getNextExecutionNumber(job.ID),
		Status:          "running",
		StartedAt:       time.Now(),
		RetryAttempt:    job.CurrentRetryCount,
	}

	if err := s.executionRepo.Create(s.ctx, execution); err != nil {
		s.logger.Error("Failed to create execution record", "job_id", job.ID, "error", err)
		s.releaseLock(job)
		return
	}

	// Update job status
	job.Status = "running"
	now := time.Now()
	job.StartedAt = &now

	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job status", "job_id", job.ID, "error", err)
		s.releaseLock(job)
		return
	}

	// Create execution context with cancellation
	jobCtx, cancel := context.WithCancel(s.ctx)

	jobExec := &JobExecution{
		Job:       job,
		Execution: execution,
		Context:   jobCtx,
		Cancel:    cancel,
		StartTime: time.Now(),
	}

	// Track active job
	s.activeJobsMu.Lock()
	s.activeJobs[job.ID] = jobExec
	s.activeJobsMu.Unlock()

	s.metrics.IncrementRunning()

	// Execute in goroutine
	go s.runJob(jobExec)
}

// runJob executes the actual crawl job.
func (s *IntervalScheduler) runJob(jobExec *JobExecution) {
	defer func() {
		// Remove from active jobs
		s.activeJobsMu.Lock()
		delete(s.activeJobs, jobExec.Job.ID)
		s.activeJobsMu.Unlock()

		s.metrics.DecrementRunning()

		// Release lock
		s.releaseLock(jobExec.Job)
	}()

	job := jobExec.Job

	s.logger.Info("Executing job",
		"job_id", job.ID,
		"source_id", job.SourceID,
		"url", job.URL,
		"retry_attempt", job.CurrentRetryCount)

	// Validate source ID
	if job.SourceID == "" {
		s.handleJobFailure(jobExec, errors.New("job missing required source_id"), nil)
		return
	}

	// Execute crawler
	startTime := time.Now()
	err := s.crawler.Start(jobExec.Context, job.SourceID)

	if err != nil {
		s.logger.Error("Crawler start failed",
			"job_id", job.ID,
			"source_id", job.SourceID,
			"error", err)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	// Wait for completion
	err = s.crawler.Wait()

	if err != nil {
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	s.handleJobSuccess(jobExec, &startTime)
}

// handleJobSuccess handles successful job completion.
func (s *IntervalScheduler) handleJobSuccess(jobExec *JobExecution, startTime *time.Time) {
	job := jobExec.Job
	execution := jobExec.Execution

	now := time.Now()
	durationMs := time.Since(*startTime).Milliseconds()

	// Update execution record
	execution.Status = "completed"
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs

	if err := s.executionRepo.Update(s.ctx, execution); err != nil {
		s.logger.Error("Failed to update execution", "execution_id", execution.ID, "error", err)
	}

	// Update job
	job.Status = "completed"
	job.CompletedAt = &now
	job.CurrentRetryCount = 0
	job.ErrorMessage = nil

	// If recurring, schedule next run
	if job.IntervalMinutes != nil && job.ScheduleEnabled {
		job.Status = "scheduled"
		nextRun := s.calculateNextRun(job)
		job.NextRunAt = &nextRun
	}

	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job", "job_id", job.ID, "error", err)
	}

	s.metrics.IncrementCompleted()
	s.metrics.IncrementTotalExecutions()

	s.logger.Info("Job completed successfully",
		"job_id", job.ID,
		"duration_ms", durationMs,
		"next_run_at", job.NextRunAt)
}

// handleJobFailure handles job execution failure.
func (s *IntervalScheduler) handleJobFailure(jobExec *JobExecution, execErr error, startTime *time.Time) {
	job := jobExec.Job
	execution := jobExec.Execution

	now := time.Now()
	var durationMs int64
	if startTime != nil {
		durationMs = time.Since(*startTime).Milliseconds()
	}

	// Update execution record
	execution.Status = "failed"
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs
	errMsg := execErr.Error()
	execution.ErrorMessage = &errMsg

	if err := s.executionRepo.Update(s.ctx, execution); err != nil {
		s.logger.Error("Failed to update execution", "execution_id", execution.ID, "error", err)
	}

	// Check if should retry
	if job.CurrentRetryCount < job.MaxRetries {
		// Schedule retry with backoff
		job.CurrentRetryCount++
		backoff := s.calculateBackoff(job)
		nextRun := time.Now().Add(backoff)
		job.NextRunAt = &nextRun
		job.Status = "scheduled"

		s.logger.Info("Scheduling retry",
			"job_id", job.ID,
			"retry_attempt", job.CurrentRetryCount,
			"max_retries", job.MaxRetries,
			"backoff", backoff,
			"next_run_at", nextRun,
			"error", execErr)
	} else {
		// No more retries
		job.Status = "failed"
		job.CompletedAt = &now

		s.metrics.IncrementFailed()

		s.logger.Error("Job failed after all retries",
			"job_id", job.ID,
			"error", execErr,
			"retries", job.CurrentRetryCount)
	}

	job.ErrorMessage = &errMsg
	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job", "job_id", job.ID, "error", err)
	}

	s.metrics.IncrementTotalExecutions()
}

// calculateNextRun calculates the next run time based on interval configuration.
func (s *IntervalScheduler) calculateNextRun(job *domain.Job) time.Time {
	if job.IntervalMinutes == nil {
		return time.Time{}
	}

	var duration time.Duration
	switch job.IntervalType {
	case "minutes":
		duration = time.Duration(*job.IntervalMinutes) * time.Minute
	case "hours":
		duration = time.Duration(*job.IntervalMinutes) * time.Hour
	case "days":
		duration = time.Duration(*job.IntervalMinutes) * hoursPerDay * time.Hour
	default:
		duration = time.Duration(*job.IntervalMinutes) * time.Minute
	}

	return time.Now().Add(duration)
}

// calculateBackoff calculates exponential backoff duration for retries.
func (s *IntervalScheduler) calculateBackoff(job *domain.Job) time.Duration {
	baseBackoff := time.Duration(job.RetryBackoffSeconds) * time.Second

	// Exponential backoff: base * 2^(attempt-1)
	multiplier := math.Pow(exponentialBackoffBase, float64(job.CurrentRetryCount-1))
	backoff := time.Duration(float64(baseBackoff) * multiplier)

	// Cap at 1 hour
	maxBackoff := 1 * time.Hour
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	return backoff
}

// releaseLock releases the job lock.
func (s *IntervalScheduler) releaseLock(job *domain.Job) {
	if err := s.repo.ReleaseLock(s.ctx, job.ID); err != nil {
		s.logger.Error("Failed to release lock", "job_id", job.ID, "error", err)
	} else {
		s.logger.Debug("Lock released", "job_id", job.ID)
	}
}

// cancelAllActiveJobs cancels all currently running jobs.
func (s *IntervalScheduler) cancelAllActiveJobs() {
	s.activeJobsMu.Lock()
	defer s.activeJobsMu.Unlock()

	for id, jobExec := range s.activeJobs {
		s.logger.Info("Cancelling active job", "job_id", id)
		jobExec.Cancel()
	}
}

// cleanStaleLocks periodically cleans up stale locks.
func (s *IntervalScheduler) cleanStaleLocks() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.staleLockCheckInterval)
	defer ticker.Stop()

	s.logger.Info("Stale lock cleaner started", "interval", s.staleLockCheckInterval)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Stale lock cleaner stopping")
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.lockDuration)
			count, err := s.repo.ClearStaleLocks(s.ctx, cutoff)
			if err != nil {
				s.logger.Error("Failed to clear stale locks", "error", err)
			} else if count > 0 {
				s.logger.Info("Cleared stale locks", "count", count)
				s.metrics.AddStaleLocksCleared(count)
			}
		}
	}
}

// collectMetrics periodically updates scheduler metrics.
func (s *IntervalScheduler) collectMetrics() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.metricsInterval)
	defer ticker.Stop()

	s.logger.Info("Metrics collector started", "interval", s.metricsInterval)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Metrics collector stopping")
			return
		case <-ticker.C:
			s.updateMetrics()
		}
	}
}

// updateMetrics calculates and updates scheduler metrics.
func (s *IntervalScheduler) updateMetrics() {
	// Get aggregate stats from database
	stats, err := s.executionRepo.GetAggregateStats(s.ctx)
	if err != nil {
		s.logger.Error("Failed to get aggregate stats", "error", err)
		return
	}

	s.metrics.UpdateAggregateMetrics(stats.AvgDurationMs, stats.SuccessRate)

	s.logger.Debug("Metrics updated",
		"avg_duration_ms", stats.AvgDurationMs,
		"success_rate", stats.SuccessRate,
		"active_jobs", stats.ActiveJobs,
		"scheduled_jobs", stats.ScheduledJobs)
}

// GetMetrics returns a snapshot of current scheduler metrics.
func (s *IntervalScheduler) GetMetrics() SchedulerMetrics {
	return s.metrics.Snapshot()
}

// getNextExecutionNumber gets the next execution number for a job.
func (s *IntervalScheduler) getNextExecutionNumber(jobID string) int {
	count, err := s.executionRepo.CountByJobID(s.ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get execution count", "job_id", jobID, "error", err)
		return 1
	}
	return count + 1
}

// CancelJob cancels a running job by its ID.
// This is called externally (e.g., via API) to cancel a job mid-execution.
func (s *IntervalScheduler) CancelJob(jobID string) error {
	s.activeJobsMu.RLock()
	jobExec, exists := s.activeJobs[jobID]
	s.activeJobsMu.RUnlock()

	if !exists {
		return fmt.Errorf("job not currently running: %s", jobID)
	}

	s.logger.Info("Cancelling job execution", "job_id", jobID)
	jobExec.Cancel()

	s.metrics.IncrementCancelled()

	return nil
}
