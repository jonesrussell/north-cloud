package crawler

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	colly "github.com/gocolly/colly/v2"
	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

// Start begins the crawling process for a given source by ID.
// Refactored to use component-based architecture for better separation of concerns.
func (c *Crawler) Start(ctx context.Context, sourceID string) error {
	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Starting crawler",
		logs.String("source_id", sourceID),
		logs.Bool("debug_enabled", c.cfg.Debug),
	)

	// Reset components for new execution (supports concurrent jobs)
	c.lifecycle.Reset()
	c.signals.Reset()

	// Initialize start URL hash map if nil (first execution)
	if c.startURLHashesMu == nil {
		c.startURLHashesMu = &sync.RWMutex{}
	}
	c.startURLHashesMu.Lock()
	if c.startURLHashes == nil {
		c.startURLHashes = make(map[string]string)
	}
	c.startURLHashesMu.Unlock()

	// Start cleanup goroutine
	c.signals.StartCleanupGoroutine(ctx, c.cleanupResources)

	// Ensure abort signal is sent on exit
	defer c.signals.SignalAbort()
	defer c.clearCrawlContext()

	// Validate and setup
	source, err := c.validateAndSetup(ctx, sourceID)
	if err != nil {
		return err
	}

	// Create channel to signal when initial page is fully processed (OnScraped fired)
	// This ensures all OnHTML callbacks have queued their links before Wait() starts
	initialPageReady := make(chan struct{})
	initialPageURL := source.URL
	initialPageScraped := &atomic.Bool{}
	initialPageLinkCount := &atomic.Int64{}

	// Set up callbacks for initial page tracking
	c.setupInitialPageTracking(
		initialPageURL,
		initialPageReady,
		initialPageScraped,
		initialPageLinkCount,
	)

	// Visit the source URL
	if visitErr := c.collector.Visit(source.URL); visitErr != nil {
		return fmt.Errorf("failed to visit source URL: %w", visitErr)
	}

	// Wait for initial page to be fully processed (OnScraped fired) before starting Wait()
	// This ensures all OnHTML callbacks have queued their links
	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Waiting for initial page processing")
	select {
	case <-initialPageReady:
		c.GetJobLogger().Debug(logs.CategoryLifecycle, "Initial page processing completed")
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeoutWarningInterval):
		c.GetJobLogger().Warn(logs.CategoryLifecycle, "Timeout waiting for initial page processing")
	}

	// Wait for collector to complete
	waitDone := make(chan struct{})
	go func() {
		c.collector.Wait()
		close(waitDone)
	}()

	// Wait with context cancellation support
	select {
	case <-waitDone:
		c.GetJobLogger().Info(logs.CategoryLifecycle, "Collector finished", logs.String("source_id", sourceID))
	case <-ctx.Done():
		c.GetJobLogger().Info(logs.CategoryLifecycle, "Context cancelled, aborting", logs.String("source_id", sourceID))
		c.signals.SignalAbort()
		// Give collector a moment to finish after abort
		select {
		case <-waitDone:
		case <-time.After(collectorTimeoutDuration):
			c.GetJobLogger().Warn(logs.CategoryLifecycle, "Collector did not finish after cancellation", logs.String("source_id", sourceID))
		}
		return ctx.Err()
	}

	// Signal abort to cleanup goroutine
	// Note: We don't wait for it to finish as it's designed to run periodically
	// and may be executing cleanup operations. It will exit on the next iteration.
	c.signals.SignalAbort()

	// Stop the crawler state
	c.state.Stop()

	// Signal completion
	c.lifecycle.SignalDone()

	return nil
}

// validateAndSetup validates the source by ID and sets up the collector.
// Fetches the source once and stores it in CrawlContext for link handler reuse.
func (c *Crawler) validateAndSetup(ctx context.Context, sourceID string) (*configtypes.Source, error) {
	// Validate source by ID (single fetch per crawl)
	source, err := c.sources.ValidateSourceByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to validate source: %w", err)
	}

	// Pre-crawl redirect detection: abort early if the source URL redirects
	// to a different domain that is not in AllowedDomains.
	if redirectErr := c.checkRedirect(ctx, source); redirectErr != nil {
		return nil, fmt.Errorf("pre-crawl redirect check: %w", redirectErr)
	}

	// Cache source config for link handler (avoids repeated ValidateSourceByID calls per link)
	c.crawlContextMu.Lock()
	c.crawlContext = &CrawlContext{
		SourceID:        sourceID,
		Source:          source,
		ArticlePatterns: compileArticlePatterns(source.ArticleURLPatterns),
	}
	c.crawlContextMu.Unlock()

	// Set up collector
	if setupErr := c.setupCollector(ctx, source); setupErr != nil {
		return nil, fmt.Errorf("failed to setup collector: %w", setupErr)
	}

	// Set up collector callbacks (discovery, article detection, extraction)
	c.setupCallbacks(ctx)

	// Start the crawler state
	c.state.Start(ctx, sourceID)

	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Waiting for collector",
		logs.String("source_id", sourceID),
		logs.URL(source.URL),
	)

	return source, nil
}

// setupInitialPageTracking sets up callbacks to track the initial page processing
// and link discovery for diagnostic purposes.
func (c *Crawler) setupInitialPageTracking(
	initialPageURL string,
	initialPageReady chan struct{},
	initialPageScraped *atomic.Bool,
	initialPageLinkCount *atomic.Int64,
) {
	// Set up OnScraped callback to signal when initial page is done
	// This must be set up before Visit() is called
	// Note: This is in addition to the OnScraped callback in setupCallbacks
	c.collector.OnScraped(func(r *colly.Response) {
		if c.isInitialPage(r.Request.URL.String(), initialPageURL) {
			c.handleInitialPageScraped(
				initialPageURL,
				initialPageReady,
				initialPageScraped,
				initialPageLinkCount,
			)
		}
	})

	// Track link discoveries on the initial page
	// Note: We set up a separate OnHTML callback here to track the initial page link count
	// This is in addition to the callback in setupCallbacks, but we only increment the counter,
	// not call HandleLink again (to avoid double processing)
	// The actual link handling is done by the callback in setupCallbacks
	c.collector.OnHTML("a[href]", func(e *colly.HTMLElement) {
		requestURL := e.Request.URL.String()
		if c.isInitialPage(requestURL, initialPageURL) {
			initialPageLinkCount.Add(1)
		}
		// Note: Don't call HandleLink here - it's already called by the callback in setupCallbacks
		// This callback is only for counting links on the initial page
	})
}

// isInitialPage checks if a URL matches the initial page URL,
// handling potential trailing slash differences.
func (c *Crawler) isInitialPage(requestURL, initialPageURL string) bool {
	return requestURL == initialPageURL ||
		requestURL+"/" == initialPageURL ||
		requestURL == initialPageURL+"/"
}

// handleInitialPageScraped handles the initial page scraped event,
// logging link counts and warnings if needed.
func (c *Crawler) handleInitialPageScraped(
	initialPageURL string,
	initialPageReady chan struct{},
	initialPageScraped *atomic.Bool,
	initialPageLinkCount *atomic.Int64,
) {
	// Only signal once
	if !initialPageScraped.Load() {
		if initialPageScraped.CompareAndSwap(false, true) {
			linkCount := initialPageLinkCount.Load()
			c.GetJobLogger().Info(logs.CategoryQueue, "Initial page scraped",
				logs.URL(initialPageURL),
				logs.Int64("links_discovered", linkCount),
			)

			// Warn if no links were discovered on the initial page
			if linkCount == 0 {
				c.logWarnNoLinksDiscovered(initialPageURL)
			}

			close(initialPageReady)
		}
	}
}

// logWarnNoLinksDiscovered logs a warning when no links are discovered on the initial page.
func (c *Crawler) logWarnNoLinksDiscovered(initialPageURL string) {
	c.GetJobLogger().Warn(logs.CategoryQueue, "No links discovered on initial page",
		logs.URL(initialPageURL),
	)
}

// cleanupResources performs periodic cleanup of crawler resources
func (c *Crawler) cleanupResources() {
	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Cleaning up crawler resources")

	// Clean up processors
	for _, p := range c.processors {
		if cleaner, ok := p.(interface{ Cleanup() }); ok {
			cleaner.Cleanup()
		}
	}

	// Clean up archiver
	if c.archiver != nil {
		c.GetJobLogger().Debug(logs.CategoryLifecycle, "Closing MinIO archiver")
		if err := c.archiver.Close(); err != nil {
			c.GetJobLogger().Error(logs.CategoryError, "Failed to close archiver", logs.Err(err))
		}
	}

	// Clean up state
	c.state.Reset()

	c.GetJobLogger().Debug(logs.CategoryLifecycle, "Finished cleaning up crawler resources")
}
