// Package classifier implements the classifier drift analysis category.
package classifier

import (
	"context"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

// Category implements category.Category for classifier drift detection.
type Category struct {
	esClient       *es.Client
	maxEvents      int
	modelTier      string
	lastPopulation PopulationStats
}

// New creates a new classifier drift Category.
// esClient may be nil (for unit tests that don't call Sample).
func New(esClient *es.Client, maxEvents int, modelTier string) *Category {
	return &Category{
		esClient:  esClient,
		maxEvents: maxEvents,
		modelTier: modelTier,
	}
}

// Name returns "classifier".
func (c *Category) Name() string { return "classifier" }

// MaxEventsPerRun returns the configured event cap.
func (c *Category) MaxEventsPerRun() int { return c.maxEvents }

// ModelTier returns the configured model tier.
func (c *Category) ModelTier() string { return c.modelTier }

// Sample queries ES for recent classified documents.
// Population stats (total, avg confidence, borderline rate) are cached on the Category
// and passed to Analyze so the LLM receives full population context.
func (c *Category) Sample(ctx context.Context, window time.Duration) ([]category.Event, error) {
	result, err := sample(ctx, c.esClient, window, c.maxEvents)
	if err != nil {
		return nil, err
	}
	c.lastPopulation = result.Population
	return result.Events, nil
}

// Analyze runs the AI drift detection pass.
func (c *Category) Analyze(ctx context.Context, events []category.Event, p provider.LLMProvider) ([]category.Insight, error) {
	return analyze(ctx, events, c.lastPopulation, p, c.modelTier)
}
