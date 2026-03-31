package main

import (
	"flag"
	"fmt"
	"os"

	signalconfig "github.com/jonesrussell/north-cloud/signal-crawler/internal/config"
	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := flag.String("config", "config.yml", "path to config file")
	dryRun := flag.Bool("dry-run", false, "print signals without publishing")
	flag.Parse()

	cfgPath := infraconfig.GetConfigPath(*configPath)

	cfg, err := signalconfig.Load(cfgPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	log, err := infralogger.New(infralogger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("signal-crawler started",
		infralogger.String("northops_url", cfg.NorthOps.URL),
		infralogger.String("dedup_db", cfg.Dedup.DBPath),
		infralogger.Bool("dry_run", *dryRun),
	)

	fmt.Println("signal-crawler started")
	return nil
}
