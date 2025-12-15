// Package httpd implements the HTTP server command for the search API.
package httpd

import (
	"context"
	"errors"
	"fmt"
	"net/http"

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
	"github.com/spf13/cobra"
)

// Cmd represents the HTTP server command
var Cmd = &cobra.Command{
	Use:   "httpd",
	Short: "Start the HTTP server for search",
	Long: `This command starts an HTTP server that listens for search requests.
You can send POST requests to /search with a JSON body containing the search parameters.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
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
			crawlerInstance, err := createCrawlerForJobs(&deps, storageResult)
			if err != nil {
				deps.Logger.Warn("Failed to create crawler for jobs, scheduler disabled", "error", err)
			} else {
				// Create and start database scheduler
				dbScheduler = job.NewDBScheduler(deps.Logger, jobRepo, crawlerInstance)
				if err := dbScheduler.Start(cmd.Context()); err != nil {
					deps.Logger.Error("Failed to start database scheduler", "error", err)
				} else {
					deps.Logger.Info("Database scheduler started successfully")
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

		// Wait for interrupt signal or error
		select {
		case serverErr := <-errChan:
			deps.Logger.Error("Server error", "error", serverErr)
			return fmt.Errorf("server error: %w", serverErr)
		case <-cmd.Context().Done():
			// Graceful shutdown with timeout
			deps.Logger.Info("Shutdown signal received")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.DefaultShutdownTimeout)
			defer cancel()

			// Stop scheduler first
			if dbScheduler != nil {
				deps.Logger.Info("Stopping database scheduler")
				if err := dbScheduler.Stop(); err != nil {
					deps.Logger.Error("Failed to stop scheduler", "error", err)
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
	},
}

// Command returns the httpd command for use in the root command
func Command() *cobra.Command {
	return Cmd
}

// createCrawlerForJobs creates a crawler instance for job execution
func createCrawlerForJobs(deps *cmdcommon.CommandDeps, storageResult *cmdcommon.StorageResult) (crawler.Interface, error) {
	// Create event bus
	bus := events.NewEventBus(deps.Logger)

	// Get crawler config
	crawlerCfg := deps.Config.GetCrawlerConfig()

	// Create source manager using LoadSources (which uses API loader internally)
	sourceManager, err := sources.LoadSources(deps.Config, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load sources: %w", err)
	}

	// Create article and page services
	articleService := articles.NewContentService(deps.Logger, storageResult.Storage, constants.DefaultContentIndex)
	pageService := page.NewContentService(deps.Logger, storageResult.Storage, constants.DefaultContentIndex)

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
