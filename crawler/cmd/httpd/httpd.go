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
	"syscall"

	"github.com/jmoiron/sqlx"
	cmdcommon "github.com/jonesrussell/north-cloud/crawler/cmd/common"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/constants"
	"github.com/jonesrussell/north-cloud/crawler/internal/content/page"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	infracontext "github.com/north-cloud/infrastructure/context"
)

// Start starts the HTTP server and runs until interrupted.
// It handles graceful shutdown on SIGINT or SIGTERM signals.
func Start() error {
	// Get dependencies
	deps, err := cmdcommon.NewCommandDeps()
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	// Create storage using common function
	storageResult, err := cmdcommon.CreateStorage(deps.Config, deps.Logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Create search manager
	searchManager := storage.NewSearchManager(storageResult.Storage, deps.Logger)

	// Initialize jobs handler and scheduler
	jobsHandler, dbScheduler, db := setupJobsAndScheduler(&deps, storageResult)
	if db != nil {
		defer db.Close()
	}

	// Create and start HTTP server
	srv, errChan, err := startHTTPServer(&deps, searchManager, jobsHandler)
	if err != nil {
		return err
	}

	// Run server until interrupted
	return runServerUntilInterrupt(deps.Logger, srv, dbScheduler, errChan)
}

// setupJobsAndScheduler initializes the jobs handler and scheduler if database is available.
// Returns jobsHandler, dbScheduler, and db connection (if available).
func setupJobsAndScheduler(
	deps *cmdcommon.CommandDeps,
	storageResult *cmdcommon.StorageResult,
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
	deps *cmdcommon.CommandDeps,
	storageResult *cmdcommon.StorageResult,
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
	deps *cmdcommon.CommandDeps,
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
	deps *cmdcommon.CommandDeps,
	storageResult *cmdcommon.StorageResult,
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

	pageService := page.NewContentServiceWithSources(
		deps.Logger,
		storageResult.Storage,
		constants.DefaultContentIndex,
		sourceManager,
	)

	// Create crawler
	crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
		Logger:       deps.Logger,
		Bus:          bus,
		IndexManager: storageResult.IndexManager,
		Sources:      sourceManager,
		Config:       crawlerCfg,
		PageService:  pageService,
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
