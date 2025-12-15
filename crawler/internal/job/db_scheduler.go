// Package job provides database-backed job scheduler implementation.
package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jonesrussell/gocrawl/internal/crawler"
	"github.com/jonesrussell/gocrawl/internal/database"
	"github.com/jonesrussell/gocrawl/internal/domain"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/robfig/cron/v3"
)

const (
	// checkInterval is how often to check for new jobs
	checkInterval = 10 * time.Second
	// reloadInterval is how often to reload job schedules
	reloadInterval = 5 * time.Minute
)

// DBScheduler implements a database-backed job scheduler.
type DBScheduler struct {
	logger         logger.Interface
	repo           *database.JobRepository
	crawler        crawler.Interface
	cron           *cron.Cron
	activeJobs     map[string]context.CancelFunc
	activeJobsMu   sync.RWMutex
	scheduledJobs  map[string]cron.EntryID
	scheduledJobsMu sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

// NewDBScheduler creates a new database-backed scheduler.
func NewDBScheduler(log logger.Interface, repo *database.JobRepository, crawlerInstance crawler.Interface) *DBScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &DBScheduler{
		logger:        log,
		repo:          repo,
		crawler:       crawlerInstance,
		cron:          cron.New(),
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

	// Load initial jobs
	if err := s.reloadJobs(); err != nil {
		s.logger.Error("Failed to load initial jobs", "error", err)
	}

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
	jobs, err := s.repo.List(s.ctx, "", 1000, 0)
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
			if err := s.scheduleJob(job); err != nil {
				s.logger.Error("Failed to schedule job", "job_id", job.ID, "error", err)
			}
		}
	}

	return nil
}

// scheduleJob schedules a job using cron.
func (s *DBScheduler) scheduleJob(job *domain.Job) error {
	if job.ScheduleTime == nil || *job.ScheduleTime == "" {
		return fmt.Errorf("job has no schedule time")
	}

	scheduleTime := *job.ScheduleTime
	s.logger.Info("Scheduling job", "job_id", job.ID, "schedule", scheduleTime)

	// Parse cron expression
	entryID, err := s.cron.AddFunc(scheduleTime, func() {
		s.executeJob(job.ID)
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.scheduledJobsMu.Lock()
	s.scheduledJobs[job.ID] = entryID
	s.scheduledJobsMu.Unlock()

	return nil
}

// processImmediateJobs checks for and executes immediate jobs (schedule_enabled: false).
func (s *DBScheduler) processImmediateJobs() {
	defer s.wg.Done()

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Immediate job processor stopped")
			return
		case <-ticker.C:
			// Get pending jobs with schedule_enabled: false
			jobs, err := s.repo.List(s.ctx, "pending", 100, 0)
			if err != nil {
				s.logger.Error("Failed to list pending jobs", "error", err)
				continue
			}

			for _, job := range jobs {
				if !job.ScheduleEnabled {
					s.logger.Info("Found immediate job", "job_id", job.ID)
					s.executeJob(job.ID)
				}
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

	if err := s.repo.Update(s.ctx, job); err != nil {
		s.logger.Error("Failed to update job status", "job_id", jobID, "error", err)
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

		s.logger.Info("Executing job", "job_id", jobID, "url", job.URL)

		// Execute crawler
		if err := s.crawler.Start(jobCtx, job.URL); err != nil {
			s.logger.Error("Failed to start crawler", "job_id", jobID, "error", err)
			s.updateJobStatus(jobID, "failed", err)
			return
		}

		// Wait for crawler to complete
		if err := s.crawler.Wait(); err != nil {
			s.logger.Error("Crawler failed", "job_id", jobID, "error", err)
			s.updateJobStatus(jobID, "failed", err)
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
