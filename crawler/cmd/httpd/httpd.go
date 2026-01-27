// Package httpd implements the HTTP server for the crawler service.
package httpd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	crawlerconfigtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
	"github.com/north-cloud/infrastructure/sse"
)

// === Types ===

// CommandDeps holds common dependencies for the HTTP server.
type CommandDeps struct {
	Logger infralogger.Logger
	Config config.Interface
}

// StorageResult holds both storage interface and index manager.
type StorageResult struct {
	Storage      types.Interface
	IndexManager types.IndexManager
}

// JobsSchedulerResult holds the results from setupJobsAndScheduler.
type JobsSchedulerResult struct {
	JobsHandler            *api.JobsHandler
	DiscoveredLinksHandler *api.DiscoveredLinksHandler
	LogsHandler            *api.LogsHandler
	Scheduler              *scheduler.IntervalScheduler
	DB                     *sqlx.DB
	ExecutionRepo          *database.ExecutionRepository
	SSEBroker              sse.Broker
	SSEHandler             *api.SSEHandler
}

// === Constants ===

const (
	signalChannelBufferSize = 1
	defaultShutdownTimeout  = 30 * time.Second
)

// === Errors ===

var (
	// errLoggerRequired is returned when CommandDeps.Logger is nil
	errLoggerRequired = errors.New("logger is required")
	// errConfigRequired is returned when CommandDeps.Config is nil
	errConfigRequired = errors.New("config is required")
)

// === Main Entry Point ===

// Start starts the HTTP server and runs until interrupted.
// It handles graceful shutdown on SIGINT or SIGTERM signals.
func Start() error {
	// Phase 0: Start profiling servers (if enabled)
	profiling.StartPprofServer()

	// Start Pyroscope continuous profiling (if enabled)
	pyroscopeProfiler, err := profiling.StartPyroscope("crawler")
	if err != nil {
		return fmt.Errorf("failed to start Pyroscope profiler: %w", err)
	}
	if pyroscopeProfiler != nil {
		defer func() {
			if stopErr := pyroscopeProfiler.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to stop Pyroscope profiler: %v\n", stopErr)
			}
		}()
	}

	// Phase 1: Initialize dependencies
	deps, err := newCommandDeps()
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	// Phase 3: Setup storage
	storageResult, err := createStorage(deps.Config, deps.Logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Phase 4: Setup jobs handler and scheduler
	jsResult, err := setupJobsAndScheduler(deps, storageResult)
	if err != nil {
		return fmt.Errorf("failed to setup jobs and scheduler: %w", err)
	}
	defer jsResult.DB.Close()

	// Phase 5: Start HTTP server
	server, errChan := startHTTPServer(
		deps, jsResult.JobsHandler, jsResult.DiscoveredLinksHandler,
		jsResult.LogsHandler, jsResult.ExecutionRepo, jsResult.SSEHandler,
	)

	// Phase 6: Run server until interrupted
	return runServerUntilInterrupt(deps.Logger, server, jsResult.Scheduler, jsResult.SSEBroker, errChan)
}

// === Dependency Setup ===

// newCommandDeps creates CommandDeps by loading config and creating logger.
func newCommandDeps() (*CommandDeps, error) {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Create logger from config
	log, err := createLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	// Add service name to all log entries
	log = log.With(infralogger.String("service", "crawler"))

	deps := &CommandDeps{
		Logger: log,
		Config: cfg,
	}

	if validateErr := deps.validate(); validateErr != nil {
		return nil, fmt.Errorf("validate deps: %w", validateErr)
	}

	return deps, nil
}

// loadConfig loads configuration from the config package.
func loadConfig() (config.Interface, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	return config.Load(configPath)
}

// createLogger creates a logger instance from configuration using infrastructure logger.
func createLogger(cfg config.Interface) (infralogger.Logger, error) {
	loggingCfg := cfg.GetLoggingConfig()

	logLevel := normalizeLogLevel(loggingCfg.Level)
	if logLevel == "" {
		logLevel = "info"
	}

	// Determine if we're in development mode
	appEnv := loggingCfg.Env
	if appEnv == "" {
		appEnv = "production"
	}
	isDev := appEnv == "development"
	appDebug := loggingCfg.Debug

	// Override log level if APP_DEBUG is set
	if appDebug {
		logLevel = "debug"
	}

	// Determine encoding based on environment
	encoding := loggingCfg.Format
	if encoding == "" {
		if isDev {
			encoding = "console"
		} else {
			encoding = "json"
		}
	}

	return infralogger.New(infralogger.Config{
		Level:       logLevel,
		Format:      encoding,
		Development: isDev || appDebug,
	})
}

// normalizeLogLevel normalizes log level string.
func normalizeLogLevel(level string) string {
	if level == "" {
		return "info"
	}
	return strings.ToLower(level)
}

// validate ensures all required dependencies are present.
func (d *CommandDeps) validate() error {
	if d.Logger == nil {
		return errLoggerRequired
	}
	if d.Config == nil {
		return errConfigRequired
	}
	return nil
}

// === Storage Setup ===

// createStorageClient creates an Elasticsearch client with the given config and logger.
func createStorageClient(cfg config.Interface, log infralogger.Logger) (*es.Client, error) {
	clientResult, err := storage.NewClient(storage.ClientParams{
		Config: cfg,
		Logger: log,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}
	return clientResult.Client, nil
}

// createStorage creates both storage client and storage instance in one call.
func createStorage(cfg config.Interface, log infralogger.Logger) (*StorageResult, error) {
	// Create storage client
	client, err := createStorageClient(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}

	// Create storage
	storageResult, err := storage.NewStorage(storage.StorageParams{
		Config: cfg,
		Logger: log,
		Client: client,
	})
	if err != nil {
		return nil, fmt.Errorf("create storage: %w", err)
	}

	return &StorageResult{
		Storage:      storageResult.Storage,
		IndexManager: storageResult.IndexManager,
	}, nil
}

// === Crawler Setup ===

// createCrawlerForJobs creates a crawler instance for job execution.
func createCrawlerForJobs(
	deps *CommandDeps,
	storageResult *StorageResult,
	db *sqlx.DB,
) (crawler.Interface, error) {
	// Create event bus
	bus := events.NewEventBus(deps.Logger)

	// Get crawler config
	crawlerCfg := deps.Config.GetCrawlerConfig()

	// Load source manager
	sourceManager, err := loadSourceManager(deps)
	if err != nil {
		return nil, err
	}

	// Create crawler
	return createCrawler(deps, bus, crawlerCfg, storageResult, sourceManager, db)
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
	bus *events.EventBus,
	crawlerCfg *crawlerconfigtypes.Config,
	storageResult *StorageResult,
	sourceManager sources.Interface,
	db *sqlx.DB,
) (crawler.Interface, error) {
	crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
		Logger:       deps.Logger,
		Bus:          bus,
		IndexManager: storageResult.IndexManager,
		Sources:      sourceManager,
		Config:       crawlerCfg,
		Storage:      storageResult.Storage,
		FullConfig:   deps.Config,
		DB:           db,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create crawler: %w", err)
	}
	return crawlerResult.Crawler, nil
}

// === Database & Scheduler Setup ===

// setupJobsAndScheduler initializes the jobs handler and scheduler.
// Returns JobsSchedulerResult containing all components and an error.
// Database connection is required - the crawler cannot operate without it.
func setupJobsAndScheduler(
	deps *CommandDeps,
	storageResult *StorageResult,
) (*JobsSchedulerResult, error) {
	// Convert config to database config (DRY improvement)
	dbConfig := databaseConfigFromInterface(deps.Config.GetDatabaseConfig())

	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Create repositories
	jobRepo := database.NewJobRepository(db)
	executionRepo := database.NewExecutionRepository(db)

	// Create jobs handler with both repositories
	jobsHandler := api.NewJobsHandler(jobRepo, executionRepo)

	// Create discovered links handler
	discoveredLinkRepo := database.NewDiscoveredLinkRepository(db)
	discoveredLinksHandler := api.NewDiscoveredLinksHandler(discoveredLinkRepo, jobRepo)

	// Create SSE broker for real-time events
	sseBroker := sse.NewBroker(deps.Logger)
	if startErr := sseBroker.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start SSE broker", infralogger.Error(startErr))
		// Continue without SSE - it's optional
	} else {
		deps.Logger.Info("SSE broker started successfully")
	}

	// Create SSE handler
	sseHandler := api.NewSSEHandler(sseBroker, deps.Logger)

	// Create SSE publisher for scheduler
	ssePublisher := scheduler.NewSSEPublisher(sseBroker, deps.Logger)

	// Create log service components
	logsCfg := logs.DefaultConfig()
	logArchiver, archiveErr := logs.NewArchiver(
		deps.Config.GetMinIOConfig(),
		logsCfg.MinioBucket,
		deps.Logger,
	)
	if archiveErr != nil {
		deps.Logger.Warn("Failed to create log archiver, log archiving disabled", infralogger.Error(archiveErr))
	}
	logsPublisher := logs.NewPublisher(sseBroker, deps.Logger, logsCfg.SSEEnabled)
	logService := logs.NewService(logsCfg, logArchiver, logsPublisher, executionRepo, deps.Logger)

	// Create logs handler
	logsHandler := api.NewLogsHandler(logService, executionRepo, sseBroker, deps.Logger)

	// Create and start scheduler
	intervalScheduler := createAndStartScheduler(deps, storageResult, jobRepo, executionRepo, db)
	if intervalScheduler != nil {
		jobsHandler.SetScheduler(intervalScheduler)
		discoveredLinksHandler.SetScheduler(intervalScheduler)
		// Connect SSE publisher to scheduler
		intervalScheduler.SetSSEPublisher(ssePublisher)
	}

	return &JobsSchedulerResult{
		JobsHandler:            jobsHandler,
		DiscoveredLinksHandler: discoveredLinksHandler,
		LogsHandler:            logsHandler,
		Scheduler:              intervalScheduler,
		DB:                     db,
		ExecutionRepo:          executionRepo,
		SSEBroker:              sseBroker,
		SSEHandler:             sseHandler,
	}, nil
}

// databaseConfigFromInterface converts config database config to database.Config.
// This eliminates the DRY violation of repeated field mapping.
func databaseConfigFromInterface(cfg *dbconfig.Config) database.Config {
	return database.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		DBName:   cfg.DBName,
		SSLMode:  cfg.SSLMode,
	}
}

// createAndStartScheduler creates and starts the interval-based scheduler.
// Returns nil if scheduler cannot be created or started.
// Note: The scheduler manages its own context lifecycle internally.
func createAndStartScheduler(
	deps *CommandDeps,
	storageResult *StorageResult,
	jobRepo *database.JobRepository,
	executionRepo *database.ExecutionRepository,
	db *sqlx.DB,
) *scheduler.IntervalScheduler {
	// Create crawler for job execution
	crawlerInstance, err := createCrawlerForJobs(deps, storageResult, db)
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

// === Server Setup ===

// startHTTPServer creates and starts the HTTP server.
// Returns the server and an error channel for server errors.
func startHTTPServer(
	deps *CommandDeps,
	jobsHandler *api.JobsHandler,
	discoveredLinksHandler *api.DiscoveredLinksHandler,
	logsHandler *api.LogsHandler,
	executionRepo *database.ExecutionRepository,
	sseHandler *api.SSEHandler,
) (server *infragin.Server, errChan <-chan error) {
	// Use the logger that already has the service field attached
	// Create server using the new infrastructure gin package
	server = api.NewServer(deps.Config, jobsHandler, discoveredLinksHandler, logsHandler, executionRepo, deps.Logger, sseHandler)

	// Start server asynchronously
	deps.Logger.Info("Starting HTTP server", infralogger.String("addr", deps.Config.GetServerConfig().Address))
	errChan = server.StartAsync()

	return
}

// runServerUntilInterrupt runs the server until interrupted by signal or error.
func runServerUntilInterrupt(
	log infralogger.Logger,
	server *infragin.Server,
	intervalScheduler *scheduler.IntervalScheduler,
	sseBroker sse.Broker,
	errChan <-chan error,
) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, signalChannelBufferSize)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal or error
	select {
	case serverErr := <-errChan:
		log.Error("Server error", infralogger.Error(serverErr))
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-sigChan:
		return shutdownServer(log, server, intervalScheduler, sseBroker, sig)
	}
}

// shutdownServer performs graceful shutdown of the server and scheduler.
func shutdownServer(
	log infralogger.Logger,
	server *infragin.Server,
	intervalScheduler *scheduler.IntervalScheduler,
	sseBroker sse.Broker,
	sig os.Signal,
) error {
	log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))

	// Stop SSE broker first (closes all client connections)
	if sseBroker != nil {
		log.Info("Stopping SSE broker")
		if err := sseBroker.Stop(); err != nil {
			log.Error("Failed to stop SSE broker", infralogger.Error(err))
		}
	}

	// Stop scheduler
	if intervalScheduler != nil {
		log.Info("Stopping interval scheduler")
		if err := intervalScheduler.Stop(); err != nil {
			log.Error("Failed to stop scheduler", infralogger.Error(err))
		}
	}

	// Stop HTTP server using infrastructure server's graceful shutdown
	log.Info("Stopping HTTP server")
	if err := server.ShutdownWithTimeout(defaultShutdownTimeout); err != nil {
		log.Error("Failed to stop server", infralogger.Error(err))
		return fmt.Errorf("failed to stop server: %w", err)
	}

	log.Info("Server stopped successfully")
	return nil
}
