package scheduler

import (
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

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

// recoverOrphanedJobs runs once at startup to recover jobs left in "running" state
// from a prior container lifecycle. At startup, activeJobs is empty, so any job
// marked "running" in the DB is guaranteed to be orphaned.
func (s *IntervalScheduler) recoverOrphanedJobs() {
	orphanedJobs, err := s.executionRepo.GetOrphanedRunningJobs(s.ctx)
	if err != nil {
		s.logger.Error("Failed to check for orphaned jobs at startup", infralogger.Error(err))
		return
	}

	if len(orphanedJobs) == 0 {
		return
	}

	s.logger.Warn("Recovering orphaned jobs from prior container lifecycle",
		infralogger.Int("count", len(orphanedJobs)),
	)

	for _, job := range orphanedJobs {
		s.logger.Warn("Recovering orphaned job",
			infralogger.String("job_id", job.ID),
			infralogger.String("url", job.URL),
		)

		s.failStuckExecution(job.ID)

		now := time.Now()
		errMsg := "recovered: job orphaned by container restart"
		s.resetJobAfterFailure(job, &errMsg, &now)
		s.metrics.IncrementFailed()
		s.metrics.IncrementTotalExecutions()
	}
}

// recoverStuckJobs periodically checks for and recovers jobs stuck in "running" state.
// This is a safety net for cases where the execution timeout or panic recovery failed.
func (s *IntervalScheduler) recoverStuckJobs() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.stuckJobCheckInterval)
	defer ticker.Stop()

	s.logger.Info("Stuck job recovery started",
		infralogger.Duration("interval", s.stuckJobCheckInterval),
		infralogger.Duration("threshold", s.executionTimeout),
	)

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Stuck job recovery stopping")
			return
		case <-ticker.C:
			s.checkForStuckJobs()
		}
	}
}

// checkForStuckJobs queries for stuck jobs and resets them.
func (s *IntervalScheduler) checkForStuckJobs() {
	stuckJobs, err := s.executionRepo.GetStuckJobs(s.ctx, s.executionTimeout)
	if err != nil {
		s.logger.Error("Failed to check for stuck jobs", infralogger.Error(err))
		return
	}

	for _, job := range stuckJobs {
		s.resetStuckJob(job)
	}
}

// resetStuckJob recovers a single job that is stuck in "running" state.
func (s *IntervalScheduler) resetStuckJob(job *domain.Job) {
	// Skip if this job is actively running in this scheduler instance.
	// The execution timeout context cancellation will handle it.
	s.activeJobsMu.RLock()
	_, isActive := s.activeJobs[job.ID]
	s.activeJobsMu.RUnlock()

	if isActive {
		return
	}

	s.logger.Warn("Recovering stuck job",
		infralogger.String("job_id", job.ID),
		infralogger.String("source_id", job.SourceID),
		infralogger.String("url", job.URL),
	)

	// Mark the stuck execution as failed
	s.failStuckExecution(job.ID)

	// Reset the job itself
	now := time.Now()
	errMsg := "recovered: job exceeded maximum execution time"
	s.resetJobAfterFailure(job, &errMsg, &now)
	s.metrics.IncrementFailed()
	s.metrics.IncrementTotalExecutions()
}

// failStuckExecution marks the latest running execution for a job as failed.
func (s *IntervalScheduler) failStuckExecution(jobID string) {
	latestExec, err := s.executionRepo.GetLatestByJobID(s.ctx, jobID)
	if err != nil {
		s.logger.Error("Failed to get latest execution for stuck job",
			infralogger.String("job_id", jobID),
			infralogger.Error(err),
		)
		return
	}

	if latestExec.Status != string(StateRunning) {
		return
	}

	now := time.Now()
	errMsg := "recovered: job exceeded maximum execution time"
	latestExec.Status = string(StateFailed)
	latestExec.CompletedAt = &now
	latestExec.ErrorMessage = &errMsg

	if !latestExec.StartedAt.IsZero() {
		durationMs := now.Sub(latestExec.StartedAt).Milliseconds()
		latestExec.DurationMs = &durationMs
	}

	if updateErr := s.executionRepo.Update(s.ctx, latestExec); updateErr != nil {
		s.logger.Error("Failed to update stuck execution",
			infralogger.String("job_id", jobID),
			infralogger.Error(updateErr),
		)
	}
}
