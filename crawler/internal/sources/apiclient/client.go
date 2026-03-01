// Package apiclient provides HTTP client functionality for interacting with the source-manager API.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/golang-jwt/jwt/v5"
	infrahttp "github.com/north-cloud/infrastructure/http"
)

const (
	// DefaultBaseURL is the default base URL for the source-manager API.
	DefaultBaseURL = "http://localhost:8050/api/v1/sources"
	// DefaultTimeout is the default timeout for API requests.
	DefaultTimeout = 30 * time.Second
	// ServiceTokenExpirationHours is the expiration time for service-to-service JWT tokens.
	ServiceTokenExpirationHours = 24
)

// ErrNotFound is returned when the API responds with 404 (e.g. no source for identity_key).
var ErrNotFound = errors.New("source not found")

// Client is an HTTP client for interacting with the source-manager API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	jwtSecret  string
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithBaseURL sets the base URL for the API client.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithTimeout sets the timeout for API requests.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithJWTSecret sets the JWT secret for generating service tokens.
func WithJWTSecret(secret string) Option {
	return func(c *Client) {
		c.jwtSecret = secret
	}
}

// NewClient creates a new source-manager API client.
func NewClient(opts ...Option) *Client {
	client := &Client{
		baseURL: DefaultBaseURL,
		httpClient: infrahttp.NewClient(&infrahttp.ClientConfig{
			Timeout: DefaultTimeout,
		}),
	}

	for _, opt := range opts {
		opt(client)
	}

	return client
}

// ListSources retrieves all sources from the API.
func (c *Client) ListSources(ctx context.Context) ([]APISource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var response ListSourcesResponse
	if doErr := c.doRequest(req, &response); doErr != nil {
		return nil, fmt.Errorf("failed to list sources: %w", doErr)
	}

	return response.Sources, nil
}

// GetSource retrieves a specific source by ID.
func (c *Client) GetSource(ctx context.Context, id string) (*APISource, error) {
	sourceURL, err := url.JoinPath(c.baseURL, id)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var source APISource
	if doErr := c.doRequest(req, &source); doErr != nil {
		return nil, fmt.Errorf("failed to get source: %w", doErr)
	}

	return &source, nil
}

// GetByIdentityKey retrieves a source by its identity_key (query param).
// Returns (nil, ErrNotFound) when no source matches (404 or empty key). Used by the Source Identity Resolver.
func (c *Client) GetByIdentityKey(ctx context.Context, identityKey string) (*APISource, error) {
	if identityKey == "" {
		return nil, ErrNotFound
	}
	reqURL := c.baseURL + "/by-identity?identity_key=" + url.QueryEscape(identityKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var source APISource
	if doErr := c.doRequest(req, &source); doErr != nil {
		if errors.Is(doErr, ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get source by identity_key: %w", doErr)
	}

	return &source, nil
}

// CreateSource creates a new source via the API.
func (c *Client) CreateSource(ctx context.Context, source *APISource) (*APISource, error) {
	body, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var created APISource
	if doErr := c.doRequest(req, &created); doErr != nil {
		return nil, fmt.Errorf("failed to create source: %w", doErr)
	}

	return &created, nil
}

// UpdateSource updates an existing source via the API.
func (c *Client) UpdateSource(ctx context.Context, id string, source *APISource) (*APISource, error) {
	sourceURL, err := url.JoinPath(c.baseURL, id)
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	body, err := json.Marshal(source)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal source: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sourceURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var updated APISource
	if doErr := c.doRequest(req, &updated); doErr != nil {
		return nil, fmt.Errorf("failed to update source: %w", doErr)
	}

	return &updated, nil
}

// DeleteSource deletes a source via the API.
func (c *Client) DeleteSource(ctx context.Context, id string) error {
	sourceURL, err := url.JoinPath(c.baseURL, id)
	if err != nil {
		return fmt.Errorf("failed to construct URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, sourceURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if doErr := c.doRequest(req, nil); doErr != nil {
		return fmt.Errorf("failed to delete source: %w", doErr)
	}

	return nil
}

// feedSourcesPageSize is the number of sources fetched per page when listing active feed sources.
const feedSourcesPageSize = 100

// ListActiveFeedSources retrieves all sources with active feeds by paginating through the API.
func (c *Client) ListActiveFeedSources(ctx context.Context) ([]APISource, error) {
	var all []APISource

	for offset := 0; ; offset += feedSourcesPageSize {
		pageURL := fmt.Sprintf("%s?feed_active=true&limit=%d&offset=%d", c.baseURL, feedSourcesPageSize, offset)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, http.NoBody)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}

		var response ListSourcesResponse
		if doErr := c.doRequest(req, &response); doErr != nil {
			return nil, fmt.Errorf("list active feed sources (offset %d): %w", offset, doErr)
		}

		all = append(all, response.Sources...)

		if len(all) >= response.Total || len(response.Sources) < feedSourcesPageSize {
			break
		}
	}

	return all, nil
}

// DisableFeed disables a source's feed via the source-manager API.
func (c *Client) DisableFeed(ctx context.Context, sourceID, reason string) error {
	disableURL, err := url.JoinPath(c.baseURL, sourceID, "feed-disable")
	if err != nil {
		return fmt.Errorf("construct disable URL: %w", err)
	}

	body, marshalErr := json.Marshal(map[string]string{"reason": reason})
	if marshalErr != nil {
		return fmt.Errorf("marshal disable request: %w", marshalErr)
	}

	req, reqErr := http.NewRequestWithContext(
		ctx, http.MethodPatch, disableURL, bytes.NewReader(body),
	)
	if reqErr != nil {
		return fmt.Errorf("create disable request: %w", reqErr)
	}
	req.Header.Set("Content-Type", "application/json")

	var response map[string]any
	return c.doRequest(req, &response)
}

// EnableFeed clears a source's feed disabled state via the source-manager API.
func (c *Client) EnableFeed(ctx context.Context, sourceID string) error {
	enableURL, err := url.JoinPath(c.baseURL, sourceID, "feed-enable")
	if err != nil {
		return fmt.Errorf("construct enable URL: %w", err)
	}

	req, reqErr := http.NewRequestWithContext(
		ctx, http.MethodPatch, enableURL, http.NoBody,
	)
	if reqErr != nil {
		return fmt.Errorf("create enable request: %w", reqErr)
	}

	var response map[string]any
	return c.doRequest(req, &response)
}

// generateServiceToken generates a JWT token for service-to-service authentication.
func (c *Client) generateServiceToken() (string, error) {
	if c.jwtSecret == "" {
		return "", errors.New("JWT secret not configured")
	}

	now := time.Now()
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(ServiceTokenExpirationHours * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		Subject:   "crawler-service",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(c.jwtSecret))
}

// doRequest executes an HTTP request and decodes the response.
func (c *Client) doRequest(req *http.Request, result any) error {
	// Add JWT token if secret is configured
	if c.jwtSecret != "" {
		token, err := c.generateServiceToken()
		if err != nil {
			return fmt.Errorf("failed to generate service token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Provide more helpful error message for connection issues
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			if urlErr.Op == "dial" || urlErr.Err != nil {
				return fmt.Errorf("failed to connect to sources API at %s: %w. "+
					"Ensure the source-manager service is running and accessible", c.baseURL, err)
			}
		}
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("failed to read response body: %w", readErr)
	}

	// Check for error status codes
	const (
		minErrorStatusCode = 400
		statusNotFound     = 404
	)
	if resp.StatusCode >= minErrorStatusCode {
		if resp.StatusCode == statusNotFound {
			return fmt.Errorf("%w: API error (status 404)", ErrNotFound)
		}
		var errResp ErrorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error != "" {
			return fmt.Errorf("API error (status %d): %s - %s", resp.StatusCode, errResp.Error, errResp.Message)
		}
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// For DELETE requests with 204 No Content, don't try to decode
	if resp.StatusCode == http.StatusNoContent || result == nil {
		return nil
	}

	// Decode the response
	if unmarshalErr := json.Unmarshal(body, result); unmarshalErr != nil {
		return fmt.Errorf("failed to decode response: %w", unmarshalErr)
	}

	return nil
}
