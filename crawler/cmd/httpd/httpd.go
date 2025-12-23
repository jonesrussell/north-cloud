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

	cmdcommon "github.com/jonesrussell/gocrawl/cmd/common"
	"github.com/jonesrussell/gocrawl/internal/api"
	"github.com/jonesrussell/gocrawl/internal/constants"
	"github.com/jonesrussell/gocrawl/internal/content/articles"
	"github.com/jonesrussell/gocrawl/internal/content/page"
	"github.com/jonesrussell/gocrawl/internal/crawler"
	"github.com/jonesrussell/gocrawl/internal/crawler/events"
	"github.com/jonesrussell/gocrawl/internal/database"
	"github.com/jonesrussell/gocrawl/internal/job"
	"github.com/jonesrussell/gocrawl/internal/sources"
	"github.com/jonesrussell/gocrawl/internal/storage"
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
	}

	// Create jobs handler and scheduler if database is available
	var jobsHandler *api.JobsHandler
	var dbScheduler *job.DBScheduler
	if db != nil {
		defer db.Close()
		jobRepo := database.NewJobRepository(db)
		jobsHandler = api.NewJobsHandler(jobRepo)

		// Create crawler for job execution
		crawlerInstance, crawlerErr := createCrawlerForJobs(&deps, storageResult)
		if crawlerErr != nil {
			deps.Logger.Warn("Failed to create crawler for jobs, scheduler disabled", "error", crawlerErr)
		} else {
			// Create context for scheduler
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Create and start database scheduler
			dbScheduler = job.NewDBScheduler(deps.Logger, jobRepo, crawlerInstance)
			if startErr := dbScheduler.Start(ctx); startErr != nil {
				deps.Logger.Error("Failed to start database scheduler", "error", startErr)
			} else {
				deps.Logger.Info("Database scheduler started successfully")
				// Connect scheduler to jobs handler so it can trigger immediate reloads
				jobsHandler.SetScheduler(dbScheduler)
			}
		}
	}

	// Create HTTP server
	srv, _, err := api.StartHTTPServer(deps.Logger, searchManager, deps.Config, jobsHandler)
	if err != nil {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	// Start server in goroutine
	deps.Logger.Info("Starting HTTP server", "addr", deps.Config.GetServerConfig().Address)
	errChan := make(chan error, 1)
	go func() {
		if serveErr := srv.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			errChan <- serveErr
		}
	}()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal or error
	select {
	case serverErr := <-errChan:
		deps.Logger.Error("Server error", "error", serverErr)
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-sigChan:
		// Graceful shutdown with timeout
		deps.Logger.Info("Shutdown signal received", "signal", sig.String())
		shutdownCtx, cancel := infracontext.WithTimeout(constants.DefaultShutdownTimeout)
		defer cancel()

		// Stop scheduler first
		if dbScheduler != nil {
			deps.Logger.Info("Stopping database scheduler")
			if stopErr := dbScheduler.Stop(); stopErr != nil {
				deps.Logger.Error("Failed to stop scheduler", "error", stopErr)
			}
		}

		deps.Logger.Info("Stopping HTTP server")
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			deps.Logger.Error("Failed to stop server", "error", shutdownErr)
			return fmt.Errorf("failed to stop server: %w", shutdownErr)
		}

		deps.Logger.Info("Server stopped successfully")
		return nil
	}
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

	// Create article service with raw content indexer
	articleService := articles.NewContentServiceWithRawIndexer(
		deps.Logger,
		storageResult.Storage,
		constants.DefaultContentIndex,
		sourceManager,
		rawIndexer,
	)
	pageService := page.NewContentServiceWithSources(
		deps.Logger,
		storageResult.Storage,
		constants.DefaultContentIndex,
		sourceManager,
	)

	// Create crawler
	crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
		Logger:         deps.Logger,
		Bus:            bus,
		IndexManager:   storageResult.IndexManager,
		Sources:        sourceManager,
		Config:         crawlerCfg,
		ArticleService: articleService,
		PageService:    pageService,
		Storage:        storageResult.Storage,
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
