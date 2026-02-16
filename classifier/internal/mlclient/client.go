// classifier/internal/mlclient/client.go
package mlclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

// ErrUnavailable indicates the crime ML service is unreachable.
var ErrUnavailable = errors.New("crime ML service unavailable")

// Client is an HTTP client for the StreetCode ML service.
type Client struct {
	baseURL string
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
	return &Client{baseURL: baseURL}
}

// Classify sends a classification request to the ML service.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	req := &mltransport.ClassifyRequest{Title: title, Body: body}
	var result ClassifyResponse
	if _, _, err := mltransport.DoClassify(ctx, c.baseURL, req, &result); err != nil {
		return nil, fmt.Errorf("classify: %w", err)
	}
	return &result, nil
}

// Health checks if the ML service is healthy.
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
