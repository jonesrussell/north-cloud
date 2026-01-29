package crawler

import (
	"context"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SignalCoordinator manages cancellation signals and cleanup operations.
// It coordinates abort channels, cleanup goroutines, and timeout handling.
type SignalCoordinator struct {
	abortChan   chan struct{}
	abortOnce   sync.Once
	cleanupDone chan struct{}
	cfg         *crawler.Config
	logger      infralogger.Logger
	jobLogger   logs.JobLogger
}

// NewSignalCoordinator creates a new signal coordinator.
func NewSignalCoordinator(cfg *crawler.Config, log infralogger.Logger) *SignalCoordinator {
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

// SetJobLogger sets the job logger for the current job execution.
func (sc *SignalCoordinator) SetJobLogger(logger logs.JobLogger) {
	sc.jobLogger = logger
}

// getJobLogger returns the job logger, or NoopJobLogger if not set.
func (sc *SignalCoordinator) getJobLogger() logs.JobLogger {
	if sc.jobLogger == nil {
		return logs.NoopJobLogger()
	}
	return sc.jobLogger
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
			sc.getJobLogger().Warn(logs.CategoryLifecycle, "Invalid cleanup interval, using default",
				logs.Duration("invalid_interval", cleanupInterval),
				logs.Duration("default", crawler.DefaultCleanupInterval))
			cleanupInterval = crawler.DefaultCleanupInterval
		}

		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		defer close(sc.cleanupDone)

		for {
			select {
			case <-ctx.Done():
				sc.getJobLogger().Debug(logs.CategoryLifecycle, "Cleanup goroutine stopping: context cancelled")
				return
			case <-sc.abortChan:
				sc.getJobLogger().Debug(logs.CategoryLifecycle, "Cleanup goroutine stopping: abort signal")
				return
			case <-ticker.C:
				// Check for abort before running cleanup to avoid blocking
				select {
				case <-sc.abortChan:
					sc.getJobLogger().Debug(logs.CategoryLifecycle, "Cleanup goroutine stopping: abort during cleanup")
					return
				case <-ctx.Done():
					sc.getJobLogger().Debug(logs.CategoryLifecycle, "Cleanup goroutine stopping: context cancelled during cleanup")
					return
				default:
					sc.getJobLogger().Debug(logs.CategoryLifecycle, "Running periodic cleanup")
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
		sc.getJobLogger().Debug(logs.CategoryLifecycle, "No cleanup goroutine to wait for")
		return
	}

	select {
	case <-sc.cleanupDone:
		sc.getJobLogger().Debug(logs.CategoryLifecycle, "Cleanup goroutine finished")
	case <-ctx.Done():
		sc.getJobLogger().Debug(logs.CategoryLifecycle, "Cleanup wait cancelled by context")
	case <-time.After(timeout):
		sc.getJobLogger().Warn(logs.CategoryLifecycle, "Cleanup goroutine did not finish within timeout",
			logs.Duration("timeout", timeout))
	}
}
