package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
)

// ErrNoClassifiedDocs is returned when no classified documents exist in the query window.
var ErrNoClassifiedDocs = errors.New("no classified documents found in window")

const (
	classifiedIndexPattern = "*_classified_content"
	// maxCollectorDocs is the maximum number of docs to fetch for distribution building.
	maxCollectorDocs = 5000
)

// Collector queries ES for classified content and builds current-window distributions.
type Collector struct {
	esClient *es.Client
}

// NewCollector creates a new drift Collector.
func NewCollector(esClient *es.Client) *Collector {
	return &Collector{esClient: esClient}
}

// CollectCurrentWindow queries docs classified within the given window and builds distributions.
// Returns a Baseline struct representing the current window (not a stored baseline).
func (c *Collector) CollectCurrentWindow(ctx context.Context, window time.Duration) (*Baseline, error) {
	docs, err := c.queryDocs(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("query docs: %w", err)
	}

	if len(docs) == 0 {
		return nil, ErrNoClassifiedDocs
	}

	catDist := BuildCategoryDistribution(docs)
	confHist := BuildConfidenceHistograms(docs)
	matrix, matrixCounts := BuildCrossMatrix(docs)

	return &Baseline{
		ComputedAt:           time.Now().UTC().Format(time.RFC3339),
		SampleCount:          len(docs),
		CategoryDistribution: catDist,
		ConfidenceHistograms: confHist,
		CrossMatrix:          matrix,
		CrossMatrixCounts:    matrixCounts,
	}, nil
}

// CollectBaselineWindow queries docs over a multi-day window for baseline computation.
func (c *Collector) CollectBaselineWindow(ctx context.Context, days int) (*Baseline, error) {
	hoursPerDay := 24
	window := time.Duration(days) * time.Duration(hoursPerDay) * time.Hour
	baseline, err := c.CollectCurrentWindow(ctx, window)
	if err != nil {
		return nil, err
	}
	if baseline != nil {
		baseline.WindowDays = days
	}
	return baseline, nil
}

func (c *Collector) queryDocs(ctx context.Context, window time.Duration) ([]ClassifiedDoc, error) {
	since := time.Now().UTC().Add(-window)

	query := map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"classified_at": map[string]any{
					"gte": since.Format(time.RFC3339),
				},
			},
		},
		"size": maxCollectorDocs,
		"_source": []string{
			"source_region", "topics", "confidence",
			"crime.final_confidence", "mining.final_confidence",
			"classified_at",
		},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal collector query: %w", err)
	}

	res, err := c.esClient.Search(
		c.esClient.Search.WithContext(ctx),
		c.esClient.Search.WithIndex(classifiedIndexPattern),
		c.esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("es search error: %s", res.String())
	}

	return decodeClassifiedHits(res.Body)
}

func decodeClassifiedHits(body io.Reader) ([]ClassifiedDoc, error) {
	var result struct {
		Hits struct {
			Hits []struct {
				Source ClassifiedDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode classified hits: %w", err)
	}

	docs := make([]ClassifiedDoc, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		docs = append(docs, hit.Source)
	}

	return docs, nil
}
