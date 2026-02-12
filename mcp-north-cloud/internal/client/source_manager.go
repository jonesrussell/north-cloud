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

// SourceManagerClient is a client for the source-manager API
type SourceManagerClient struct {
	baseURL    string
	httpClient *AuthenticatedClient
}

// NewSourceManagerClient creates a new source-manager client
func NewSourceManagerClient(baseURL string, authClient *AuthenticatedClient) *SourceManagerClient {
	return &SourceManagerClient{
		baseURL:    baseURL,
		httpClient: authClient,
	}
}

// Source represents a content source
type Source struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Type      string         `json:"type"`
	Selectors map[string]any `json:"selectors"`
	Active    bool           `json:"active"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// CreateSourceRequest represents a request to create a source
type CreateSourceRequest struct {
	Name      string         `json:"name"`
	URL       string         `json:"url"`
	Type      string         `json:"type"`
	Selectors map[string]any `json:"selectors"`
	Active    bool           `json:"active"`
}

// UpdateSourceRequest represents a request to update a source
type UpdateSourceRequest struct {
	Name      string         `json:"name,omitempty"`
	URL       string         `json:"url,omitempty"`
	Type      string         `json:"type,omitempty"`
	Selectors map[string]any `json:"selectors,omitempty"`
	Active    *bool          `json:"active,omitempty"`
}

// TestCrawlRequest represents a request to test crawl a source
type TestCrawlRequest struct {
	URL       string         `json:"url"`
	Selectors map[string]any `json:"selectors"`
}

// TestCrawlResponse represents the response from test crawl
type TestCrawlResponse struct {
	Success      bool             `json:"success"`
	ArticleCount int              `json:"article_count"`
	SuccessRate  float64          `json:"success_rate"`
	Warnings     []string         `json:"warnings"`
	Articles     []map[string]any `json:"articles"`
}

// CreateSource creates a new source
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) CreateSource(ctx context.Context, req CreateSourceRequest) (*Source, error) {
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
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var source Source
	if err = json.Unmarshal(respBody, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// ListSources lists all sources
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) ListSources(ctx context.Context) ([]Source, error) {
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

	var response struct {
		Sources []Source `json:"sources"`
		Total   int      `json:"total"`
	}
	if err = json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Sources, nil
}

// GetSource gets a source by ID
func (c *SourceManagerClient) GetSource(ctx context.Context, sourceID string) (*Source, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

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
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var source Source
	if err = json.Unmarshal(body, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// UpdateSource updates a source
func (c *SourceManagerClient) UpdateSource(ctx context.Context, sourceID string, req UpdateSourceRequest) (*Source, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
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
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var source Source
	if err = json.Unmarshal(respBody, &source); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &source, nil
}

// DeleteSource deletes a source
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) DeleteSource(ctx context.Context, sourceID string) error {
	endpoint := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

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
			return fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// TestCrawl tests crawling a source without saving
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *SourceManagerClient) TestCrawl(ctx context.Context, req TestCrawlRequest) (*TestCrawlResponse, error) {
	endpoint := fmt.Sprintf("%s/api/v1/sources/test-crawl", c.baseURL)

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

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("source-manager error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var testResp TestCrawlResponse
	if err = json.Unmarshal(respBody, &testResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &testResp, nil
}
