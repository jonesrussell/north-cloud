package v2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/jonesrussell/north-cloud/crawler/internal/coordination"
	basedomain "github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/queue"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/observability"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/schedule"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler/v2/triggers"
	"github.com/jonesrussell/north-cloud/crawler/internal/worker"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

var (
	// ErrSchedulerNotRunning is returned when the scheduler is not running.
	ErrSchedulerNotRunning = errors.New("scheduler not running")

	// ErrSchedulerAlreadyRunning is returned when the scheduler is already running.
	ErrSchedulerAlreadyRunning = errors.New("scheduler already running")

	// ErrJobNotFound is returned when a job is not found.
	ErrJobNotFound = errors.New("job not found")

	// ErrInvalidJobState is returned when a job is in an invalid state for the operation.
	ErrInvalidJobState = errors.New("invalid job state for operation")
)

// JobExecutor is the interface for executing crawler jobs.
type JobExecutor interface {
	Execute(ctx context.Context, job *domain.JobV2) error
}

// JobRepository is the interface for job persistence.
type JobRepository interface {
	GetV2Job(ctx context.Context, id string) (*domain.JobV2, error)
	GetV2ReadyJobs(ctx context.Context, limit int) ([]*domain.JobV2, error)
	UpdateJobStatus(ctx context.Context, id, status string) error
	UpdateNextRunAt(ctx context.Context, id string, nextRunAt time.Time) error
	GetBaseJob(ctx context.Context, id string) (*basedomain.Job, error)
}

// Scheduler is the V2 scheduler orchestrator.
type Scheduler struct {
	config Config

	// Core components
	redisClient    *redis.Client
	jobRepo        JobRepository
	jobExecutor    JobExecutor
	workerPool     *worker.Pool
	producer       *queue.Producer
	consumer       *queue.Consumer
	leaderElection *coordination.LeaderElection
	logger         infralogger.Logger

	// Scheduling components
	cronScheduler *schedule.CronScheduler
	eventMatcher  *schedule.EventMatcher
	triggerRouter *triggers.Router

	// Observability
	metrics *observability.Metrics
	tracer  *observability.Tracer
	log     *observability.Logger

	// State
	mu       sync.RWMutex
	running  bool
	isLeader bool
	cancelFn context.CancelFunc
	wg       sync.WaitGroup
	draining bool
}

// NewScheduler creates a new V2 scheduler.
func NewScheduler(
	config Config,
	redisClient *redis.Client,
	jobRepo JobRepository,
	jobExecutor JobExecutor,
	logger infralogger.Logger,
	reg prometheus.Registerer,
) (*Scheduler, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	s := &Scheduler{
		config:      config,
		redisClient: redisClient,
		jobRepo:     jobRepo,
		jobExecutor: jobExecutor,
		logger:      logger,
		metrics:     observability.NewMetrics(reg),
		tracer:      observability.NewTracer(),
		log:         observability.NewLogger(logger),
	}

	// Initialize queue components
	streamsClient := queue.NewStreamsClientFromRedis(redisClient, config.StreamPrefix)
	s.producer = queue.NewProducer(streamsClient, queue.ProducerConfig{})

	consumer, consumerErr := queue.NewConsumer(streamsClient, queue.ConsumerConfig{
		ConsumerGroup: "scheduler",
		ConsumerID:    config.LeaderKey,
		BatchSize:     int64(config.QueueBatchSize),
	})
	if consumerErr != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", consumerErr)
	}
	s.consumer = consumer

	// Initialize worker pool
	poolConfig := worker.DefaultConfig()
	poolConfig.PoolSize = config.WorkerPoolSize
	poolConfig.JobTimeout = config.JobTimeout

	workerPool, poolErr := worker.NewPool(poolConfig, s.handleWorkerJob, logger)
	if poolErr != nil {
		return nil, fmt.Errorf("failed to create worker pool: %w", poolErr)
	}
	s.workerPool = workerPool

	// Initialize cron scheduler if enabled
	if config.EnableCronScheduling {
		s.cronScheduler = schedule.NewCronScheduler()
	}

	// Initialize event matcher and triggers if enabled
	if config.EnableEventTriggers {
		s.eventMatcher = schedule.NewEventMatcher()
		s.triggerRouter = triggers.NewRouter(
			triggers.RouterConfig{
				EnableWebhooks: true,
				EnablePubSub:   true,
			},
			s.eventMatcher,
			s.handleEventTrigger,
			redisClient,
		)
	}

	// Initialize leader election
	leaderElection, leaderErr := coordination.NewLeaderElection(
		redisClient,
		coordination.LeaderConfig{
			Key: config.LeaderKey,
			TTL: config.LeaderTTL,
		},
		logger,
	)
	if leaderErr != nil {
		return nil, fmt.Errorf("failed to create leader election: %w", leaderErr)
	}
	s.leaderElection = leaderElection

	return s, nil
}

// Start starts the scheduler.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return ErrSchedulerAlreadyRunning
	}

	// Create cancellable context
	schedulerCtx, cancel := context.WithCancel(ctx)
	s.cancelFn = cancel
	s.running = true
	s.mu.Unlock()

	// Start worker pool
	if err := s.workerPool.Start(); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}

	// Start cron scheduler if enabled
	if s.cronScheduler != nil {
		if err := s.cronScheduler.Start(schedulerCtx); err != nil {
			return fmt.Errorf("failed to start cron scheduler: %w", err)
		}
	}

	// Start trigger router if enabled
	if s.triggerRouter != nil {
		if err := s.triggerRouter.Start(schedulerCtx); err != nil {
			return fmt.Errorf("failed to start trigger router: %w", err)
		}
	}

	// Start leader election loop
	s.wg.Add(1)
	go s.leaderElectionLoop(schedulerCtx)

	// Start job polling loop
	s.wg.Add(1)
	go s.jobPollingLoop(schedulerCtx)

	// Start metrics collection loop
	s.wg.Add(1)
	go s.metricsCollectionLoop(schedulerCtx)

	s.log.SchedulerStarted(
		s.config.WorkerPoolSize,
		s.config.EnableCronScheduling,
		s.config.EnableEventTriggers,
	)

	return nil
}

// Stop stops the scheduler gracefully.
func (s *Scheduler) Stop(ctx context.Context) error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}

	s.draining = true
	s.mu.Unlock()

	// Get active job count for logging
	stats := s.workerPool.Stats()
	s.log.SchedulerDraining(stats.BusyWorkers)

	// Cancel context to signal shutdown
	if s.cancelFn != nil {
		s.cancelFn()
	}

	// Create drain context with timeout
	drainCtx, cancel := context.WithTimeout(ctx, s.config.DrainTimeout)
	defer cancel()

	// Stop worker pool (waits for active jobs)
	if err := s.workerPool.Stop(drainCtx); err != nil {
		s.log.Error("Error stopping worker pool", err)
	}

	// Stop cron scheduler
	if s.cronScheduler != nil {
		s.cronScheduler.Stop()
	}

	// Stop trigger router
	if s.triggerRouter != nil {
		if err := s.triggerRouter.Stop(); err != nil {
			s.log.Error("Error stopping trigger router", err)
		}
	}

	// Wait for goroutines
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.log.SchedulerStopped(true)
	case <-drainCtx.Done():
		s.log.SchedulerStopped(false)
	}

	s.mu.Lock()
	s.running = false
	s.draining = false
	s.mu.Unlock()

	return nil
}

// ScheduleJob schedules a job for execution.
func (s *Scheduler) ScheduleJob(ctx context.Context, job *domain.JobV2) error {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	if !running {
		return ErrSchedulerNotRunning
	}

	// Handle different schedule types
	switch job.ScheduleType {
	case domain.ScheduleTypeCron:
		return s.scheduleCronJob(ctx, job)
	case domain.ScheduleTypeInterval:
		return s.scheduleIntervalJob(ctx, job)
	case domain.ScheduleTypeImmediate:
		return s.scheduleImmediateJob(ctx, job)
	case domain.ScheduleTypeEvent:
		return s.scheduleEventJob(ctx, job)
	default:
		return fmt.Errorf("unsupported schedule type: %s", job.ScheduleType)
	}
}

// scheduleCronJob schedules a cron-based job.
func (s *Scheduler) scheduleCronJob(ctx context.Context, job *domain.JobV2) error {
	if s.cronScheduler == nil {
		return errors.New("cron scheduling not enabled")
	}

	if job.CronExpression == nil || *job.CronExpression == "" {
		return errors.New("cron expression required for cron jobs")
	}

	jobID := job.ID

	handler := func(cronCtx context.Context, jid string) error {
		return s.enqueueJob(cronCtx, job)
	}

	if err := s.cronScheduler.ScheduleJob(ctx, jobID, *job.CronExpression, handler); err != nil {
		return fmt.Errorf("failed to schedule cron job: %w", err)
	}

	s.metrics.RecordJobScheduled("cron", job.GetPriority().String())
	s.log.JobScheduled(ctx, jobID, "cron", job.GetPriority().String(), time.Time{})

	return nil
}

// scheduleIntervalJob schedules an interval-based job.
func (s *Scheduler) scheduleIntervalJob(ctx context.Context, job *domain.JobV2) error {
	// Get interval minutes (default to 0 if nil)
	intervalMinutes := 0
	if job.IntervalMinutes != nil {
		intervalMinutes = *job.IntervalMinutes
	}

	// Calculate next run time based on interval
	interval := schedule.IntervalConfig{
		Minutes: intervalMinutes,
		Type:    job.IntervalType,
	}

	if err := interval.Validate(); err != nil {
		return fmt.Errorf("invalid interval: %w", err)
	}

	nextRunAt := schedule.CalculateNextRunAtFromNow(interval)
	jobID := job.ID

	if err := s.jobRepo.UpdateNextRunAt(ctx, jobID, nextRunAt); err != nil {
		return fmt.Errorf("failed to update next run time: %w", err)
	}

	s.metrics.RecordJobScheduled("interval", job.GetPriority().String())
	s.log.JobScheduled(ctx, jobID, "interval", job.GetPriority().String(), nextRunAt)

	return nil
}

// scheduleImmediateJob schedules a job for immediate execution.
func (s *Scheduler) scheduleImmediateJob(ctx context.Context, job *domain.JobV2) error {
	if err := s.enqueueJob(ctx, job); err != nil {
		return fmt.Errorf("failed to enqueue immediate job: %w", err)
	}

	jobID := job.ID
	s.metrics.RecordJobScheduled("immediate", job.GetPriority().String())
	s.log.JobScheduled(ctx, jobID, "immediate", job.GetPriority().String(), time.Now())

	return nil
}

// scheduleEventJob registers a job for event-based triggering.
func (s *Scheduler) scheduleEventJob(ctx context.Context, job *domain.JobV2) error {
	if s.triggerRouter == nil {
		return errors.New("event triggers not enabled")
	}

	jobID := job.ID

	// Register webhook trigger if configured
	if job.TriggerWebhook != nil && *job.TriggerWebhook != "" {
		if err := s.triggerRouter.RegisterWebhookTrigger(jobID, *job.TriggerWebhook); err != nil {
			return fmt.Errorf("failed to register webhook trigger: %w", err)
		}
	}

	// Register channel trigger if configured
	if job.TriggerChannel != nil && *job.TriggerChannel != "" {
		if err := s.triggerRouter.RegisterChannelTrigger(ctx, jobID, *job.TriggerChannel); err != nil {
			return fmt.Errorf("failed to register channel trigger: %w", err)
		}
	}

	s.metrics.RecordJobScheduled("event", job.GetPriority().String())
	s.log.JobScheduled(ctx, jobID, "event", job.GetPriority().String(), time.Time{})

	return nil
}

// enqueueJob adds a job to the priority queue.
func (s *Scheduler) enqueueJob(ctx context.Context, job *domain.JobV2) error {
	priority := job.GetPriority()
	jobID := job.ID

	// Enqueue uses the base Job, not V2
	_, enqueueErr := s.producer.Enqueue(ctx, job.Job, priority, nil)
	if enqueueErr != nil {
		return fmt.Errorf("failed to enqueue job: %w", enqueueErr)
	}

	s.metrics.RecordEnqueue(priority.String())
	s.log.QueueEnqueued(ctx, jobID, priority.String(), priority.String())

	return nil
}

// handleWorkerJob is the callback for the worker pool to execute jobs.
// It receives a base domain.Job and must look up the V2 job.
func (s *Scheduler) handleWorkerJob(ctx context.Context, job *basedomain.Job) error {
	jobID := job.ID

	// Get the V2 job from repository
	v2Job, err := s.jobRepo.GetV2Job(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get V2 job %s: %w", jobID, err)
	}

	// Record job start
	s.metrics.RecordJobStarted()
	s.log.JobStarted(ctx, jobID, job.SourceID, job.URL)

	startTime := time.Now()

	// Execute the job
	execErr := s.jobExecutor.Execute(ctx, v2Job)

	duration := time.Since(startTime)

	// Record job completion
	s.metrics.RecordJobFinished()

	if execErr != nil {
		s.metrics.RecordJobExecuted("failed", job.SourceID, duration.Seconds())
		s.log.JobFailed(ctx, jobID, execErr, duration)
		return execErr
	}

	s.metrics.RecordJobExecuted("completed", job.SourceID, duration.Seconds())
	s.log.JobCompleted(ctx, jobID, duration, 0, 0)

	return nil
}

// handleEventTrigger handles an event trigger.
func (s *Scheduler) handleEventTrigger(ctx context.Context, jobID string, event schedule.Event) error {
	job, err := s.jobRepo.GetV2Job(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}

	s.metrics.RecordTriggerFired(string(event.Type), true)
	s.log.TriggerFired(ctx, string(event.Type), event.Pattern, 1)

	return s.enqueueJob(ctx, job)
}

// ForceRun queues a job for immediate execution (run now).
// Allowed for jobs in status scheduled, paused, or pending.
func (s *Scheduler) ForceRun(ctx context.Context, jobID string) error {
	job, err := s.jobRepo.GetV2Job(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to get job: %w", err)
	}
	status := job.Status
	switch status {
	case "running":
		return fmt.Errorf("%w: job is already running", ErrInvalidJobState)
	case "completed", "failed", "cancelled":
		return fmt.Errorf("%w: job in terminal state %q", ErrInvalidJobState, status)
	case "scheduled", "paused", "pending":
		// allowed
	default:
		return fmt.Errorf("%w: job status %q", ErrInvalidJobState, status)
	}
	return s.enqueueJob(ctx, job)
}

// leaderElectionLoop monitors leader election status.
func (s *Scheduler) leaderElectionLoop(ctx context.Context) {
	defer s.wg.Done()

	// Start the leader election
	s.leaderElection.Start(ctx)

	const leaderCheckDivisor = 2
	ticker := time.NewTicker(s.config.LeaderTTL / leaderCheckDivisor)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Stop leader election on shutdown
			if stopErr := s.leaderElection.Stop(context.Background()); stopErr != nil {
				s.log.Error("Error stopping leader election", stopErr)
			}
			s.isLeader = false
			s.metrics.SetIsLeader(false)
			return

		case <-ticker.C:
			wasLeader := s.isLeader
			isLeader := s.leaderElection.IsLeader()

			s.mu.Lock()
			s.isLeader = isLeader
			s.mu.Unlock()

			s.metrics.RecordLeaderElectionAttempt(isLeader)
			s.metrics.SetIsLeader(isLeader)

			if isLeader && !wasLeader {
				s.log.LeaderElected(s.config.LeaderKey)
			} else if !isLeader && wasLeader {
				s.log.LeaderLost(s.config.LeaderKey)
			}
		}
	}
}

// jobPollingLoop polls for ready jobs and queues them.
func (s *Scheduler) jobPollingLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.mu.RLock()
			isLeader := s.isLeader
			draining := s.draining
			s.mu.RUnlock()

			// Only poll if we're the leader and not draining
			if !isLeader || draining {
				continue
			}

			s.pollAndQueueJobs(ctx)
		}
	}
}

// pollAndQueueJobs polls for ready jobs and queues them.
func (s *Scheduler) pollAndQueueJobs(ctx context.Context) {
	jobs, err := s.jobRepo.GetV2ReadyJobs(ctx, s.config.QueueBatchSize)
	if err != nil {
		s.log.Error("Failed to get ready jobs", err)
		return
	}

	for _, job := range jobs {
		jobID := job.ID

		if queueErr := s.enqueueJob(ctx, job); queueErr != nil {
			s.log.Error("Failed to queue job", queueErr,
				infralogger.String("job_id", jobID),
			)
			continue
		}

		// Update status to scheduled
		if statusErr := s.jobRepo.UpdateJobStatus(ctx, jobID, "scheduled"); statusErr != nil {
			s.log.Error("Failed to update job status", statusErr,
				infralogger.String("job_id", jobID),
			)
		}
	}
}

// metricsCollectionLoop periodically collects metrics.
func (s *Scheduler) metricsCollectionLoop(ctx context.Context) {
	defer s.wg.Done()

	const metricsInterval = 10 * time.Second
	ticker := time.NewTicker(metricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			s.collectMetrics()
		}
	}
}

// collectMetrics collects current metrics.
func (s *Scheduler) collectMetrics() {
	// Worker pool metrics
	stats := s.workerPool.Stats()
	s.metrics.SetWorkerPoolMetrics(stats.PoolSize, stats.BusyWorkers, stats.IdleWorkers)
}

// Health returns the scheduler health status.
func (s *Scheduler) Health() SchedulerHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()

	health := SchedulerHealth{
		Running:  s.running,
		IsLeader: s.isLeader,
		Draining: s.draining,
	}

	if s.workerPool != nil {
		stats := s.workerPool.Stats()
		health.WorkerPool = WorkerPoolHealth{
			TotalWorkers: stats.PoolSize,
			BusyWorkers:  stats.BusyWorkers,
			IdleWorkers:  stats.IdleWorkers,
			Status:       string(stats.State),
		}
	}

	if s.cronScheduler != nil {
		health.CronEnabled = true
		health.CronRunning = s.cronScheduler.IsRunning()
		health.CronJobCount = s.cronScheduler.ScheduledJobCount()
	}

	if s.triggerRouter != nil {
		health.TriggersEnabled = true
		routerHealth := s.triggerRouter.Health()
		health.TriggerRouter = TriggerRouterHealth{
			Running:            routerHealth.Running,
			WebhooksEnabled:    routerHealth.WebhooksEnabled,
			PubSubEnabled:      routerHealth.PubSubEnabled,
			PubSubRunning:      routerHealth.PubSubRunning,
			RegisteredWebhooks: len(routerHealth.RegisteredWebhooks),
			RegisteredChannels: len(routerHealth.RegisteredChannels),
		}
	}

	return health
}

// SchedulerHealth contains health information about the scheduler.
type SchedulerHealth struct {
	Running         bool
	IsLeader        bool
	Draining        bool
	WorkerPool      WorkerPoolHealth
	CronEnabled     bool
	CronRunning     bool
	CronJobCount    int
	TriggersEnabled bool
	TriggerRouter   TriggerRouterHealth
}

// WorkerPoolHealth contains health information about the worker pool.
type WorkerPoolHealth struct {
	TotalWorkers int
	BusyWorkers  int
	IdleWorkers  int
	Status       string
}

// TriggerRouterHealth contains health information about the trigger router.
type TriggerRouterHealth struct {
	Running            bool
	WebhooksEnabled    bool
	PubSubEnabled      bool
	PubSubRunning      bool
	RegisteredWebhooks int
	RegisteredChannels int
}

// GetWebhookHandler returns the webhook HTTP handler.
func (s *Scheduler) GetWebhookHandler() (*triggers.WebhookHandler, error) {
	if s.triggerRouter == nil {
		return nil, errors.New("event triggers not enabled")
	}
	return s.triggerRouter.WebhookHandler()
}
