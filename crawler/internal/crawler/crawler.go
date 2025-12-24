// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"time"

	colly "github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/content"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/metrics"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	storagetypes "github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
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
	// timeoutWarningInterval is the interval for logging timeout warnings
	timeoutWarningInterval = 30 * time.Second
)

// Crawler implements the Processor interface for web crawling.
// Refactored to use focused component pattern for better SRP compliance.
type Crawler struct {
	logger              logger.Interface
	collector           *colly.Collector
	bus                 *events.EventBus
	indexManager        storagetypes.IndexManager
	sources             sources.Interface
	rawContentProcessor content.Processor
	state               *State
	processors          []content.Processor
	linkHandler         *LinkHandler
	htmlProcessor       *HTMLProcessor
	cfg                 *crawler.Config
	maxDepthOverride    int32 // Override for source's max_depth (0 means use source default), accessed atomically

	// Extracted components for better separation of concerns
	lifecycle    *LifecycleManager
	signals      *SignalCoordinator
	monitor      *CollectorMonitor
}

var _ Interface = (*Crawler)(nil)
var _ CrawlerInterface = (*Crawler)(nil)
var _ CrawlerMetrics = (*Crawler)(nil)

// Core Crawler Methods
// -------------------

// Start begins the crawling process for a given source.
// Refactored to use component-based architecture for better separation of concerns.
func (c *Crawler) Start(ctx context.Context, sourceName string) error {
	c.logger.Debug("Starting crawler",
		"source", sourceName,
		"debug_enabled", c.cfg.Debug,
	)

	// Reset components for new execution (supports concurrent jobs)
	c.lifecycle.Reset()
	c.signals.Reset()

	// Start cleanup goroutine
	c.signals.StartCleanupGoroutine(ctx, c.cleanupResources)

	// Ensure abort signal is sent on exit
	defer c.signals.SignalAbort()

	// Validate and setup
	source, err := c.validateAndSetup(ctx, sourceName)
	if err != nil {
		return err
	}

	// Visit the source URL
	if visitErr := c.collector.Visit(source.URL); visitErr != nil {
		return fmt.Errorf("failed to visit source URL: %w", visitErr)
	}

	// Wait for collector to complete using monitor
	if waitErr := c.monitor.WaitForCompletion(ctx, sourceName); waitErr != nil {
		return waitErr
	}

	// Wait for cleanup to finish
	c.signals.WaitForCleanup(ctx, cleanupTimeoutDuration)

	// Stop the crawler state
	c.state.Stop()

	// Signal completion
	c.lifecycle.SignalDone()

	return nil
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

// cleanupResources performs periodic cleanup of crawler resources
func (c *Crawler) cleanupResources() {
	c.logger.Debug("Cleaning up crawler resources")

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
	c.signals.SignalAbort()

	// Wait for the collector to finish
	c.collector.Wait()

	// Wait for either the wait group to finish or the context to be done
	waitDone := c.lifecycle.WaitWithChannel()
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
	// Wait for the done channel to be closed (Start() handles closing it via lifecycle)
	<-c.lifecycle.Done()
	return nil
}

// Done returns a channel that's closed when the crawler is done.
func (c *Crawler) Done() <-chan struct{} {
	return c.lifecycle.Done()
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


// GetIndexManager returns the index manager.
func (c *Crawler) GetIndexManager() storagetypes.IndexManager {
	return c.indexManager
}
