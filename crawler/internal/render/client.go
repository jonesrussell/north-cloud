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
	URL       string        `json:"url"`
	TimeoutMs int           `json:"timeout_ms,omitempty"`
	WaitUntil string        `json:"wait_until,omitempty"`
	Config    *RenderConfig `json:"config,omitempty"`
}

// RenderConfig holds per-source M2 render configuration.
// All fields are optional — omitted fields use render-worker defaults.
type RenderConfig struct {
	Scroll    *ScrollConfig     `json:"scroll,omitempty"`
	Selectors *SelectorConfig   `json:"selectors,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Viewport  *ViewportConfig   `json:"viewport,omitempty"`
	Priority  string            `json:"priority,omitempty"`
	SourceID  string            `json:"source_id,omitempty"`
}

// ScrollConfig controls page scrolling behavior.
type ScrollConfig struct {
	Strategy     string `json:"strategy,omitempty"`
	MaxScrollMs  int    `json:"max_scroll_ms,omitempty"`
	ScrollDelay  int    `json:"scroll_delay_ms,omitempty"`
	Pixels       int    `json:"pixels,omitempty"`
	Percent      int    `json:"percent,omitempty"`
}

// SelectorConfig controls element waiting and extraction.
type SelectorConfig struct {
	WaitFor       string `json:"wait_for,omitempty"`
	WaitTimeoutMs int    `json:"wait_timeout_ms,omitempty"`
	Extract       string `json:"extract,omitempty"`
}

// ViewportConfig controls browser viewport dimensions.
type ViewportConfig struct {
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
}

// ScrollResult holds metadata about the scroll operation performed.
type ScrollResult struct {
	StrategyUsed   string `json:"strategy_used"`
	PixelsScrolled int    `json:"pixels_scrolled"`
	ScrollSteps    int    `json:"scroll_steps"`
	ScrollTimeMs   int    `json:"scroll_time_ms"`
}

// RenderResponse is the JSON body returned by the render worker on success.
type RenderResponse struct {
	HTML         string        `json:"html"`
	FinalURL     string        `json:"final_url"`
	RenderTimeMs int           `json:"render_time_ms"`
	StatusCode   int           `json:"status_code"`
	Scroll       *ScrollResult `json:"scroll,omitempty"`
	QueueWaitMs  int           `json:"queue_wait_ms,omitempty"`
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

// RenderWithConfig sends a URL with M2 render configuration to the render worker.
// This method supports scroll strategies, selectors, custom headers, and priority.
// For simple renders without config, use Render() instead.
func (c *Client) RenderWithConfig(ctx context.Context, pageURL string, cfg *RenderConfig) (*RenderResponse, error) {
	reqBody := RenderRequest{URL: pageURL, Config: cfg}

	bodyBytes, marshalErr := json.Marshal(reqBody)
	if marshalErr != nil {
		return nil, fmt.Errorf("marshal render request: %w", marshalErr)
	}

	req, reqErr := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/render", bytes.NewReader(bodyBytes),
	)
	if reqErr != nil {
		return nil, fmt.Errorf("create render request: %w", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return nil, fmt.Errorf("render request failed: %w", doErr)
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
