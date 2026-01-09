package crawler

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	colly "github.com/gocolly/colly/v2"
	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
)

// Start begins the crawling process for a given source by ID.
// Refactored to use component-based architecture for better separation of concerns.
func (c *Crawler) Start(ctx context.Context, sourceID string) error {
	c.logger.Debug("Starting crawler",
		"source_id", sourceID,
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
	c.logger.Debug("Waiting for initial page processing to complete before starting Wait()")
	select {
	case <-initialPageReady:
		c.logger.Debug("Initial page processing completed, starting Wait()")
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(timeoutWarningInterval):
		c.logger.Warn("Timeout waiting for initial page processing, proceeding anyway")
	}

	// Wait for collector to complete
	// Colly's Wait() blocks until all queued requests are processed
	waitDone := make(chan struct{})
	go func() {
		c.collector.Wait()
		close(waitDone)
	}()

	// Wait with context cancellation support
	select {
	case <-waitDone:
		c.logger.Info("Collector finished", "source_id", sourceID)
	case <-ctx.Done():
		c.logger.Info("Context cancelled, aborting collector", "source_id", sourceID)
		c.signals.SignalAbort()
		// Give collector a moment to finish after abort
		select {
		case <-waitDone:
		case <-time.After(collectorTimeoutDuration):
			c.logger.Warn("Collector did not finish after cancellation", "source_id", sourceID)
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
func (c *Crawler) validateAndSetup(ctx context.Context, sourceID string) (*configtypes.Source, error) {
	// Validate source by ID
	source, err := c.sources.ValidateSourceByID(ctx, sourceID)
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
	c.state.Start(ctx, sourceID)

	c.logger.Debug("Starting to wait for collector to complete",
		"source_id", sourceID,
		"url", source.URL)

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
			c.logger.Debug("Initial page scraped, all links should be queued",
				"url", initialPageURL,
				"links_discovered", linkCount)

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
	const (
		noLinksSuggestion = "This may indicate: 1) Cloudflare/challenge page " +
			"(requires JavaScript), 2) Page has no links, 3) Links are dynamically " +
			"generated by JavaScript, 4) Links are blocked by robots.txt or domain restrictions"
		noLinksNote = "Check logs for Cloudflare detection warnings or link skipping reasons"
	)

	c.logger.Warn("No links discovered on initial page",
		"url", initialPageURL,
		"suggestion", noLinksSuggestion,
		"note", noLinksNote)
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

	// Clean up archiver
	if c.archiver != nil {
		c.logger.Debug("Closing MinIO archiver")
		if err := c.archiver.Close(); err != nil {
			c.logger.Error("Failed to close archiver", "error", err)
		}
	}

	// Clean up state
	c.state.Reset()

	c.logger.Debug("Finished cleaning up crawler resources")
}
