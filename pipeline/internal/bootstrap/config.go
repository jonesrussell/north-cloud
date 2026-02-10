package bootstrap

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/pipeline/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// LoadConfig loads and validates the service configuration.
func LoadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")

	cfg, loadErr := config.Load(configPath)
	if loadErr != nil {
		return nil, fmt.Errorf("load config: %w", loadErr)
	}

	return cfg, nil
}

// CreateLogger creates a structured logger for the service.
func CreateLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, logErr := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if logErr != nil {
		return nil, fmt.Errorf("create logger: %w", logErr)
	}

	return log.With(infralogger.String("service", "pipeline")), nil
}
