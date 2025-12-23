// Package job provides database-backed job scheduler implementation.
package job

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/robfig/cron/v3"
)

const (
	// checkInterval is how often to check for new jobs
	checkInterval = 10 * time.Second
	// reloadInterval is how often to reload job schedules
	reloadInterval = 5 * time.Minute
	// maxJobsListLimit is the maximum number of jobs to list when reloading
	maxJobsListLimit = 1000
	// pendingJobsListLimit is the limit for listing pending jobs
	pendingJobsListLimit = 100
)

// DBScheduler implements a database-backed job scheduler.
type DBScheduler struct {
	logger          logger.Interface
	repo            *database.JobRepository
	crawler         crawler.Interface
	cron            *cron.Cron
	cronParser      cron.Parser
	activeJobs      map[string]context.CancelFunc
	activeJobsMu    sync.RWMutex
	scheduledJobs   map[string]cron.EntryID
	scheduledJobsMu sync.RWMutex
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
}

// NewDBScheduler creates a new database-backed scheduler.
func NewDBScheduler(
	log logger.Interface,
	repo *database.JobRepository,
	crawlerInstance crawler.Interface,
) *DBScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	// Use standard 5-field cron parser (minute hour day month weekday)
	// This is the default, but we're being explicit
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	c := cron.New(cron.WithParser(cronParser), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	return &DBScheduler{
		logger:        log,
		repo:          repo,
		crawler:       crawlerInstance,
		cron:          c,
		cronParser:    cronParser,
		activeJobs:    make(map[string]context.CancelFunc),
		scheduledJobs: make(map[string]cron.EntryID),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start starts the database scheduler.
func (s *DBScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting database scheduler")

	// Start cron scheduler
	s.cron.Start()
	s.logger.Info("Cron scheduler started")

	// Load initial jobs
	if err := s.reloadJobs(); err != nil {
		s.logger.Error("Failed to load initial jobs", "error", err)
	}

	// Log number of scheduled jobs
	s.scheduledJobsMu.RLock()
	scheduledCount := len(s.scheduledJobs)
	s.scheduledJobsMu.RUnlock()
	s.logger.Info("Scheduled jobs loaded", "count", scheduledCount)

	// Process any immediate jobs that are already pending
	s.processPendingImmediateJobs()

	// Start immediate job processor
	s.wg.Add(1)
	go s.processImmediateJobs()

	// Start periodic job reloader
	s.wg.Add(1)
	go s.periodicReload()

	return nil
}

// Stop stops the database scheduler.
func (s *DBScheduler) Stop() error {
	s.logger.Info("Stopping database scheduler")

	// Cancel context to stop all goroutines
	s.cancel()

	// Stop cron scheduler
	cronCtx := s.cron.Stop()
	<-cronCtx.Done()

	// Cancel all active jobs
	s.activeJobsMu.Lock()
	for id, cancel := range s.activeJobs {
		s.logger.Info("Cancelling active job", "job_id", id)
		cancel()
	}
	s.activeJobsMu.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.logger.Info("Database scheduler stopped")
	return nil
}

// reloadJobs reloads all jobs from the database and updates schedules.
func (s *DBScheduler) reloadJobs() error {
	s.logger.Info("Reloading jobs from database")

	// Get all jobs
	jobs, err := s.repo.List(s.ctx, "", maxJobsListLimit, 0)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	s.logger.Info("Found jobs", "count", len(jobs))

	// Remove old scheduled jobs
	s.scheduledJobsMu.Lock()
	for id, entryID := range s.scheduledJobs {
		s.cron.Remove(entryID)
		delete(s.scheduledJobs, id)
	}
	s.scheduledJobsMu.Unlock()

	// Add scheduled jobs
	for _, job := range jobs {
		if job.ScheduleEnabled && job.ScheduleTime != nil && *job.ScheduleTime != "" {
			if scheduleErr := s.scheduleJob(job); scheduleErr != nil {
				s.logger.Error("Failed to schedule job", "job_id", job.ID, "error", scheduleErr)
			}
		}
	}

	return nil
}

// ReloadJob reloads a single job by ID and adds it to the scheduler if it's scheduled.
// This is useful when a job is created or updated and needs to be immediately scheduled.
func (s *DBScheduler) ReloadJob(jobID string) error {
	// Get the job from database
	job, err := s.repo.GetByID(s.ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	s.logger.Debug("Reloading job",
		"job_id", jobID,
		"schedule_enabled", job.ScheduleEnabled,
		"schedule_time", func() string {
			if job.ScheduleTime != nil {
				return *job.ScheduleTime
			}
			return "nil"
		}(),
		"status", job.Status)

	// Remove existing scheduled job if it exists
	s.scheduledJobsMu.Lock()
	if entryID, exists := s.scheduledJobs[jobID]; exists {
		s.logger.Debug("Removing existing scheduled job", "job_id", jobID, "entry_id", entryID)
		s.cron.Remove(entryID)
		delete(s.scheduledJobs, jobID)
	}
	s.scheduledJobsMu.Unlock()

	// Add the job to scheduler if it's scheduled
	if job.ScheduleEnabled && job.ScheduleTime != nil && *job.ScheduleTime != "" {
		if scheduleErr := s.scheduleJob(job); scheduleErr != nil {
			s.logger.Error("Failed to schedule job during reload",
				"job_id", jobID,
				"schedule", *job.ScheduleTime,
				"error", scheduleErr)
			return fmt.Errorf("failed to schedule job: %w", scheduleErr)
		}
		s.logger.Info("Job reloaded and scheduled successfully",
			"job_id", jobID,
			"schedule", *job.ScheduleTime,
			"status", job.Status)
	} else {
		s.logger.Debug("Job is not scheduled, skipping scheduler registration",
			"job_id", jobID,
			"schedule_enabled", job.ScheduleEnabled,
			"has_schedule_time", job.ScheduleTime != nil && *job.ScheduleTime != "")
	}

	return nil
}

// scheduleJob schedules a job using cron.
func (s *DBScheduler) scheduleJob(job *domain.Job) error {
	if job.ScheduleTime == nil || *job.ScheduleTime == "" {
		return errors.New("job has no schedule time")
	}

	scheduleTime := *job.ScheduleTime
	now := time.Now()
	s.logger.Info("Scheduling job",
		"job_id", job.ID,
		"schedule", scheduleTime,
		"schedule_enabled", job.ScheduleEnabled,
		"current_time", now.Format("15:04:05"))

	// Parse the cron schedule to get next run time for logging
	// Use the same parser that the cron instance uses
	schedule, err := s.cronParser.Parse(scheduleTime)
	if err != nil {
		s.logger.Error("Failed to parse cron expression for validation",
			"job_id", job.ID,
			"schedule", scheduleTime,
			"error", err)
		return fmt.Errorf("failed to parse cron expression: %w", err)
	}

	nextRun := schedule.Next(now)
	timeUntilNext := time.Until(nextRun)
	willRunToday := nextRun.Day() == now.Day() && nextRun.Month() == now.Month() && nextRun.Year() == now.Year()

	s.logger.Info("Cron schedule parsed successfully",
		"job_id", job.ID,
		"schedule", scheduleTime,
		"next_run", nextRun.Format("2006-01-02 15:04:05"),
		"next_run_relative", timeUntilNext.String(),
		"will_run_today", willRunToday)

	// Parse cron expression - robfig/cron uses standard 5-field format by default
	// Format: minute hour day month weekday
	// Capture job.ID in a local variable to avoid closure issues
	jobID := job.ID
	entryID, err := s.cron.AddFunc(scheduleTime, func() {
		triggerTime := time.Now()
		s.logger.Info("Cron triggered for job",
			"job_id", jobID,
			"schedule", scheduleTime,
			"triggered_at", triggerTime.Format("2006-01-02 15:04:05"))
		s.executeJob(jobID)
	})

	if err != nil {
		s.logger.Error("Failed to add cron job to scheduler",
			"job_id", job.ID,
			"schedule", scheduleTime,
			"error", err)
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.scheduledJobsMu.Lock()
	s.scheduledJobs[job.ID] = entryID
	s.scheduledJobsMu.Unlock()

	// Log successful scheduling with next run info
	s.logger.Info("Job successfully scheduled",
		"job_id", job.ID,
		"schedule", scheduleTime,
		"entry_id", entryID,
		"next_run", nextRun.Format("2006-01-02 15:04:05"),
		"time_until_next", timeUntilNext.String())

	// If the next run is very soon (within 2 minutes) and today,
	// the cron library should handle it, but we log it for visibility
	if willRunToday && timeUntilNext > 0 && timeUntilNext < 2*time.Minute {
		s.logger.Info("Job scheduled to run very soon",
			"job_id", job.ID,
			"next_run", nextRun.Format("15:04:05"),
			"time_until", timeUntilNext.String())
	}

	// Check if the next run time is in the past (shouldn't happen, but handle it)
	if timeUntilNext < 0 {
		s.logger.Warn("Job next run time is in the past - cron may have calculated incorrectly",
			"job_id", job.ID,
			"next_run", nextRun.Format("2006-01-02 15:04:05"),
			"current_time", now.Format("2006-01-02 15:04:05"),
			"time_diff", timeUntilNext.String())
		// The cron library should still schedule it for the next occurrence,
		// but we log a warning
	}

	return nil
}

// processPendingImmediateJobs processes any immediate jobs that are already pending.
func (s *DBScheduler) processPendingImmediateJobs() {
	jobs, err := s.repo.List(s.ctx, "pending", pendingJobsListLimit, 0)
	if err != nil {
		s.logger.Error("Failed to list pending jobs on startup", "error", err)
		return
	}

	immediateCount := 0
	for _, job := range jobs {
		if !job.ScheduleEnabled {
			immediateCount++
			s.logger.Info("Processing pending immediate job on startup", "job_id", job.ID, "url", job.URL)
			s.executeJob(job.ID)
		}
	}
	if immediateCount > 0 {
		s.logger.Info("Processed immediate jobs on startup", "count", immediateCount)
	}
}

// processImmediateJobs checks for and executes immediate jobs (schedule_enabled: false).
func (s *DBScheduler) processImmediateJobs() {
	defer s.wg.Done()

	s.logger.Info("Starting immediate job processor", "check_interval", checkInterval)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Immediate job processor stopped")
			return
		case <-ticker.C:
			// Get pending jobs with schedule_enabled: false
			jobs, err := s.repo.List(s.ctx, "pending", pendingJobsListLimit, 0)
			if err != nil {
				s.logger.Error("Failed to list pending jobs", "error", err)
				continue
			}

			s.logger.Debug("Checking for immediate jobs", "pending_count", len(jobs))
			immediateCount := 0
			for _, job := range jobs {
				if !job.ScheduleEnabled {
					immediateCount++
					s.logger.Info("Found immediate job", "job_id", job.ID, "url", job.URL)
					s.executeJob(job.ID)
				}
			}
			if immediateCount > 0 {
				s.logger.Info("Processing immediate jobs", "count", immediateCount)
			}
		}
	}
}

// periodicReload periodically reloads jobs from the database.
func (s *DBScheduler) periodicReload() {
	defer s.wg.Done()

	ticker := time.NewTicker(reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Periodic reload stopped")
			return
		case <-ticker.C:
			if err := s.reloadJobs(); err != nil {
				s.logger.Error("Failed to reload jobs", "error", err)
			}
		}
	}
}

// executeJob executes a job by ID.
func (s *DBScheduler) executeJob(jobID string) {
	// Check if job is already running
	s.activeJobsMu.RLock()
	if _, exists := s.activeJobs[jobID]; exists {
		s.logger.Warn("Job already running", "job_id", jobID)
		s.activeJobsMu.RUnlock()
		return
	}
	s.activeJobsMu.RUnlock()

	// Get job from database
	job, err := s.repo.GetByID(s.ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get job", "job_id", jobID, "error", err)
		return
	}

	// Update job status to processing
	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now

	if updateErr := s.repo.Update(s.ctx, job); updateErr != nil {
		s.logger.Error("Failed to update job status", "job_id", jobID, "error", updateErr)
		return
	}

	// Create job context
	jobCtx, cancel := context.WithCancel(s.ctx)

	// Track active job
	s.activeJobsMu.Lock()
	s.activeJobs[jobID] = cancel
	s.activeJobsMu.Unlock()

	// Execute job in goroutine
	go func() {
		defer func() {
			// Remove from active jobs
			s.activeJobsMu.Lock()
			delete(s.activeJobs, jobID)
			s.activeJobsMu.Unlock()
		}()

		// Validate source name is present
		if job.SourceName == nil || *job.SourceName == "" {
			s.logger.Error("Job missing source name", "job_id", jobID)
			s.updateJobStatus(jobID, "failed", errors.New("job missing required source_name"))
			return
		}

		sourceName := *job.SourceName
		s.logger.Info("Executing job", "job_id", jobID, "source_name", sourceName, "url", job.URL)

		// Execute crawler - Start expects a source name, not a URL
		if startErr := s.crawler.Start(jobCtx, sourceName); startErr != nil {
			s.logger.Error("Failed to start crawler", "job_id", jobID, "error", startErr)
			s.updateJobStatus(jobID, "failed", startErr)
			return
		}

		// Wait for crawler to complete
		if waitErr := s.crawler.Wait(); waitErr != nil {
			s.logger.Error("Crawler failed", "job_id", jobID, "error", waitErr)
			s.updateJobStatus(jobID, "failed", waitErr)
			return
		}

		s.logger.Info("Job completed successfully", "job_id", jobID)
		s.updateJobStatus(jobID, "completed", nil)
	}()
}

// updateJobStatus updates the job status in the database.
func (s *DBScheduler) updateJobStatus(jobID, status string, err error) {
	job, getErr := s.repo.GetByID(s.ctx, jobID)
	if getErr != nil {
		s.logger.Error("Failed to get job for status update", "job_id", jobID, "error", getErr)
		return
	}

	now := time.Now()
	job.Status = status

	if status == "completed" || status == "failed" {
		job.CompletedAt = &now
	}

	if err != nil {
		errMsg := err.Error()
		job.ErrorMessage = &errMsg
	}

	if updateErr := s.repo.Update(s.ctx, job); updateErr != nil {
		s.logger.Error("Failed to update job status", "job_id", jobID, "error", updateErr)
	}
}
