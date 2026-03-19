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
	httpClient *AuthenticatedClient
}

// NewSearchClient creates a new search client
func NewSearchClient(baseURL string, authClient *AuthenticatedClient) *SearchClient {
	return &SearchClient{
		baseURL:    baseURL,
		httpClient: authClient,
	}
}

// SearchRequest represents a search request (MCP input params)
type SearchRequest struct {
	Query           string   `json:"query"`
	Topics          []string `json:"topics,omitempty"`
	ContentType     string   `json:"content_type,omitempty"`
	MinQualityScore int      `json:"min_quality_score,omitempty"`
	Page            int      `json:"page,omitempty"`
	PageSize        int      `json:"page_size,omitempty"`
}

// searchAPIRequest is the request body sent to the search service POST /api/v1/search
type searchAPIRequest struct {
	Query      string               `json:"query"`
	Filters    *searchAPIFilters    `json:"filters,omitempty"`
	Pagination *searchAPIPagination `json:"pagination,omitempty"`
	Options    *searchAPIOptions    `json:"options,omitempty"`
}

type searchAPIFilters struct {
	Topics          []string `json:"topics,omitempty"`
	ContentType     string   `json:"content_type,omitempty"`
	MinQualityScore int      `json:"min_quality_score,omitempty"`
}

type searchAPIPagination struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

type searchAPIOptions struct {
	IncludeHighlights bool `json:"include_highlights"`
	IncludeFacets     bool `json:"include_facets"`
}

// SearchResult represents a single search hit from the search service
type SearchResult struct {
	ID            string              `json:"id"`
	Title         string              `json:"title"`
	URL           string              `json:"url"`
	SourceName    string              `json:"source_name"`
	PublishedDate *time.Time          `json:"published_date,omitempty"`
	CrawledAt     *time.Time          `json:"crawled_at,omitempty"`
	QualityScore  int                 `json:"quality_score"`
	ContentType   string              `json:"content_type"`
	Topics        []string            `json:"topics,omitempty"`
	Score         float64             `json:"score"`
	Snippet       string              `json:"snippet,omitempty"`
	OGImage       string              `json:"og_image,omitempty"`
	Highlight     map[string][]string `json:"highlight,omitempty"`
}

// SearchResponse represents a search response from the search service
type SearchResponse struct {
	Results     []SearchResult `json:"results"`
	Total       int64          `json:"total"`
	TotalPages  int            `json:"total_pages"`
	CurrentPage int            `json:"current_page"`
	PageSize    int            `json:"page_size"`
	Facets      map[string]any `json:"facets,omitempty"`
	TookMs      int64          `json:"took_ms"`
}

// searchAPIResponse matches the actual search service JSON response
type searchAPIResponse struct {
	Hits        []SearchResult `json:"hits"`
	TotalHits   int64          `json:"total_hits"`
	TotalPages  int            `json:"total_pages"`
	CurrentPage int            `json:"current_page"`
	PageSize    int            `json:"page_size"`
	TookMs      int64          `json:"took_ms"`
	Facets      map[string]any `json:"facets,omitempty"`
}

// Search performs a full-text search
func (c *SearchClient) Search(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/search", c.baseURL)

	// Build the API request with proper nested structure
	apiReq := searchAPIRequest{
		Query: req.Query,
		Options: &searchAPIOptions{
			IncludeHighlights: true,
			IncludeFacets:     true,
		},
	}

	if len(req.Topics) > 0 || req.ContentType != "" || req.MinQualityScore > 0 {
		apiReq.Filters = &searchAPIFilters{
			Topics:          req.Topics,
			ContentType:     req.ContentType,
			MinQualityScore: req.MinQualityScore,
		}
	}

	page := req.Page
	if page == 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize == 0 {
		pageSize = 20
	}
	apiReq.Pagination = &searchAPIPagination{Page: page, Size: pageSize}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
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

	// Unmarshal into the API response type (uses "hits" / "total_hits")
	var apiResp searchAPIResponse
	if err = json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Translate to MCP response type (uses "results" / "total")
	return &SearchResponse{
		Results:     apiResp.Hits,
		Total:       apiResp.TotalHits,
		TotalPages:  apiResp.TotalPages,
		CurrentPage: apiResp.CurrentPage,
		PageSize:    apiResp.PageSize,
		Facets:      apiResp.Facets,
		TookMs:      apiResp.TookMs,
	}, nil
}
