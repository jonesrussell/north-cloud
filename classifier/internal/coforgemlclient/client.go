package coforgemlclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

// ErrUnavailable indicates the coforge ML service is unreachable.
var ErrUnavailable = errors.New("coforge ML service unavailable")

// Client is an HTTP client for the Coforge ML service.
type Client struct {
	baseURL string
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
	return &Client{baseURL: baseURL}
}

// Classify sends a classification request to the Coforge ML service.
// Returns ErrUnavailable when the service is unreachable.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	req := &mltransport.ClassifyRequest{Title: title, Body: body}
	var result ClassifyResponse
	if err := mltransport.DoClassify(ctx, c.baseURL, req, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return &result, nil
}

// Health checks if the Coforge ML service is healthy.
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
