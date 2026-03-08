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
	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
	anthprovider "github.com/jonesrussell/north-cloud/ai-observer/internal/provider/anthropic"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/scheduler"
	"github.com/jonesrussell/north-cloud/infrastructure/logger"
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

	p := anthprovider.New(cfg.Anthropic.APIKey, cfg.Anthropic.DefaultModel)
	writer := insights.NewWriter(esClient, cfg.Service.Version)
	cats := buildCategories(cfg, esClient)

	sched := scheduler.New(cats, writer, p, scheduler.Config{
		IntervalSeconds:      cfg.Observer.IntervalSeconds,
		MaxTokensPerInterval: cfg.Observer.MaxTokensPerInterval,
		WindowDuration:       time.Hour,
		DryRun:               cfg.Observer.DryRun,
	}).WithLogger(log)

	log.Info("Scheduler configured",
		logger.Int("categories", len(cats)),
		logger.Int("interval_seconds", cfg.Observer.IntervalSeconds),
	)

	go sched.Run(ctx)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("AI Observer stopped")
	return nil
}

// buildCategories constructs the enabled category list from config.
func buildCategories(cfg Config, esClient *es.Client) []category.Category {
	const maxCategories = 1 // v0: classifier only
	cats := make([]category.Category, 0, maxCategories)
	if cfg.Observer.Categories.ClassifierEnabled {
		cats = append(cats, classifiercategory.New(
			esClient,
			cfg.Observer.Categories.ClassifierMaxEvents,
			cfg.Observer.Categories.ClassifierModel,
		))
	}
	return cats
}
