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
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Slug         string       `json:"slug"`
	RedisChannel string       `json:"redis_channel"`
	Description  string       `json:"description"`
	Rules        ChannelRules `json:"rules"`
	RulesVersion int          `json:"rules_version"`
	Enabled      bool         `json:"enabled"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

// ChannelRules defines filtering rules for a channel
type ChannelRules struct {
	IncludeTopics   []string `json:"include_topics"`
	ExcludeTopics   []string `json:"exclude_topics"`
	MinQualityScore int      `json:"min_quality_score"`
	ContentTypes    []string `json:"content_types"`
}

// PublishHistory represents a publish history record
type PublishHistory struct {
	ID           string    `json:"id"`
	ContentID    string    `json:"content_id"`
	ChannelName  string    `json:"channel_name"`
	QualityScore int       `json:"quality_score"`
	PublishedAt  time.Time `json:"published_at"`
}

// PublisherStats represents publisher statistics
type PublisherStats struct {
	TotalPublished int              `json:"total_published"`
	ItemsByChannel map[string]int   `json:"items_by_channel"`
	RecentActivity []PublishHistory `json:"recent_activity"`
}

// GetPublishHistory gets publish history with pagination
func (c *PublisherClient) GetPublishHistory(ctx context.Context, channelName string, limit, offset int) ([]PublishHistory, error) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
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
func (c *PublisherClient) GetStats(ctx context.Context) (*PublisherStats, error) {
	endpoint := fmt.Sprintf("%s/api/v1/stats/overview", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
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
func (c *PublisherClient) ListSources(ctx context.Context) ([]PublisherSource, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
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
func (c *PublisherClient) CreatePublisherSource(ctx context.Context, req CreatePublisherSourceRequest) (*PublisherSource, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources", c.baseURL)

	body, err := json.Marshal(req)
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
func (c *PublisherClient) ListChannels(ctx context.Context) ([]Channel, error) {
	endpoint := fmt.Sprintf("%s/api/v1/channels", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
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
	Name         string        `json:"name"`
	Slug         string        `json:"slug"`
	RedisChannel string        `json:"redis_channel"`
	Description  string        `json:"description,omitempty"`
	Rules        *ChannelRules `json:"rules,omitempty"`
	Enabled      *bool         `json:"enabled,omitempty"`
}

// CreateChannel creates a new publishing channel
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) CreateChannel(ctx context.Context, req CreateChannelRequest) (*Channel, error) {
	endpoint := fmt.Sprintf("%s/api/v1/channels", c.baseURL)

	body, err := json.Marshal(req)
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

// DeleteChannel deletes a publishing channel
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) DeleteChannel(ctx context.Context, channelID string) error {
	endpoint := fmt.Sprintf("%s/api/v1/channels/%s", c.baseURL, channelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, http.NoBody)
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

// ChannelPreview represents the preview response for a channel
type ChannelPreview struct {
	Channel      Channel `json:"channel"`
	RulesSummary any     `json:"rules_summary"`
}

// PreviewChannel previews a channel's configuration and matching rules
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *PublisherClient) PreviewChannel(ctx context.Context, channelID string) (*ChannelPreview, error) {
	endpoint := fmt.Sprintf("%s/api/v1/channels/%s/preview", c.baseURL, channelID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
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

	var preview ChannelPreview
	if err = json.Unmarshal(body, &preview); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &preview, nil
}
