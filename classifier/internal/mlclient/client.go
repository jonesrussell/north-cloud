// classifier/internal/mlclient/client.go
package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 5 * time.Second

// Client is an HTTP client for the StreetCode ML service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// ClassifyRequest is the request body for /classify.
type ClassifyRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// ClassifyResponse is the response body from /classify.
type ClassifyResponse struct {
	Relevance           string             `json:"relevance"`
	RelevanceConfidence float64            `json:"relevance_confidence"`
	CrimeTypes          []string           `json:"crime_types"`
	CrimeTypeScores     map[string]float64 `json:"crime_type_scores"`
	Location            string             `json:"location"`
	LocationConfidence  float64            `json:"location_confidence"`
	ProcessingTimeMs    int64              `json:"processing_time_ms"`
}

// NewClient creates a new ML client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Classify sends a classification request to the ML service.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	reqBody, err := json.Marshal(ClassifyRequest{Title: title, Body: body})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/classify", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ml service returned %d", resp.StatusCode)
	}

	var result ClassifyResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	return &result, nil
}

// Health checks if the ML service is healthy.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unhealthy: %d", resp.StatusCode)
	}

	return nil
}
