// Package logger provides shared logger initialization utilities.
// Note: Each service has its own logger implementation, but this package
// provides common patterns for logger initialization with fallback handling.
package logger

import (
	"fmt"
	"os"
)

// InitializeFunc is a function type for creating a logger.
// It should return a logger and an error.
type InitializeFunc func(debug bool) (interface{}, error)

// InitializeWithFallback creates a logger using the provided function,
// with automatic fallback to a debug logger if initialization fails.
// This pattern is used across services for consistent error handling
// during early initialization when the main logger might not be available.
//
// Example usage:
//   logger, err := InitializeWithFallback(
//     func(debug bool) (interface{}, error) {
//       return logger.NewLogger(debug)
//     },
//     true, // use debug mode for fallback
//   )
func InitializeWithFallback(initFn InitializeFunc, debug bool) (interface{}, error) {
	logger, err := initFn(debug)
	if err != nil {
		// Fallback: try to create a debug logger
		fallbackLogger, fallbackErr := initFn(true)
		if fallbackErr != nil {
			return nil, fmt.Errorf("failed to initialize logger: %w; fallback also failed: %v", err, fallbackErr)
		}
		// Log the original error using the fallback logger
		if logErr, ok := fallbackLogger.(interface {
			Error(msg string, fields ...interface{})
		}); ok {
			logErr.Error("Failed to initialize logger with requested mode, using debug fallback",
				"error", err,
				"requested_debug", debug,
			)
		}
		return fallbackLogger, nil
	}
	return logger, nil
}

// CreateTempLogger creates a temporary logger for early error logging
// before the main logger is initialized. This is a common pattern in main.go
// files when configuration loading fails.
//
// Example usage:
//   tempLogger, _ := CreateTempLogger(func(debug bool) (interface{}, error) {
//     return logger.NewLogger(debug)
//   })
//   tempLogger.Error("Failed to load config", ...)
func CreateTempLogger(initFn InitializeFunc) (interface{}, error) {
	return initFn(true) // Always use debug mode for temp logger
}

// LoggerInterface defines the minimal interface that loggers must implement.
type LoggerInterface interface {
	Error(msg string, fields ...interface{})
	Sync() error
}

// LogAndExit logs an error message and exits the program.
// This is a helper for main.go files when critical initialization fails.
func LogAndExit(logger interface{}, msg string, err error) {
	if logErr, ok := logger.(LoggerInterface); ok {
		logErr.Error(msg, "error", err)
		_ = logErr.Sync()
	} else {
		// Ultimate fallback: use standard error output
		fmt.Fprintf(os.Stderr, "%s: %v\n", msg, err)
	}
	os.Exit(1)
}

// LogErrorAndExit is a convenience function that creates a temp logger,
// logs an error, and exits. Use this when config loading fails.
func LogErrorAndExit(initFn InitializeFunc, msg string, err error) {
	tempLogger, _ := CreateTempLogger(initFn)
	LogAndExit(tempLogger, msg, err)
}

