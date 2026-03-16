package drillmlclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

const (
	anthropicVersion = "2023-06-01"
	defaultTimeout   = 30 * time.Second
	systemPrompt     = `Extract drill results from this mining article. Return ONLY a JSON array.
Each result must have: hole_id, commodity, intercept_m (float, meters), grade (float), unit (one of: g/t, %, ppm, oz/t).

Normalize commodities to: gold, silver, copper, nickel, zinc, lithium, uranium, iron-ore, rare-earths, lead, cobalt, tin, platinum, palladium.

Normalize units: "g per tonne" → "g/t", "grams per tonne" → "g/t", "percent" → "%", "parts per million" → "ppm".

If intercept is given as from-to (e.g., "from 45m to 57.5m"), calculate the length (12.5m).

If no drill results are present, return an empty array: []

Return ONLY valid JSON. No explanation.`
)

// Client calls the Anthropic Messages API for drill results extraction.
type Client struct {
	baseURL      string
	apiKey       string
	model        string
	maxBodyChars int
	httpClient   *http.Client
}

// New creates a new Anthropic API client for drill extraction.
func New(baseURL, apiKey, model string, maxBodyChars int) *Client {
	return &Client{
		baseURL:      baseURL,
		apiKey:       apiKey,
		model:        model,
		maxBodyChars: maxBodyChars,
		httpClient:   &http.Client{Timeout: defaultTimeout},
	}
}

// messagesRequest is the Anthropic Messages API request body.
type messagesRequest struct {
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
	System      string    `json:"system"`
	Messages    []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// messagesResponse is the Anthropic Messages API response body.
type messagesResponse struct {
	Content []contentBlock `json:"content"`
	Usage   usage          `json:"usage"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// ExtractResult includes results and token usage for observability.
type ExtractResult struct {
	Results      []domain.DrillResult
	InputTokens  int
	OutputTokens int
	LatencyMs    int64
}

// Extract sends article body to Claude Haiku and returns parsed drill results.
func (c *Client) Extract(body string) ([]domain.DrillResult, error) {
	result, err := c.ExtractWithMetrics(body)
	if err != nil {
		return nil, err
	}
	return result.Results, nil
}

// ExtractWithMetrics is like Extract but also returns token usage and latency.
func (c *Client) ExtractWithMetrics(body string) (*ExtractResult, error) {
	start := time.Now()

	// Truncate body if needed
	if c.maxBodyChars > 0 && len(body) > c.maxBodyChars {
		body = body[:c.maxBodyChars]
	}

	reqBody := messagesRequest{
		Model:       c.model,
		MaxTokens:   1024,
		Temperature: 0,
		System:      systemPrompt,
		Messages: []message{
			{Role: "user", Content: body},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := c.baseURL + "/v1/messages"
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var apiResp messagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(apiResp.Content) == 0 {
		return &ExtractResult{LatencyMs: time.Since(start).Milliseconds()}, nil
	}

	// Parse the JSON array from the text content
	text := apiResp.Content[0].Text
	var results []domain.DrillResult
	if err := json.Unmarshal([]byte(text), &results); err != nil {
		return nil, fmt.Errorf("parse drill results JSON: %w (raw: %s)", err, text)
	}

	return &ExtractResult{
		Results:      results,
		InputTokens:  apiResp.Usage.InputTokens,
		OutputTokens: apiResp.Usage.OutputTokens,
		LatencyMs:    time.Since(start).Milliseconds(),
	}, nil
}
