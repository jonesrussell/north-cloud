package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ClassifierClient is a client for the classifier API
type ClassifierClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewClassifierClient creates a new classifier client
func NewClassifierClient(baseURL string) *ClassifierClient {
	return &ClassifierClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
	}
}

// ClassifyRequest represents a classification request
type ClassifyRequest struct {
	Title    string         `json:"title"`
	RawText  string         `json:"raw_text"`
	URL      string         `json:"url"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// ClassificationResult represents the result of classification
type ClassificationResult struct {
	ContentType      string   `json:"content_type"`
	QualityScore     int      `json:"quality_score"`
	IsCrimeRelated   bool     `json:"is_crime_related"`
	Topics           []string `json:"topics"`
	SourceReputation float64  `json:"source_reputation"`
	SourceCategory   string   `json:"source_category"`
	Confidence       float64  `json:"confidence"`
}

// Classify classifies a single article
//
//nolint:dupl // Similar HTTP client pattern across different services is acceptable
func (c *ClassifierClient) Classify(req ClassifyRequest) (*ClassificationResult, error) {
	endpoint := fmt.Sprintf("%s/api/v1/classify", c.baseURL)

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
			return nil, fmt.Errorf("classifier error: %s", errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result ClassificationResult
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}
