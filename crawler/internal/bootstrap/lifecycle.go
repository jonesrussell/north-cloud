package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	crawlerintevents "github.com/jonesrussell/north-cloud/crawler/internal/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
	infragin "github.com/north-cloud/infrastructure/gin"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/north-cloud/infrastructure/sse"
)

// === Constants ===

const (
	signalChannelBufferSize = 1
	defaultShutdownTimeout  = 30 * time.Second
)

// RunUntilInterrupt runs the server until interrupted by signal or error.
func RunUntilInterrupt(
	log infralogger.Logger,
	server *infragin.Server,
	intervalScheduler *scheduler.IntervalScheduler,
	sseBroker sse.Broker,
	logService logs.Service,
	eventConsumer *crawlerintevents.Consumer,
	feedPollerCancel context.CancelFunc,
	errChan <-chan error,
) error {
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, signalChannelBufferSize)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Wait for interrupt signal or error
	select {
	case serverErr := <-errChan:
		log.Error("Server error", infralogger.Error(serverErr))
		return fmt.Errorf("server error: %w", serverErr)
	case sig := <-sigChan:
		return Shutdown(log, server, intervalScheduler, sseBroker, logService, eventConsumer, feedPollerCancel, sig)
	}
}

// Shutdown performs graceful shutdown of all services in the correct order.
func Shutdown(
	log infralogger.Logger,
	server *infragin.Server,
	intervalScheduler *scheduler.IntervalScheduler,
	sseBroker sse.Broker,
	logService logs.Service,
	eventConsumer *crawlerintevents.Consumer,
	feedPollerCancel context.CancelFunc,
	sig os.Signal,
) error {
	log.Info("Shutdown signal received", infralogger.String("signal", sig.String()))

	// Stop feed poller first (cancels polling goroutine)
	if feedPollerCancel != nil {
		log.Info("Stopping feed poller")
		feedPollerCancel()
	}

	// Stop event consumer (stops reading from Redis)
	if eventConsumer != nil {
		log.Info("Stopping event consumer")
		eventConsumer.Stop()
	}

	// Stop SSE broker (closes all client connections)
	if sseBroker != nil {
		log.Info("Stopping SSE broker")
		if err := sseBroker.Stop(); err != nil {
			log.Error("Failed to stop SSE broker", infralogger.Error(err))
		}
	}

	// Stop scheduler
	if intervalScheduler != nil {
		log.Info("Stopping interval scheduler")
		if err := intervalScheduler.Stop(); err != nil {
			log.Error("Failed to stop scheduler", infralogger.Error(err))
		}
	}

	// Stop log service (archives any pending logs)
	if logService != nil {
		log.Info("Stopping log service")
		if err := logService.Close(); err != nil {
			log.Error("Failed to stop log service", infralogger.Error(err))
		}
	}

	// Stop HTTP server using infrastructure server's graceful shutdown
	log.Info("Stopping HTTP server")
	if err := server.ShutdownWithTimeout(defaultShutdownTimeout); err != nil {
		log.Error("Failed to stop server", infralogger.Error(err))
		return fmt.Errorf("failed to stop server: %w", err)
	}

	log.Info("Server stopped successfully")
	return nil
}
