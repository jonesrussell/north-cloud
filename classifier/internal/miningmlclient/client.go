package miningmlclient

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

// ErrUnavailable indicates the mining ML service is unreachable.
var ErrUnavailable = errors.New("mining ML service unavailable")

// Client is an HTTP client for the Mining ML service.
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
	Relevance             string             `json:"relevance"`
	RelevanceConfidence   float64            `json:"relevance_confidence"`
	MiningStage           string             `json:"mining_stage"`
	MiningStageConfidence float64            `json:"mining_stage_confidence"`
	Commodities           []string           `json:"commodities"`
	CommodityScores       map[string]float64 `json:"commodity_scores"`
	Location              string             `json:"location"`
	LocationConfidence    float64            `json:"location_confidence"`
	ProcessingTimeMs      int64              `json:"processing_time_ms"`
	ModelVersion          string             `json:"model_version"`
}

// NewClient creates a new Mining ML client.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

// Classify sends a classification request to the Mining ML service.
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
		return nil, fmt.Errorf("mining ML service returned %d", resp.StatusCode)
	}

	var result ClassifyResponse
	decodeErr := json.NewDecoder(resp.Body).Decode(&result)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	return &result, nil
}

// Health checks if the Mining ML service is healthy.
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
		return fmt.Errorf("mining ML unhealthy: %d", resp.StatusCode)
	}

	return nil
}
