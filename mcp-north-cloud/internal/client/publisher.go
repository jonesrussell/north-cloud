package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PublisherClient is a client for the publisher API
type PublisherClient struct {
	baseURL    string
	httpClient *AuthenticatedClient
}

// NewPublisherClient creates a new publisher client
func NewPublisherClient(baseURL string, authClient *AuthenticatedClient) *PublisherClient {
	return &PublisherClient{
		baseURL:    baseURL,
		httpClient: authClient,
	}
}

// PublisherSource represents a publisher source
type PublisherSource struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	IndexPrefix string    `json:"index_prefix"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Channel represents a publishing channel
type Channel struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Route represents a publishing route
type Route struct {
	ID              string    `json:"id"`
	SourceID        string    `json:"source_id"`
	ChannelID       string    `json:"channel_id"`
	SourceName      string    `json:"source_name,omitempty"`
	ChannelName     string    `json:"channel_name,omitempty"`
	MinQualityScore int       `json:"min_quality_score"`
	Topics          []string  `json:"topics"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PublishHistory represents a publish history record
type PublishHistory struct {
	ID           string    `json:"id"`
	ArticleID    string    `json:"article_id"`
	ChannelName  string    `json:"channel_name"`
	QualityScore int       `json:"quality_score"`
	PublishedAt  time.Time `json:"published_at"`
}

// PublisherStats represents publisher statistics
type PublisherStats struct {
	TotalPublished    int              `json:"total_published"`
	ArticlesByChannel map[string]int   `json:"articles_by_channel"`
	RecentActivity    []PublishHistory `json:"recent_activity"`
}

// PreviewArticle represents an article in preview
type PreviewArticle struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	QualityScore int       `json:"quality_score"`
	Topics       []string  `json:"topics"`
	PublishedAt  time.Time `json:"published_at"`
}

// CreateRouteRequest represents a request to create a route
type CreateRouteRequest struct {
	SourceID        string   `json:"source_id"`
	ChannelID       string   `json:"channel_id"`
	MinQualityScore int      `json:"min_quality_score"`
	Topics          []string `json:"topics"`
	Active          bool     `json:"active"`
}

// ListRoutes lists all publishing routes
func (c *PublisherClient) ListRoutes(sourceID, channelID string) ([]Route, error) {
	endpoint := fmt.Sprintf("%s/api/v1/routes", c.baseURL)

	params := url.Values{}
	if sourceID != "" {
		params.Add("source_id", sourceID)
	}
	if channelID != "" {
		params.Add("channel_id", channelID)
	}

	if len(params) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Publisher returns {"routes": [...], "count": N}
	var response struct {
		Routes []Route `json:"routes"`
		Count  int     `json:"count"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Routes, nil
}

// CreateRoute creates a new publishing route
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) CreateRoute(req CreateRouteRequest) (*Route, error) {
	endpoint := fmt.Sprintf("%s/api/v1/routes", c.baseURL)

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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("publisher error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var route Route
	if err = json.Unmarshal(respBody, &route); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &route, nil
}

// DeleteRoute deletes a publishing route
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) DeleteRoute(routeID string) error {
	endpoint := fmt.Sprintf("%s/api/v1/routes/%s", c.baseURL, routeID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodDelete, endpoint, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return fmt.Errorf("publisher error: %s", errorResp.Error)
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// PreviewRoute previews articles matching route filters
func (c *PublisherClient) PreviewRoute(routeID string) ([]PreviewArticle, error) {
	endpoint := fmt.Sprintf("%s/api/v1/routes/preview?route_id=%s", c.baseURL, routeID)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("publisher error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Publisher returns {"estimated_count": N, "filters": {...}, "sample_articles": [...]}
	var response struct {
		SampleArticles []PreviewArticle `json:"sample_articles"`
		EstimatedCount int              `json:"estimated_count"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.SampleArticles, nil
}

// GetPublishHistory gets publish history with pagination
func (c *PublisherClient) GetPublishHistory(channelName string, limit, offset int) ([]PublishHistory, error) {
	endpoint := fmt.Sprintf("%s/api/v1/publish-history", c.baseURL)

	params := url.Values{}
	if channelName != "" {
		params.Add("channel_name", channelName)
	}
	if limit > 0 {
		params.Add("limit", strconv.Itoa(limit))
	}
	if offset > 0 {
		params.Add("offset", strconv.Itoa(offset))
	}

	if len(params) > 0 {
		endpoint = fmt.Sprintf("%s?%s", endpoint, params.Encode())
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Publisher returns {"history": [...], "count": N, "limit": X, "offset": Y}
	var response struct {
		History []PublishHistory `json:"history"`
		Count   int              `json:"count"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.History, nil
}

// GetStats gets publisher statistics
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) GetStats() (*PublisherStats, error) {
	endpoint := fmt.Sprintf("%s/api/v1/stats/overview", c.baseURL)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var stats PublisherStats
	if err = json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &stats, nil
}

// ListSources lists all publisher sources
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) ListSources() ([]PublisherSource, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources", c.baseURL)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Publisher returns {"sources": [...], "count": N}
	var response struct {
		Sources []PublisherSource `json:"sources"`
		Count   int               `json:"count"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Sources, nil
}

// CreatePublisherSourceRequest represents a request to create a publisher source
type CreatePublisherSourceRequest struct {
	Name         string `json:"name"`
	IndexPattern string `json:"index_pattern"`
	Enabled      *bool  `json:"enabled,omitempty"`
}

// CreatePublisherSource creates a new publisher source (Elasticsearch index mapping)
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) CreatePublisherSource(req CreatePublisherSourceRequest) (*PublisherSource, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources", c.baseURL)

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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("publisher error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var source PublisherSource
	if err = json.Unmarshal(respBody, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// ListChannels lists all channels
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) ListChannels() ([]Channel, error) {
	endpoint := fmt.Sprintf("%s/api/v1/channels", c.baseURL)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	// Publisher returns {"channels": [...], "count": N}
	var response struct {
		Channels []Channel `json:"channels"`
		Count    int       `json:"count"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Channels, nil
}

// CreateChannelRequest represents a request to create a channel
type CreateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

// CreateChannel creates a new publishing channel
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) CreateChannel(req CreateChannelRequest) (*Channel, error) {
	endpoint := fmt.Sprintf("%s/api/v1/channels", c.baseURL)

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

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("publisher error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var channel Channel
	if err = json.Unmarshal(respBody, &channel); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &channel, nil
}
