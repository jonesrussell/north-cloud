package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/adapter"
)

const (
	defaultHTTPTimeout = 30 * time.Second
	maxErrorBodyBytes  = 512
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
			Timeout: defaultHTTPTimeout,
		},
	}
}

// Post sends a signal to the NorthOps /api/signals ingest endpoint.
// The signal is wrapped in {"signals": [sig]} to match the NorthOps contract.
func (c *Client) Post(ctx context.Context, sig adapter.Signal) error {
	envelope := struct {
		Signals []adapter.Signal `json:"signals"`
	}{Signals: []adapter.Signal{sig}}

	body, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("ingest: marshal signal: %w", err)
	}

	url := c.baseURL + "/api/signals"
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

	if resp.StatusCode >= http.StatusMultipleChoices {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return fmt.Errorf("ingest: unexpected status %d: %s", resp.StatusCode, string(errBody))
	}

	return nil
}
