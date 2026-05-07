package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/config"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
	redispkg "github.com/jonesrussell/north-cloud/alert-crawler/internal/redis"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/runner"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/scope"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/severity"
)

// defaultExpiry is the default alert expiry window when no per-source expiry is set.
const defaultExpiry = 720 * time.Hour

// Phase order (mirrors signal-crawler):
//
//  1. flags
//  2. context with signal handler (SIGINT/SIGTERM)
//  3. logger
//  4. config
//  5. catalogue (sqlite)
//  6. elasticsearch indexer (+ EnsureIndex)
//  7. redis publisher
//  8. metrics, resolver, severity table, fetcher
//  9. runner
//  10. r.Run(ctx) or r.Backfill(ctx) when --backfill
//  11. deferred cleanup via defer
func main() {
	configPath := flag.String("config", "/etc/alert-crawler/config.yml", "path to config.yml")
	backfill := flag.Bool("backfill", false, "run in backfill mode (WP17)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger := mustLogger("info", "json")
	defer func() { _ = logger.Sync() }()

	cfg := mustConfig(logger, *configPath)

	if cfg.Observability.LogLevel != "" || cfg.Observability.LogFormat != "" {
		logger = mustLogger(cfg.Observability.LogLevel, cfg.Observability.LogFormat)
	}

	store := mustCatalogue(ctx, logger, cfg.Database.Path)
	defer func() { _ = store.Close() }()

	indexer := mustIndexer(ctx, logger, cfg.Elasticsearch.URL, cfg.Elasticsearch.Index)

	pub := mustRedis(logger, cfg.Redis.URL, cfg.Redis.Channel)
	defer func() { _ = pub.Close() }()

	r := buildRunner(cfg, logger, store, indexer, pub)

	runStart := time.Now()
	runErr := execute(ctx, r, *backfill)
	if runErr != nil {
		logger.Error("runner", infralogger.Error(runErr))
		os.Exit(1)
	}

	logger.Info("alert-crawler exit",
		infralogger.Int64("duration_ms", time.Since(runStart).Milliseconds()),
	)
}

// mustLogger constructs a logger or calls log.Fatal on failure.
func mustLogger(level, format string) infralogger.Logger {
	if level == "" {
		level = "info"
	}
	if format == "" {
		format = "json"
	}
	logger, err := infralogger.New(infralogger.Config{Level: level, Format: format})
	if err != nil {
		log.Fatalf("logger init: %v", err)
	}
	return logger
}

// mustConfig loads and returns config, logging + exiting on failure.
func mustConfig(logger infralogger.Logger, path string) *config.Config {
	cfg, err := config.Load(path)
	if err != nil {
		logger.Error("config load", infralogger.Error(err))
		os.Exit(1)
	}
	return cfg
}

// mustCatalogue opens the SQLite catalogue, logging + exiting on failure.
func mustCatalogue(ctx context.Context, logger infralogger.Logger, path string) *catalogue.Store {
	store, err := catalogue.Open(ctx, path)
	if err != nil {
		logger.Error("catalogue open", infralogger.Error(err))
		os.Exit(1)
	}
	return store
}

// mustIndexer creates an Elasticsearch indexer and ensures the index exists.
func mustIndexer(ctx context.Context, logger infralogger.Logger, baseURL, index string) *elasticsearch.Indexer {
	indexer := elasticsearch.New(elasticsearch.Config{BaseURL: baseURL, Index: index})
	if err := indexer.EnsureIndex(ctx); err != nil {
		logger.Error("ensure index", infralogger.Error(err))
		os.Exit(1)
	}
	return indexer
}

// mustRedis creates a Redis publisher, logging + exiting on failure.
// The URL field from config maps to Address in the redis.Config.
func mustRedis(logger infralogger.Logger, address, channel string) *redispkg.Publisher {
	pub, err := redispkg.New(redispkg.Config{Address: address, Channel: channel})
	if err != nil {
		logger.Error("redis publisher", infralogger.Error(err))
		os.Exit(1)
	}
	return pub
}

// buildRunner assembles the runner.Dependencies and returns a ready Runner.
func buildRunner(
	cfg *config.Config,
	logger infralogger.Logger,
	store *catalogue.Store,
	indexer *elasticsearch.Indexer,
	pub *redispkg.Publisher,
) *runner.Runner {
	metrics := observability.New(logger)
	resolver := scope.New()
	sevTable := severity.NewTable(cfg.Severity.Table)
	sevInfer := func(h domain.Hazard) domain.Severity {
		return severity.Infer(h, sevTable)
	}
	fetcher := rss.New()

	return runner.New(runner.Dependencies{
		Fetch:         fetcher,
		Store:         store,
		Indexer:       indexer,
		Pub:           pub,
		Resolver:      resolver,
		SevInfer:      sevInfer,
		Metrics:       metrics,
		Sources:       cfg.Sources,
		DefaultExpiry: defaultExpiry,
	})
}

// execute runs the appropriate mode: normal poll or backfill.
func execute(ctx context.Context, r *runner.Runner, backfill bool) error {
	if backfill {
		// Backfill is implemented in WP17. The stub returns a placeholder error.
		return r.Backfill(ctx)
	}
	return r.Run(ctx)
}
