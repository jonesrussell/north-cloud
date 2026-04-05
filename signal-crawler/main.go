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
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter/jobs"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/config"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/dedup"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/ingest"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/render"
	"github.com/jonesrussell/north-cloud/signal-crawler/internal/runner"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "Print signals without POSTing to NorthOps")
	configPath := flag.String("config", "", "Path to config.yml (optional)")
	sourceFilter := flag.String("source", "", "Run only this adapter (hn, funding)")
	flag.Parse()

	cfg, log, dedupStore, err := setup(*configPath, *dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "startup error: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = log.Sync() }()
	defer func() { _ = dedupStore.Close() }()

	var renderer *render.Client
	if cfg.Renderer.Enabled {
		renderer = render.New(cfg.Renderer.URL)
		log.Info("renderer enabled", infralogger.String("url", cfg.Renderer.URL))
	}

	sources, err := buildSources(cfg, *sourceFilter, log, renderer)
	if err != nil {
		log.Error("failed to build sources", infralogger.Error(err))
		os.Exit(1)
	}

	ingestClient := ingest.New(cfg.NorthOps.URL, cfg.NorthOps.APIKey)
	r := runner.New(sources, dedupStore, ingestClient, *dryRun, log)

	log.Info("signal-crawler starting",
		infralogger.Bool("dry_run", *dryRun),
		infralogger.Int("sources", len(sources)),
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	stats := r.Run(ctx)
	logStats(log, stats)
}

func setup(configPath string, dryRun bool) (*config.Config, infralogger.Logger, *dedup.Store, error) {
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = infraconfig.GetConfigPath("config.yml")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("loading config: %w", err)
	}

	if !dryRun {
		if validateErr := cfg.Validate(); validateErr != nil {
			return nil, nil, nil, fmt.Errorf("config validation: %w", validateErr)
		}
	}

	log, err := infralogger.New(infralogger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("creating logger: %w", err)
	}

	dbDir := filepath.Dir(cfg.Dedup.DBPath)
	if mkdirErr := os.MkdirAll(dbDir, 0o755); mkdirErr != nil {
		return nil, nil, nil, fmt.Errorf("creating dedup db directory %s: %w", dbDir, mkdirErr)
	}

	dedupStore, err := dedup.New(cfg.Dedup.DBPath)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("opening dedup store: %w", err)
	}

	return cfg, log, dedupStore, nil
}

func buildSources(cfg *config.Config, sourceFilter string, log infralogger.Logger, renderer *render.Client) ([]adapter.Source, error) {
	// Build job boards list
	var rendererBoard jobs.Renderer
	if renderer != nil {
		rendererBoard = renderer
	}

	boards := []jobs.Board{
		jobs.NewRemoteOK(cfg.Jobs.RemoteOKURL),
		jobs.NewWWR(cfg.Jobs.WWRURL),
		jobs.NewHNHiring("", "", cfg.Jobs.HNMaxComments),
		jobs.NewGCJobs(cfg.Jobs.GCJobsURL, rendererBoard),
		jobs.NewWorkBC(cfg.Jobs.WorkBCURL, rendererBoard),
	}

	all := []adapter.Source{
		hn.New(cfg.HN.BaseURL, cfg.HN.MaxItems, log),
		funding.New(cfg.Funding.URLs),
		jobs.New(boards, log),
	}

	if sourceFilter == "" {
		return all, nil
	}

	var filtered []adapter.Source
	for _, src := range all {
		if src.Name() == sourceFilter {
			filtered = append(filtered, src)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("unknown source %q", sourceFilter)
	}

	return filtered, nil
}

func logStats(log infralogger.Logger, stats []runner.Stats) {
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
