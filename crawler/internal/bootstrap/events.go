package bootstrap

import (
	"context"
	"errors"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	crawlerintevents "github.com/jonesrussell/north-cloud/crawler/internal/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/job"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SetupEventConsumer creates and starts the event consumer if Redis events are enabled.
// Returns nil if events are disabled or Redis is unavailable.
func SetupEventConsumer(
	deps *CommandDeps,
	jobRepo *database.JobRepository,
	processedEventsRepo *database.ProcessedEventsRepository,
) *crawlerintevents.Consumer {
	redisClient, err := CreateRedisClient(deps.Config.GetRedisConfig())
	if err != nil {
		if !errors.Is(err, ErrRedisDisabled) {
			deps.Logger.Warn("Redis not available, event consumer disabled",
				infralogger.Error(err),
			)
		}
		return nil
	}

	// Create source client for fetching source data
	sourceManagerCfg := deps.Config.GetSourceManagerConfig()
	sourceClient := sources.NewHTTPClient(sourceManagerCfg.URL, nil)

	// Create EventService as the event handler
	scheduleComputer := job.NewScheduleComputer()
	eventService := job.NewEventService(jobRepo, processedEventsRepo, scheduleComputer, sourceClient, deps.Logger)

	consumer := crawlerintevents.NewConsumer(redisClient, "", eventService, deps.Logger)

	if startErr := consumer.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start event consumer", infralogger.Error(startErr))
		return nil
	}

	deps.Logger.Info("Event consumer started with EventService handler")
	return consumer
}

// SetupMigrator creates the migrator service for Phase 3 job migration.
func SetupMigrator(deps *CommandDeps, jobRepo *database.JobRepository) *job.Migrator {
	sourceManagerCfg := deps.Config.GetSourceManagerConfig()
	sourceClient := sources.NewHTTPClient(sourceManagerCfg.URL, nil)
	scheduleComputer := job.NewScheduleComputer()

	return job.NewMigrator(jobRepo, sourceClient, scheduleComputer, deps.Logger)
}
