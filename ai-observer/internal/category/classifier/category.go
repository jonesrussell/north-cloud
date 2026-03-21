// Package classifier implements the classifier drift analysis category.
package classifier

import (
	"context"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

// Config holds classifier category configuration.
type Config struct {
	MaxEvents         int
	ModelTier         string
	SuppressedSources map[string]bool
	MinDomainSamples  int
}

// Category implements category.Category for classifier drift detection.
type Category struct {
	esClient       *es.Client
	cfg            Config
	lastPopulation PopulationStats
}

// New creates a new classifier drift Category.
// esClient may be nil (for unit tests that don't call Sample).
func New(esClient *es.Client, cfg Config) *Category {
	return &Category{
		esClient: esClient,
		cfg:      cfg,
	}
}

// Name returns "classifier".
func (c *Category) Name() string { return "classifier" }

// MaxEventsPerRun returns the configured event cap.
func (c *Category) MaxEventsPerRun() int { return c.cfg.MaxEvents }

// ModelTier returns the configured model tier.
func (c *Category) ModelTier() string { return c.cfg.ModelTier }

// Sample queries ES for recent classified documents.
// Population stats (total, avg confidence, borderline rate) are cached on the Category
// and passed to Analyze so the LLM receives full population context.
func (c *Category) Sample(ctx context.Context, window time.Duration) ([]category.Event, error) {
	result, err := sample(ctx, c.esClient, window, c.cfg.MaxEvents)
	if err != nil {
		return nil, err
	}
	c.lastPopulation = result.Population
	return result.Events, nil
}

// Analyze runs the AI drift detection pass.
func (c *Category) Analyze(ctx context.Context, events []category.Event, p provider.LLMProvider) ([]category.Insight, error) {
	return analyze(ctx, events, c.lastPopulation, p, c.cfg)
}
