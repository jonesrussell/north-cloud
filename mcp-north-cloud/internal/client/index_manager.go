package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IndexManagerClient is a client for the index-manager API
type IndexManagerClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewIndexManagerClient creates a new index-manager client
func NewIndexManagerClient(baseURL string) *IndexManagerClient {
	return &IndexManagerClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DeleteIndex deletes an index by name
func (c *IndexManagerClient) DeleteIndex(indexName string) error {
	url := fmt.Sprintf("%s/api/v1/indexes/%s", c.baseURL, indexName)

	req, err := http.NewRequest(http.MethodDelete, url, nil)
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
	if err := json.Unmarshal(body, &result); err != nil {
		// Response might not be JSON, that's okay
		return nil
	}

	return nil
}

// ListIndices lists all indices (optional helper method)
func (c *IndexManagerClient) ListIndices() ([]string, error) {
	url := fmt.Sprintf("%s/api/v1/indexes", c.baseURL)

	resp, err := c.httpClient.Get(url)
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

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	indices := make([]string, len(result.Indices))
	for i, idx := range result.Indices {
		indices[i] = idx.Name
	}

	return indices, nil
}
