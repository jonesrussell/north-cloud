// Package job provides database-backed job scheduler implementation.
package job

import (
	"context"
	"sync"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/robfig/cron/v3"
)

// DBScheduler implements a database-backed job scheduler.
type DBScheduler struct {
	logger       infralogger.Logger
	repo         *database.JobRepository
	crawler      crawler.Interface
	cron         *cron.Cron
	cronParser   cron.Parser
	activeJobs   map[string]context.CancelFunc
	activeJobsMu sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

// NewDBScheduler creates a new database-backed scheduler.
func NewDBScheduler(
	log infralogger.Logger,
	repo *database.JobRepository,
	crawlerInstance crawler.Interface,
) *DBScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	// Use standard 5-field cron parser (minute hour day month weekday)
	// This is the default, but we're being explicit
	cronParser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	c := cron.New(cron.WithParser(cronParser), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	return &DBScheduler{
		logger:     log,
		repo:       repo,
		crawler:    crawlerInstance,
		cron:       c,
		cronParser: cronParser,
		activeJobs: make(map[string]context.CancelFunc),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the database scheduler.
func (s *DBScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting database scheduler")

	// Start cron scheduler
	s.cron.Start()
	s.logger.Info("Cron scheduler started")

	// NOTE: DBScheduler is legacy. The interval-based scheduler (IntervalScheduler) is now the recommended scheduler.
	// These methods were in cron_manager.go which was removed. DBScheduler should not be used in new code.
	s.logger.Warn("DBScheduler is deprecated. Use IntervalScheduler instead.")
	s.logger.Info("Scheduled jobs loaded", infralogger.Int("count", 0))

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
		s.logger.Info("Cancelling active job", infralogger.String("job_id", id))
		cancel()
	}
	s.activeJobsMu.Unlock()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.logger.Info("Database scheduler stopped")
	return nil
}
