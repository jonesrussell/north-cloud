package storage

import (
	"fmt"
	"log"
	"strings"
)

// SimpleLogger implements the processor.Logger interface using standard log package
type SimpleLogger struct {
	prefix string
}

// NewSimpleLogger creates a new simple logger
func NewSimpleLogger(prefix string) *SimpleLogger {
	return &SimpleLogger{prefix: prefix}
}

// Debug logs a debug message
func (l *SimpleLogger) Debug(msg string, keysAndValues ...any) {
	log.Printf("[DEBUG] %s%s %s", l.prefix, msg, formatKeyValues(keysAndValues...))
}

// Info logs an info message
func (l *SimpleLogger) Info(msg string, keysAndValues ...any) {
	log.Printf("[INFO] %s%s %s", l.prefix, msg, formatKeyValues(keysAndValues...))
}

// Warn logs a warning message
func (l *SimpleLogger) Warn(msg string, keysAndValues ...any) {
	log.Printf("[WARN] %s%s %s", l.prefix, msg, formatKeyValues(keysAndValues...))
}

// Error logs an error message
func (l *SimpleLogger) Error(msg string, keysAndValues ...any) {
	log.Printf("[ERROR] %s%s %s", l.prefix, msg, formatKeyValues(keysAndValues...))
}

func formatKeyValues(keysAndValues ...any) string {
	if len(keysAndValues) == 0 {
		return ""
	}

	var builder strings.Builder
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 < len(keysAndValues) {
			builder.WriteString(fmt.Sprintf("%v=%v ", keysAndValues[i], keysAndValues[i+1]))
		}
	}
	return builder.String()
}
