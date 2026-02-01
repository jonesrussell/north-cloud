package bootstrap

import (
	"fmt"
	"log"

	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// LoadConfig loads configuration. Uses defaults if file doesn't exist.
func LoadConfig() (*config.Config, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Warning: Failed to load config file (%s), using defaults: %v", configPath, err)
		cfg = &config.Config{}
		if cfg.Service.Port == 0 {
			cfg.Service.Port = 8070
		}
		return cfg, nil
	}
	if cfg.Service.Port == 0 {
		cfg.Service.Port = defaultClassifierPort
	}
	return cfg, nil
}

// CreateLogger creates a logger instance from configuration.
func CreateLogger(cfg *config.Config) (infralogger.Logger, error) {
	logger, err := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	return logger.With(infralogger.String("service", "classifier")), nil
}
