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
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/north-cloud/infrastructure/logger"
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
	logger        infralogger.Logger
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

	// SSE integration (optional)
	ssePublisher *SSEPublisher

	// Log service for job log capture (optional)
	logService logs.Service

	// Load balancing
	bucketMap *BucketMap
}

// NewIntervalScheduler creates a new interval-based scheduler.
func NewIntervalScheduler(
	log infralogger.Logger,
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
		bucketMap:              NewBucketMap(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// rebuildBucketMap rebuilds the bucket map from database state on startup.
func (s *IntervalScheduler) rebuildBucketMap() error {
	if s.bucketMap == nil {
		return nil // Load balancing disabled
	}

	jobs, err := s.repo.GetScheduledJobs(s.ctx)
	if err != nil {
		return fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	for _, job := range jobs {
		if job.NextRunAt != nil {
			s.bucketMap.AddJob(job.ID, SlotKey(*job.NextRunAt))
		}
	}

	s.logger.Info("Bucket map rebuilt",
		infralogger.Int("job_count", len(jobs)),
	)
	return nil
}

// Start starts the interval scheduler.
func (s *IntervalScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting interval scheduler",
		infralogger.Duration("check_interval", s.checkInterval),
		infralogger.Duration("lock_duration", s.lockDuration),
		infralogger.Duration("metrics_interval", s.metricsInterval),
	)

	// Rebuild bucket map from existing scheduled jobs
	if err := s.rebuildBucketMap(); err != nil {
		return fmt.Errorf("failed to rebuild bucket map: %w", err)
	}

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

	s.logger.Info("Job poller started", infralogger.Duration("interval", s.checkInterval))

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
		s.logger.Error("Failed to get jobs ready to run", infralogger.Error(err))
		return
	}

	if len(jobs) > 0 {
		s.logger.Debug("Found jobs ready to run", infralogger.Int("count", len(jobs)))
	}

	for _, job := range jobs {
		// Try to acquire lock
		acquired, lockErr := s.acquireJobLock(job)
		if lockErr != nil {
			s.logger.Error("Failed to acquire lock",
				infralogger.String("job_id", job.ID),
				infralogger.Error(lockErr),
			)
			continue
		}

		if !acquired {
			s.logger.Debug("Job already locked by another instance", infralogger.String("job_id", job.ID))
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
		s.logger.Debug("Lock acquired",
			infralogger.String("job_id", job.ID),
		)
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
		s.logger.Warn("Job already running", infralogger.String("job_id", job.ID))
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
		s.logger.Error("Failed to create execution record",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
		s.releaseLock(job)
		return
	}

	// Update job status
	job.Status = "running"
	now := time.Now()
	job.StartedAt = &now

	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job status",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
		s.releaseLock(job)
		return
	}

	// Publish SSE event for job start
	s.publishJobStatus(s.ctx, job)

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

// writeLog writes a log entry if the log writer is available.
func writeLog(w logs.Writer, level, message, jobID, execID string, fields map[string]any) {
	if w == nil {
		return
	}
	w.WriteEntry(logs.LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		JobID:     jobID,
		ExecID:    execID,
		Fields:    fields,
	})
}

// startLogCapture starts log capture for a job execution.
func (s *IntervalScheduler) startLogCapture(jobExec *JobExecution) logs.Writer {
	if s.logService == nil {
		return nil
	}

	job := jobExec.Job
	execution := jobExec.Execution

	logWriter, captureErr := s.logService.StartCapture(
		jobExec.Context,
		job.ID,
		execution.ID,
		execution.ExecutionNumber,
	)
	if captureErr != nil {
		s.logger.Warn("Failed to start log capture, continuing without logging",
			infralogger.String("job_id", job.ID),
			infralogger.Error(captureErr),
		)
		return nil
	}
	return logWriter
}

// stopLogCapture stops log capture for a job execution.
func (s *IntervalScheduler) stopLogCapture(jobExec *JobExecution, logWriter logs.Writer) {
	if s.logService == nil || logWriter == nil {
		return
	}
	if _, stopErr := s.logService.StopCapture(s.ctx, jobExec.Job.ID, jobExec.Execution.ID); stopErr != nil {
		s.logger.Error("Failed to stop log capture",
			infralogger.String("job_id", jobExec.Job.ID),
			infralogger.Error(stopErr),
		)
	}
}

// createCaptureFunc creates a capture function from a logs.Writer.
func createCaptureFunc(w logs.Writer) func(logs.LogEntry) {
	if w == nil {
		return nil
	}
	return func(entry logs.LogEntry) {
		w.WriteEntry(entry)
	}
}

// runJob executes the actual crawl job.
func (s *IntervalScheduler) runJob(jobExec *JobExecution) {
	job := jobExec.Job
	execution := jobExec.Execution
	logWriter := s.startLogCapture(jobExec)

	// Create and set JobLogger for this execution
	captureFunc := createCaptureFunc(logWriter)
	noThrottling := 0 // No throttling for normal verbosity
	jobLogger := logs.NewJobLoggerImpl(
		job.ID,
		execution.ID,
		logs.VerbosityNormal, // Default verbosity, can be made configurable later
		captureFunc,
		noThrottling,
	)
	s.crawler.SetJobLogger(jobLogger)

	// Start heartbeat for long-running jobs
	jobLogger.StartHeartbeat(jobExec.Context)

	defer func() {
		s.stopLogCapture(jobExec, logWriter)
		s.activeJobsMu.Lock()
		delete(s.activeJobs, job.ID)
		s.activeJobsMu.Unlock()
		s.metrics.DecrementRunning()
		s.releaseLock(job)
	}()

	// Write initial log entry
	writeLog(logWriter, "info", "Starting job execution", job.ID, execution.ID, map[string]any{
		"source_id":     job.SourceID,
		"url":           job.URL,
		"retry_attempt": job.CurrentRetryCount,
	})

	s.logger.Info("Executing job",
		infralogger.String("job_id", job.ID),
		infralogger.String("source_id", job.SourceID),
		infralogger.String("url", job.URL),
		infralogger.Int("retry_attempt", job.CurrentRetryCount),
	)

	// Validate source ID
	if job.SourceID == "" {
		writeLog(logWriter, "error", "Job missing required source_id", job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, errors.New("job missing required source_id"), nil)
		return
	}

	// Execute crawler
	startTime := time.Now()
	writeLog(logWriter, "info", "Starting crawler", job.ID, execution.ID, map[string]any{
		"source_id": job.SourceID,
	})

	err := s.crawler.Start(jobExec.Context, job.SourceID)
	if err != nil {
		writeLog(logWriter, "error", "Crawler start failed: "+err.Error(), job.ID, execution.ID, nil)
		s.logger.Error("Crawler start failed",
			infralogger.String("job_id", job.ID),
			infralogger.String("source_id", job.SourceID),
			infralogger.Error(err),
		)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	// Wait for completion
	writeLog(logWriter, "info", "Waiting for crawler to complete", job.ID, execution.ID, nil)

	err = s.crawler.Wait()
	if err != nil {
		writeLog(logWriter, "error", "Crawler failed: "+err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	// Get metrics for final log
	crawlerMetrics := s.crawler.GetMetrics()
	writeLog(logWriter, "info", "Job completed successfully", job.ID, execution.ID, map[string]any{
		"duration_ms":   time.Since(startTime).Milliseconds(),
		"items_crawled": crawlerMetrics.ProcessedCount,
		"items_indexed": crawlerMetrics.ProcessedCount,
		"error_count":   crawlerMetrics.ErrorCount,
	})

	s.handleJobSuccess(jobExec, &startTime)
}

// handleJobSuccess handles successful job completion.
func (s *IntervalScheduler) handleJobSuccess(jobExec *JobExecution, startTime *time.Time) {
	job := jobExec.Job
	execution := jobExec.Execution

	now := time.Now()
	durationMs := time.Since(*startTime).Milliseconds()

	// Get metrics from crawler
	metrics := s.crawler.GetMetrics()
	itemsCrawled := int(metrics.ProcessedCount)
	// Items are indexed immediately after crawling, so indexed count equals crawled count
	itemsIndexed := itemsCrawled

	// Update execution record
	execution.Status = string(StateCompleted)
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs
	execution.ItemsCrawled = itemsCrawled
	execution.ItemsIndexed = itemsIndexed

	if err := s.executionRepo.Update(s.ctx, execution); err != nil {
		s.logger.Error("Failed to update execution",
			infralogger.String("execution_id", execution.ID),
			infralogger.Error(err),
		)
	}

	// Update job
	job.Status = string(StateCompleted)
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
		s.logger.Error("Failed to update job",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
	}

	s.metrics.IncrementCompleted()
	s.metrics.IncrementTotalExecutions()

	s.logger.Info("Job completed successfully",
		infralogger.String("job_id", job.ID),
		infralogger.Int64("duration_ms", durationMs),
		infralogger.Int("items_crawled", itemsCrawled),
		infralogger.Int("items_indexed", itemsIndexed),
		infralogger.Any("next_run_at", job.NextRunAt),
	)

	// Publish SSE event for job completion
	s.publishJobCompleted(s.ctx, job, execution)
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
	execution.Status = string(StateFailed)
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs
	errMsg := execErr.Error()
	execution.ErrorMessage = &errMsg

	if err := s.executionRepo.Update(s.ctx, execution); err != nil {
		s.logger.Error("Failed to update execution",
			infralogger.String("execution_id", execution.ID),
			infralogger.Error(err),
		)
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
			infralogger.String("job_id", job.ID),
			infralogger.Int("retry_attempt", job.CurrentRetryCount),
			infralogger.Int("max_retries", job.MaxRetries),
			infralogger.Duration("backoff", backoff),
			infralogger.Time("next_run_at", nextRun),
			infralogger.Error(execErr),
		)
	} else {
		// No more retries
		job.Status = string(StateFailed)
		job.CompletedAt = &now

		s.metrics.IncrementFailed()

		s.logger.Error("Job failed after all retries",
			infralogger.String("job_id", job.ID),
			infralogger.Error(execErr),
			infralogger.Int("retries", job.CurrentRetryCount),
		)
	}

	job.ErrorMessage = &errMsg
	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
	}

	s.metrics.IncrementTotalExecutions()

	// Publish SSE event for job failure
	s.publishJobCompleted(s.ctx, job, execution)
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
		s.logger.Error("Failed to release lock",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
	} else {
		s.logger.Debug("Lock released", infralogger.String("job_id", job.ID))
	}
}

// cancelAllActiveJobs cancels all currently running jobs.
func (s *IntervalScheduler) cancelAllActiveJobs() {
	s.activeJobsMu.Lock()
	defer s.activeJobsMu.Unlock()

	for id, jobExec := range s.activeJobs {
		s.logger.Info("Cancelling active job", infralogger.String("job_id", id))
		jobExec.Cancel()
	}
}

// cleanStaleLocks periodically cleans up stale locks.
func (s *IntervalScheduler) cleanStaleLocks() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.staleLockCheckInterval)
	defer ticker.Stop()

	s.logger.Info("Stale lock cleaner started", infralogger.Duration("interval", s.staleLockCheckInterval))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Stale lock cleaner stopping")
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-s.lockDuration)
			count, err := s.repo.ClearStaleLocks(s.ctx, cutoff)
			if err != nil {
				s.logger.Error("Failed to clear stale locks", infralogger.Error(err))
			} else if count > 0 {
				s.logger.Info("Cleared stale locks", infralogger.Int("count", count))
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

	s.logger.Info("Metrics collector started", infralogger.Duration("interval", s.metricsInterval))
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
		s.logger.Error("Failed to get aggregate stats", infralogger.Error(err))
		return
	}

	s.metrics.UpdateAggregateMetrics(stats.AvgDurationMs, stats.SuccessRate)

	s.logger.Debug("Metrics updated",
		infralogger.Float64("avg_duration_ms", stats.AvgDurationMs),
		infralogger.Float64("success_rate", stats.SuccessRate),
		infralogger.Int64("active_jobs", stats.ActiveJobs),
		infralogger.Int64("scheduled_jobs", stats.ScheduledJobs),
	)
}

// GetMetrics returns a snapshot of current scheduler metrics.
func (s *IntervalScheduler) GetMetrics() SchedulerMetrics {
	return s.metrics.Snapshot()
}

// getNextExecutionNumber gets the next execution number for a job.
func (s *IntervalScheduler) getNextExecutionNumber(jobID string) int {
	count, err := s.executionRepo.CountByJobID(s.ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get execution count",
			infralogger.String("job_id", jobID),
			infralogger.Error(err),
		)
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

	s.logger.Info("Cancelling job execution", infralogger.String("job_id", jobID))
	jobExec.Cancel()

	s.metrics.IncrementCancelled()

	return nil
}

// SetSSEPublisher sets the SSE publisher for real-time event streaming.
// This is optional - if not set, no SSE events will be published.
//
// IMPORTANT: This method must be called before Start() to avoid data races.
// The ssePublisher field is not synchronized because it's intended to be
// set once during initialization and never changed during the scheduler's lifetime.
func (s *IntervalScheduler) SetSSEPublisher(publisher *SSEPublisher) {
	s.ssePublisher = publisher
}

// SetLogService sets the log service for job log capture.
// This is optional - if not set, no logs will be captured during job execution.
//
// IMPORTANT: This method must be called before Start() to avoid data races.
func (s *IntervalScheduler) SetLogService(logService logs.Service) {
	s.logService = logService
}

// publishJobStatus publishes a job status event if SSE is enabled.
func (s *IntervalScheduler) publishJobStatus(ctx context.Context, job *domain.Job) {
	if s.ssePublisher != nil {
		s.ssePublisher.PublishJobStatus(ctx, job, nil)
	}
}

// publishJobCompleted publishes a job completion event if SSE is enabled.
func (s *IntervalScheduler) publishJobCompleted(ctx context.Context, job *domain.Job, execution *domain.JobExecution) {
	if s.ssePublisher != nil {
		s.ssePublisher.PublishJobCompleted(ctx, job, execution)
	}
}
