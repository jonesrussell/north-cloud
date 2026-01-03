package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/search/internal/config"
)

// Logger provides structured logging
type Logger struct {
	config config.LoggingConfig
}

// New creates a new logger instance
func New(cfg config.LoggingConfig) *Logger {
	return &Logger{
		config: cfg,
	}
}

// Debug logs debug-level messages
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	if l.shouldLog("debug") {
		l.log("DEBUG", msg, keysAndValues...)
	}
}

// Info logs info-level messages
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	if l.shouldLog("info") {
		l.log("INFO", msg, keysAndValues...)
	}
}

// Warn logs warning-level messages
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	if l.shouldLog("warn") {
		l.log("WARN", msg, keysAndValues...)
	}
}

// Error logs error-level messages
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	if l.shouldLog("error") {
		l.log("ERROR", msg, keysAndValues...)
	}
}

// Fatal logs fatal-level messages and exits
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	l.log("FATAL", msg, keysAndValues...)
	os.Exit(1)
}

// log performs the actual logging
func (l *Logger) log(level, msg string, keysAndValues ...interface{}) {
	if l.config.Format == "json" {
		l.logJSON(level, msg, keysAndValues...)
	} else {
		l.logConsole(level, msg, keysAndValues...)
	}
}

// logJSON logs in JSON format
func (l *Logger) logJSON(level, msg string, keysAndValues ...interface{}) {
	entry := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"level":     level,
		"message":   msg,
	}

	// Add key-value pairs
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			key := fmt.Sprint(keysAndValues[i])
			entry[key] = keysAndValues[i+1]
		}
	}

	jsonBytes, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to marshal log entry: %v\n", err)
		return
	}

	if _, writeErr := fmt.Fprintln(os.Stdout, string(jsonBytes)); writeErr != nil {
		// Logging to stdout failed, but we can't log this error
		// as it would cause infinite recursion. Silently ignore.
		_ = writeErr
	}
}

// logConsole logs in console format
func (l *Logger) logConsole(level, msg string, keysAndValues ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stdout, "[%s] %s %s", level, timestamp, msg)

	// Add key-value pairs
	if len(keysAndValues) > 0 {
		fmt.Fprint(os.Stdout, " ")
		for i := 0; i < len(keysAndValues); i += 2 {
			if i+1 < len(keysAndValues) {
				fmt.Fprintf(os.Stdout, "%v=%v ", keysAndValues[i], keysAndValues[i+1])
			}
		}
	}

	fmt.Fprintln(os.Stdout)
}

const (
	logLevelDebug = 0
	logLevelInfo  = 1
	logLevelWarn  = 2
	logLevelError = 3
)

// shouldLog determines if a message should be logged based on level
func (l *Logger) shouldLog(messageLevel string) bool {
	levels := map[string]int{
		"debug": logLevelDebug,
		"info":  logLevelInfo,
		"warn":  logLevelWarn,
		"error": logLevelError,
	}

	configLevel, ok := levels[l.config.Level]
	if !ok {
		configLevel = logLevelInfo // Default to info
	}

	msgLevel, ok := levels[messageLevel]
	if !ok {
		return false
	}

	return msgLevel >= configLevel
}
