package job

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// reloadJobs reloads all jobs from the database and updates schedules.
func (s *DBScheduler) reloadJobs(ctx context.Context) error {
	s.logger.Info("Reloading jobs from database")

	// Get all jobs
	jobs, err := s.repo.List(ctx, "", maxJobsListLimit, 0)
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
		// Use scheduler's lifecycle context for cron-triggered jobs
		s.executeJob(s.ctx, jobID)
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
func (s *DBScheduler) processPendingImmediateJobs(ctx context.Context) {
	jobs, err := s.repo.List(ctx, "pending", pendingJobsListLimit, 0)
	if err != nil {
		s.logger.Error("Failed to list pending jobs on startup", "error", err)
		return
	}

	immediateCount := 0
	for _, job := range jobs {
		if !job.ScheduleEnabled {
			immediateCount++
			s.logger.Info("Processing pending immediate job on startup", "job_id", job.ID, "url", job.URL)
			s.executeJob(ctx, job.ID)
		}
	}
	if immediateCount > 0 {
		s.logger.Info("Processed immediate jobs on startup", "count", immediateCount)
	}
}

// processImmediateJobs checks for and executes immediate jobs (schedule_enabled: false).
func (s *DBScheduler) processImmediateJobs() {
	defer s.wg.Done()

	ctx := s.ctx // Use scheduler's lifecycle context for goroutine
	s.logger.Info("Starting immediate job processor", "check_interval", checkInterval)
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Immediate job processor stopped")
			return
		case <-ticker.C:
			// Get pending jobs with schedule_enabled: false
			jobs, err := s.repo.List(ctx, "pending", pendingJobsListLimit, 0)
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
					s.executeJob(ctx, job.ID)
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

	ctx := s.ctx // Use scheduler's lifecycle context for goroutine
	ticker := time.NewTicker(reloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Periodic reload stopped")
			return
		case <-ticker.C:
			if err := s.reloadJobs(ctx); err != nil {
				s.logger.Error("Failed to reload jobs", "error", err)
			}
		}
	}
}
