// Package httpd implements the HTTP server for the crawler service.
package httpd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/constants"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infracontext "github.com/north-cloud/infrastructure/context"
	"github.com/spf13/viper"
)

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

var (
	// errLoggerRequired is returned when CommandDeps.Logger is nil
	errLoggerRequired = errors.New("logger is required")
	// errConfigRequired is returned when CommandDeps.Config is nil
	errConfigRequired = errors.New("config is required")
)

// Start starts the HTTP server and runs until interrupted.
// It handles graceful shutdown on SIGINT or SIGTERM signals.
func Start() error {
	// Get dependencies
	deps, err := newCommandDeps()
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	// Create storage
	storageResult, err := createStorage(deps.Config, deps.Logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Create search manager
	searchManager := storage.NewSearchManager(storageResult.Storage, deps.Logger)

	// Initialize jobs handler and scheduler
	jobsHandler, dbScheduler, db := setupJobsAndScheduler(deps, storageResult)
	if db != nil {
		defer db.Close()
	}

	// Create and start HTTP server
	srv, errChan, err := startHTTPServer(deps, searchManager, jobsHandler)
	if err != nil {
		return err
	}

	// Run server until interrupted
	return runServerUntilInterrupt(deps.Logger, srv, dbScheduler, errChan)
}

// setupJobsAndScheduler initializes the jobs handler and scheduler if database is available.
// Returns jobsHandler, dbScheduler, and db connection (if available).
func setupJobsAndScheduler(
	deps *CommandDeps,
	storageResult *StorageResult,
) (*api.JobsHandler, *job.DBScheduler, *sqlx.DB) {
	// Initialize database connection
	dbConfig := database.Config{
		Host:     deps.Config.GetDatabaseConfig().Host,
		Port:     deps.Config.GetDatabaseConfig().Port,
		User:     deps.Config.GetDatabaseConfig().User,
		Password: deps.Config.GetDatabaseConfig().Password,
		DBName:   deps.Config.GetDatabaseConfig().DBName,
		SSLMode:  deps.Config.GetDatabaseConfig().SSLMode,
	}

	db, err := database.NewPostgresConnection(dbConfig)
	if err != nil {
		deps.Logger.Warn("Failed to connect to database, jobs API will use fallback", "error", err)
		return nil, nil, nil
	}

	// Create jobs handler
	jobRepo := database.NewJobRepository(db)
	jobsHandler := api.NewJobsHandler(jobRepo)

	// Create and start scheduler
	dbScheduler := createAndStartScheduler(deps, storageResult, jobRepo)
	if dbScheduler != nil {
		jobsHandler.SetScheduler(dbScheduler)
	}

	return jobsHandler, dbScheduler, db
}

// createAndStartScheduler creates and starts the database scheduler.
// Returns nil if scheduler cannot be created or started.
func createAndStartScheduler(
	deps *CommandDeps,
	storageResult *StorageResult,
	jobRepo *database.JobRepository,
) *job.DBScheduler {
	// Create crawler for job execution
	crawlerInstance, err := createCrawlerForJobs(deps, storageResult)
	if err != nil {
		deps.Logger.Warn("Failed to create crawler for jobs, scheduler disabled", "error", err)
		return nil
	}

	// Create context for scheduler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create and start database scheduler
	dbScheduler := job.NewDBScheduler(deps.Logger, jobRepo, crawlerInstance)
	if startErr := dbScheduler.Start(ctx); startErr != nil {
		deps.Logger.Error("Failed to start database scheduler", "error", startErr)
		return nil
	}

	deps.Logger.Info("Database scheduler started successfully")
	return dbScheduler
}

// startHTTPServer creates and starts the HTTP server.
// Returns the server and an error channel for server errors.
func startHTTPServer(
	deps *CommandDeps,
	searchManager api.SearchManager,
	jobsHandler *api.JobsHandler,
) (*http.Server, chan error, error) {
	srv, _, err := api.StartHTTPServer(deps.Logger, searchManager, deps.Config, jobsHandler)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start server in goroutine
	deps.Logger.Info("Starting HTTP server", "addr", deps.Config.GetServerConfig().Address)
	errChan := make(chan error, 1)
	go func() {
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errChan <- serveErr
		}
	}()

	return srv, errChan, nil
}

// runServerUntilInterrupt runs the server until interrupted by signal or error.
func runServerUntilInterrupt(
	log logger.Interface,
	srv *http.Server,
	dbScheduler *job.DBScheduler,
	errChan chan error,
) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal or error
	select {
	case serverErr := <-errChan:
		log.Error("Server error", "error", serverErr)
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-sigChan:
		return shutdownServer(log, srv, dbScheduler, sig)
	}
}

// shutdownServer performs graceful shutdown of the server and scheduler.
func shutdownServer(
	log logger.Interface,
	srv *http.Server,
	dbScheduler *job.DBScheduler,
	sig os.Signal,
) error {
	log.Info("Shutdown signal received", "signal", sig.String())
	shutdownCtx, cancel := infracontext.WithTimeout(constants.DefaultShutdownTimeout)
	defer cancel()

	// Stop scheduler first
	if dbScheduler != nil {
		log.Info("Stopping database scheduler")
		if err := dbScheduler.Stop(); err != nil {
			log.Error("Failed to stop scheduler", "error", err)
		}
	}

	// Stop HTTP server
	log.Info("Stopping HTTP server")
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("Failed to stop server", "error", err)
		return fmt.Errorf("failed to stop server: %w", err)
	}

	log.Info("Server stopped successfully")
	return nil
}

// createCrawlerForJobs creates a crawler instance for job execution
func createCrawlerForJobs(
	deps *CommandDeps,
	storageResult *StorageResult,
) (crawler.Interface, error) {
	// Create event bus
	bus := events.NewEventBus(deps.Logger)

	// Get crawler config
	crawlerCfg := deps.Config.GetCrawlerConfig()

	// Create source manager using LoadSources (which uses API loader internally)
	sourceManager, err := sources.LoadSources(deps.Config, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load sources: %w", err)
	}

	// Create raw content indexer for classifier integration
	rawIndexer := storage.NewRawContentIndexer(storageResult.Storage, deps.Logger)

	// Ensure raw content indexes exist for all sources
	allSources, err := sourceManager.GetSources()
	if err != nil {
		deps.Logger.Warn("Failed to get sources for raw content index creation", "error", err)
	} else {
		ctx, cancel := infracontext.WithTimeout(constants.DefaultShutdownTimeout)
		defer cancel()
		for i := range allSources {
			// Extract hostname from source URL for index naming (e.g., "www.sudbury.com")
			// instead of using the source's Name field (e.g., "sudbury _ local news")
			sourceHostname := extractHostnameFromURL(allSources[i].URL)
			if sourceHostname == "" {
				// Fallback to source name if URL parsing fails
				sourceHostname = allSources[i].Name
			}
			if indexErr := rawIndexer.EnsureRawContentIndex(ctx, sourceHostname); indexErr != nil {
				deps.Logger.Warn("Failed to ensure raw content index",
					"source", allSources[i].Name,
					"source_url", allSources[i].URL,
					"hostname", sourceHostname,
					"error", indexErr)
				// Continue with other sources - not fatal
			}
		}
	}

	// Create crawler
	crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
		Logger:       deps.Logger,
		Bus:          bus,
		IndexManager: storageResult.IndexManager,
		Sources:      sourceManager,
		Config:       crawlerCfg,
		Storage:      storageResult.Storage,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create crawler: %w", err)
	}

	return crawlerResult.Crawler, nil
}

// extractHostnameFromURL extracts the hostname from a URL for use in index naming.
// Example: "https://www.sudbury.com/article" → "www.sudbury.com"
func extractHostnameFromURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return ""
	}
	return hostname
}

// newCommandDeps creates CommandDeps by loading config and creating logger.
func newCommandDeps() (*CommandDeps, error) {
	// Initialize config first
	if err := initConfig(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Get logger configuration from Viper
	logLevel := viper.GetString("logger.level")
	if logLevel == "" {
		logLevel = "info"
	}
	logLevel = strings.ToLower(logLevel)

	logCfg := &logger.Config{
		Level:       logger.Level(logLevel),
		Development: viper.GetBool("logger.development"),
		Encoding:    viper.GetString("logger.encoding"),
		OutputPaths: viper.GetStringSlice("logger.output_paths"),
		EnableColor: viper.GetBool("logger.enable_color"),
	}

	log, err := logger.New(logCfg)
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

// initConfig initializes Viper configuration from environment variables and config files.
func initConfig() error {
	// Load .env file first (ignore error if file doesn't exist)
	_ = godotenv.Load()

	// Enable automatic environment variable reading
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Set defaults
	setDefaults()

	// Set config file paths
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")

	// Read config file (optional, ignore error if file doesn't exist)
	_ = viper.ReadInConfig()

	// Bind environment variables
	if err := bindAppEnvVars(); err != nil {
		return fmt.Errorf("failed to bind app env vars: %w", err)
	}

	if err := bindElasticsearchEnvVars(); err != nil {
		return fmt.Errorf("failed to bind elasticsearch env vars: %w", err)
	}

	// Set development logging settings
	setupDevelopmentLogging()

	return nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// App defaults - production safe
	viper.SetDefault("app", map[string]any{
		"name":        "crawler",
		"version":     "1.0.0",
		"environment": "production",
		"debug":       false,
	})

	// Logger defaults - production safe
	viper.SetDefault("logger", map[string]any{
		"level":        "info",
		"development":  false,
		"encoding":     "json",
		"output_paths": []string{"stdout"},
		"enable_color": false,
		"caller":       false,
		"stacktrace":   false,
		"max_size":     config.DefaultMaxLogSize,
		"max_backups":  config.DefaultMaxLogBackups,
		"max_age":      config.DefaultMaxLogAge,
		"compress":     true,
	})

	// Server defaults - production safe
	viper.SetDefault("server", map[string]any{
		"address":          ":8080",
		"read_timeout":     "15s",
		"write_timeout":    "15s",
		"idle_timeout":     "60s",
		"security_enabled": true,
	})

	// Elasticsearch defaults - production safe
	viper.SetDefault("elasticsearch", map[string]any{
		"addresses": []string{"http://127.0.0.1:9200"},
		"tls": map[string]any{
			"enabled":              true,
			"insecure_skip_verify": false,
		},
		"retry": map[string]any{
			"enabled":      true,
			"initial_wait": "1s",
			"max_wait":     "30s",
			"max_retries":  crawlerconfig.DefaultMaxRetries,
		},
		"bulk_size":      config.DefaultBulkSize,
		"flush_interval": "1s",
		"index_prefix":   "crawler",
		"discover_nodes": false,
	})

	// Crawler defaults - production safe
	viper.SetDefault("crawler", map[string]any{
		"max_depth":          crawlerconfig.DefaultMaxDepth,
		"max_concurrency":    crawlerconfig.DefaultParallelism,
		"request_timeout":    "30s",
		"user_agent":         crawlerconfig.DefaultUserAgent,
		"respect_robots_txt": true,
		"delay":              "1s",
		"random_delay":       "0s",
		"source_file":        "sources.yml",
		"debugger": map[string]any{
			"enabled": false,
			"level":   "info",
			"format":  "json",
			"output":  "stdout",
		},
		"rate_limit":  "2s",
		"parallelism": crawlerconfig.DefaultParallelism,
		"tls": map[string]any{
			"insecure_skip_verify": false,
		},
		"retry_delay":      "5s",
		"max_retries":      crawlerconfig.DefaultMaxRetries,
		"follow_redirects": true,
		"max_redirects":    crawlerconfig.DefaultMaxRedirects,
		"validate_urls":    true,
		"cleanup_interval": crawlerconfig.DefaultCleanupInterval.String(),
	})
}

// bindAppEnvVars binds application and logger environment variables to config keys.
func bindAppEnvVars() error {
	if err := viper.BindEnv("app.environment", "APP_ENV"); err != nil {
		return fmt.Errorf("failed to bind APP_ENV: %w", err)
	}
	if err := viper.BindEnv("app.debug", "APP_DEBUG"); err != nil {
		return fmt.Errorf("failed to bind APP_DEBUG: %w", err)
	}
	if err := viper.BindEnv("logger.level", "LOG_LEVEL"); err != nil {
		return fmt.Errorf("failed to bind LOG_LEVEL: %w", err)
	}
	if err := viper.BindEnv("logger.encoding", "LOG_FORMAT"); err != nil {
		return fmt.Errorf("failed to bind LOG_FORMAT: %w", err)
	}
	// Bind crawler sources API URL
	if err := viper.BindEnv("crawler.sources_api_url", "CRAWLER_SOURCES_API_URL"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_SOURCES_API_URL: %w", err)
	}
	return nil
}

// bindElasticsearchEnvVars binds Elasticsearch environment variables to config keys.
func bindElasticsearchEnvVars() error {
	// Support both ELASTICSEARCH_HOSTS and ELASTICSEARCH_ADDRESSES
	if err := viper.BindEnv("elasticsearch.addresses", "ELASTICSEARCH_HOSTS", "ELASTICSEARCH_ADDRESSES"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch addresses: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.password", "ELASTIC_PASSWORD", "ELASTICSEARCH_PASSWORD"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch password: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.tls.insecure_skip_verify", "ELASTICSEARCH_SKIP_TLS"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch TLS skip verify: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.api_key", "ELASTICSEARCH_API_KEY"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch API key: %w", err)
	}
	// Bind index_name (supports both ELASTICSEARCH_INDEX_PREFIX and ELASTICSEARCH_INDEX_NAME)
	if err := viper.BindEnv("elasticsearch.index_name",
		"ELASTICSEARCH_INDEX_PREFIX", "ELASTICSEARCH_INDEX_NAME"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch index name: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.retry.max_retries", "ELASTICSEARCH_MAX_RETRIES"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch max retries: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.retry.initial_wait", "ELASTICSEARCH_RETRY_INITIAL_WAIT"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch retry initial wait: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.retry.max_wait", "ELASTICSEARCH_RETRY_MAX_WAIT"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch retry max wait: %w", err)
	}
	return nil
}

// setupDevelopmentLogging configures development logging settings based on environment.
func setupDevelopmentLogging() {
	debugFlag := viper.GetBool("app.debug")
	isDev := viper.GetString("app.environment") == "development"

	// Only set debug level if explicitly requested via APP_DEBUG
	if debugFlag {
		viper.Set("logger.level", "debug")
	}

	// Set development mode features (formatting, colors, etc.) if in development environment
	if isDev {
		viper.Set("logger.development", true)
		viper.Set("logger.enable_color", true)
		viper.Set("logger.caller", true)
		viper.Set("logger.stacktrace", true)
		viper.Set("logger.encoding", "console")
		// Only set debug level if explicitly requested
		if debugFlag {
			viper.Set("logger.level", "debug")
		}
	}
}

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
