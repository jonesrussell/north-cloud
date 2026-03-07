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
	indexPattern = "*_classified_content"
	// highConfidenceSkipMod controls regression sampling: include 1 in N high-confidence docs.
	highConfidenceSkipMod = 20
)

// classifiedDoc is a minimal projection of an ES classified_content document.
type classifiedDoc struct {
	Domain       string             `json:"domain"`
	ContentType  string             `json:"content_type"`
	Topics       []string           `json:"topics"`
	TopicScores  map[string]float64 `json:"topic_scores"`
	Confidence   float64            `json:"confidence"`
	QualityScore int                `json:"quality_score"`
	ClassifiedAt time.Time          `json:"classified_at"`
}

// sample queries ES for documents within the given window.
func sample(ctx context.Context, esClient *es.Client, window time.Duration, maxEvents int) ([]category.Event, error) {
	since := time.Now().UTC().Add(-window)

	query := buildQuery(since, maxEvents)

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	res, err := esClient.Search(
		esClient.Search.WithContext(ctx),
		esClient.Search.WithIndex(indexPattern),
		esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("es search error: %s", res.String())
	}

	return decodeHits(res.Body)
}

func buildQuery(since time.Time, maxEvents int) map[string]any {
	return map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"classified_at": map[string]any{
					"gte": since.Format(time.RFC3339),
				},
			},
		},
		"size": maxEvents,
		"sort": []map[string]any{
			{"classified_at": map[string]any{"order": "desc"}},
		},
		"_source": []string{
			"domain", "content_type", "topics",
			"topic_scores", "confidence", "quality_score", "classified_at",
		},
	}
}

func decodeHits(body io.Reader) ([]category.Event, error) {
	var result struct {
		Hits struct {
			Hits []struct {
				Source classifiedDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	events := make([]category.Event, 0, len(result.Hits.Hits))
	for i, hit := range result.Hits.Hits {
		doc := hit.Source
		// Include all borderline docs; sample 1 in highConfidenceSkipMod high-confidence docs.
		if doc.Confidence >= borderlineThreshold && i%highConfidenceSkipMod != 0 {
			continue
		}
		events = append(events, toEvent(doc))
	}
	return events, nil
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
