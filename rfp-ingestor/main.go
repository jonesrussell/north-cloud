package main

import (
	"fmt"
	"os"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Load configuration
	configPath := infraconfig.GetConfigPath("config.yml")

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		return 1
	}

	// Initialize logger
	log, err := createLogger(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting rfp-ingestor",
		infralogger.String("name", cfg.Service.Name),
		infralogger.String("version", cfg.Service.Version),
		infralogger.Int("port", cfg.Service.Port),
		infralogger.Bool("debug", cfg.Service.Debug),
	)

	// TODO: Wire up HTTP server, feed fetcher, and ingestion loop (Task 6)

	return 0
}

// createLogger creates a logger instance from configuration.
func createLogger(cfg *config.Config) (infralogger.Logger, error) {
	log, err := infralogger.New(infralogger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		return nil, err
	}

	return log.With(infralogger.String("service", "rfp-ingestor")), nil
}
