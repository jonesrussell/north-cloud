package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

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

	// Create execution context with timeout to prevent indefinite hangs
	jobCtx, cancel := context.WithTimeout(s.ctx, s.executionTimeout)

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

	// Run in a goroutine; defer cancel releases the WithTimeout timer when runJob returns
	// (idempotent if CancelJob or shutdown already called cancel).
	go func() {
		defer cancel()
		s.runJob(jobExec)
	}()
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

// createJobCrawler creates an isolated crawler instance for a single job execution.
// Returns the crawler instance or nil with an error logged.
func (s *IntervalScheduler) createJobCrawler(jobExec *JobExecution, logWriter logs.Writer) (crawler.Interface, error) {
	crawlerInstance, err := s.factory.Create()
	if err != nil {
		writeLog(logWriter, "error", "Failed to create crawler: "+err.Error(),
			jobExec.Job.ID, jobExec.Execution.ID, nil)
		return nil, fmt.Errorf("create crawler for job %s: %w", jobExec.Job.ID, err)
	}

	// Create and set JobLogger for this execution
	captureFunc := createCaptureFunc(logWriter)
	noThrottling := 0
	jobLogger := logs.NewJobLoggerImpl(
		jobExec.Job.ID,
		jobExec.Execution.ID,
		logs.VerbosityNormal,
		captureFunc,
		noThrottling,
	)
	crawlerInstance.SetJobLogger(jobLogger)
	jobLogger.StartHeartbeat(jobExec.Context)

	jobExec.Crawler = crawlerInstance
	return crawlerInstance, nil
}

// runJob dispatches job execution by type.
func (s *IntervalScheduler) runJob(jobExec *JobExecution) {
	job := jobExec.Job
	execution := jobExec.Execution

	// Panic recovery registered first — runs last (LIFO) after cleanup defer.
	defer s.recoverFromPanic(job, execution)

	logWriter := s.startLogCapture(jobExec)

	defer func() {
		s.stopLogCapture(jobExec, logWriter)
		s.activeJobsMu.Lock()
		delete(s.activeJobs, job.ID)
		s.activeJobsMu.Unlock()
		s.metrics.DecrementRunning()
		s.releaseLock(job)
	}()

	// Dispatch by job type
	switch job.Type {
	case domain.JobTypeLeadershipScrape:
		s.runLeadershipJob(jobExec, logWriter)
	default: // "crawl" or empty (backward compat with pre-type jobs)
		s.runCrawlJob(jobExec, logWriter)
	}
}

// runCrawlJob executes a standard web crawl job.
func (s *IntervalScheduler) runCrawlJob(jobExec *JobExecution, logWriter logs.Writer) {
	job := jobExec.Job
	execution := jobExec.Execution

	// Create an isolated crawler for this job
	crawlerInstance, err := s.createJobCrawler(jobExec, logWriter)
	if err != nil {
		s.handleJobFailure(jobExec, err, nil)
		return
	}

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

	if job.SourceID == "" {
		writeLog(logWriter, "error", "Job missing required source_id", job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, errors.New("job missing required source_id"), nil)
		return
	}

	// Capture startTime AFTER crawler creation to match original timing semantics
	startTime := time.Now()
	writeLog(logWriter, "info", "Starting crawler", job.ID, execution.ID, map[string]any{
		"source_id": job.SourceID,
	})

	err = crawlerInstance.Start(jobExec.Context, job.SourceID)
	if err != nil {
		s.logCrawlerStartError(job, execution.ID, err, logWriter)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	writeLog(logWriter, "info", "Waiting for crawler to complete", job.ID, execution.ID, nil)

	err = crawlerInstance.Wait()
	if err != nil {
		writeLog(logWriter, "error", "Crawler failed: "+err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	jobSummary := crawlerInstance.GetJobLogger().BuildSummary()
	writeLog(logWriter, "info", "Job completed successfully", job.ID, execution.ID, map[string]any{
		"duration_ms":     time.Since(startTime).Milliseconds(),
		"pages_crawled":   jobSummary.PagesCrawled,
		"items_extracted": jobSummary.ItemsExtracted,
		"error_count":     jobSummary.ErrorsCount,
	})

	s.handleJobSuccess(jobExec, &startTime)
}

// runLeadershipJob executes a leadership scrape job.
func (s *IntervalScheduler) runLeadershipJob(jobExec *JobExecution, logWriter logs.Writer) {
	job := jobExec.Job
	execution := jobExec.Execution
	startTime := time.Now()

	writeLog(logWriter, "info", "Starting leadership scrape job", job.ID, execution.ID, nil)

	s.logger.Info("Executing leadership scrape job",
		infralogger.String("job_id", job.ID),
	)

	if s.scraperConfig == nil {
		err := errors.New("leadership scrape: scraper not configured")
		writeLog(logWriter, "error", err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	err := RunLeadershipScrapeJob(jobExec.Context, *s.scraperConfig, s.logger)
	if err != nil {
		writeLog(logWriter, "error", "Leadership scrape failed: "+err.Error(), job.ID, execution.ID, nil)
		s.handleJobFailure(jobExec, err, &startTime)
		return
	}

	writeLog(logWriter, "info", "Leadership scrape completed successfully", job.ID, execution.ID, map[string]any{
		"duration_ms": time.Since(startTime).Milliseconds(),
	})

	s.handleLeadershipJobSuccess(jobExec, &startTime)
}

// handleLeadershipJobSuccess handles successful leadership scrape completion.
// Uses calculateNextRun (not adaptive) because leadership scrape jobs have no
// content hash tracking — adaptive scheduling only applies to crawl jobs.
func (s *IntervalScheduler) handleLeadershipJobSuccess(jobExec *JobExecution, startTime *time.Time) {
	s.completeJob(jobExec, startTime, nil)
}

// logCrawlerStartError logs a crawler start error at the appropriate level.
func (s *IntervalScheduler) logCrawlerStartError(
	job *domain.Job, execID string, err error, logWriter logs.Writer,
) {
	if errors.Is(err, context.DeadlineExceeded) {
		writeLog(logWriter, "warn", "Crawl timed out (context deadline exceeded): "+err.Error(), job.ID, execID, nil)
		s.logger.Warn("Crawl timed out: context deadline exceeded",
			infralogger.String("job_id", job.ID),
			infralogger.String("source_id", job.SourceID),
			infralogger.String("url", job.URL),
			infralogger.Error(err),
		)
	} else if isExpectedStartError(err) {
		writeLog(logWriter, "warn", "Crawler start failed (expected): "+err.Error(), job.ID, execID, nil)
		s.logger.Warn("Crawler start failed (expected)",
			infralogger.String("job_id", job.ID),
			infralogger.String("source_id", job.SourceID),
			infralogger.String("url", job.URL),
			infralogger.Error(err),
		)
	} else {
		writeLog(logWriter, "error", "Crawler start failed: "+err.Error(), job.ID, execID, nil)
		s.logger.Error("Crawler start failed",
			infralogger.String("job_id", job.ID),
			infralogger.String("source_id", job.SourceID),
			infralogger.String("url", job.URL),
			infralogger.Error(err),
		)
	}
}

// recoverFromPanic catches panics in job execution and marks the job as failed.
// This prevents the goroutine from crashing and leaving the job stuck in "running" state.
func (s *IntervalScheduler) recoverFromPanic(job *domain.Job, execution *domain.JobExecution) {
	r := recover()
	if r == nil {
		return
	}

	s.logger.Error("Recovered from panic in job execution",
		infralogger.String("job_id", job.ID),
		infralogger.Any("panic", r),
	)

	now := time.Now()
	errMsg := fmt.Sprintf("panic: %v", r)

	// Mark execution as failed
	execution.Status = string(StateFailed)
	execution.CompletedAt = &now
	execution.ErrorMessage = &errMsg
	if updateErr := s.executionRepo.Update(s.ctx, execution); updateErr != nil {
		s.logger.Error("Failed to update panicked execution", infralogger.Error(updateErr))
	}

	// Reset job: schedule next run if recurring, otherwise mark failed
	s.resetJobAfterFailure(job, &errMsg, &now)
	s.metrics.IncrementFailed()
	s.metrics.IncrementTotalExecutions()
}

// resetJobAfterFailure resets a job after a failure (panic or stuck recovery).
// Recurring jobs are rescheduled; one-time jobs are marked failed.
func (s *IntervalScheduler) resetJobAfterFailure(job *domain.Job, errMsg *string, now *time.Time) {
	if job.IntervalMinutes != nil && job.ScheduleEnabled {
		job.Status = string(StateScheduled)
		nextRun := s.calculateNextRun(job)
		job.NextRunAt = &nextRun
	} else {
		job.Status = string(StateFailed)
		job.CompletedAt = now
	}

	job.ErrorMessage = errMsg

	if updateErr := s.repo.Update(s.ctx, job); updateErr != nil {
		s.logger.Error("Failed to update job after failure",
			infralogger.String("job_id", job.ID),
			infralogger.Error(updateErr),
		)
	}
}

// handleJobSuccess handles successful crawl job completion.
func (s *IntervalScheduler) handleJobSuccess(jobExec *JobExecution, startTime *time.Time) {
	summary := jobExec.Crawler.GetJobLogger().BuildSummary()
	s.completeJob(jobExec, startTime, summary)
}

// completeJob is the shared success handler for all job types.
// When summary is nil (e.g. leadership scrape), crawler metrics are skipped
// and the fixed-interval scheduler is used instead of adaptive scheduling.
func (s *IntervalScheduler) completeJob(jobExec *JobExecution, startTime *time.Time, summary *logs.JobSummary) {
	job := jobExec.Job
	execution := jobExec.Execution

	now := time.Now()
	durationMs := time.Since(*startTime).Milliseconds()

	// Update execution record
	execution.Status = string(StateCompleted)
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs

	if summary != nil {
		execution.ItemsCrawled = int(summary.PagesCrawled)
		execution.ItemsIndexed = int(summary.ItemsExtracted)
		execution.Metadata = BuildExecutionMetadata(summary)
	}

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
		job.Status = string(StateScheduled)
		if summary != nil {
			nextRun := s.calculateAdaptiveOrFixedNextRun(jobExec, job)
			job.NextRunAt = &nextRun
		} else {
			nextRun := s.calculateNextRun(job)
			job.NextRunAt = &nextRun
		}
	}

	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
	}

	s.metrics.IncrementCompleted()
	s.metrics.IncrementTotalExecutions()

	logFields := []infralogger.Field{
		infralogger.String("job_id", job.ID),
		infralogger.Int64("duration_ms", durationMs),
		infralogger.Any("next_run_at", job.NextRunAt),
	}
	if summary != nil {
		logFields = append(logFields,
			infralogger.Int("items_crawled", int(summary.PagesCrawled)),
			infralogger.Int("items_indexed", int(summary.ItemsExtracted)),
		)
	}
	s.logger.Info("Job completed successfully", logFields...)

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

	// Capture crawl metrics before updating execution (Crawler may be nil if factory.Create failed)
	summary := &logs.JobSummary{}
	if jobExec.Crawler != nil {
		summary = jobExec.Crawler.GetJobLogger().BuildSummary()
	}

	// Update execution record
	execution.Status = string(StateFailed)
	execution.CompletedAt = &now
	execution.DurationMs = &durationMs
	errMsg := execErr.Error()
	execution.ErrorMessage = &errMsg
	execution.Metadata = BuildExecutionMetadata(summary)

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
			infralogger.String("url", job.URL),
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

// getIntervalDuration converts job interval settings to a time.Duration.
func getIntervalDuration(job *domain.Job) time.Duration {
	if job.IntervalMinutes == nil {
		return searchWindowDefault // Default for one-time jobs
	}
	switch job.IntervalType {
	case "hours":
		return time.Duration(*job.IntervalMinutes) * time.Hour
	case "days":
		return time.Duration(*job.IntervalMinutes) * hoursPerDay * time.Hour
	default: // "minutes"
		return time.Duration(*job.IntervalMinutes) * time.Minute
	}
}

// calculateNextRun calculates the next run time based on interval configuration.
// Uses rhythm preservation when load balancing is enabled.
func (s *IntervalScheduler) calculateNextRun(job *domain.Job) time.Time {
	if job.IntervalMinutes == nil {
		return time.Time{}
	}

	interval := getIntervalDuration(job)

	// Use rhythm preservation when load balancing is enabled
	if s.bucketMap != nil {
		return s.bucketMap.CalculateNextRunPreserveRhythm(job.ID, interval)
	}

	// Fallback to original behavior
	return time.Now().Add(interval)
}

// calculateAdaptiveOrFixedNextRun calculates the next run time.
// If adaptive scheduling is enabled and hash data is available,
// uses content change detection. Otherwise falls back to the fixed interval.
func (s *IntervalScheduler) calculateAdaptiveOrFixedNextRun(
	jobExec *JobExecution,
	job *domain.Job,
) time.Time {
	if !job.AdaptiveScheduling {
		return s.calculateNextRun(job)
	}

	hashTracker := s.factory.GetHashTracker()
	if hashTracker == nil {
		return s.calculateNextRun(job)
	}

	hash := s.factory.GetStartURLHash(job.SourceID)
	if hash == "" {
		return s.calculateNextRun(job)
	}

	baseline := getIntervalDuration(job)

	state, changed, err := hashTracker.CompareAndUpdate(
		jobExec.Context, job.SourceID, hash, baseline,
	)
	if err != nil {
		s.logger.Warn(
			"Adaptive scheduling hash comparison failed, using fixed interval",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)

		return s.calculateNextRun(job)
	}

	s.logger.Info("Adaptive scheduling decision",
		infralogger.String("job_id", job.ID),
		infralogger.Bool("content_changed", changed),
		infralogger.Int("unchanged_count", state.UnchangedCount),
		infralogger.Duration("adaptive_interval", state.CurrentInterval),
		infralogger.Duration("baseline_interval", baseline),
	)

	return time.Now().Add(state.CurrentInterval)
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

// isExpectedStartError returns true for crawler start errors that are expected
// and should be logged at WARN instead of ERROR (e.g., "already visited", "Forbidden domain",
// context deadline exceeded).
func isExpectedStartError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "already visited") ||
		strings.Contains(msg, "Already visited") ||
		strings.Contains(msg, "Forbidden domain") ||
		strings.Contains(msg, "forbidden domain")
}
