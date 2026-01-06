package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// SearchClient is a client for the search API
type SearchClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewSearchClient creates a new search client
func NewSearchClient(baseURL string) *SearchClient {
	return &SearchClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// SearchRequest represents a search request
type SearchRequest struct {
	Query           string    `json:"query"`
	Topics          []string  `json:"topics,omitempty"`
	ContentType     string    `json:"content_type,omitempty"`
	MinQualityScore int       `json:"min_quality_score,omitempty"`
	DateFrom        time.Time `json:"date_from,omitempty"`
	DateTo          time.Time `json:"date_to,omitempty"`
	Page            int       `json:"page,omitempty"`
	PageSize        int       `json:"page_size,omitempty"`
}

// SearchResult represents a search result
type SearchResult struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	Body         string         `json:"body"`
	URL          string         `json:"url"`
	QualityScore int            `json:"quality_score"`
	Topics       []string       `json:"topics"`
	ContentType  string         `json:"content_type"`
	PublishedAt  time.Time      `json:"published_at"`
	Highlights   map[string]any `json:"highlights,omitempty"`
	Score        float64        `json:"score"`
}

// SearchResponse represents a search response
type SearchResponse struct {
	Results  []SearchResult `json:"results"`
	Total    int            `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Facets   map[string]any `json:"facets,omitempty"`
	TookMs   int            `json:"took_ms"`
}

// Search performs a full-text search
func (c *SearchClient) Search(req SearchRequest) (*SearchResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/search", c.baseURL)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("search error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var searchResp SearchResponse
	if err = json.Unmarshal(respBody, &searchResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &searchResp, nil
}
