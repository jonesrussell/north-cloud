package coforgemlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 5 * time.Second

// ErrUnavailable indicates the coforge ML service is unreachable.
var ErrUnavailable = errors.New("coforge ML service unavailable")

// Client is an HTTP client for the Coforge ML service.
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
	Audience            string             `json:"audience"`
	AudienceConfidence  float64            `json:"audience_confidence"`
	Topics              []string           `json:"topics"`
	TopicScores         map[string]float64 `json:"topic_scores"`
	Industries          []string           `json:"industries"`
	IndustryScores      map[string]float64 `json:"industry_scores"`
	ProcessingTimeMs    int64              `json:"processing_time_ms"`
	ModelVersion        string             `json:"model_version"`
}

// NewClient creates a new Coforge ML client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Classify sends a classification request to the Coforge ML service.
// Returns ErrUnavailable when the service is unreachable.
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
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("coforge ML service returned %d", resp.StatusCode)
	}

	var result ClassifyResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	return &result, nil
}

// Health checks if the Coforge ML service is healthy.
func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", http.NoBody)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("coforge ML unhealthy: %d", resp.StatusCode)
	}

	return nil
}
