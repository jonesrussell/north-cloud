package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests
	defaultHTTPTimeout = 30 * time.Second
)

// IndexManagerClient is a client for the index-manager API
type IndexManagerClient struct {
	baseURL    string
	httpClient *AuthenticatedClient
}

// NewIndexManagerClient creates a new index-manager client
func NewIndexManagerClient(baseURL string, authClient *AuthenticatedClient) *IndexManagerClient {
	return &IndexManagerClient{
		baseURL:    baseURL,
		httpClient: authClient,
	}
}

// DeleteIndex deletes an index by name
func (c *IndexManagerClient) DeleteIndex(ctx context.Context, indexName string) error {
	url := fmt.Sprintf("%s/api/v1/indexes/%s", c.baseURL, indexName)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(body, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return fmt.Errorf("index-manager error: %s", errorResp.Error)
		}
		return fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Message string `json:"message"`
	}
	if jsonErr := json.Unmarshal(body, &result); jsonErr != nil {
		// Response might not be JSON, that's okay
		return nil
	}

	return nil
}

// ListIndices lists all indices (optional helper method)
func (c *IndexManagerClient) ListIndices(ctx context.Context) ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/indexes", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
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

	var result struct {
		Indices []struct {
			Name string `json:"name"`
		} `json:"indices"`
	}

	if jsonErr := json.Unmarshal(body, &result); jsonErr != nil {
		return nil, fmt.Errorf("failed to parse response: %w", jsonErr)
	}

	indices := make([]string, len(result.Indices))
	for i, idx := range result.Indices {
		indices[i] = idx.Name
	}

	return indices, nil
}
