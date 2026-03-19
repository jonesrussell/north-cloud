// Package scheduler provides interval-based job scheduling with distributed locking.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	defaultCheckInterval         = 10 * time.Second
	defaultLockDuration          = 5 * time.Minute
	defaultMetricsInterval       = 30 * time.Second
	defaultExecutionTimeout      = 1 * time.Hour
	defaultStuckJobCheckInterval = 2 * time.Minute
	hoursPerDay                  = 24
	exponentialBackoffBase       = 2
)

// JobExecution represents an active job execution with its context.
type JobExecution struct {
	Job       *domain.Job
	Execution *domain.JobExecution
	Context   context.Context
	Cancel    context.CancelFunc
	StartTime time.Time
	Crawler   crawler.Interface // Per-job isolated crawler instance
}

// IntervalScheduler replaces the cron-based scheduler with interval-based scheduling.
type IntervalScheduler struct {
	logger        infralogger.Logger
	repo          database.JobRepositoryInterface
	executionRepo database.ExecutionRepositoryInterface
	factory       crawler.FactoryInterface

	// Scheduler control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Job execution
	activeJobs   map[string]*JobExecution
	activeJobsMu sync.RWMutex

	// Configuration
	checkInterval          time.Duration
	lockDuration           time.Duration
	metricsInterval        time.Duration
	staleLockCheckInterval time.Duration
	executionTimeout       time.Duration
	stuckJobCheckInterval  time.Duration

	// Metrics
	metrics *SchedulerMetrics

	// SSE integration (optional)
	ssePublisher *SSEPublisher

	// Log service for job log capture (optional)
	logService logs.Service

	// Load balancing
	bucketMap *BucketMap

	// Scraper config for leadership_scrape jobs
	scraperConfig *ScraperConfig
}

// NewIntervalScheduler creates a new interval-based scheduler.
func NewIntervalScheduler(
	log infralogger.Logger,
	repo database.JobRepositoryInterface,
	executionRepo database.ExecutionRepositoryInterface,
	crawlerFactory crawler.FactoryInterface,
	opts ...SchedulerOption,
) *IntervalScheduler {
	ctx, cancel := context.WithCancel(context.Background())

	s := &IntervalScheduler{
		logger:                 log,
		repo:                   repo,
		executionRepo:          executionRepo,
		factory:                crawlerFactory,
		ctx:                    ctx,
		cancel:                 cancel,
		activeJobs:             make(map[string]*JobExecution),
		checkInterval:          defaultCheckInterval,
		lockDuration:           defaultLockDuration,
		metricsInterval:        defaultMetricsInterval,
		staleLockCheckInterval: 1 * time.Minute,
		executionTimeout:       defaultExecutionTimeout,
		stuckJobCheckInterval:  defaultStuckJobCheckInterval,
		metrics:                &SchedulerMetrics{},
		bucketMap:              NewBucketMap(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the interval scheduler.
func (s *IntervalScheduler) Start(ctx context.Context) error {
	s.logger.Info("Starting interval scheduler",
		infralogger.Duration("check_interval", s.checkInterval),
		infralogger.Duration("lock_duration", s.lockDuration),
		infralogger.Duration("metrics_interval", s.metricsInterval),
	)

	// Rebuild bucket map from existing scheduled jobs
	if err := s.rebuildBucketMap(); err != nil {
		return fmt.Errorf("failed to rebuild bucket map: %w", err)
	}

	// Recover jobs orphaned by a prior container restart
	s.recoverOrphanedJobs()

	// Start job poller
	s.wg.Add(1)
	go s.pollJobs()

	// Start metrics collector
	s.wg.Add(1)
	go s.collectMetrics()

	// Start stale lock cleaner
	s.wg.Add(1)
	go s.cleanStaleLocks()

	// Start stuck job recovery
	s.wg.Add(1)
	go s.recoverStuckJobs()

	s.logger.Info("Interval scheduler started successfully")
	return nil
}

// Stop stops the interval scheduler gracefully.
func (s *IntervalScheduler) Stop() error {
	s.logger.Info("Stopping interval scheduler")

	// Cancel context to stop all goroutines
	s.cancel()

	// Cancel all active jobs
	s.cancelAllActiveJobs()

	// Wait for all goroutines to finish
	s.wg.Wait()

	s.logger.Info("Interval scheduler stopped")
	return nil
}

// pollJobs continuously checks for jobs that need to be executed.
func (s *IntervalScheduler) pollJobs() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	s.logger.Info("Job poller started", infralogger.Duration("interval", s.checkInterval))

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info("Job poller stopping")
			return
		case <-ticker.C:
			s.checkAndExecuteJobs()
		}
	}
}

// checkAndExecuteJobs finds jobs ready to run and executes them.
func (s *IntervalScheduler) checkAndExecuteJobs() {
	s.metrics.UpdateLastCheck()

	// Get jobs ready to run
	jobs, err := s.repo.GetJobsReadyToRun(s.ctx)
	if err != nil {
		s.logger.Error("Failed to get jobs ready to run", infralogger.Error(err))
		return
	}

	if len(jobs) > 0 {
		s.logger.Debug("Found jobs ready to run", infralogger.Int("count", len(jobs)))
	}

	for _, job := range jobs {
		// Try to acquire lock
		acquired, lockErr := s.acquireJobLock(job)
		if lockErr != nil {
			s.logger.Error("Failed to acquire lock",
				infralogger.String("job_id", job.ID),
				infralogger.Error(lockErr),
			)
			continue
		}

		if !acquired {
			s.logger.Debug("Job already locked by another instance", infralogger.String("job_id", job.ID))
			continue
		}

		// Execute job
		s.executeJob(job)
	}
}

// acquireJobLock attempts to acquire a distributed lock for a job.
func (s *IntervalScheduler) acquireJobLock(job *domain.Job) (bool, error) {
	lockToken := uuid.New()
	now := time.Now()

	acquired, err := s.repo.AcquireLock(s.ctx, job.ID, lockToken, now, s.lockDuration)
	if err != nil {
		return false, fmt.Errorf("lock acquisition failed: %w", err)
	}

	if acquired {
		s.logger.Debug("Lock acquired",
			infralogger.String("job_id", job.ID),
		)
		// Update job with lock token for tracking
		job.LockToken = new(string)
		*job.LockToken = lockToken.String()
		job.LockAcquiredAt = &now
	}

	return acquired, nil
}

// releaseLock releases the job lock.
func (s *IntervalScheduler) releaseLock(job *domain.Job) {
	if err := s.repo.ReleaseLock(s.ctx, job.ID); err != nil {
		s.logger.Error("Failed to release lock",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
	} else {
		s.logger.Debug("Lock released", infralogger.String("job_id", job.ID))
	}
}

// cancelAllActiveJobs cancels all currently running jobs.
func (s *IntervalScheduler) cancelAllActiveJobs() {
	s.activeJobsMu.Lock()
	defer s.activeJobsMu.Unlock()

	for id, jobExec := range s.activeJobs {
		s.logger.Info("Cancelling active job", infralogger.String("job_id", id))
		jobExec.Cancel()
	}
}

// CancelJob cancels a running job by its ID.
// This is called externally (e.g., via API) to cancel a job mid-execution.
func (s *IntervalScheduler) CancelJob(jobID string) error {
	s.activeJobsMu.RLock()
	jobExec, exists := s.activeJobs[jobID]
	s.activeJobsMu.RUnlock()

	if !exists {
		return fmt.Errorf("job not currently running: %s", jobID)
	}

	s.logger.Info("Cancelling job execution", infralogger.String("job_id", jobID))
	jobExec.Cancel()

	s.metrics.IncrementCancelled()

	return nil
}

// SetSSEPublisher sets the SSE publisher for real-time event streaming.
// This is optional - if not set, no SSE events will be published.
//
// IMPORTANT: This method must be called before Start() to avoid data races.
// The ssePublisher field is not synchronized because it's intended to be
// set once during initialization and never changed during the scheduler's lifetime.
func (s *IntervalScheduler) SetSSEPublisher(publisher *SSEPublisher) {
	s.ssePublisher = publisher
}

// SetLogService sets the log service for job log capture.
// This is optional - if not set, no logs will be captured during job execution.
//
// IMPORTANT: This method must be called before Start() to avoid data races.
func (s *IntervalScheduler) SetLogService(logService logs.Service) {
	s.logService = logService
}

// publishJobStatus publishes a job status event if SSE is enabled.
func (s *IntervalScheduler) publishJobStatus(ctx context.Context, job *domain.Job) {
	if s.ssePublisher != nil {
		s.ssePublisher.PublishJobStatus(ctx, job, nil)
	}
}

// publishJobCompleted publishes a job completion event if SSE is enabled.
func (s *IntervalScheduler) publishJobCompleted(ctx context.Context, job *domain.Job, execution *domain.JobExecution) {
	if s.ssePublisher != nil {
		s.ssePublisher.PublishJobCompleted(ctx, job, execution)
	}
}
