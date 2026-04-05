package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/funding"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/hn"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/config"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/dedup"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/ingest"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/runner"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "Print signals without POSTing to NorthOps")
	configPath := flag.String("config", "", "Path to config.yml (optional)")
	sourceFilter := flag.String("source", "", "Run only this adapter (hn, funding)")
	flag.Parse()

	cfgPath := *configPath
	if cfgPath == "" {
		cfgPath = infraconfig.GetConfigPath("config.yml")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if !*dryRun {
		if validateErr := cfg.Validate(); validateErr != nil {
			fmt.Fprintf(os.Stderr, "Config validation error: %v\n", validateErr)
			os.Exit(1)
		}
	}

	log, err := infralogger.New(infralogger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()

	// Ensure dedup DB directory exists
	dbDir := filepath.Dir(cfg.Dedup.DBPath)
	if mkdirErr := os.MkdirAll(dbDir, 0o755); mkdirErr != nil {
		log.Error("failed to create dedup db directory", infralogger.String("path", dbDir), infralogger.Error(mkdirErr))
		os.Exit(1)
	}

	dedupStore, err := dedup.New(cfg.Dedup.DBPath)
	if err != nil {
		log.Error("failed to open dedup store", infralogger.Error(err))
		os.Exit(1)
	}
	defer func() { _ = dedupStore.Close() }()

	ingestClient := ingest.New(cfg.NorthOps.URL, cfg.NorthOps.APIKey)

	sources := []adapter.Source{
		hn.New(cfg.HN.BaseURL, cfg.HN.MaxItems, log),
		funding.New(cfg.Funding.URLs),
	}

	if *sourceFilter != "" {
		var filtered []adapter.Source
		for _, src := range sources {
			if src.Name() == *sourceFilter {
				filtered = append(filtered, src)
			}
		}
		if len(filtered) == 0 {
			log.Error("unknown source", infralogger.String("source", *sourceFilter))
			os.Exit(1)
		}
		sources = filtered
	}

	r := runner.New(sources, dedupStore, ingestClient, *dryRun, log)

	log.Info("signal-crawler starting",
		infralogger.Bool("dry_run", *dryRun),
		infralogger.Int("sources", len(sources)),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	stats := r.Run(ctx)

	for _, s := range stats {
		log.Info("source complete",
			infralogger.String("source", s.Source),
			infralogger.Int("scanned", s.Scanned),
			infralogger.Int("ingested", s.Ingested),
			infralogger.Int("skipped", s.Skipped),
			infralogger.Int("errors", s.Errors),
		)
	}
}
