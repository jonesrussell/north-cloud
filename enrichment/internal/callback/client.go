package callback

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
)

const (
	HeaderAPIKey      = "X-Api-Key" //nolint:gosec // Header name, not a credential value.
	contentTypeHeader = "Content-Type"
	jsonContentType   = "application/json"
)

var (
	defaultBackoffs = []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, time.Second}
	errRetryable    = errors.New("retryable callback failure")
	errClient       = errors.New("non-retryable callback client failure")
)

// EnrichmentResult is the callback payload for one enrichment type.
type EnrichmentResult struct {
	LeadID     string         `json:"lead_id"`
	Type       string         `json:"type"`
	Status     string         `json:"status"`
	Confidence float64        `json:"confidence"`
	Data       map[string]any `json:"data,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// Client posts enrichment results to a request-provided callback URL.
type Client struct {
	httpClient *http.Client
	backoffs   []time.Duration
}

// Config customizes the callback client. Zero values use safe defaults.
type Config struct {
	HTTPClient     *http.Client
	RequestTimeout time.Duration
	Backoffs       []time.Duration
}

// New constructs a callback client.
func New(cfg Config) *Client {
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.RequestTimeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	backoffs := cfg.Backoffs
	if backoffs == nil {
		backoffs = defaultBackoffs
	}

	return &Client{
		httpClient: httpClient,
		backoffs:   append([]time.Duration(nil), backoffs...),
	}
}

// SendEnrichment sends one enrichment result to Waaseyaa's callback endpoint.
func (c *Client) SendEnrichment(
	ctx context.Context,
	callbackURL string,
	apiKey string,
	result EnrichmentResult,
) error {
	if _, err := parseCallbackURL(callbackURL); err != nil {
		return err
	}

	payload, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal callback payload: %w", err)
	}

	operation := func(ctx context.Context) error {
		return c.post(ctx, callbackURL, apiKey, payload)
	}
	if err := retry(ctx, c.backoffs, operation); err != nil {
		return err
	}
	return nil
}

func (c *Client) post(ctx context.Context, callbackURL string, apiKey string, payload []byte) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, callbackURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build callback request for %s: %w", safeURL(callbackURL), err)
	}
	request.Header.Set(contentTypeHeader, jsonContentType)
	request.Header.Set(HeaderAPIKey, apiKey)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: post callback to %s: %v", errRetryable, safeURL(callbackURL), err)
	}
	defer func() { _, _ = io.Copy(io.Discard, response.Body); _ = response.Body.Close() }()

	switch {
	case response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices:
		return nil
	case response.StatusCode >= http.StatusInternalServerError:
		return fmt.Errorf("%w: callback to %s returned %d", errRetryable, safeURL(callbackURL), response.StatusCode)
	case response.StatusCode >= http.StatusBadRequest:
		return fmt.Errorf("%w: callback to %s returned %d", errClient, safeURL(callbackURL), response.StatusCode)
	default:
		return fmt.Errorf("callback to %s returned unexpected status %d", safeURL(callbackURL), response.StatusCode)
	}
}

func retry(ctx context.Context, backoffs []time.Duration, operation func(context.Context) error) error {
	attempts := len(backoffs) + 1
	var lastErr error

	for attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = operation(ctx)
		if lastErr == nil {
			return nil
		}
		if !errors.Is(lastErr, errRetryable) || attempt == len(backoffs) {
			return lastErr
		}
		if err := sleep(ctx, backoffs[attempt]); err != nil {
			return err
		}
	}

	return lastErr
}

func sleep(ctx context.Context, backoff time.Duration) error {
	if backoff <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseCallbackURL(rawURL string) (*url.URL, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse callback URL: %w", err)
	}
	if !parsed.IsAbs() || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("callback URL must be absolute http or https URL")
	}
	return parsed, nil
}

func safeURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "<invalid-url>"
	}
	parsed.User = nil
	return parsed.String()
}
