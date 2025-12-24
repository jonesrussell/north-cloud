package crawler

import (
	"context"
	"errors"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
)

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

// Event Management
// ---------------

// Subscribe subscribes to crawler events.
func (c *Crawler) Subscribe(handler events.EventHandler) {
	c.bus.Subscribe(handler)
}
