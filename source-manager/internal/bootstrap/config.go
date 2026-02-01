package bootstrap

import (
	"flag"
	"fmt"

	"github.com/jonesrussell/north-cloud/source-manager/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// LoadConfig loads configuration. Uses -config flag with infraconfig default.
func LoadConfig() (*config.Config, error) {
	configPath := flag.String("config", infraconfig.GetConfigPath("config.yml"), "Path to configuration file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	if validationErr := cfg.Validate(); validationErr != nil {
		return nil, fmt.Errorf("validate config: %w", validationErr)
	}
	return cfg, nil
}

// CreateLogger creates a logger instance from configuration.
func CreateLogger(cfg *config.Config, version string) (infralogger.Logger, error) {
	log, err := infralogger.New(infralogger.Config{
		Level:       "info",
		Format:      "json",
		Development: cfg.Debug,
	})
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}
	return log.With(
		infralogger.String("service", "source-manager"),
		infralogger.String("version", version),
	), nil
}
