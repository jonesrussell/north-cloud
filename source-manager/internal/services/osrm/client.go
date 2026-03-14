// Package osrm provides a client for the Open Source Routing Machine (OSRM) API.
package osrm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	defaultBaseURL     = "http://router.project-osrm.org"
	defaultHTTPTimeout = 30 * time.Second
	minDurationCols    = 2
)

// TravelTimeResult holds the computed travel time and distance between two points.
type TravelTimeResult struct {
	DurationSeconds int `json:"duration_seconds"`
	DistanceMeters  int `json:"distance_meters"`
}

// tableResponse represents the OSRM table API response.
type tableResponse struct {
	Code      string      `json:"code"`
	Durations [][]float64 `json:"durations"`
	Distances [][]float64 `json:"distances"`
}

// Client communicates with an OSRM routing engine.
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     infralogger.Logger
}

// NewClient creates a new OSRM client. If baseURL is empty, the public demo server is used.
func NewClient(baseURL string, log infralogger.Logger) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultHTTPTimeout,
		},
		logger: log,
	}
}

// GetTravelTime computes the travel time and distance between two coordinates.
// Mode should be one of: car, bicycle, foot.
func (c *Client) GetTravelTime(
	ctx context.Context,
	originLat, originLon, destLat, destLon float64,
	mode string,
) (*TravelTimeResult, error) {
	url := fmt.Sprintf(
		"%s/table/v1/%s/%f,%f;%f,%f?annotations=duration,distance",
		c.baseURL, mode,
		originLon, originLat,
		destLon, destLat,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create OSRM request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OSRM request failed: %w", err)
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, fmt.Errorf("read OSRM response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OSRM returned status %d: %s", resp.StatusCode, string(body))
	}

	return parseTableResponse(body)
}

// parseTableResponse extracts travel time from an OSRM table API response.
func parseTableResponse(body []byte) (*TravelTimeResult, error) {
	var tableResp tableResponse
	if err := json.Unmarshal(body, &tableResp); err != nil {
		return nil, fmt.Errorf("parse OSRM response: %w", err)
	}

	if tableResp.Code != "Ok" {
		return nil, fmt.Errorf("OSRM error code: %s", tableResp.Code)
	}

	if len(tableResp.Durations) == 0 || len(tableResp.Durations[0]) < minDurationCols {
		return nil, errors.New("OSRM response missing duration data")
	}

	result := &TravelTimeResult{
		DurationSeconds: int(tableResp.Durations[0][1]),
	}

	if len(tableResp.Distances) > 0 && len(tableResp.Distances[0]) >= minDurationCols {
		result.DistanceMeters = int(tableResp.Distances[0][1])
	}

	return result, nil
}
