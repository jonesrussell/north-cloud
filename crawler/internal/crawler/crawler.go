// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	colly "github.com/gocolly/colly/v2"
	"github.com/jonesrussell/gocrawl/internal/config/crawler"
	configtypes "github.com/jonesrussell/gocrawl/internal/config/types"
	"github.com/jonesrussell/gocrawl/internal/content"
	"github.com/jonesrussell/gocrawl/internal/crawler/events"
	"github.com/jonesrussell/gocrawl/internal/domain"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/metrics"
	"github.com/jonesrussell/gocrawl/internal/sources"
	storagetypes "github.com/jonesrussell/gocrawl/internal/storage/types"
)

// Core Interfaces

// CrawlerInterface defines the core functionality of a crawler.
type CrawlerInterface interface {
	// Start begins crawling from the given source.
	Start(ctx context.Context, sourceName string) error
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

// Interface defines the complete crawler interface.
type Interface interface {
	// Embed the core crawler interface
	CrawlerInterface

	// SetRateLimit sets the rate limit for the crawler
	SetRateLimit(duration time.Duration) error
	// SetMaxDepth sets the maximum depth for the crawler
	SetMaxDepth(depth int)
	// SetCollector sets the collector for the crawler
	SetCollector(collector *colly.Collector)
	// GetIndexManager returns the index manager
	GetIndexManager() storagetypes.IndexManager
	// Wait waits for the crawler to complete
	Wait() error
	// GetLogger returns the logger
	GetLogger() logger.Interface
	// GetSource returns the source
	GetSource() sources.Interface
	// GetProcessors returns the processors
	GetProcessors() []content.Processor
	// GetArticleChannel returns the article channel
	GetArticleChannel() chan *domain.Article
	// Done returns a channel that's closed when the crawler is done
	Done() <-chan struct{}
}

const (
	// collectorTimeoutDuration is the timeout for waiting for collector to finish after cancellation
	collectorTimeoutDuration = 2 * time.Second
	// collectorCompletionTimeout is the timeout for waiting for collector to finish in normal case
	collectorCompletionTimeout = 5 * time.Minute
	// cleanupTimeoutDuration is the timeout for waiting for cleanup goroutine to finish
	cleanupTimeoutDuration = 5 * time.Second
	// nilString is the string representation of nil
	nilString = "nil"
	// timeoutWarningInterval is the interval for logging timeout warnings
	timeoutWarningInterval = 30 * time.Second
)

// Crawler implements the Processor interface for web crawling.
type Crawler struct {
	logger           logger.Interface
	collector        *colly.Collector
	bus              *events.EventBus
	indexManager     storagetypes.IndexManager
	sources          sources.Interface
	articleProcessor content.Processor
	pageProcessor    content.Processor
	state            *State
	done             chan struct{}
	doneOnce         sync.Once // Ensures done channel is only closed once
	wg               sync.WaitGroup
	articleChannel   chan *domain.Article
	processors       []content.Processor
	linkHandler      *LinkHandler
	htmlProcessor    *HTMLProcessor
	cfg              *crawler.Config
	abortChan        chan struct{} // Channel to signal abort
	maxDepthOverride int32         // Override for source's max_depth (0 means use source default), accessed atomically
}

var _ Interface = (*Crawler)(nil)
var _ CrawlerInterface = (*Crawler)(nil)
var _ CrawlerMetrics = (*Crawler)(nil)

// Core Crawler Methods
// -------------------

// Start begins the crawling process for a given source.
func (c *Crawler) Start(ctx context.Context, sourceName string) error {
	c.logger.Debug("Starting crawler",
		"source", sourceName,
		"debug_enabled", c.cfg.Debug,
	)

	// Create a new done channel for this execution to support concurrent jobs
	c.done = make(chan struct{})
	c.doneOnce = sync.Once{}

	// Initialize abort channel
	c.abortChan = make(chan struct{})
	var abortChanOnce sync.Once

	// Start cleanup goroutine
	cleanupDone := c.startCleanupGoroutine(ctx)

	// Ensure abortChan is closed on exit
	defer func() {
		abortChanOnce.Do(func() {
			close(c.abortChan)
		})
	}()

	// Validate and setup
	source, err := c.validateAndSetup(ctx, sourceName)
	if err != nil {
		return err
	}

	// Visit the source URL
	if visitErr := c.collector.Visit(source.URL); visitErr != nil {
		return fmt.Errorf("failed to visit source URL: %w", visitErr)
	}

	// Wait for collector to complete
	if waitErr := c.waitForCollector(ctx, sourceName, &abortChanOnce); waitErr != nil {
		return waitErr
	}

	// Signal cleanup goroutine to stop
	abortChanOnce.Do(func() {
		close(c.abortChan)
	})

	// Wait for cleanup goroutine to finish
	c.waitForCleanup(ctx, cleanupDone)

	// Stop the crawler state
	c.state.Stop()

	// Signal completion by closing the done channel
	c.doneOnce.Do(func() {
		close(c.done)
	})

	return nil
}

// startCleanupGoroutine starts a goroutine for periodic cleanup.
func (c *Crawler) startCleanupGoroutine(ctx context.Context) chan struct{} {
	cleanupDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(c.cfg.CleanupInterval)
		defer ticker.Stop()
		defer close(cleanupDone)

		for {
			select {
			case <-ctx.Done():
				return
			case <-c.abortChan:
				return
			case <-ticker.C:
				c.cleanupResources()
			}
		}
	}()
	return cleanupDone
}

// validateAndSetup validates the source and sets up the collector.
func (c *Crawler) validateAndSetup(ctx context.Context, sourceName string) (*configtypes.Source, error) {
	// Validate source
	source, err := c.sources.ValidateSource(ctx, sourceName, c.indexManager)
	if err != nil {
		return nil, fmt.Errorf("failed to validate source: %w", err)
	}

	// Set up collector
	if setupErr := c.setupCollector(source); setupErr != nil {
		return nil, fmt.Errorf("failed to setup collector: %w", setupErr)
	}

	// Set up callbacks
	c.setupCallbacks(ctx)

	// Start the crawler state
	c.state.Start(ctx, sourceName)

	c.logger.Debug("Starting to wait for collector to complete",
		"source", sourceName,
		"url", source.URL)

	return source, nil
}

// waitForCollector waits for the collector to complete, handling timeouts and cancellations.
func (c *Crawler) waitForCollector(ctx context.Context, sourceName string, abortChanOnce *sync.Once) error {
	waitDone := make(chan struct{})
	waitStartTime := time.Now()

	// Start collector wait goroutine
	go c.runCollectorWait(waitDone, sourceName, waitStartTime)

	// Start timeout warning goroutine
	timeoutTicker := time.NewTicker(timeoutWarningInterval)
	defer timeoutTicker.Stop()
	go c.logTimeoutWarnings(ctx, waitDone, waitStartTime, sourceName, timeoutTicker)

	// Wait for completion, cancellation, or timeout
	return c.handleCollectorCompletion(ctx, waitDone, waitStartTime, sourceName, abortChanOnce)
}

// runCollectorWait runs the collector wait in a goroutine.
func (c *Crawler) runCollectorWait(waitDone chan struct{}, sourceName string, waitStartTime time.Time) {
	c.logger.Debug("Calling collector.Wait()")
	c.collector.Wait()
	waitDuration := time.Since(waitStartTime)
	c.logger.Debug("Collector.Wait() completed",
		"duration", waitDuration,
		"source", sourceName)
	close(waitDone)
}

// logTimeoutWarnings logs warnings if the collector takes too long.
func (c *Crawler) logTimeoutWarnings(
	ctx context.Context,
	waitDone chan struct{},
	waitStartTime time.Time,
	sourceName string,
	ticker *time.Ticker,
) {
	timeoutWarningSent := &atomic.Bool{}
	for {
		select {
		case <-waitDone:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			waitDuration := time.Since(waitStartTime)
			if waitDuration > collectorCompletionTimeout/2 && !timeoutWarningSent.Load() {
				timeoutWarningSent.Store(true)
				c.logger.Warn("Collector is still processing requests",
					"duration", waitDuration,
					"source", sourceName,
					"processed_count", c.state.GetProcessedCount(),
					"note", "Wait() will return when all async requests complete. This is normal for sites with many links.")
			}
		}
	}
}

// handleCollectorCompletion handles the completion, cancellation, or timeout of the collector.
func (c *Crawler) handleCollectorCompletion(
	ctx context.Context,
	waitDone chan struct{},
	waitStartTime time.Time,
	sourceName string,
	abortChanOnce *sync.Once,
) error {
	select {
	case <-waitDone:
		waitDuration := time.Since(waitStartTime)
		c.logger.Info("Collector finished normally",
			"duration", waitDuration,
			"source", sourceName)
		return nil
	case <-ctx.Done():
		return c.handleContextCancellation(ctx, waitDone, waitStartTime, sourceName, abortChanOnce)
	case <-time.After(collectorCompletionTimeout):
		return c.handleCollectorTimeout(waitDone, waitStartTime, sourceName, abortChanOnce)
	}
}

// handleContextCancellation handles context cancellation.
func (c *Crawler) handleContextCancellation(
	ctx context.Context,
	waitDone chan struct{},
	waitStartTime time.Time,
	sourceName string,
	abortChanOnce *sync.Once,
) error {
	c.logger.Info("Context cancelled, aborting collector", "source", sourceName)
	abortChanOnce.Do(func() {
		close(c.abortChan)
	})

	select {
	case <-waitDone:
		waitDuration := time.Since(waitStartTime)
		c.logger.Info("Collector finished after abort",
			"duration", waitDuration,
			"source", sourceName)
	case <-time.After(collectorTimeoutDuration):
		waitDuration := time.Since(waitStartTime)
		c.logger.Warn("Collector did not finish within timeout after cancellation",
			"timeout", collectorTimeoutDuration,
			"duration", waitDuration,
			"source", sourceName)
	}
	return ctx.Err()
}

// handleCollectorTimeout handles collector timeout.
func (c *Crawler) handleCollectorTimeout(
	waitDone chan struct{},
	waitStartTime time.Time,
	sourceName string,
	abortChanOnce *sync.Once,
) error {
	waitDuration := time.Since(waitStartTime)
	c.logger.Error("Collector did not finish within completion timeout, forcing completion",
		"timeout", collectorCompletionTimeout,
		"duration", waitDuration,
		"source", sourceName,
		"error", "Collector appears to be hanging - proceeding with cleanup")

	abortChanOnce.Do(func() {
		close(c.abortChan)
	})

	select {
	case <-waitDone:
		c.logger.Info("Collector finished after timeout abort",
			"duration", time.Since(waitStartTime),
			"source", sourceName)
	case <-time.After(collectorTimeoutDuration):
		c.logger.Warn("Collector did not respond to abort, proceeding anyway",
			"source", sourceName)
	}
	return nil
}

// waitForCleanup waits for the cleanup goroutine to finish.
func (c *Crawler) waitForCleanup(ctx context.Context, cleanupDone chan struct{}) {
	select {
	case <-cleanupDone:
		c.logger.Debug("Cleanup goroutine finished")
	case <-ctx.Done():
		// Context cancelled, but we'll continue with cleanup
	case <-time.After(cleanupTimeoutDuration):
		c.logger.Warn("Cleanup goroutine did not finish within timeout")
	}
}

// cleanupResources performs periodic cleanup of crawler resources
func (c *Crawler) cleanupResources() {
	c.logger.Debug("Cleaning up crawler resources")

	// Clean up article channel
	select {
	case <-c.articleChannel: // Try to read one item
	default: // Channel is empty
	}

	// Clean up processors
	for _, p := range c.processors {
		if cleaner, ok := p.(interface{ Cleanup() }); ok {
			cleaner.Cleanup()
		}
	}

	// Clean up state
	c.state.Reset()

	c.logger.Debug("Finished cleaning up crawler resources")
}

// Stop stops the crawler.
func (c *Crawler) Stop(ctx context.Context) error {
	c.logger.Debug("Stopping crawler")
	if !c.state.IsRunning() {
		c.logger.Debug("Crawler already stopped")
		return nil
	}

	// Cancel the context
	c.state.Cancel()

	// Signal abort to all goroutines
	close(c.abortChan)

	// Wait for the collector to finish
	c.collector.Wait()

	// Create a done channel for the wait group
	waitDone := make(chan struct{})

	// Start a goroutine to wait for the wait group
	go func() {
		c.wg.Wait()
		close(waitDone)
	}()

	// Wait for either the wait group to finish or the context to be done
	select {
	case <-waitDone:
		c.state.Stop()
		c.cleanupResources() // Final cleanup
		c.logger.Debug("Crawler stopped successfully")
		return nil
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			c.logger.Warn("Crawler shutdown timed out",
				"timeout", ctx.Err())
		} else {
			c.logger.Warn("Crawler shutdown cancelled",
				"error", ctx.Err())
		}
		return ctx.Err()
	}
}

// Wait waits for the crawler to complete.
// Since Start() already waits for the collector to finish and closes the done channel,
// this method just waits for the done channel to be closed (which happens in Start()).
func (c *Crawler) Wait() error {
	// Wait for the done channel to be closed (Start() handles closing it)
	<-c.done
	return nil
}

// Done returns a channel that's closed when the crawler is done.
func (c *Crawler) Done() <-chan struct{} {
	return c.done
}

// IsRunning returns whether the crawler is running.
func (c *Crawler) IsRunning() bool {
	return c.state.IsRunning()
}

// Context returns the crawler's context.
func (c *Crawler) Context() context.Context {
	return c.state.Context()
}

// Cancel cancels the crawler's context.
func (c *Crawler) Cancel() {
	c.state.Cancel()
}

// State Management
// ---------------

// CurrentSource returns the current source being crawled.
func (c *Crawler) CurrentSource() string {
	return c.state.CurrentSource()
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

// Event Management
// ---------------

// Subscribe subscribes to crawler events.
func (c *Crawler) Subscribe(handler events.EventHandler) {
	c.bus.Subscribe(handler)
}

// Getter Methods
// -------------

// GetLogger returns the logger.
func (c *Crawler) GetLogger() logger.Interface {
	return c.logger
}

// GetSource returns the source.
func (c *Crawler) GetSource() sources.Interface {
	return c.sources
}

// GetArticleChannel returns the article channel.
func (c *Crawler) GetArticleChannel() chan *domain.Article {
	return c.articleChannel
}

// GetIndexManager returns the index manager.
func (c *Crawler) GetIndexManager() storagetypes.IndexManager {
	return c.indexManager
}
