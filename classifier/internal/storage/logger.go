package storage

import (
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// NewComponentLogger creates a logger with a structured "component" field
// for identifying the subsystem (e.g., "processor").
func NewComponentLogger(component string) (infralogger.Logger, error) {
	logger, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: true,
	})
	if err != nil {
		return nil, err
	}
	return logger.With(infralogger.String("component", component)), nil
}
