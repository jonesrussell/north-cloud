// Package job provides database-backed job scheduler implementation.
package job

import (
	"context"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
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
