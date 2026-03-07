package bootstrap

import (
	"github.com/north-cloud/infrastructure/logger"
)

// CreateLogger initializes the structured logger.
func CreateLogger(_ Config) (logger.Logger, error) {
	return logger.New(logger.Config{
		Level:  "info",
		Format: "json",
	})
}
