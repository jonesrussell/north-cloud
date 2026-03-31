package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
)

// Client posts signals to NorthOps ingest endpoints.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates an ingest client with a 30-second timeout.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Post sends a signal to the appropriate NorthOps ingest endpoint.
// It uses sig.Endpoint() to determine the path, marshals the signal as JSON,
// and returns an error if the response status is >= 300.
func (c *Client) Post(ctx context.Context, sig adapter.Signal) error {
	body, err := json.Marshal(sig)
	if err != nil {
		return fmt.Errorf("ingest: marshal signal: %w", err)
	}

	url := c.baseURL + sig.Endpoint()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("ingest: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ingest: post signal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("ingest: unexpected status %d", resp.StatusCode)
	}

	return nil
}
