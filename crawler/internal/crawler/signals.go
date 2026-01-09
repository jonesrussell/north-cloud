package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
)

// SignalCoordinator manages cancellation signals and cleanup operations.
// It coordinates abort channels, cleanup goroutines, and timeout handling.
type SignalCoordinator struct {
	abortChan   chan struct{}
	abortOnce   sync.Once
	cleanupDone chan struct{}
	cfg         *crawler.Config
	logger      logger.Interface
}

// NewSignalCoordinator creates a new signal coordinator.
func NewSignalCoordinator(cfg *crawler.Config, log logger.Interface) *SignalCoordinator {
	return &SignalCoordinator{
		abortChan: make(chan struct{}),
		abortOnce: sync.Once{},
		cfg:       cfg,
		logger:    log,
	}
}

// Reset prepares the signal coordinator for a new execution.
func (sc *SignalCoordinator) Reset() {
	sc.abortChan = make(chan struct{})
	sc.abortOnce = sync.Once{}
	sc.cleanupDone = nil
}

// AbortChannel returns the abort signal channel.
func (sc *SignalCoordinator) AbortChannel() <-chan struct{} {
	return sc.abortChan
}

// SignalAbort signals all goroutines to abort.
// Safe to call multiple times - only the first call has effect.
func (sc *SignalCoordinator) SignalAbort() {
	sc.abortOnce.Do(func() {
		close(sc.abortChan)
	})
}

// StartCleanupGoroutine starts a periodic cleanup goroutine.
// Returns a channel that's closed when cleanup is done.
func (sc *SignalCoordinator) StartCleanupGoroutine(
	ctx context.Context,
	cleanupFunc func(),
) <-chan struct{} {
	sc.cleanupDone = make(chan struct{})

	go func() {
		// Ensure cleanup interval is positive (NewTicker requires > 0)
		cleanupInterval := sc.cfg.CleanupInterval
		if cleanupInterval <= 0 {
			sc.logger.Warn("Invalid cleanup interval, using default",
				"invalid_interval", cleanupInterval,
				"default", crawler.DefaultCleanupInterval)
			cleanupInterval = crawler.DefaultCleanupInterval
		}

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		defer close(sc.cleanupDone)

		for {
			select {
			case <-ctx.Done():
				sc.logger.Debug("Cleanup goroutine stopping: context cancelled")
				return
			case <-sc.abortChan:
				sc.logger.Debug("Cleanup goroutine stopping: abort signal received")
				return
			case <-ticker.C:
				// Check for abort before running cleanup to avoid blocking
				select {
				case <-sc.abortChan:
					sc.logger.Debug("Cleanup goroutine stopping: abort signal received during cleanup")
					return
				case <-ctx.Done():
					sc.logger.Debug("Cleanup goroutine stopping: context cancelled during cleanup")
					return
				default:
					sc.logger.Debug("Running periodic cleanup")
					cleanupFunc()
				}
			}
		}
	}()

	return sc.cleanupDone
}

// WaitForCleanup waits for the cleanup goroutine to finish with timeout.
func (sc *SignalCoordinator) WaitForCleanup(ctx context.Context, timeout time.Duration) {
	if sc.cleanupDone == nil {
		sc.logger.Debug("No cleanup goroutine to wait for")
		return
	}

	select {
	case <-sc.cleanupDone:
		sc.logger.Debug("Cleanup goroutine finished successfully")
	case <-ctx.Done():
		sc.logger.Debug("Cleanup wait cancelled by context")
	case <-time.After(timeout):
		sc.logger.Warn("Cleanup goroutine did not finish within timeout",
			"timeout", timeout)
	}
}
