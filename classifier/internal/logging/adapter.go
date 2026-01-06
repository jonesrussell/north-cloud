// Package logging provides a logger adapter for the classifier service.
package logging

import (
	"github.com/north-cloud/infrastructure/logger"
)

// Logger defines the interface for structured logging in the classifier service.
type Logger interface {
	Info(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Debug(msg string, keysAndValues ...interface{})
}

// Adapter wraps infrastructure logger to match the service Logger interface.
type Adapter struct {
	log logger.Logger
}

// NewAdapter creates a new logger adapter.
func NewAdapter(log logger.Logger) *Adapter {
	return &Adapter{log: log}
}

// Info logs an info message with key-value pairs.
func (a *Adapter) Info(msg string, keysAndValues ...interface{}) {
	a.log.Info(msg, toFields(keysAndValues)...)
}

// Error logs an error message with key-value pairs.
func (a *Adapter) Error(msg string, keysAndValues ...interface{}) {
	a.log.Error(msg, toFields(keysAndValues)...)
}

// Warn logs a warning message with key-value pairs.
func (a *Adapter) Warn(msg string, keysAndValues ...interface{}) {
	a.log.Warn(msg, toFields(keysAndValues)...)
}

// Debug logs a debug message with key-value pairs.
func (a *Adapter) Debug(msg string, keysAndValues ...interface{}) {
	a.log.Debug(msg, toFields(keysAndValues)...)
}

// toFields converts key-value pairs to logger.Field slice.
func toFields(keysAndValues []interface{}) []logger.Field {
	fields := make([]logger.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		if i+1 >= len(keysAndValues) {
			break
		}
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, logger.Any(key, keysAndValues[i+1]))
	}
	return fields
}
