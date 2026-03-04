package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/api"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/config"
	esindex "github.com/jonesrussell/north-cloud/rfp-ingestor/internal/elasticsearch"
	"github.com/jonesrussell/north-cloud/rfp-ingestor/internal/ingestor"
	infraconfig "github.com/north-cloud/infrastructure/config"
	"github.com/north-cloud/infrastructure/logger"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load(infraconfig.GetConfigPath("config.yml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		return 1
	}

	log, err := logger.New(logger.Config{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Service.Debug,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		return 1
	}
	defer func() { _ = log.Sync() }()
	log = log.With(logger.String("service", cfg.Service.Name))

	// CLI subcommand: backfill
	if len(os.Args) > 1 && os.Args[1] == "backfill" {
		return runBackfill(cfg, log)
	}

	return runServer(cfg, log)
}

func runServer(cfg *config.Config, log logger.Logger) int {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	status := &api.Status{}

	server := api.NewServer(cfg.Service.Name, cfg.Service.Port, cfg.Service.Version, cfg.Service.Debug, log, status)
	errCh := server.StartAsync()

	log.Info("RFP Ingestor started",
		logger.Int("port", cfg.Service.Port),
		logger.Int("poll_interval_minutes", cfg.Ingestion.PollIntervalMinutes),
	)

	ing := ingestor.NewIngestor(ingestor.Config{
		FeedURL:  cfg.Feeds.NewURL,
		ESURL:    cfg.Elasticsearch.URL,
		ESIndex:  cfg.Elasticsearch.Index,
		BulkSize: cfg.Elasticsearch.BulkSize,
	}, log)

	// Ensure ES index exists on startup.
	if err := ensureESIndex(ctx, cfg); err != nil {
		log.Error("Failed to ensure ES index", logger.Error(err))
	}

	// Initial ingestion.
	runIngestion(ctx, ing, log, status)

	// Schedule periodic ingestion.
	ticker := time.NewTicker(time.Duration(cfg.Ingestion.PollIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info("Shutting down")
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Error("Server shutdown error", logger.Error(err))
			}
			return 0
		case err := <-errCh:
			log.Error("Server error", logger.Error(err))
			return 1
		case <-ticker.C:
			runIngestion(ctx, ing, log, status)
		}
	}
}

func runIngestion(ctx context.Context, ing *ingestor.Ingestor, log logger.Logger, status *api.Status) {
	log.Info("Starting ingestion cycle")

	result, err := ing.RunOnce(ctx)
	if err != nil {
		log.Error("Ingestion failed", logger.Error(err))
		status.Update(result.Fetched, result.Indexed, result.Failed+1, result.Duration, true)
		return
	}

	status.Update(result.Fetched, result.Indexed, result.Failed, result.Duration, false)
	log.Info("Ingestion complete",
		logger.Int("fetched", result.Fetched),
		logger.Int("indexed", result.Indexed),
		logger.Int("failed", result.Failed),
		logger.Int64("duration_ms", result.Duration.Milliseconds()),
	)
}

func runBackfill(cfg *config.Config, log logger.Logger) int {
	ctx := context.Background()
	log.Info("Starting historical backfill", logger.String("feed", cfg.Feeds.ArchiveURL))

	// Ensure ES index exists before backfill.
	if err := ensureESIndex(ctx, cfg); err != nil {
		log.Error("Failed to ensure ES index", logger.Error(err))
		return 1
	}

	ing := ingestor.NewIngestor(ingestor.Config{
		FeedURL:  cfg.Feeds.ArchiveURL,
		ESURL:    cfg.Elasticsearch.URL,
		ESIndex:  cfg.Elasticsearch.Index,
		BulkSize: cfg.Elasticsearch.BulkSize,
	}, log)

	result, err := ing.RunOnce(ctx)
	if err != nil {
		log.Error("Backfill failed", logger.Error(err))
		return 1
	}

	log.Info("Backfill complete",
		logger.Int("fetched", result.Fetched),
		logger.Int("indexed", result.Indexed),
		logger.Int("failed", result.Failed),
		logger.Int64("duration_ms", result.Duration.Milliseconds()),
	)
	return 0
}

// ensureESIndex creates the RFP index if it does not already exist.
func ensureESIndex(ctx context.Context, cfg *config.Config) error {
	indexer, err := esindex.NewIndexer(cfg.Elasticsearch.URL, cfg.Elasticsearch.Index, cfg.Elasticsearch.BulkSize)
	if err != nil {
		return fmt.Errorf("create indexer: %w", err)
	}
	return indexer.EnsureIndex(ctx, esindex.RFPIndexMapping())
}
