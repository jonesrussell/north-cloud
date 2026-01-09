package storage

import (
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// SimpleLogger wraps infrastructure logger for processor command with prefix
type SimpleLogger struct {
	logger infralogger.Logger
	prefix string
}

// NewSimpleLogger creates a new simple logger using infrastructure logger
func NewSimpleLogger(prefix string) (*SimpleLogger, error) {
	logger, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "console",
		Development: true,
	})
	if err != nil {
		return nil, err
	}
	return &SimpleLogger{
		logger: logger,
		prefix: prefix,
	}, nil
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(msg string, fields ...infralogger.Field) {
	l.logger.Debug(l.prefix+msg, fields...)
}

// Info logs an info message
func (l *SimpleLogger) Info(msg string, fields ...infralogger.Field) {
	l.logger.Info(l.prefix+msg, fields...)
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(msg string, fields ...infralogger.Field) {
	l.logger.Warn(l.prefix+msg, fields...)
}

// Error logs an error message
func (l *SimpleLogger) Error(msg string, fields ...infralogger.Field) {
	l.logger.Error(l.prefix+msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *SimpleLogger) Fatal(msg string, fields ...infralogger.Field) {
	l.logger.Fatal(l.prefix+msg, fields...)
}

// Sync flushes any buffered log entries
func (l *SimpleLogger) Sync() error {
	return l.logger.Sync()
}

// With creates a new logger with the given fields
func (l *SimpleLogger) With(fields ...infralogger.Field) infralogger.Logger {
	return &SimpleLogger{
		logger: l.logger.With(fields...),
		prefix: l.prefix,
	}
}
