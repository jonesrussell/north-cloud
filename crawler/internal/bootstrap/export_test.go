package bootstrap

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	crawlerintevents "github.com/jonesrussell/north-cloud/crawler/internal/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	infragin "github.com/jonesrussell/north-cloud/infrastructure/gin"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/sse"
)

// MapExtractedToRawContentForTest exposes mapExtractedToRawContent for testing.
func MapExtractedToRawContentForTest(
	content *fetcher.ExtractedContent,
	sourceName string,
	logger infralogger.Logger,
) *storage.RawContent {
	return mapExtractedToRawContent(content, sourceName, logger)
}

// ParsePublishedDateForTest exposes parsePublishedDate for testing.
func ParsePublishedDateForTest(raw string) (time.Time, bool) {
	return parsePublishedDate(raw)
}

// NormalizeLogLevel exposes normalizeLogLevel for testing.
var NormalizeLogLevel = normalizeLogLevel

// ErrRedisDisabledVar exposes ErrRedisDisabled for testing.
var ErrRedisDisabledVar = ErrRedisDisabled

// DatabaseConfigFromInterface exposes databaseConfigFromInterface for testing.
func DatabaseConfigFromInterface(cfg *dbconfig.Config) database.Config {
	return databaseConfigFromInterface(cfg)
}

// RunStaleURLRecoveryForTest exposes runStaleURLRecovery for testing.
func RunStaleURLRecoveryForTest(
	ctx context.Context,
	repo StaleURLRecoverer,
	log infralogger.Logger,
	staleTimeout, checkInterval time.Duration,
) {
	runStaleURLRecovery(ctx, repo, log, staleTimeout, checkInterval)
}

// ToInfraFieldsForTest exposes toInfraFields for testing.
func ToInfraFieldsForTest(fields []any) []infralogger.Field {
	return toInfraFields(fields)
}

// DiscoveryConfigAdapterForTest is an interface that exposes discoveryConfigAdapter methods.
type DiscoveryConfigAdapterForTest interface {
	Allowlist() []string
	Blocklist() []string
	MaxNewCandidatesPerRun() int
	GlobalCrawlBudgetPerDay() int
}

// NewDiscoveryConfigAdapter creates a discoveryConfigAdapter for testing.
func NewDiscoveryConfigAdapter(cfg *config.DiscoveryConfig) DiscoveryConfigAdapterForTest {
	return &discoveryConfigAdapter{cfg: cfg}
}

// NewLogAdapterForTest creates a logAdapter for testing.
func NewLogAdapterForTest(log infralogger.Logger) *logAdapter {
	return &logAdapter{log: log}
}

// LogAdapterInfo exposes logAdapter.Info for testing.
func LogAdapterInfo(a *logAdapter, msg string, fields ...any) {
	a.Info(msg, fields...)
}

// LogAdapterWarn exposes logAdapter.Warn for testing.
func LogAdapterWarn(a *logAdapter, msg string, fields ...any) {
	a.Warn(msg, fields...)
}

// LogAdapterError exposes logAdapter.Error for testing.
func LogAdapterError(a *logAdapter, msg string, fields ...any) {
	a.Error(msg, fields...)
}

// BuildProxiedHTTPClientForTest exposes buildProxiedHTTPClient for testing.
func BuildProxiedHTTPClientForTest(timeout time.Duration, log infralogger.Logger) *http.Client {
	return buildProxiedHTTPClient(nil, timeout, log)
}

// RunFrontierStatsLoggerForTest exposes runFrontierStatsLogger for testing.
func RunFrontierStatsLoggerForTest(ctx context.Context, repo api.FrontierRepoForHandler, log infralogger.Logger) {
	runFrontierStatsLogger(ctx, repo, log)
}

// BackgroundCancelsForTest is a type alias for backgroundCancels.
type BackgroundCancelsForTest = backgroundCancels

// SetupEventConsumerForTest exposes SetupEventConsumer for testing.
func SetupEventConsumerForTest(
	deps *CommandDeps,
	jobRepo *database.JobRepository,
	processedEventsRepo *database.ProcessedEventsRepository,
) *crawlerintevents.Consumer {
	return SetupEventConsumer(deps, jobRepo, processedEventsRepo)
}

// BuildSharedProxyPoolIsNilForTest returns true if buildSharedProxyPool returns nil.
func BuildSharedProxyPoolIsNilForTest(deps *CommandDeps) bool {
	return buildSharedProxyPool(deps) == nil
}

// DiscoveryFrontierNopForTest is a type alias for discoveryFrontierNop.
type DiscoveryFrontierNopForTest = discoveryFrontierNop

// ShutdownForTest exposes Shutdown for testing with a real server.
func ShutdownForTest(
	log infralogger.Logger,
	server *infragin.Server,
	bg BackgroundCancelsForTest,
	sig os.Signal,
) error {
	return Shutdown(log, server, nil, nil, nil, nil, bg, sig)
}

// SetupSSEForTest exposes setupSSE for testing.
func SetupSSEForTest(deps *CommandDeps) (sseBrokerResult sse.Broker) {
	broker, _, _ := setupSSE(deps)
	return broker
}

// StartBackgroundWorkersForTest exposes startBackgroundWorkers for testing.
func StartBackgroundWorkersForTest(deps *CommandDeps, sc *ServiceComponents) BackgroundCancelsForTest {
	return startBackgroundWorkers(deps, sc)
}

// NewBackgroundCancelsWithAll creates a backgroundCancels with all cancel functions set.
func NewBackgroundCancelsWithAll(
	feedPoller, feedDiscovery, workerPool, frontierStats, staleRecovery context.CancelFunc,
) BackgroundCancelsForTest {
	return backgroundCancels{
		feedPollerCancel:    feedPoller,
		feedDiscoveryCancel: feedDiscovery,
		workerPoolCancel:    workerPool,
		frontierStatsCancel: frontierStats,
		staleRecoveryCancel: staleRecovery,
	}
}

// ShutdownFullForTest exposes Shutdown with all optional components.
func ShutdownFullForTest(
	log infralogger.Logger,
	server *infragin.Server,
	sseBroker sse.Broker,
	logService logs.Service,
	eventConsumer *crawlerintevents.Consumer,
	bg BackgroundCancelsForTest,
	sig os.Signal,
) error {
	return Shutdown(log, server, nil, sseBroker, logService, eventConsumer, bg, sig)
}

// SetupSSEFullForTest exposes setupSSE and returns all three components.
type SSEComponentsForTest struct {
	Broker    sse.Broker
	Handler   any // *api.SSEHandler
	Publisher any // *scheduler.SSEPublisher
}

// SetupSSEFullForTest creates SSE components for testing.
func SetupSSEFullForTest(deps *CommandDeps) SSEComponentsForTest {
	broker, handler, publisher := setupSSE(deps)
	return SSEComponentsForTest{
		Broker:    broker,
		Handler:   handler,
		Publisher: publisher,
	}
}

// SetupMigratorForTest exposes SetupMigrator for testing.
func SetupMigratorForTest(deps *CommandDeps) any {
	return SetupMigrator(deps, nil)
}

// SetupLogServiceForTest exposes setupLogService for testing.
type LogServiceResultForTest = LogServiceResult

// SetupLogServiceForTest creates a log service for testing.
func SetupLogServiceForTest(deps *CommandDeps, broker sse.Broker) LogServiceResultForTest {
	return setupLogService(deps, broker, nil)
}

// CreateFeedPollerNilForTest tests the disabled path of createFeedPoller.
func CreateFeedPollerIsNilForTest(deps *CommandDeps) bool {
	poller, _ := createFeedPoller(deps, nil, nil, nil)
	return poller == nil
}

// CreateFeedDiscovererIsNilForTest tests the disabled path of createFeedDiscoverer.
func CreateFeedDiscovererIsNilForTest(deps *CommandDeps) bool {
	discoverer, _ := createFeedDiscoverer(deps, nil)
	return discoverer == nil
}

// CreateFrontierWorkerPoolIsNilForTest tests the disabled path of createFrontierWorkerPool.
func CreateFrontierWorkerPoolIsNilForTest(deps *CommandDeps) bool {
	pool := createFrontierWorkerPool(deps, nil, nil, nil)
	return pool == nil
}

// RunUntilInterruptForTest exposes RunUntilInterrupt for testing.
func RunUntilInterruptForTest(
	log infralogger.Logger,
	server *infragin.Server,
	bg BackgroundCancelsForTest,
	errChan <-chan error,
) error {
	return RunUntilInterrupt(log, server, nil, nil, nil, nil, bg, errChan)
}
