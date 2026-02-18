// Package bootstrap handles application initialization and lifecycle management
// for the crawler service.
//
// The bootstrap process follows these phases:
//   - Phase 0: Profiling - Start pprof and Pyroscope profilers (if enabled)
//   - Phase 1: Config & Logger - Load configuration and create logger
//   - Phase 2: Storage - Initialize Elasticsearch client and storage
//   - Phase 3: Database - Connect to PostgreSQL and create repositories
//   - Phase 4: Services - Create crawler, scheduler, SSE, and log services
//   - Phase 5: Events - Setup event consumer (if Redis enabled)
//   - Phase 6: Server - Create and start HTTP server
//   - Phase 7: Run - Wait for interrupt signal or error
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"time"

	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/profiling"
)

// backgroundCancels holds cancel functions for background goroutines.
type backgroundCancels struct {
	feedPollerCancel    context.CancelFunc
	feedDiscoveryCancel context.CancelFunc
	workerPoolCancel    context.CancelFunc
}

// startBackgroundWorkers launches background goroutines for feed polling,
// feed discovery, and the frontier worker pool.
func startBackgroundWorkers(deps *CommandDeps, sc *ServiceComponents) backgroundCancels {
	var bg backgroundCancels

	if sc.FeedPoller != nil {
		feedCfg := deps.Config.GetFeedConfig()
		pollerCtx, cancel := context.WithCancel(context.Background())
		bg.feedPollerCancel = cancel
		interval := time.Duration(feedCfg.PollIntervalMinutes) * time.Minute
		go func() {
			_ = sc.FeedPoller.RunPollingLoop(pollerCtx, interval, sc.ListDue)
		}()
		deps.Logger.Info("Feed poller started",
			infralogger.Int("interval_minutes", feedCfg.PollIntervalMinutes))
	}

	if sc.FeedDiscoverer != nil {
		feedCfg := deps.Config.GetFeedConfig()
		dCtx, cancel := context.WithCancel(context.Background())
		bg.feedDiscoveryCancel = cancel
		interval := time.Duration(feedCfg.DiscoveryIntervalMinutes) * time.Minute
		go func() {
			_ = sc.FeedDiscoverer.RunDiscoveryLoop(dCtx, interval, sc.ListUndiscovered)
		}()
		deps.Logger.Info("Feed discovery started",
			infralogger.Int("interval_minutes", feedCfg.DiscoveryIntervalMinutes))
	}

	if sc.FrontierWorkerPool != nil {
		wpCtx, cancel := context.WithCancel(context.Background())
		bg.workerPoolCancel = cancel
		go func() {
			_ = sc.FrontierWorkerPool.Start(wpCtx)
		}()
		fetcherCfg := deps.Config.GetFetcherConfig()
		deps.Logger.Info("Frontier worker pool started",
			infralogger.Int("worker_count", fetcherCfg.WorkerCount))
	}

	return bg
}

// Start initializes and starts the crawler application.
// It handles all phases of bootstrap and returns an error if any phase fails.
// The function blocks until the server is interrupted or encounters an error.
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

	// Phase 1: Initialize config and logger
	deps, err := NewCommandDeps()
	if err != nil {
		return fmt.Errorf("failed to initialize dependencies: %w", err)
	}

	// Phase 2: Setup storage (Elasticsearch)
	storageComponents, err := SetupStorage(deps.Config, deps.Logger)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}

	// Phase 3: Setup database (PostgreSQL) and repositories
	dbComponents, err := SetupDatabase(deps.Config)
	if err != nil {
		return fmt.Errorf("failed to setup database: %w", err)
	}
	defer dbComponents.DB.Close()

	// Phase 4: Setup services (crawler, scheduler, SSE, logs)
	serviceComponents, err := SetupServices(deps, storageComponents, dbComponents)
	if err != nil {
		return fmt.Errorf("failed to setup services: %w", err)
	}

	// Phase 5: Setup event consumer (if Redis events enabled)
	eventConsumer := SetupEventConsumer(deps, dbComponents.JobRepo, dbComponents.ProcessedEventsRepo)

	// Phase 6: Setup migrator and HTTP server
	migrator := SetupMigrator(deps, dbComponents.JobRepo)

	serverDeps := &HTTPServerDeps{
		Config:                 deps.Config,
		Logger:                 deps.Logger,
		JobsHandler:            serviceComponents.JobsHandler,
		DiscoveredLinksHandler: serviceComponents.DiscoveredLinksHandler,
		LogsHandler:            serviceComponents.LogsHandler,
		LogsV2Handler:          serviceComponents.LogsV2Handler,
		ExecutionRepo:          dbComponents.ExecutionRepo,
		SSEHandler:             serviceComponents.SSEHandler,
		Migrator:               migrator,
		JobRepo:                dbComponents.JobRepo,
		FrontierRepo:           dbComponents.FrontierRepo,
	}
	serverComponents := SetupHTTPServer(serverDeps)

	// Phase 6b: Start background goroutines (feed poller, discovery, worker pool)
	bg := startBackgroundWorkers(deps, serviceComponents)

	// Phase 7: Run until interrupt or error
	return RunUntilInterrupt(
		deps.Logger,
		serverComponents.Server,
		serviceComponents.Scheduler,
		serviceComponents.SSEBroker,
		serviceComponents.LogService,
		eventConsumer,
		bg.feedPollerCancel,
		bg.feedDiscoveryCancel,
		bg.workerPoolCancel,
		serverComponents.ErrorChan,
	)
}
