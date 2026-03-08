// Package render provides an HTTP client for the Playwright render worker,
// enabling dynamic rendering of JavaScript-heavy pages.
package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

// RenderRequest is the JSON body sent to the render worker.
type RenderRequest struct {
	URL       string `json:"url"`
	TimeoutMs int    `json:"timeout_ms,omitempty"`
	WaitUntil string `json:"wait_until,omitempty"`
}

// RenderResponse is the JSON body returned by the render worker on success.
type RenderResponse struct {
	HTML         string `json:"html"`
	FinalURL     string `json:"final_url"`
	RenderTimeMs int    `json:"render_time_ms"`
	StatusCode   int    `json:"status_code"`
}

// ErrorResponse is the JSON body returned by the render worker on failure.
type ErrorResponse struct {
	Error string `json:"error"`
}

// Client is an HTTP client for the render worker service.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new render worker client targeting the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

// Render sends a URL to the render worker and returns the rendered HTML.
func (c *Client) Render(ctx context.Context, pageURL string) (*RenderResponse, error) {
	reqBody := RenderRequest{URL: pageURL}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal render request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/render", bytes.NewReader(bodyBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("create render request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("render request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, parseErrorResponse(resp)
	}

	var result RenderResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		return nil, fmt.Errorf("decode render response: %w", decodeErr)
	}

	return &result, nil
}

// parseErrorResponse extracts a meaningful error from a non-200 render worker response.
func parseErrorResponse(resp *http.Response) error {
	var errResp ErrorResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&errResp); decodeErr == nil && errResp.Error != "" {
		return fmt.Errorf("render worker error (HTTP %d): %s", resp.StatusCode, errResp.Error)
	}

	return fmt.Errorf("render worker returned HTTP %d", resp.StatusCode)
}
