// Package bootstrap handles initialization for the ai-observer service.
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	classifiercategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/classifier"
	driftcategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/drift"
	driftpkg "github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
	anthprovider "github.com/jonesrussell/north-cloud/ai-observer/internal/provider/anthropic"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/scheduler"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	// driftWindowHours is the default window duration for drift category sampling.
	driftWindowHours = 6
)

// Start initializes and runs the ai-observer service.
func Start() error {
	cfg, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	log, err := CreateLogger(cfg)
	if err != nil {
		return fmt.Errorf("logger: %w", err)
	}
	defer func() { _ = log.Sync() }()

	log.Info("Starting AI Observer",
		logger.String("service", cfg.Service.Name),
		logger.String("version", cfg.Service.Version),
		logger.Bool("dry_run", cfg.Observer.DryRun),
		logger.Bool("enabled", cfg.Observer.Enabled),
	)

	if !cfg.Observer.Enabled {
		log.Info("AI Observer disabled via AI_OBSERVER_ENABLED — exiting")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	esClient, err := SetupElasticsearch(ctx, cfg, log)
	if err != nil {
		return fmt.Errorf("elasticsearch: %w", err)
	}
	log.Info("Elasticsearch connected", logger.String("url", cfg.ES.URL))

	if err = insights.EnsureMapping(ctx, esClient); err != nil {
		return fmt.Errorf("ai_insights mapping: %w", err)
	}
	log.Info("ai_insights index mapping ready")

	if err = driftpkg.EnsureBaselineMapping(ctx, esClient); err != nil {
		return fmt.Errorf("drift_baselines mapping: %w", err)
	}
	log.Info("drift_baselines index mapping ready")

	p := anthprovider.New(cfg.Anthropic.APIKey, cfg.Anthropic.DefaultModel)
	dedup := insights.NewDeduplicator(esClient, cfg.Observer.InsightCooldownHours)
	writer := insights.NewWriter(esClient, cfg.Service.Version).WithDedup(dedup)
	cleaner := insights.NewCleaner(esClient, cfg.Observer.InsightRetentionDays)
	fast, slow := buildCategories(cfg, esClient)

	sched := scheduler.New(fast, slow, writer, p, scheduler.Config{
		IntervalSeconds:      cfg.Observer.IntervalSeconds,
		MaxTokensPerInterval: cfg.Observer.MaxTokensPerInterval,
		WindowDuration:       time.Hour,
		DryRun:               cfg.Observer.DryRun,
		DriftIntervalSeconds: cfg.Observer.Categories.DriftIntervalSeconds,
		DriftWindowDuration:  driftWindowHours * time.Hour,
	}).WithLogger(log).WithCleaner(cleaner)

	totalCats := len(fast) + len(slow)
	log.Info("Scheduler configured",
		logger.Int("fast_categories", len(fast)),
		logger.Int("slow_categories", len(slow)),
		logger.Int("total_categories", totalCats),
		logger.Int("interval_seconds", cfg.Observer.IntervalSeconds),
		logger.Int("drift_interval_seconds", cfg.Observer.Categories.DriftIntervalSeconds),
	)

	go sched.Run(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("AI Observer stopped")
	return nil
}

// buildCategories constructs the enabled category lists from config.
// Returns fast (classifier) and slow (drift) category slices.
func buildCategories(cfg Config, esClient *es.Client) (fast, slow []category.Category) {
	if cfg.Observer.Categories.ClassifierEnabled {
		fast = append(fast, classifiercategory.New(
			esClient,
			cfg.Observer.Categories.ClassifierMaxEvents,
			cfg.Observer.Categories.ClassifierModel,
		))
	}
	if cfg.Observer.Categories.DriftEnabled {
		slow = append(slow, driftcategory.New(esClient, driftcategory.Config{
			KLThreshold:        cfg.Observer.Categories.DriftKLThreshold,
			PSIThreshold:       cfg.Observer.Categories.DriftPSIThreshold,
			MatrixThreshold:    cfg.Observer.Categories.DriftMatrixThreshold,
			BaselineWindowDays: cfg.Observer.Categories.DriftBaselineWindowDays,
		}))
	}
	return fast, slow
}
