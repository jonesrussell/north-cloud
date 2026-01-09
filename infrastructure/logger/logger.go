// Package logger provides a unified structured logging interface for all North Cloud services.
package logger

import (
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger defines the interface for structured logging.
// All North Cloud services must use this interface for logging.
type Logger interface {
	// Debug logs a message at debug level.
	Debug(msg string, fields ...Field)
	// Info logs a message at info level.
	Info(msg string, fields ...Field)
	// Warn logs a message at warning level.
	Warn(msg string, fields ...Field)
	// Error logs a message at error level.
	Error(msg string, fields ...Field)
	// Fatal logs a message at fatal level and exits.
	Fatal(msg string, fields ...Field)
	// With returns a new logger with the given fields attached.
	With(fields ...Field) Logger
	// Sync flushes any buffered log entries.
	Sync() error
}

// Field is a type alias for zap.Field.
// It represents a key-value pair that can be attached to a log entry.
type Field = zap.Field

// zapLogger is a zap-based implementation of the Logger interface.
type zapLogger struct {
	logger *zap.Logger
}

// New creates a new Logger instance with the given configuration.
// If config is nil, default values are used.
func New(cfg Config) (Logger, error) {
	cfg.SetDefaults()

	level := parseLevel(cfg.Level)

	// Always use JSON format for consistency across all environments
	// This ensures better log aggregation, parsing, and querying in Grafana/Loki
	zapCfg := zap.NewProductionConfig()
	zapCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	zapCfg.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	zapCfg.Level = zap.NewAtomicLevelAt(level)
	if len(cfg.OutputPaths) > 0 {
		zapCfg.OutputPaths = cfg.OutputPaths
	}

	// Disable sampling in development for all logs to be visible
	if cfg.Development {
		zapCfg.Sampling = nil
	}

	z, err := zapCfg.Build(
		zap.AddCallerSkip(1), // Skip the wrapper function
		zap.AddStacktrace(zapcore.ErrorLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("build zap logger: %w", err)
	}

	return &zapLogger{logger: z}, nil
}

// parseLevel converts a string level to zapcore.Level.
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// Debug logs a message at debug level.
func (l *zapLogger) Debug(msg string, fields ...Field) {
	l.logger.Debug(msg, fields...)
}

// Info logs a message at info level.
func (l *zapLogger) Info(msg string, fields ...Field) {
	l.logger.Info(msg, fields...)
}

// Warn logs a message at warning level.
func (l *zapLogger) Warn(msg string, fields ...Field) {
	l.logger.Warn(msg, fields...)
}

// Error logs a message at error level.
func (l *zapLogger) Error(msg string, fields ...Field) {
	l.logger.Error(msg, fields...)
}

// Fatal logs a message at fatal level and exits.
func (l *zapLogger) Fatal(msg string, fields ...Field) {
	l.logger.Fatal(msg, fields...)
}

// With returns a new logger with the given fields attached.
func (l *zapLogger) With(fields ...Field) Logger {
	return &zapLogger{
		logger: l.logger.With(fields...),
	}
}

// Sync flushes any buffered log entries.
func (l *zapLogger) Sync() error {
	return l.logger.Sync()
}

// Must creates a new Logger and panics if it fails.
// Use this for initialization where failure should be fatal.
func Must(cfg Config) Logger {
	l, err := New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}
	return l
}

// NewFromLoggingConfig creates a Logger from a LoggingConfig struct.
// This is a convenience function for services using the standard config pattern.
func NewFromLoggingConfig(level, format string) (Logger, error) {
	return New(Config{
		Level:  level,
		Format: format,
	})
}

// String creates a string field.
func String(key, val string) Field {
	return zap.String(key, val)
}

// Int creates an int field.
func Int(key string, val int) Field {
	return zap.Int(key, val)
}

// Int64 creates an int64 field.
func Int64(key string, val int64) Field {
	return zap.Int64(key, val)
}

// Uint64 creates a uint64 field.
func Uint64(key string, val uint64) Field {
	return zap.Uint64(key, val)
}

// Float64 creates a float64 field.
func Float64(key string, val float64) Field {
	return zap.Float64(key, val)
}

// Bool creates a bool field.
func Bool(key string, val bool) Field {
	return zap.Bool(key, val)
}

// Duration creates a duration field.
func Duration(key string, val time.Duration) Field {
	return zap.Duration(key, val)
}

// Time creates a time field.
func Time(key string, val time.Time) Field {
	return zap.Time(key, val)
}

// Error creates an error field with the key "error".
func Error(err error) Field {
	return zap.Error(err)
}

// NamedError creates an error field with a custom key.
func NamedError(key string, err error) Field {
	return zap.NamedError(key, err)
}

// Any creates a field that can hold any value.
func Any(key string, val any) Field {
	return zap.Any(key, val)
}

// Strings creates a string slice field.
func Strings(key string, val []string) Field {
	return zap.Strings(key, val)
}
