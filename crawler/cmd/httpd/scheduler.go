package httpd

import (
	"context"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
)

// setupJobsAndScheduler initializes the jobs handler and scheduler if database is available.
// Returns jobsHandler, dbScheduler, and db connection (if available).
func setupJobsAndScheduler(
	deps *CommandDeps,
	storageResult *StorageResult,
) (*api.JobsHandler, *job.DBScheduler, *sqlx.DB) {
	// Convert config to database config (DRY improvement)
	dbConfig := databaseConfigFromInterface(deps.Config.GetDatabaseConfig())

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
// Note: The scheduler manages its own context lifecycle internally.
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

	// Create and start database scheduler
	// The scheduler manages its own context internally, so we pass a background context
	// which the scheduler will use to derive its own context.
	dbScheduler := job.NewDBScheduler(deps.Logger, jobRepo, crawlerInstance)
	if startErr := dbScheduler.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start database scheduler", "error", startErr)
		return nil
	}

	deps.Logger.Info("Database scheduler started successfully")
	return dbScheduler
}
