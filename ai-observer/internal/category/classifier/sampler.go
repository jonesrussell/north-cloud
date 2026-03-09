package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
)

const (
	// borderlineThreshold is the confidence below which a doc is considered borderline.
	borderlineThreshold = 0.6
	// indexPattern is the ES index pattern for classified content.
	// The minus-prefix exclusion removes rfp_classified_content which has a different schema
	// and would otherwise skew confidence stats with its distinct content_type values.
	indexPattern = "*_classified_content,-rfp_classified_content"
	// pageContentType is excluded from sampling — page docs have naturally low confidence
	// (~0.37–0.49) and are not article classifier output, so including them creates false signal.
	pageContentType = "page"
	// statsAggName is the ES aggregation name for population stats.
	statsAggName = "pop_stats"
)

// classifiedDoc is a minimal projection of an ES classified_content document.
type classifiedDoc struct {
	Domain       string             `json:"source_name"`
	ContentType  string             `json:"content_type"`
	Topics       []string           `json:"topics"`
	TopicScores  map[string]float64 `json:"topic_scores"`
	Confidence   float64            `json:"confidence"`
	QualityScore int                `json:"quality_score"`
	ClassifiedAt time.Time          `json:"classified_at"`
}

// PopulationStats holds population-level metrics fetched alongside the sample.
type PopulationStats struct {
	TotalDocs      int     `json:"total_docs"`
	AvgConfidence  float64 `json:"avg_confidence"`
	BorderlineRate float64 `json:"borderline_rate"`
}

// sampleResult bundles the sampled events with population context.
type sampleResult struct {
	Events     []category.Event
	Population PopulationStats
}

// sample queries ES for documents within the given window.
// It fetches population stats (total, avg confidence, borderline rate) in one request
// alongside a random representative sample so the LLM has full context.
func sample(ctx context.Context, esClient *es.Client, window time.Duration, maxEvents int) (sampleResult, error) {
	since := time.Now().UTC().Add(-window)

	query := buildQuery(since, maxEvents)

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return sampleResult{}, fmt.Errorf("marshal query: %w", err)
	}

	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(indexPattern),
		esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return sampleResult{}, fmt.Errorf("es search: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return sampleResult{}, fmt.Errorf("es search error: %s", res.String())
	}

	return decodeResponse(res.Body)
}

// baseFilter returns the bool-filter clauses common to all queries in this package:
// time window + exclude page content_type.
func baseFilter(since time.Time) []map[string]any {
	return []map[string]any{
		{
			"range": map[string]any{
				"classified_at": map[string]any{
					"gte": since.Format(time.RFC3339),
				},
			},
		},
		{
			"bool": map[string]any{
				"must_not": map[string]any{
					"term": map[string]any{
						"content_type.keyword": pageContentType,
					},
				},
			},
		},
	}
}

func buildQuery(since time.Time, maxEvents int) map[string]any {
	return map[string]any{
		// function_score with random_score gives a random representative sample
		// instead of sorting by confidence ASC which would fill the result set with
		// only borderline docs and cause the LLM to see a skewed population.
		"query": map[string]any{
			"function_score": map[string]any{
				"query": map[string]any{
					"bool": map[string]any{
						"filter": baseFilter(since),
					},
				},
				"random_score": map[string]any{},
				"boost_mode":   "replace",
			},
		},
		"size": maxEvents,
		"_source": []string{
			"source_name", "content_type", "topics",
			"topic_scores", "confidence", "quality_score", "classified_at",
		},
		// Population aggregations fetched in the same request.
		"aggs": map[string]any{
			"avg_confidence": map[string]any{
				"avg": map[string]any{"field": "confidence"},
			},
			statsAggName: map[string]any{
				"filter": map[string]any{
					"range": map[string]any{
						"confidence": map[string]any{"lt": borderlineThreshold},
					},
				},
			},
		},
	}
}

func decodeResponse(body io.Reader) (sampleResult, error) {
	var raw struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
			Hits []struct {
				Source classifiedDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
		Aggregations struct {
			AvgConfidence struct {
				Value float64 `json:"value"`
			} `json:"avg_confidence"`
			PopStats struct {
				DocCount int `json:"doc_count"`
			} `json:"pop_stats"`
		} `json:"aggregations"`
	}

	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return sampleResult{}, fmt.Errorf("decode response: %w", err)
	}

	total := raw.Hits.Total.Value
	borderlineCount := raw.Aggregations.PopStats.DocCount

	var borderlineRate float64
	if total > 0 {
		borderlineRate = float64(borderlineCount) / float64(total)
	}

	events := make([]category.Event, 0, len(raw.Hits.Hits))
	for _, hit := range raw.Hits.Hits {
		events = append(events, toEvent(hit.Source))
	}

	pop := PopulationStats{
		TotalDocs:      total,
		AvgConfidence:  raw.Aggregations.AvgConfidence.Value,
		BorderlineRate: borderlineRate,
	}

	return sampleResult{Events: events, Population: pop}, nil
}

func toEvent(doc classifiedDoc) category.Event {
	return category.Event{
		Source:     doc.Domain,
		Label:      doc.ContentType,
		Confidence: doc.Confidence,
		Timestamp:  doc.ClassifiedAt,
		Metadata: map[string]any{
			"topics":        doc.Topics,
			"topic_scores":  doc.TopicScores,
			"quality_score": doc.QualityScore,
		},
	}
}
