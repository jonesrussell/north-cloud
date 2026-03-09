package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	es "github.com/elastic/go-elasticsearch/v8"
)

// ErrNoBaseline is returned when no baseline documents exist in the index.
var ErrNoBaseline = errors.New("no baseline found")

// Store handles reading and writing baselines to Elasticsearch.
type Store struct {
	esClient *es.Client
}

// NewStore creates a new drift Store.
func NewStore(esClient *es.Client) *Store {
	return &Store{esClient: esClient}
}

// StoreBaseline writes a baseline to the drift_baselines index.
func (s *Store) StoreBaseline(ctx context.Context, baseline *Baseline) error {
	docBytes, err := json.Marshal(baseline)
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}

	res, err := s.esClient.Index(
		baselinesIndex,
		bytes.NewReader(docBytes),
		s.esClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("index baseline: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return fmt.Errorf("index baseline error: %s", res.String())
	}

	return nil
}

// LoadLatestBaseline reads the most recent baseline from the drift_baselines index.
// Returns ErrNoBaseline if no baselines exist.
func (s *Store) LoadLatestBaseline(ctx context.Context) (*Baseline, error) {
	query := map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  1,
		"sort":  []map[string]any{{"computed_at": map[string]any{"order": "desc"}}},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	res, err := s.esClient.Search(
		s.esClient.Search.WithContext(ctx),
		s.esClient.Search.WithIndex(baselinesIndex),
		s.esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("search baselines: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("search baselines error: %s", res.String())
	}

	return decodeBaselineHit(res.Body)
}

func decodeBaselineHit(body io.Reader) (*Baseline, error) {
	var result struct {
		Hits struct {
			Hits []struct {
				Source Baseline `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode baseline response: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, ErrNoBaseline
	}

	baseline := result.Hits.Hits[0].Source
	return &baseline, nil
}
