package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
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
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	URL              string    `json:"url"`
	RateLimit        string    `json:"rate_limit"`
	MaxDepth         int       `json:"max_depth"`
	Enabled          bool      `json:"enabled"`
	Priority         string    `json:"priority"`
	IndigenousRegion *string   `json:"indigenous_region,omitempty"`
	RenderMode       string    `json:"render_mode,omitempty"`
}

// Client defines the interface for fetching source data.
type Client interface {
	GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error)
	ListSources(ctx context.Context) ([]*SourceListItem, error)
	ListIndigenousSources(ctx context.Context) ([]*SourceListItem, error)
}

// HTTPClient implements Client using HTTP requests to source-manager.
type HTTPClient struct {
	baseURL    string
	httpClient *http.Client
	logger     infralogger.Logger // optional; nil disables warning logs
}

// Default timeouts and limits for HTTP client.
const (
	defaultHTTPTimeout = 10 * time.Second

	// indigenousSourcesLimit is the max sources fetched from the indigenous endpoint.
	// The dataset is currently ~186 sources; 500 is a safe ceiling.
	// WARNING: if this limit is ever reached, results will be silently truncated.
	// Add pagination support if the dataset approaches this size.
	indigenousSourcesLimit = 500
)

// NewHTTPClient creates a new HTTP client for source-manager.
// If httpClient is nil, a default client with 10 second timeout is used.
// An optional logger may be passed to enable runtime warning logs (e.g. truncation detection).
func NewHTTPClient(baseURL string, httpClient *http.Client, log ...infralogger.Logger) *HTTPClient {
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: defaultHTTPTimeout,
		}
	}
	c := &HTTPClient{
		baseURL:    baseURL,
		httpClient: httpClient,
	}
	if len(log) > 0 {
		c.logger = log[0]
	}
	return c
}

// GetSource fetches a source by ID from source-manager.
func (c *HTTPClient) GetSource(ctx context.Context, sourceID uuid.UUID) (*Source, error) {
	url := fmt.Sprintf("%s/api/v1/sources/%s", c.baseURL, sourceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
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

	resp, err := c.httpClient.Do(req)
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

// ListIndigenousSources fetches all indigenous sources from the dedicated source-manager endpoint.
// It calls /api/v1/sources/indigenous with a high limit to bypass the default pagination cap.
func (c *HTTPClient) ListIndigenousSources(ctx context.Context) ([]*SourceListItem, error) {
	url := fmt.Sprintf("%s/api/v1/sources/indigenous?limit=%d", c.baseURL, indigenousSourcesLimit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch indigenous sources: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var payload struct {
		Sources []*SourceListItem `json:"sources"`
		Total   int               `json:"total"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&payload); decodeErr != nil {
		return nil, fmt.Errorf("decode response: %w", decodeErr)
	}

	if payload.Sources == nil {
		payload.Sources = []*SourceListItem{}
	}

	if payload.Total > len(payload.Sources) && c.logger != nil {
		c.logger.Warn("ListIndigenousSources: result truncated by API limit",
			infralogger.Int("fetched", len(payload.Sources)),
			infralogger.Int("total", payload.Total),
			infralogger.Int("limit", indigenousSourcesLimit),
		)
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

// ListIndigenousSources always returns an empty slice for NoOpClient.
func (c *NoOpClient) ListIndigenousSources(_ context.Context) ([]*SourceListItem, error) {
	return []*SourceListItem{}, nil
}
