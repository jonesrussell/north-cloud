package job

import (
	"context"
	"errors"
	"time"

	infralogger "github.com/north-cloud/infrastructure/logger"
)

// executeJob executes a job by ID.
//
//nolint:unused // Part of deprecated DBScheduler API, kept for backward compatibility
func (s *DBScheduler) executeJob(ctx context.Context, jobID string) {
	// Check if job is already running
	s.activeJobsMu.RLock()
	if _, exists := s.activeJobs[jobID]; exists {
		s.logger.Warn("Job already running", infralogger.String("job_id", jobID))
		s.activeJobsMu.RUnlock()
		return
	}
	s.activeJobsMu.RUnlock()

	// Get job from database
	job, err := s.repo.GetByID(ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get job",
			infralogger.String("job_id", jobID),
			infralogger.Error(err),
		)
		return
	}

	// Update job status to processing
	now := time.Now()
	job.Status = "processing"
	job.StartedAt = &now

	if updateErr := s.repo.Update(ctx, job); updateErr != nil {
		s.logger.Error("Failed to update job status",
			infralogger.String("job_id", jobID),
			infralogger.Error(updateErr),
		)
		return
	}

	// Create job context derived from passed context
	// Also ensure it's cancelled when scheduler stops by listening to s.ctx
	jobCtx, cancel := context.WithCancel(ctx)

	// Cancel job context when scheduler stops
	go func() {
		select {
		case <-s.ctx.Done():
			cancel()
		case <-jobCtx.Done():
			// Job context already cancelled, nothing to do
		}
	}()

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

		// Validate source ID is present
		if job.SourceID == "" {
			s.logger.Error("Job missing source ID", infralogger.String("job_id", jobID))
			s.updateJobStatus(jobCtx, jobID, "failed", errors.New("job missing required source_id"))
			return
		}

		s.logger.Info("Executing job",
			infralogger.String("job_id", jobID),
			infralogger.String("source_id", job.SourceID),
			infralogger.String("url", job.URL),
		)
		// Execute crawler - Start expects a source ID
		if startErr := s.crawler.Start(jobCtx, job.SourceID); startErr != nil {
			s.logger.Error("Failed to start crawler",
				infralogger.String("job_id", jobID),
				infralogger.Error(startErr),
			)
			s.updateJobStatus(jobCtx, jobID, "failed", startErr)
			return
		}

		// Wait for crawler to complete
		if waitErr := s.crawler.Wait(); waitErr != nil {
			s.logger.Error("Crawler failed",
				infralogger.String("job_id", jobID),
				infralogger.Error(waitErr),
			)
			s.updateJobStatus(jobCtx, jobID, "failed", waitErr)
			return
		}

		s.logger.Info("Job completed successfully", infralogger.String("job_id", jobID))
		s.updateJobStatus(jobCtx, jobID, "completed", nil)
	}()
}

// updateJobStatus updates the job status in the database.
//
//nolint:unused // Part of deprecated DBScheduler API, kept for backward compatibility
func (s *DBScheduler) updateJobStatus(ctx context.Context, jobID, status string, err error) {
	job, getErr := s.repo.GetByID(ctx, jobID)
	if getErr != nil {
		s.logger.Error("Failed to get job for status update",
			infralogger.String("job_id", jobID),
			infralogger.Error(getErr),
		)
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

	if updateErr := s.repo.Update(ctx, job); updateErr != nil {
		s.logger.Error("Failed to update job status",
			infralogger.String("job_id", jobID),
			infralogger.Error(updateErr),
		)
	}
}
