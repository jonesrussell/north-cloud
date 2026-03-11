package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
)

const (
	// dedupAggName is the ES aggregation name for the dedup query.
	dedupAggName = "recent_summaries"
	// dedupMaxBuckets limits the number of unique summaries returned.
	dedupMaxBuckets = 500
)

// Deduplicator checks recent insights to suppress repeated findings.
type Deduplicator struct {
	esClient      *es.Client
	cooldownHours int
}

// NewDeduplicator creates a Deduplicator with the given cooldown window.
func NewDeduplicator(esClient *es.Client, cooldownHours int) *Deduplicator {
	return &Deduplicator{esClient: esClient, cooldownHours: cooldownHours}
}

// Filter removes insights whose summary already exists in the ai_insights index
// within the cooldown window. Returns only novel insights.
func (d *Deduplicator) Filter(ctx context.Context, insightList []category.Insight) ([]category.Insight, error) {
	if len(insightList) == 0 || d.cooldownHours <= 0 {
		return insightList, nil
	}

	existing, err := d.recentSummaries(ctx)
	if err != nil {
		return insightList, fmt.Errorf("dedup query: %w", err)
	}

	if len(existing) == 0 {
		return insightList, nil
	}

	novel := make([]category.Insight, 0, len(insightList))
	for _, ins := range insightList {
		if !existing[ins.Summary] {
			novel = append(novel, ins)
		}
	}

	return novel, nil
}

// recentSummaries queries ES for unique summaries within the cooldown window.
func (d *Deduplicator) recentSummaries(ctx context.Context) (map[string]bool, error) {
	query := map[string]any{
		"size": 0,
		"query": map[string]any{
			"range": map[string]any{
				"created_at": map[string]any{
					"gte": fmt.Sprintf("now-%dh", d.cooldownHours),
				},
			},
		},
		"aggs": map[string]any{
			dedupAggName: map[string]any{
				"terms": map[string]any{
					"field": "summary.keyword",
					"size":  dedupMaxBuckets,
				},
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal dedup query: %w", err)
	}

	res, err := d.esClient.Search(
		d.esClient.Search.WithContext(ctx),
		d.esClient.Search.WithIndex(insightsIndex),
		d.esClient.Search.WithBody(bytes.NewReader(body)),
	)
	if err != nil {
		return nil, fmt.Errorf("dedup search: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("dedup search error: %s", res.String())
	}

	return parseSummaryBuckets(res.Body)
}
