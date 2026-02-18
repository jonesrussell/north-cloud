package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ErrSourceNotFound is returned when a source is not found.
var ErrSourceNotFound = errors.New("source not found")

// Source represents a content source from source-manager.
type Source struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	RateLimit int       `json:"rate_limit"`
	MaxDepth  int       `json:"max_depth"`
	Enabled   bool      `json:"enabled"`
	Priority  string    `json:"priority"`
}

// SourceListItem represents a source from the list endpoint (rate_limit is string from API).
type SourceListItem struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	RateLimit string    `json:"rate_limit"`
	MaxDepth  int       `json:"max_depth"`
	Enabled   bool      `json:"enabled"`
	Priority  string    `json:"priority"`
}

// Client defines the interface for fetching source data.
type Client interface {
	GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error)
	ListSources(ctx context.Context) ([]*SourceListItem, error)
}

// HTTPClient implements Client using HTTP requests to source-manager.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

// Default timeouts for HTTP client.
const (
	defaultHTTPTimeout = 10 * time.Second
)

// NewHTTPClient creates a new HTTP client for source-manager.
// If httpClient is nil, a default client with 10 second timeout is used.
func NewHTTPClient(baseURL string, httpClient *http.Client) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
	return &HTTPClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// GetSource fetches a source by ID from source-manager.
func (c *HTTPClient) GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error) {
	url := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL from config
	if err != nil {
		return nil, fmt.Errorf("fetch source: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrSourceNotFound
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var source Source
	if decodeErr := json.NewDecoder(resp.Body).Decode(&source); decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	return &source, nil
}

// ListSources fetches all sources from source-manager.
// Source-manager returns rate_limit as a string (e.g. "1s"); callers must parse it.
func (c *HTTPClient) ListSources(ctx context.Context) ([]*SourceListItem, error) {
	url := fmt.Sprintf("%s/api/v1/sources", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req) //nolint:gosec // G704: URL from config
	if err != nil {
		return nil, fmt.Errorf("fetch sources: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload struct {
		Sources []*SourceListItem `json:"sources"`
		Count   int               `json:"count"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	if payload.Sources == nil {
		payload.Sources = []*SourceListItem{}
	}
	return payload.Sources, nil
}

// NoOpClient is a client that always returns nil (for testing/disabled mode).
type NoOpClient struct{}

// NewNoOpClient creates a no-op client.
func NewNoOpClient() *NoOpClient {
	return &NoOpClient{}
}

// GetSource always returns ErrSourceNotFound for NoOpClient.
func (c *NoOpClient) GetSource(_ context.Context, _ uuid.UUID) (*Source, error) {
	return nil, ErrSourceNotFound
}

// ListSources always returns an empty slice for NoOpClient.
func (c *NoOpClient) ListSources(_ context.Context) ([]*SourceListItem, error) {
	return []*SourceListItem{}, nil
}
