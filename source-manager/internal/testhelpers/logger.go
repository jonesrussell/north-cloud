package testhelpers

import (
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// NewTestLogger creates a logger suitable for testing (discards output by default)
func NewTestLogger() infralogger.Logger {
	// Use infrastructure logger's no-op logger for testing
	log, err := infralogger.New(infralogger.Config{
		Level:       "debug",
		Format:      "json",
		Development: false,
	})
	if err != nil {
		// If logger creation fails, return a basic no-op logger
		// This should never happen in practice, but handle gracefully
		return infralogger.Must(infralogger.Config{
			Level:       "debug",
			Format:      "json",
			Development: false,
		})
	}
	return log
}
