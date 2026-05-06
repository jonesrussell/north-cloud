package rss

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

const (
	defaultUserAgent = "alert-crawler/1.0 (+https://northcloud.one)"
	defaultTimeout   = 30 * time.Second
	maxFeedBytes     = 5 * 1024 * 1024 // 5 MB safety cap
)

// Client is an HTTP client for fetching RSS/Atom feeds with conditional GET support.
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// Option is a functional option for configuring a Client.
type Option func(*Client)

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		c.userAgent = ua
	}
}

// WithTimeout overrides the default HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpClient.Timeout = d
	}
}

// WithHTTPClient replaces the underlying *http.Client entirely.
// Useful in tests to inject an httptest-backed transport.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// New constructs a Client with sensible defaults, overridden by opts.
func New(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		userAgent:  defaultUserAgent,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// FetchInput carries per-request parameters for a conditional GET.
type FetchInput struct {
	Source       domain.AlertSource
	LastETag     string
	LastModified string
}

// FetchOutput carries the raw feed bytes and updated cache headers.
type FetchOutput struct {
	Body         []byte
	ETag         string
	LastModified string
	StatusCode   int
}

// Fetch performs an HTTP GET for the feed described by in.Source.FeedURL.
// It sends If-None-Match / If-Modified-Since when the caller supplies cached
// values. Responses are classified as follows:
//
//   - 304 → ErrNotModified (sentinel; no body)
//   - 5xx → wrapped ErrTransient (retry-worthy)
//   - 4xx → wrapped ErrStructural (non-retryable)
//   - 200 → FetchOutput with body capped at 5 MB
func (c *Client) Fetch(ctx context.Context, in FetchInput) (*FetchOutput, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.Source.FeedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml")

	if in.LastETag != "" {
		req.Header.Set("If-None-Match", in.LastETag)
	}

	if in.LastModified != "" {
		req.Header.Set("If-Modified-Since", in.LastModified)
	}

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		return nil, fmt.Errorf("http do: %w", doErr)
	}
	defer resp.Body.Close() //nolint:errcheck

	switch {
	case resp.StatusCode == http.StatusNotModified:
		return nil, ErrNotModified
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("upstream %d: %w", resp.StatusCode, ErrTransient)
	case resp.StatusCode >= 400:
		return nil, fmt.Errorf("upstream %d: %w", resp.StatusCode, ErrStructural)
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes))
	if readErr != nil {
		return nil, fmt.Errorf("read body: %w", readErr)
	}

	return &FetchOutput{
		Body:         body,
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		StatusCode:   resp.StatusCode,
	}, nil
}
