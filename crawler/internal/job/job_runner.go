package job

import (
	"context"
	"errors"
	"time"
)

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
