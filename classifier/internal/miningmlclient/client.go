package miningmlclient

import (
	"context"
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/mltransport"
)

// ErrUnavailable indicates the mining ML service is unreachable.
var ErrUnavailable = errors.New("mining ML service unavailable")

// Client is an HTTP client for the Mining ML service.
type Client struct {
	baseURL string
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
	return &Client{baseURL: baseURL}
}

// Classify sends a classification request to the Mining ML service.
// Returns ErrUnavailable when the service is unreachable.
func (c *Client) Classify(ctx context.Context, title, body string) (*ClassifyResponse, error) {
	req := &mltransport.ClassifyRequest{Title: title, Body: body}
	var result ClassifyResponse
	if err := mltransport.DoClassify(ctx, c.baseURL, req, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	return &result, nil
}

// Health checks if the Mining ML service is healthy.
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
