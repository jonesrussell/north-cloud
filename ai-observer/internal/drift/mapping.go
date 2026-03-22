package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	es "github.com/elastic/go-elasticsearch/v8"
)

const baselinesIndex = "drift_baselines"

var baselineMapping = map[string]any{
	"settings": map[string]any{
		"number_of_replicas": 0,
	},
	"mappings": map[string]any{
		"properties": map[string]any{
			"computed_at":           map[string]any{"type": "date"},
			"window_days":           map[string]any{"type": "integer"},
			"sample_count":          map[string]any{"type": "integer"},
			"category_distribution": map[string]any{"type": "flattened"},
			"confidence_histograms": map[string]any{"type": "flattened"},
			"cross_matrix":          map[string]any{"type": "flattened"},
			"cross_matrix_counts":   map[string]any{"type": "flattened"},
		},
	},
}

// EnsureBaselineMapping creates the drift_baselines index if it does not exist.
func EnsureBaselineMapping(ctx context.Context, esClient *es.Client) error {
	mappingBytes, err := json.Marshal(baselineMapping)
	if err != nil {
		return fmt.Errorf("marshal baseline mapping: %w", err)
	}

	res, err := esClient.Indices.Create(
		baselinesIndex,
		esClient.Indices.Create.WithContext(ctx),
		esClient.Indices.Create.WithBody(bytes.NewReader(mappingBytes)),
	)
	if err != nil {
		return fmt.Errorf("create baseline index: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() && !strings.Contains(res.String(), "resource_already_exists_exception") {
		return fmt.Errorf("create baseline index error: %s", res.String())
	}

	return nil
}
