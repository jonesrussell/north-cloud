package bootstrap

import (
	"context"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	crawlerconfigtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	crawlerevents "github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
	"github.com/north-cloud/infrastructure/sse"
)

// ServiceComponents holds all initialized services and handlers.
type ServiceComponents struct {
	// Handlers
	JobsHandler            *api.JobsHandler
	DiscoveredLinksHandler *api.DiscoveredLinksHandler
	LogsHandler            *api.LogsHandler
	LogsV2Handler          *api.LogsStreamV2Handler

	// Services
	Scheduler  *scheduler.IntervalScheduler
	LogService logs.Service

	// SSE components
	SSEBroker    sse.Broker
	SSEHandler   *api.SSEHandler
	SSEPublisher *scheduler.SSEPublisher
}

// LogServiceResult holds the log service and optional Redis writer.
type LogServiceResult struct {
	Service     logs.Service
	Config      *logs.Config
	RedisWriter *logs.RedisStreamWriter // nil if Redis not enabled/available
}

// SetupServices initializes all service components.
func SetupServices(
	deps *CommandDeps,
	storage *StorageComponents,
	db *DatabaseComponents,
) (*ServiceComponents, error) {
	// Create handlers
	jobsHandler := api.NewJobsHandler(db.JobRepo, db.ExecutionRepo)
	discoveredLinksHandler := api.NewDiscoveredLinksHandler(db.DiscoveredLinkRepo, db.JobRepo)

	// Setup SSE
	sseBroker, sseHandler, ssePublisher := setupSSE(deps)

	// Create log service with optional Redis persistence
	logResult := setupLogService(deps, sseBroker, db.ExecutionRepo)

	// Create logs handler
	logsHandler := api.NewLogsHandler(logResult.Service, db.ExecutionRepo, sseBroker, deps.Logger)

	// Create v2 logs handler (if Redis is available)
	var logsV2Handler *api.LogsStreamV2Handler
	if logResult.RedisWriter != nil {
		logsV2Handler = api.NewLogsStreamV2Handler(logResult.RedisWriter, deps.Logger)
		logsHandler.SetStreamV2Available(true)
		deps.Logger.Info("V2 log streaming endpoint enabled (Redis-backed)")
	}

	// Set logger for observability
	jobsHandler.SetLogger(deps.Logger)
	discoveredLinksHandler.SetLogger(deps.Logger)

	// Create and start scheduler
	intervalScheduler := createAndStartScheduler(deps, storage, db.JobRepo, db.ExecutionRepo, db.DB)
	if intervalScheduler != nil {
		jobsHandler.SetScheduler(intervalScheduler)
		discoveredLinksHandler.SetScheduler(intervalScheduler)
		// Connect SSE publisher to scheduler
		intervalScheduler.SetSSEPublisher(ssePublisher)
		// Connect log service to scheduler for job log capture
		intervalScheduler.SetLogService(logResult.Service)
	}

	return &ServiceComponents{
		JobsHandler:            jobsHandler,
		DiscoveredLinksHandler: discoveredLinksHandler,
		LogsHandler:            logsHandler,
		LogsV2Handler:          logsV2Handler,
		Scheduler:              intervalScheduler,
		LogService:             logResult.Service,
		SSEBroker:              sseBroker,
		SSEHandler:             sseHandler,
		SSEPublisher:           ssePublisher,
	}, nil
}

// setupSSE creates SSE broker, handler, and publisher.
func setupSSE(deps *CommandDeps) (sseBroker sse.Broker, sseHandler *api.SSEHandler, ssePublisher *scheduler.SSEPublisher) {
	sseBroker = sse.NewBroker(deps.Logger)
	if startErr := sseBroker.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start SSE broker", infralogger.Error(startErr))
	} else {
		deps.Logger.Info("SSE broker started successfully")
	}
	sseHandler = api.NewSSEHandler(sseBroker, deps.Logger)
	ssePublisher = scheduler.NewSSEPublisher(sseBroker, deps.Logger)
	return sseBroker, sseHandler, ssePublisher
}

// setupLogService creates the log service with optional Redis persistence.
func setupLogService(
	deps *CommandDeps,
	sseBroker sse.Broker,
	executionRepo database.ExecutionRepositoryInterface,
) LogServiceResult {
	configLogsCfg := deps.Config.GetLogsConfig()
	logsCfg := &logs.Config{
		Enabled:           configLogsCfg.Enabled,
		BufferSize:        configLogsCfg.BufferSize,
		SSEEnabled:        configLogsCfg.SSEEnabled,
		ArchiveEnabled:    configLogsCfg.ArchiveEnabled,
		RetentionDays:     configLogsCfg.RetentionDays,
		MinLevel:          configLogsCfg.MinLevel,
		MinioBucket:       configLogsCfg.MinioBucket,
		MilestoneInterval: configLogsCfg.MilestoneInterval,
		RedisEnabled:      configLogsCfg.RedisEnabled,
		RedisKeyPrefix:    configLogsCfg.RedisKeyPrefix,
		RedisTTLSeconds:   configLogsCfg.RedisTTLSeconds,
	}

	logArchiver, archiveErr := logs.NewArchiver(
		deps.Config.GetMinIOConfig(),
		logsCfg.MinioBucket,
		deps.Logger,
	)
	if archiveErr != nil {
		deps.Logger.Warn("Failed to create log archiver, log archiving disabled", infralogger.Error(archiveErr))
	}
	logsPublisher := logs.NewPublisher(sseBroker, deps.Logger, logsCfg.SSEEnabled)

	// Create optional Redis writer for log persistence
	var serviceOpts []logs.ServiceOption
	var redisWriter *logs.RedisStreamWriter
	if logsCfg.RedisEnabled {
		redisClient, redisErr := CreateRedisClient(deps.Config.GetRedisConfig())
		if redisErr != nil {
			if !errors.Is(redisErr, ErrRedisDisabled) {
				deps.Logger.Warn("Redis not available for job logs, falling back to in-memory",
					infralogger.Error(redisErr))
			}
		} else {
			redisWriter = logs.NewRedisStreamWriter(
				redisClient,
				logsCfg.RedisKeyPrefix,
				logsCfg.RedisTTLSeconds,
			)
			serviceOpts = append(serviceOpts, logs.WithRedisWriter(redisWriter))
			deps.Logger.Info("Job logs Redis persistence enabled",
				infralogger.String("prefix", logsCfg.RedisKeyPrefix))
		}
	}

	logService := logs.NewService(logsCfg, logArchiver, logsPublisher, executionRepo, deps.Logger, serviceOpts...)
	return LogServiceResult{
		Service:     logService,
		Config:      logsCfg,
		RedisWriter: redisWriter,
	}
}

// createAndStartScheduler creates and starts the interval-based scheduler.
// Returns nil if scheduler cannot be created or started.
// Note: The scheduler manages its own context lifecycle internally.
func createAndStartScheduler(
	deps *CommandDeps,
	storage *StorageComponents,
	jobRepo *database.JobRepository,
	executionRepo *database.ExecutionRepository,
	db *sqlx.DB,
) *scheduler.IntervalScheduler {
	// Create crawler for job execution
	crawlerInstance, err := createCrawlerForJobs(deps, storage, db)
	if err != nil {
		deps.Logger.Warn("Failed to create crawler for jobs, scheduler disabled", infralogger.Error(err))
		return nil
	}

	// Create interval scheduler with default options
	intervalScheduler := scheduler.NewIntervalScheduler(
		deps.Logger,
		jobRepo,
		executionRepo,
		crawlerInstance,
	)

	// Start the scheduler
	if startErr := intervalScheduler.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start interval scheduler", infralogger.Error(startErr))
		return nil
	}

	deps.Logger.Info("Interval scheduler started successfully")
	return intervalScheduler
}

// createCrawlerForJobs creates a crawler instance for job execution.
func createCrawlerForJobs(
	deps *CommandDeps,
	storage *StorageComponents,
	db *sqlx.DB,
) (crawler.Interface, error) {
	// Create event bus
	bus := crawlerevents.NewEventBus(deps.Logger)

	// Get crawler config
	crawlerCfg := deps.Config.GetCrawlerConfig()

	// Load source manager
	sourceManager, err := loadSourceManager(deps)
	if err != nil {
		return nil, err
	}

	// Create crawler
	return createCrawler(deps, bus, crawlerCfg, storage, sourceManager, db)
}

// loadSourceManager creates a sources manager with lazy loading.
// Sources will be loaded from the API when ValidateSource is first called for a job.
func loadSourceManager(deps *CommandDeps) (sources.Interface, error) {
	sourceManager, err := sources.NewSources(deps.Config, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create sources manager: %w", err)
	}
	return sourceManager, nil
}

// createCrawler creates a crawler instance with the given parameters.
func createCrawler(
	deps *CommandDeps,
	bus *crawlerevents.EventBus,
	crawlerCfg *crawlerconfigtypes.Config,
	storage *StorageComponents,
	sourceManager sources.Interface,
	db *sqlx.DB,
) (crawler.Interface, error) {
	pipelineClient := pipeline.NewClient(deps.Config.GetPipelineURL(), "crawler")

	crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
		Logger:         deps.Logger,
		Bus:            bus,
		IndexManager:   storage.IndexManager,
		Sources:        sourceManager,
		Config:         crawlerCfg,
		Storage:        storage.Storage,
		FullConfig:     deps.Config,
		DB:             db,
		PipelineClient: pipelineClient,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create crawler: %w", err)
	}
	return crawlerResult.Crawler, nil
}
