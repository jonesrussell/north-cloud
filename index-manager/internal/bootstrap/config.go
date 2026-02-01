package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/index-manager/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// LoadConfig loads and validates configuration.
func LoadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if validationErr := cfg.Validate(); validationErr != nil {
		return nil, fmt.Errorf("validate config: %w", validationErr)
	}
	return cfg, nil
}

// CreateLogger creates a logger instance from configuration.
func CreateLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, err := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	return log.With(infralogger.String("service", "index-manager")), nil
}
