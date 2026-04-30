package orchestration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/jonesrussell/north-cloud/enrichment/internal/enricher"
)

const defaultESTimeout = 10 * time.Second

// ElasticsearchConfig configures the standard-library Elasticsearch search adapter.
type ElasticsearchConfig struct {
	BaseURL    string
	HTTPClient *http.Client
}

// ElasticsearchSearcher implements enricher.Searcher with Elasticsearch's _search API.
type ElasticsearchSearcher struct {
	baseURL    string
	httpClient *http.Client
}

// NewElasticsearchSearcher creates a narrow Elasticsearch search adapter.
func NewElasticsearchSearcher(cfg ElasticsearchConfig) (*ElasticsearchSearcher, error) {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		return nil, fmt.Errorf("elasticsearch base URL is required")
	}
	parsed, err := url.Parse(cfg.BaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("elasticsearch base URL must be absolute: %q", cfg.BaseURL)
	}

	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultESTimeout}
	}

	return &ElasticsearchSearcher{
		baseURL:    strings.TrimRight(parsed.String(), "/"),
		httpClient: client,
	}, nil
}

// Search executes an Elasticsearch query and returns the hit subset enrichers need.
func (s *ElasticsearchSearcher) Search(ctx context.Context, request enricher.SearchRequest) ([]enricher.Hit, error) {
	body := request.Query
	if body == nil {
		body = map[string]any{}
	}
	if request.Size > 0 {
		body = cloneQuery(body)
		body["size"] = request.Size
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal elasticsearch query: %w", err)
	}

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, s.searchURL(request.Indexes), bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build elasticsearch request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")

	response, err := s.httpClient.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("elasticsearch search: %w", err)
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body); _ = response.Body.Close() }()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		raw, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("elasticsearch search returned %d: %s", response.StatusCode, string(raw))
	}

	return decodeSearchResponse(response.Body)
}

func (s *ElasticsearchSearcher) searchURL(indexes []string) string {
	indexPath := strings.Join(indexes, ",")
	if indexPath == "" {
		indexPath = "_all"
	}
	return s.baseURL + path.Join("/", indexPath, "_search")
}

func cloneQuery(in map[string]any) map[string]any {
	out := make(map[string]any, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	return out
}

type searchResponse struct {
	Hits struct {
		Hits []struct {
			ID     string         `json:"_id"`
			Score  float64        `json:"_score"`
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

func decodeSearchResponse(body io.Reader) ([]enricher.Hit, error) {
	var response searchResponse
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode elasticsearch response: %w", err)
	}

	hits := make([]enricher.Hit, 0, len(response.Hits.Hits))
	for _, hit := range response.Hits.Hits {
		source := hit.Source
		if source == nil {
			source = map[string]any{}
		}
		hits = append(hits, enricher.Hit{
			ID:     hit.ID,
			Score:  hit.Score,
			Source: source,
		})
	}
	return hits, nil
}
