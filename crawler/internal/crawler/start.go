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

	// Set up OnScraped callback to signal when initial page is done
	// This must be set up before Visit() is called
	// Note: This is in addition to the OnScraped callback in setupCallbacks
	c.collector.OnScraped(func(r *colly.Response) {
		// Check if this is the initial page (only signal once)
		if !initialPageScraped.Load() {
			requestURL := r.Request.URL.String()
			// Compare URLs (handle potential trailing slash differences)
			if requestURL == initialPageURL || requestURL+"/" == initialPageURL || requestURL == initialPageURL+"/" {
				if initialPageScraped.CompareAndSwap(false, true) {
					close(initialPageReady)
					c.logger.Debug("Initial page scraped, all links should be queued",
						"url", initialPageURL)
				}
			}
		}
	})

	// Visit the source URL
	if visitErr := c.collector.Visit(source.URL); visitErr != nil {
		return fmt.Errorf("failed to visit source URL: %w", visitErr)
	}

	// Wait for initial page to be fully processed (OnScraped fired) before starting Wait()
	// This ensures all OnHTML callbacks have queued their links
	if initialPageReady != nil {
		c.logger.Debug("Waiting for initial page processing to complete before starting Wait()")
		select {
		case <-initialPageReady:
			c.logger.Debug("Initial page processing completed, starting Wait()")
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(timeoutWarningInterval):
			c.logger.Warn("Timeout waiting for initial page processing, proceeding anyway")
		}
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
