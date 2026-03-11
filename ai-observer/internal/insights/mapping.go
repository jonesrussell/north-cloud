package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	es "github.com/elastic/go-elasticsearch/v8"
)

// summaryKeywordIgnoreAbove is the max character length for the summary.keyword sub-field.
const summaryKeywordIgnoreAbove = 512

// insightMapping is the explicit ES mapping for the ai_insights index.
// keyword fields enable aggregation/sorting without needing .keyword sub-fields.
// details uses "flattened" type to avoid dynamic mapping conflicts when
// LLM-generated values have inconsistent types across documents.
var insightMapping = map[string]any{
	"mappings": map[string]any{
		"properties": map[string]any{
			"id":         map[string]any{"type": "keyword"},
			"created_at": map[string]any{"type": "date"},
			"category":   map[string]any{"type": "keyword"},
			"severity":   map[string]any{"type": "keyword"},
			"summary": map[string]any{
				"type": "text",
				"fields": map[string]any{
					"keyword": map[string]any{"type": "keyword", "ignore_above": summaryKeywordIgnoreAbove},
				},
			},
			"details":           map[string]any{"type": "flattened"},
			"suggested_actions": map[string]any{"type": "text"},
			"observer_version":  map[string]any{"type": "keyword"},
			"model":             map[string]any{"type": "keyword"},
			"tokens_used":       map[string]any{"type": "integer"},
		},
	},
}

// EnsureMapping creates the ai_insights index with an explicit mapping if it does not
// already exist. Calling this repeatedly is safe — a 400 resource_already_exists_exception
// is treated as success.
func EnsureMapping(ctx context.Context, esClient *es.Client) error {
	mappingBytes, err := json.Marshal(insightMapping)
	if err != nil {
		return fmt.Errorf("marshal mapping: %w", err)
	}

	res, err := esClient.Indices.Create(
		insightsIndex,
		esClient.Indices.Create.WithContext(ctx),
		esClient.Indices.Create.WithBody(bytes.NewReader(mappingBytes)),
	)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() && !strings.Contains(res.String(), "resource_already_exists_exception") {
		return fmt.Errorf("create index error: %s", res.String())
	}

	return nil
}
