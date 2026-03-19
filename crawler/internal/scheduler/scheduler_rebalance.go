package scheduler

import (
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

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

// GetDistribution returns the current schedule distribution.
// Returns nil if load balancing is disabled.
func (s *IntervalScheduler) GetDistribution() *Distribution {
	if s.bucketMap == nil {
		return nil
	}
	dist := s.bucketMap.GetDistribution(hoursPerDay)
	return &dist
}

// ScheduleNewJob schedules a new job with load-balanced placement.
// This should be called when a job is created via API.
func (s *IntervalScheduler) ScheduleNewJob(job *domain.Job) error {
	if job.IntervalMinutes == nil || !job.ScheduleEnabled {
		// One-time job - no load balancing needed
		return nil
	}

	interval := getIntervalDuration(job)

	if s.bucketMap != nil {
		nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
		job.NextRunAt = &nextRun
		job.Status = string(StateScheduled)
	} else {
		// Fallback to original behavior
		nextRun := time.Now().Add(interval)
		job.NextRunAt = &nextRun
		job.Status = string(StateScheduled)
	}

	return s.repo.Update(s.ctx, job)
}

// HandleJobDeleted removes a job from the bucket map when deleted.
func (s *IntervalScheduler) HandleJobDeleted(jobID string) {
	if s.bucketMap != nil {
		s.bucketMap.RemoveJob(jobID)
	}
}

// HandleIntervalChange re-places a job when its interval changes.
func (s *IntervalScheduler) HandleIntervalChange(job *domain.Job) error {
	if s.bucketMap == nil {
		return nil
	}

	interval := getIntervalDuration(job)
	s.bucketMap.RemoveJob(job.ID)
	nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
	job.NextRunAt = &nextRun

	return s.repo.Update(s.ctx, job)
}

// HandleResume re-places a job when it resumes from pause.
func (s *IntervalScheduler) HandleResume(job *domain.Job) error {
	if s.bucketMap == nil {
		return nil
	}

	interval := getIntervalDuration(job)
	s.bucketMap.RemoveJob(job.ID)
	nextRun := s.bucketMap.PlaceNewJob(job.ID, interval)
	job.NextRunAt = &nextRun

	return s.repo.Update(s.ctx, job)
}

// FullRebalance redistributes all scheduled jobs for optimal load balancing.
// Returns the result of the rebalance operation.
func (s *IntervalScheduler) FullRebalance() (*RebalanceResult, error) {
	if s.bucketMap == nil {
		return nil, errors.New("load balancing is disabled")
	}

	jobs, err := s.repo.GetScheduledJobs(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	result := &RebalanceResult{
		Moved:   make([]Reassignment, 0, len(jobs)),
		Skipped: make([]SkippedJob, 0),
	}

	// Sort jobs by interval (longest first) for better placement
	sortJobsByInterval(jobs)

	// Clear the bucket map and re-place all jobs
	s.bucketMap.Clear()

	for _, job := range jobs {
		oldTime := job.NextRunAt
		reason, canMove := s.bucketMap.CanMoveJob(job.ID, job.Status, job.NextRunAt)

		if !canMove {
			result.Skipped = append(result.Skipped, SkippedJob{
				JobID:  job.ID,
				Reason: reason,
			})
			// Re-add at original position
			if oldTime != nil {
				s.bucketMap.AddJob(job.ID, SlotKey(*oldTime))
			}
			continue
		}

		interval := getIntervalDuration(job)
		newTime := s.bucketMap.PlaceNewJob(job.ID, interval)
		job.NextRunAt = &newTime

		if updateErr := s.repo.Update(s.ctx, job); updateErr != nil {
			s.logger.Error("Failed to update job during rebalance",
				infralogger.String("job_id", job.ID),
				infralogger.Error(updateErr),
			)
			continue
		}

		if oldTime != nil {
			result.Moved = append(result.Moved, Reassignment{
				JobID:   job.ID,
				OldTime: *oldTime,
				NewTime: newTime,
			})
		}
	}

	dist := s.bucketMap.GetDistribution(hoursPerDay)
	result.NewDistributionScore = dist.DistributionScore

	s.logger.Info("Rebalance completed",
		infralogger.Int("moved", len(result.Moved)),
		infralogger.Int("skipped", len(result.Skipped)),
		infralogger.Float64("score", result.NewDistributionScore),
	)

	return result, nil
}

// PreviewRebalance shows what a full rebalance would do without making changes.
func (s *IntervalScheduler) PreviewRebalance() (*RebalanceResult, error) {
	if s.bucketMap == nil {
		return nil, errors.New("load balancing is disabled")
	}

	jobs, err := s.repo.GetScheduledJobs(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled jobs: %w", err)
	}

	// Create a temporary bucket map for preview
	tempBucketMap := NewBucketMap()

	result := &RebalanceResult{
		Moved:   make([]Reassignment, 0, len(jobs)),
		Skipped: make([]SkippedJob, 0),
	}

	// Sort jobs by interval (longest first)
	sortJobsByInterval(jobs)

	for _, job := range jobs {
		oldTime := job.NextRunAt
		reason, canMove := s.bucketMap.CanMoveJob(job.ID, job.Status, job.NextRunAt)

		if !canMove {
			result.Skipped = append(result.Skipped, SkippedJob{
				JobID:  job.ID,
				Reason: reason,
			})
			if oldTime != nil {
				tempBucketMap.AddJob(job.ID, SlotKey(*oldTime))
			}
			continue
		}

		interval := getIntervalDuration(job)
		newTime := tempBucketMap.PlaceNewJob(job.ID, interval)

		if oldTime != nil {
			result.Moved = append(result.Moved, Reassignment{
				JobID:   job.ID,
				OldTime: *oldTime,
				NewTime: newTime,
			})
		}
	}

	dist := tempBucketMap.GetDistribution(hoursPerDay)
	result.NewDistributionScore = dist.DistributionScore

	return result, nil
}

// sortJobsByInterval sorts jobs by interval duration (longest first).
func sortJobsByInterval(jobs []*domain.Job) {
	for i := 1; i < len(jobs); i++ {
		for j := i; j > 0 && getIntervalDuration(jobs[j]) > getIntervalDuration(jobs[j-1]); j-- {
			jobs[j], jobs[j-1] = jobs[j-1], jobs[j]
		}
	}
}
