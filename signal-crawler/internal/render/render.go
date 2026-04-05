package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	defaultTimeout   = 45 * time.Second
	maxResponseBytes = 10 * 1024 * 1024 // 10 MB
)

// Client communicates with a Playwright renderer service to fetch
// fully-rendered HTML from JS-heavy pages.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New creates a renderer client pointing at the given base URL.
func New(baseURL string) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

type renderRequest struct {
	URL     string `json:"url"`
	WaitFor string `json:"wait_for"`
}

type renderResponse struct {
	HTML string `json:"html"`
}

// Render sends a URL to the renderer service and returns the fully-rendered HTML.
func (c *Client) Render(ctx context.Context, targetURL string) (string, error) {
	body, err := json.Marshal(renderRequest{
		URL:     targetURL,
		WaitFor: "networkidle",
	})
	if err != nil {
		return "", fmt.Errorf("render: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/render", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("render: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("render: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusMultipleChoices {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("render: HTTP %d: %s", resp.StatusCode, string(errBody))
	}

	var result renderResponse
	if decErr := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&result); decErr != nil {
		return "", fmt.Errorf("render: decode response: %w", decErr)
	}

	return result.HTML, nil
}
