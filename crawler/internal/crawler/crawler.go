// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"sync"
	"time"

	colly "github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"
	"github.com/jonesrussell/north-cloud/crawler/internal/archive"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/metrics"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	storagetypes "github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

// Core Interfaces

// CrawlerInterface defines the core functionality of a crawler.
type CrawlerInterface interface {
	// Start begins crawling from the given source by ID.
	Start(ctx context.Context, sourceID string) error
	// Stop gracefully stops the crawler.
	Stop(ctx context.Context) error
	// Subscribe adds a handler for crawler events.
	Subscribe(handler events.EventHandler)
	// GetMetrics returns the current crawler metrics.
	GetMetrics() *metrics.Metrics
}

// Config defines the configuration for a crawler.
type Config struct {
	// MaxDepth is the maximum depth to crawl.
	MaxDepth int
	// RateLimit is the delay between requests.
	RateLimit time.Duration
	// Parallelism is the number of concurrent requests.
	Parallelism int
	// AllowedDomains are the domains that can be crawled.
	AllowedDomains []string
	// UserAgent is the user agent string to use.
	UserAgent string
}

// CrawlerState manages the runtime state of a crawler.
type CrawlerState interface {
	// IsRunning returns whether the crawler is running.
	IsRunning() bool
	// StartTime returns when the crawler started.
	StartTime() time.Time
	// CurrentSource returns the current source being crawled.
	CurrentSource() string
	// Context returns the crawler's context.
	Context() context.Context
	// Cancel cancels the crawler's context.
	Cancel()
	// Stop stops the crawler.
	Stop()
}

// CrawlerMetrics tracks crawler statistics.
type CrawlerMetrics interface {
	// IncrementProcessed increments the processed count.
	IncrementProcessed()
	// IncrementError increments the error count.
	IncrementError()
	// GetProcessedCount returns the number of processed items.
	GetProcessedCount() int64
	// GetErrorCount returns the number of errors.
	GetErrorCount() int64
	// GetStartTime returns when tracking started.
	GetStartTime() time.Time
	// GetLastProcessedTime returns the time of the last processed item.
	GetLastProcessedTime() time.Time
	// GetProcessingDuration returns the total processing duration.
	GetProcessingDuration() time.Duration
	// Update updates the metrics with new values.
	Update(startTime time.Time, processed int64, errors int64)
	// Reset resets all metrics to zero.
	Reset()
}

// ContentProcessor handles content processing.
type ContentProcessor interface {
	// ProcessHTML processes HTML content.
	ProcessHTML(ctx context.Context, element *colly.HTMLElement) error
	// CanProcess returns whether the processor can handle the content.
	CanProcess(contentType string) bool
	// ContentType returns the content type this processor handles.
	ContentType() string
}

// Archiver handles HTML archiving to object storage.
type Archiver interface {
	// Archive archives HTML to storage
	Archive(ctx context.Context, task *archive.UploadTask) error
	// HealthCheck verifies archiver connectivity
	HealthCheck(ctx context.Context) error
	// Close gracefully shuts down the archiver
	Close() error
}

// Interface defines the complete crawler interface.
type Interface interface {
	// Embed the core crawler interface
	CrawlerInterface

	// SetRateLimit sets the rate limit for the crawler
	SetRateLimit(duration time.Duration) error
	// SetCollector sets the collector for the crawler
	SetCollector(collector *colly.Collector)
	// GetIndexManager returns the index manager
	GetIndexManager() storagetypes.IndexManager
	// Wait waits for the crawler to complete
	Wait() error
	// GetLogger returns the logger
	GetLogger() infralogger.Logger
	// GetSource returns the source
	GetSource() sources.Interface
	// GetProcessors returns the processors
	GetProcessors() []content.Processor
	// Done returns a channel that's closed when the crawler is done
	Done() <-chan struct{}
	// SetJobLogger sets the job logger for the current job execution
	SetJobLogger(logger logs.JobLogger)
	// GetJobLogger returns the current job logger
	GetJobLogger() logs.JobLogger
	// GetStartURLHashes returns the hashes captured during the last crawl
	GetStartURLHashes() map[string]string
	// GetHashTracker returns the hash tracker for adaptive scheduling
	GetHashTracker() *adaptive.HashTracker
}

const (
	// collectorTimeoutDuration is the timeout for waiting for collector to finish after cancellation
	collectorTimeoutDuration = 2 * time.Second
	// timeoutWarningInterval is the interval for logging timeout warnings
	timeoutWarningInterval = 30 * time.Second
)

// Crawler implements the Processor interface for web crawling.
// Refactored to use focused component pattern for better SRP compliance.
type Crawler struct {
	logger              infralogger.Logger
	jobLogger           logs.JobLogger
	collector           *colly.Collector // Link collector for discovery (multi-collector pattern)
	detailCollector     *colly.Collector // Detail collector for content extraction (multi-collector pattern)
	bus                 *events.EventBus
	indexManager        storagetypes.IndexManager
	sources             sources.Interface
	rawContentProcessor content.Processor
	state               *State
	processors          []content.Processor
	linkHandler         *LinkHandler
	htmlProcessor       *HTMLProcessor
	cfg                 *crawler.Config
	archiver            Archiver      // HTML archiver for MinIO storage
	redisClient         *redis.Client // Redis client for Colly storage (optional)

	// Adaptive scheduling: stores hashes of start URL responses
	startURLHashes   map[string]string // URL -> SHA-256 hash
	startURLHashesMu sync.RWMutex
	hashTracker      *adaptive.HashTracker // Redis-backed hash tracker (optional)

	// Extracted components for better separation of concerns
	lifecycle *LifecycleManager
	signals   *SignalCoordinator

	// Per-run cached source config (set in validateAndSetup, cleared when Start returns)
	crawlContext   *CrawlContext
	crawlContextMu sync.RWMutex
}

var _ Interface = (*Crawler)(nil)
var _ CrawlerInterface = (*Crawler)(nil)
var _ CrawlerMetrics = (*Crawler)(nil)

// Getter Methods
// -------------

// GetLogger returns the logger.
func (c *Crawler) GetLogger() infralogger.Logger {
	return c.logger
}

// GetSource returns the source.
func (c *Crawler) GetSource() sources.Interface {
	return c.sources
}

// GetIndexManager returns the index manager.
func (c *Crawler) GetIndexManager() storagetypes.IndexManager {
	return c.indexManager
}

// SetJobLogger sets the job logger for the current job execution.
// Should be called before Start() for each job.
func (c *Crawler) SetJobLogger(logger logs.JobLogger) {
	c.jobLogger = logger
}

// GetJobLogger returns the current job logger, or NoopJobLogger if not set.
func (c *Crawler) GetJobLogger() logs.JobLogger {
	if c.jobLogger == nil {
		return logs.NoopJobLogger()
	}
	return c.jobLogger
}

// Metrics Management
// -----------------

// GetMetrics returns the crawler metrics.
func (c *Crawler) GetMetrics() *metrics.Metrics {
	return &metrics.Metrics{
		ProcessedCount:     c.state.GetProcessedCount(),
		ErrorCount:         c.state.GetErrorCount(),
		LastProcessedTime:  c.state.GetLastProcessedTime(),
		ProcessingDuration: c.state.GetProcessingDuration(),
	}
}

// IncrementProcessed increments the processed count.
func (c *Crawler) IncrementProcessed() {
	c.state.IncrementProcessed()
}

// IncrementError increments the error count.
func (c *Crawler) IncrementError() {
	c.state.IncrementError()
}

// GetProcessedCount returns the number of processed items.
func (c *Crawler) GetProcessedCount() int64 {
	return c.state.GetProcessedCount()
}

// GetErrorCount returns the number of errors.
func (c *Crawler) GetErrorCount() int64 {
	return c.state.GetErrorCount()
}

// GetLastProcessedTime returns the time of the last processed item.
func (c *Crawler) GetLastProcessedTime() time.Time {
	return c.state.GetLastProcessedTime()
}

// GetProcessingDuration returns the total processing duration.
func (c *Crawler) GetProcessingDuration() time.Duration {
	return c.state.GetProcessingDuration()
}

// GetStartTime returns when tracking started.
func (c *Crawler) GetStartTime() time.Time {
	return c.state.GetStartTime()
}

// Update updates the metrics with new values.
func (c *Crawler) Update(startTime time.Time, processed, errorCount int64) {
	c.state.Update(startTime, processed, errorCount)
}

// Reset resets all metrics to zero.
func (c *Crawler) Reset() {
	c.state.Reset()
}

// getCrawlContext returns the current crawl context (cached source config). Safe for concurrent reads.
func (c *Crawler) getCrawlContext() *CrawlContext {
	c.crawlContextMu.RLock()
	defer c.crawlContextMu.RUnlock()
	return c.crawlContext
}

// clearCrawlContext clears the cached crawl context. Called when Start returns.
func (c *Crawler) clearCrawlContext() {
	c.crawlContextMu.Lock()
	defer c.crawlContextMu.Unlock()
	c.crawlContext = nil
}

// GetStartURLHashes returns the hashes captured during the last crawl.
func (c *Crawler) GetStartURLHashes() map[string]string {
	c.startURLHashesMu.RLock()
	defer c.startURLHashesMu.RUnlock()
	result := make(map[string]string, len(c.startURLHashes))
	for k, v := range c.startURLHashes {
		result[k] = v
	}
	return result
}

// GetHashTracker returns the hash tracker for adaptive scheduling.
func (c *Crawler) GetHashTracker() *adaptive.HashTracker {
	return c.hashTracker
}
