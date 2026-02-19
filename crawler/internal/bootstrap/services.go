package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	crawlerevents "github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/apiclient"
	crawlstorage "github.com/jonesrussell/north-cloud/crawler/internal/storage"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/pipeline"
	"github.com/north-cloud/infrastructure/sse"
	"github.com/redis/go-redis/v9"
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

	// Feed poller
	FeedPoller *feed.Poller
	ListDue    func(ctx context.Context) ([]feed.DueFeed, error)

	// Feed discoverer
	FeedDiscoverer   *feed.Discoverer
	ListUndiscovered func(ctx context.Context) ([]feed.UndiscoveredSource, error)

	// Frontier worker pool
	FrontierWorkerPool *fetcher.WorkerPool

	// Frontier repo for HTTP handler (logging wrapper when frontier enabled)
	FrontierRepoForHandler api.FrontierRepoForHandler

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

	// Frontier submission logging wrapper (one log per Submit for Grafana). Used for handler, feed, link submitter; claimer uses raw repo.
	var frontierForHandler api.FrontierRepoForHandler
	var frontierForSubmission crawler.LinkFrontierSubmitter
	var frontierForFeed feed.FrontierSubmitterRepo
	if db.FrontierRepo != nil {
		wrapped := NewLoggingFrontierRepo(db.FrontierRepo, deps.Logger)
		frontierForHandler = wrapped
		frontierForSubmission = wrapped
		frontierForFeed = wrapped
	}

	// Create and start scheduler
	intervalScheduler := createAndStartScheduler(deps, storage, db, frontierForSubmission)
	if intervalScheduler != nil {
		jobsHandler.SetScheduler(intervalScheduler)
		discoveredLinksHandler.SetScheduler(intervalScheduler)
		// Connect SSE publisher to scheduler
		intervalScheduler.SetSSEPublisher(ssePublisher)
		// Connect log service to scheduler for job log capture
		intervalScheduler.SetLogService(logResult.Service)
	}

	// Create feed poller (if enabled)
	feedPoller, listDue := createFeedPoller(deps, db, frontierForFeed)

	// Create feed discoverer (if enabled)
	feedDiscoverer, listUndiscovered := createFeedDiscoverer(deps)

	// Create frontier worker pool (if enabled); uses raw repo for claimer
	workerPool := createFrontierWorkerPool(deps, db, storage)

	return &ServiceComponents{
		JobsHandler:            jobsHandler,
		DiscoveredLinksHandler: discoveredLinksHandler,
		LogsHandler:            logsHandler,
		LogsV2Handler:          logsV2Handler,
		Scheduler:              intervalScheduler,
		LogService:             logResult.Service,
		FeedPoller:             feedPoller,
		ListDue:                listDue,
		FeedDiscoverer:         feedDiscoverer,
		ListUndiscovered:       listUndiscovered,
		FrontierWorkerPool:     workerPool,
		FrontierRepoForHandler: frontierForHandler,
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
	db *DatabaseComponents,
	frontierForSubmission crawler.LinkFrontierSubmitter,
) *scheduler.IntervalScheduler {
	// Create crawler factory for job execution (each job gets an isolated instance)
	crawlerFactory, err := createCrawlerFactory(deps, storage, db, frontierForSubmission)
	if err != nil {
		deps.Logger.Warn("Failed to create crawler factory, scheduler disabled", infralogger.Error(err))
		return nil
	}

	// Create interval scheduler with default options
	intervalScheduler := scheduler.NewIntervalScheduler(
		deps.Logger,
		db.JobRepo,
		db.ExecutionRepo,
		crawlerFactory,
	)

	// Start the scheduler
	if startErr := intervalScheduler.Start(context.Background()); startErr != nil {
		deps.Logger.Error("Failed to start interval scheduler", infralogger.Error(startErr))
		return nil
	}

	deps.Logger.Info("Interval scheduler started successfully")
	return intervalScheduler
}

// createCrawlerFactory creates a crawler factory for job execution.
// Each job gets an isolated crawler instance from the factory.
func createCrawlerFactory(
	deps *CommandDeps,
	storage *StorageComponents,
	db *DatabaseComponents,
	frontierForSubmission crawler.LinkFrontierSubmitter,
) (crawler.FactoryInterface, error) {
	params, err := buildCrawlerParams(deps, storage, db.DB, frontierForSubmission)
	if err != nil {
		return nil, err
	}
	return crawler.NewFactory(params), nil
}

// buildCrawlerParams assembles the CrawlerParams needed to construct crawler instances.
func buildCrawlerParams(
	deps *CommandDeps,
	storage *StorageComponents,
	db *sqlx.DB,
	frontierForSubmission crawler.LinkFrontierSubmitter,
) (crawler.CrawlerParams, error) {
	bus := crawlerevents.NewEventBus(deps.Logger)
	crawlerCfg := deps.Config.GetCrawlerConfig()

	sourceManager, err := loadSourceManager(deps)
	if err != nil {
		return crawler.CrawlerParams{}, err
	}

	var redisClient *redis.Client
	if crawlerCfg.RedisStorageEnabled {
		rc, redisErr := CreateRedisClient(deps.Config.GetRedisConfig())
		if redisErr != nil {
			if !errors.Is(redisErr, ErrRedisDisabled) {
				deps.Logger.Warn(
					"Redis not available for crawler, features disabled",
					infralogger.Error(redisErr))
			}
		} else {
			redisClient = rc
		}
	}

	pipelineClient := pipeline.NewClient(
		deps.Config.GetPipelineURL(), "crawler",
	)

	var hashTracker *adaptive.HashTracker
	if redisClient != nil {
		hashTracker = adaptive.NewHashTracker(redisClient)
	}

	var frontierSubmitter crawler.LinkFrontierSubmitter
	if frontierForSubmission != nil && deps.Config.GetFetcherConfig().Enabled {
		frontierSubmitter = frontierForSubmission
	}

	return crawler.CrawlerParams{
		Logger:            deps.Logger,
		Bus:               bus,
		IndexManager:      storage.IndexManager,
		Sources:           sourceManager,
		Config:            crawlerCfg,
		Storage:           storage.Storage,
		FullConfig:        deps.Config,
		DB:                db,
		PipelineClient:    pipelineClient,
		RedisClient:       redisClient,
		HashTracker:       hashTracker,
		FrontierSubmitter: frontierSubmitter,
	}, nil
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

// feedHTTPFetchTimeout is the timeout for HTTP requests when fetching feeds.
const feedHTTPFetchTimeout = 30 * time.Second

// createFeedPoller creates a feed poller and listDue callback.
// Returns (nil, nil) if feed polling is disabled or frontier submitter is nil.
func createFeedPoller(
	deps *CommandDeps,
	db *DatabaseComponents,
	frontierForFeed feed.FrontierSubmitterRepo,
) (poller *feed.Poller, listDueFn func(ctx context.Context) ([]feed.DueFeed, error)) {
	feedCfg := deps.Config.GetFeedConfig()
	if !feedCfg.Enabled || frontierForFeed == nil {
		if !feedCfg.Enabled {
			deps.Logger.Info("Feed polling disabled")
		}
		return nil, nil
	}

	smCfg := deps.Config.GetSourceManagerConfig()
	authCfg := deps.Config.GetAuthConfig()

	apiClient := apiclient.NewClient(
		apiclient.WithBaseURL(smCfg.URL+"/api/v1/sources"),
		apiclient.WithJWTSecret(authCfg.JWTSecret),
	)

	httpFetcher := feed.NewHTTPFetcher(&http.Client{Timeout: feedHTTPFetchTimeout})
	feedStateAdapter := feed.NewFeedStateRepoAdapter(db.FeedStateRepo)
	frontierAdapter := feed.NewFrontierRepoAdapter(frontierForFeed)
	logAdapter := &logAdapter{log: deps.Logger}

	poller = feed.NewPoller(httpFetcher, feedStateAdapter, frontierAdapter, logAdapter)

	listDueFn = buildListDueFunc(apiClient, deps.Logger)

	deps.Logger.Info("Feed poller created",
		infralogger.Int("poll_interval_minutes", feedCfg.PollIntervalMinutes))

	return poller, listDueFn
}

// buildListDueFunc creates a closure that lists sources with feed URLs from the API.
func buildListDueFunc(
	client *apiclient.Client,
	log infralogger.Logger,
) func(ctx context.Context) ([]feed.DueFeed, error) {
	return func(ctx context.Context) ([]feed.DueFeed, error) {
		apiSources, err := client.ListSources(ctx)
		if err != nil {
			return nil, fmt.Errorf("list sources for feed polling: %w", err)
		}

		var due []feed.DueFeed
		for i := range apiSources {
			if apiSources[i].FeedURL != nil && *apiSources[i].FeedURL != "" {
				due = append(due, feed.DueFeed{
					SourceID: apiSources[i].ID,
					FeedURL:  *apiSources[i].FeedURL,
				})
			}
		}

		log.Info("Listed due feeds",
			infralogger.Int("total_sources", len(apiSources)),
			infralogger.Int("feeds_due", len(due)))

		return due, nil
	}
}

// sourceFeedUpdaterAdapter adapts apiclient.Client to the feed.SourceFeedUpdater interface.
type sourceFeedUpdaterAdapter struct {
	client *apiclient.Client
	log    infralogger.Logger
}

// UpdateFeedURL persists a discovered feed URL for a source via the source-manager API.
func (a *sourceFeedUpdaterAdapter) UpdateFeedURL(ctx context.Context, sourceID, feedURL string) error {
	source, err := a.client.GetSource(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("get source for feed update: %w", err)
	}

	source.FeedURL = &feedURL

	if _, updateErr := a.client.UpdateSource(ctx, sourceID, source); updateErr != nil {
		return fmt.Errorf("update source feed URL: %w", updateErr)
	}

	return nil
}

// createFeedDiscoverer creates a feed discoverer and listUndiscovered callback.
// Returns (nil, nil) if feed discovery is disabled.
func createFeedDiscoverer(
	deps *CommandDeps,
) (discoverer *feed.Discoverer, listUndiscoveredFn func(ctx context.Context) ([]feed.UndiscoveredSource, error)) {
	feedCfg := deps.Config.GetFeedConfig()
	if !feedCfg.DiscoveryEnabled {
		deps.Logger.Info("Feed discovery disabled")
		return nil, nil
	}

	smCfg := deps.Config.GetSourceManagerConfig()
	authCfg := deps.Config.GetAuthConfig()

	apiClient := apiclient.NewClient(
		apiclient.WithBaseURL(smCfg.URL+"/api/v1/sources"),
		apiclient.WithJWTSecret(authCfg.JWTSecret),
	)

	httpFetcher := feed.NewHTTPFetcher(&http.Client{Timeout: feedHTTPFetchTimeout})
	updaterAdapter := &sourceFeedUpdaterAdapter{client: apiClient, log: deps.Logger}
	logAdapt := &logAdapter{log: deps.Logger}
	retryAfter := time.Duration(feedCfg.DiscoveryRetryHours) * time.Hour

	discoverer = feed.NewDiscoverer(httpFetcher, updaterAdapter, logAdapt, retryAfter)
	listUndiscoveredFn = buildListUndiscoveredFunc(apiClient, deps.Logger)

	deps.Logger.Info("Feed discoverer created",
		infralogger.Int("discovery_interval_minutes", feedCfg.DiscoveryIntervalMinutes),
		infralogger.Int("discovery_retry_hours", feedCfg.DiscoveryRetryHours))

	return discoverer, listUndiscoveredFn
}

// buildListUndiscoveredFunc creates a closure that lists enabled sources without feed URLs.
func buildListUndiscoveredFunc(
	client *apiclient.Client,
	log infralogger.Logger,
) func(ctx context.Context) ([]feed.UndiscoveredSource, error) {
	return func(ctx context.Context) ([]feed.UndiscoveredSource, error) {
		apiSources, err := client.ListSources(ctx)
		if err != nil {
			return nil, fmt.Errorf("list sources for feed discovery: %w", err)
		}

		var undiscovered []feed.UndiscoveredSource
		for i := range apiSources {
			if !apiSources[i].Enabled {
				continue
			}

			if apiSources[i].FeedURL != nil && *apiSources[i].FeedURL != "" {
				continue
			}

			undiscovered = append(undiscovered, feed.UndiscoveredSource{
				SourceID: apiSources[i].ID,
				BaseURL:  apiSources[i].URL,
			})
		}

		log.Info("Listed undiscovered sources",
			infralogger.Int("total_sources", len(apiSources)),
			infralogger.Int("undiscovered", len(undiscovered)))

		return undiscovered, nil
	}
}

// createFrontierWorkerPool creates a frontier worker pool if the fetcher is enabled.
// Returns nil if the fetcher is disabled.
func createFrontierWorkerPool(
	deps *CommandDeps,
	db *DatabaseComponents,
	storageComponents *StorageComponents,
) *fetcher.WorkerPool {
	fetcherCfg := deps.Config.GetFetcherConfig()
	if !fetcherCfg.Enabled {
		deps.Logger.Info("Frontier worker pool disabled")
		return nil
	}

	claimer := &frontierClaimerAdapter{repo: db.FrontierRepo}
	hostUpdater := &hostUpdaterAdapter{repo: db.HostStateRepo}

	var checkRedirect func(*http.Request, []*http.Request) error
	if fetcherCfg.FollowRedirects {
		checkRedirect = fetcher.RedirectPolicy(fetcherCfg.MaxRedirects)
	} else {
		checkRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	httpClient := &http.Client{
		Timeout:       fetcherCfg.RequestTimeout,
		CheckRedirect: checkRedirect,
	}
	robots := fetcher.NewRobotsChecker(httpClient, fetcherCfg.UserAgent, 0)
	extractor := fetcher.NewContentExtractor()

	smCfg := deps.Config.GetSourceManagerConfig()
	authCfg := deps.Config.GetAuthConfig()
	apiClient := apiclient.NewClient(
		apiclient.WithBaseURL(smCfg.URL+"/api/v1/sources"),
		apiclient.WithJWTSecret(authCfg.JWTSecret),
	)

	rawIndexer := crawlstorage.NewRawContentIndexer(storageComponents.Storage, deps.Logger)
	indexer := &contentIndexerAdapter{
		indexer:   rawIndexer,
		apiClient: apiClient,
	}

	wpLogger := &logAdapter{log: deps.Logger}

	cfg := fetcher.WorkerPoolConfig{
		WorkerCount:     fetcherCfg.WorkerCount,
		UserAgent:       fetcherCfg.UserAgent,
		MaxRetries:      fetcherCfg.MaxRetries,
		ClaimRetryDelay: fetcherCfg.ClaimRetryDelay,
		RequestTimeout:  fetcherCfg.RequestTimeout,
	}

	deps.Logger.Info("Frontier worker pool created",
		infralogger.Int("worker_count", fetcherCfg.WorkerCount))

	return fetcher.NewWorkerPool(claimer, hostUpdater, robots, extractor, indexer, wpLogger, cfg)
}

// logAdapter adapts infralogger.Logger to the feed.Logger and fetcher.WorkerLogger interfaces.
// Both interfaces have identical signatures: Info(msg, ...any) and Error(msg, ...any).
type logAdapter struct {
	log infralogger.Logger
}

func (a *logAdapter) Info(msg string, fields ...any) {
	a.log.Info(msg, toInfraFields(fields)...)
}

func (a *logAdapter) Error(msg string, fields ...any) {
	a.log.Error(msg, toInfraFields(fields)...)
}

// toInfraFields converts key-value pairs to infralogger fields.
// The feed package passes fields as alternating key, value pairs.
func toInfraFields(fields []any) []infralogger.Field {
	fieldPairSize := 2
	result := make([]infralogger.Field, 0, len(fields)/fieldPairSize)

	for i := 0; i+1 < len(fields); i += fieldPairSize {
		key, ok := fields[i].(string)
		if !ok {
			continue
		}

		result = append(result, infralogger.Any(key, fields[i+1]))
	}

	return result
}
