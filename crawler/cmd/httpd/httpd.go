// Package httpd implements the HTTP server for the crawler service.
package httpd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infracontext "github.com/north-cloud/infrastructure/context"
	"github.com/north-cloud/infrastructure/profiling"
)

// === Types ===

// CommandDeps holds common dependencies for the HTTP server.
type CommandDeps struct {
	Logger logger.Interface
	Config config.Interface
}

// StorageResult holds both storage interface and index manager.
type StorageResult struct {
	Storage      types.Interface
	IndexManager types.IndexManager
}

// === Constants ===

const (
	signalChannelBufferSize = 1
	errorChannelBufferSize  = 1
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
	jobsHandler, discoveredLinksHandler, dbScheduler, db := setupJobsAndScheduler(deps, storageResult)
	if db != nil {
		defer db.Close()
	}

	// Phase 5: Start HTTP server
	server, errChan, err := startHTTPServer(deps, jobsHandler, discoveredLinksHandler)
	if err != nil {
		return err
	}

	// Phase 6: Run server until interrupted
	return runServerUntilInterrupt(deps.Logger, server, dbScheduler, errChan)
}

// === Dependency Setup ===

// newCommandDeps creates CommandDeps by loading config and creating logger.
func newCommandDeps() (*CommandDeps, error) {
	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Create logger
	log, err := createLogger()
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

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

// createLogger creates a logger instance from environment variables.
func createLogger() (logger.Interface, error) {
	logLevel := normalizeLogLevel(getEnvOrDefault("LOG_LEVEL", "info"))

	// Determine if we're in development mode
	appEnv := getEnvOrDefault("APP_ENV", "production")
	isDev := appEnv == "development"
	appDebug := getEnvOrDefault("APP_DEBUG", "false") == "true"

	// Set development mode based on APP_ENV
	development := isDev

	// Override log level if APP_DEBUG is set
	if appDebug {
		logLevel = "debug"
	}

	// Determine encoding based on environment
	encoding := getEnvOrDefault("LOG_FORMAT", "json")
	if isDev {
		encoding = "console"
	}

	// Get output paths (default to stdout)
	outputPaths := []string{"stdout"}
	if outputPathsStr := os.Getenv("LOG_OUTPUT_PATHS"); outputPathsStr != "" {
		outputPaths = strings.Split(outputPathsStr, ",")
		for i := range outputPaths {
			outputPaths[i] = strings.TrimSpace(outputPaths[i])
		}
	}

	logCfg := &logger.Config{
		Level:       logger.Level(logLevel),
		Development: development,
		Encoding:    encoding,
		OutputPaths: outputPaths,
		EnableColor: isDev,
	}
	return logger.New(logCfg)
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
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
func createStorageClient(cfg config.Interface, log logger.Interface) (*es.Client, error) {
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
func createStorage(cfg config.Interface, log logger.Interface) (*StorageResult, error) {
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

// setupJobsAndScheduler initializes the jobs handler and scheduler if database is available.
// Returns jobsHandler, discoveredLinksHandler, intervalScheduler, and db connection (if available).
func setupJobsAndScheduler(
	deps *CommandDeps,
	storageResult *StorageResult,
) (*api.JobsHandler, *api.DiscoveredLinksHandler, *scheduler.IntervalScheduler, *sqlx.DB) {
	// Convert config to database config (DRY improvement)
	dbConfig := databaseConfigFromInterface(deps.Config.GetDatabaseConfig())

	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		deps.Logger.Warn("Failed to connect to database, jobs API will use fallback", "error", err)
		return nil, nil, nil, nil
	}

	// Create repositories
	jobRepo := database.NewJobRepository(db)
	executionRepo := database.NewExecutionRepository(db)

	// Create jobs handler with both repositories
	jobsHandler := api.NewJobsHandler(jobRepo, executionRepo)

	// Create discovered links handler
	discoveredLinkRepo := database.NewDiscoveredLinkRepository(db)
	discoveredLinksHandler := api.NewDiscoveredLinksHandler(discoveredLinkRepo, jobRepo)

	// Create and start scheduler
	intervalScheduler := createAndStartScheduler(deps, storageResult, jobRepo, executionRepo, db)
	if intervalScheduler != nil {
		jobsHandler.SetScheduler(intervalScheduler)
		discoveredLinksHandler.SetScheduler(intervalScheduler)
	}

	return jobsHandler, discoveredLinksHandler, intervalScheduler, db
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
		deps.Logger.Warn("Failed to create crawler for jobs, scheduler disabled", "error", err)
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
		deps.Logger.Error("Failed to start interval scheduler", "error", startErr)
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
) (*http.Server, chan error, error) {
	server, _, err := api.StartHTTPServer(deps.Logger, deps.Config, jobsHandler, discoveredLinksHandler)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start server in goroutine
	deps.Logger.Info("Starting HTTP server", "addr", deps.Config.GetServerConfig().Address)
	errChan := make(chan error, errorChannelBufferSize)
	go func() {
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errChan <- serveErr
		}
	}()

	return server, errChan, nil
}

// runServerUntilInterrupt runs the server until interrupted by signal or error.
func runServerUntilInterrupt(
	log logger.Interface,
	server *http.Server,
	intervalScheduler *scheduler.IntervalScheduler,
	errChan chan error,
) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, signalChannelBufferSize)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal or error
	select {
	case serverErr := <-errChan:
		log.Error("Server error", "error", serverErr)
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-sigChan:
		return shutdownServer(log, server, intervalScheduler, sig)
	}
}

// shutdownServer performs graceful shutdown of the server and scheduler.
func shutdownServer(
	log logger.Interface,
	server *http.Server,
	intervalScheduler *scheduler.IntervalScheduler,
	sig os.Signal,
) error {
	log.Info("Shutdown signal received", "signal", sig.String())
	shutdownCtx, cancel := infracontext.WithTimeout(defaultShutdownTimeout)
	defer cancel()

	// Stop scheduler first
	if intervalScheduler != nil {
		log.Info("Stopping interval scheduler")
		if err := intervalScheduler.Stop(); err != nil {
			log.Error("Failed to stop scheduler", "error", err)
		}
	}

	// Stop HTTP server
	log.Info("Stopping HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to stop server", "error", err)
		return fmt.Errorf("failed to stop server: %w", err)
	}

	log.Info("Server stopped successfully")
	return nil
}
