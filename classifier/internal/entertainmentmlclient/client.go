package entertainmentmlclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

// ErrUnavailable indicates the entertainment ML service is unreachable.
var ErrUnavailable = errors.New("entertainment ML service unavailable")

// Client is an HTTP client for the Entertainment ML service.
type Client struct {
	baseURL string
}

// ClassifyResponse is the response body from /classify.
type ClassifyResponse struct {
	Relevance           string   `json:"relevance"`
	RelevanceConfidence float64  `json:"relevance_confidence"`
	Categories          []string `json:"categories"`
	ProcessingTimeMs    int64    `json:"processing_time_ms"`
	ModelVersion        string   `json:"model_version"`
}

// NewClient creates a new Entertainment ML client.
func NewClient(baseURL string) *Client {
	return &Client{baseURL: baseURL}
}

// Classify sends a classification request to the Entertainment ML service.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	req := &mltransport.ClassifyRequest{Title: title, Body: body}
	var result ClassifyResponse
	if _, _, err := mltransport.DoClassify(ctx, c.baseURL, req, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return &result, nil
}

// Health checks if the Entertainment ML service is healthy.
func (c *Client) Health(ctx context.Context) error {
	reachable, _, _, err := mltransport.DoHealth(ctx, c.baseURL)
	if err != nil {
		if !reachable {
			return fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		return err
	}
	return nil
}
