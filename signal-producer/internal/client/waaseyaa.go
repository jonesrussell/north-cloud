// Package client provides the HTTP client used by the signal-producer to
// deliver signal batches to the Waaseyaa /api/signals endpoint.
//
// The client is intentionally narrow: it owns batch POST mechanics, retry
// semantics for transient 5xx and network failures (FR-010, FR-011, NFR-002),
// header construction (FR-009), and parsing of the IngestResult response
// (FR-012). It does not import the producer's mapper package — callers
// supply opaque payload values inside a SignalBatch wrapper.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Endpoint and header constants. Defined as named constants per C-002
// (no magic strings/numbers).
const (
	// SignalsEndpointPath is the receiver path appended to the configured
	// base URL. The contract is fixed — see contracts/signals-post.yaml.
	SignalsEndpointPath = "/api/signals"

	// HeaderAPIKey is the auth header name expected by Waaseyaa. FR-009.
	HeaderAPIKey = "X-Api-Key"

	// HeaderContentType is the standard MIME header.
	HeaderContentType = "Content-Type"

	// ContentTypeJSON is the request payload content type.
	ContentTypeJSON = "application/json"

	// DefaultRequestTimeout bounds a single HTTP attempt. Each retry uses
	// its own deadline derived from the caller's context.
	DefaultRequestTimeout = 30 * time.Second

	// httpStatusClientErrorMin / httpStatusServerErrorMin / httpStatusEnd
	// keep status-class boundaries free of magic numbers.
	httpStatusClientErrorMin = 400
	httpStatusServerErrorMin = 500
	httpStatusEnd            = 600
)

// Sentinel errors used by the retry helper to classify failures.
//
//nolint:revive // exported sentinel naming follows the unexported "errX" idiom intentionally
var (
	// errClient indicates a non-retryable 4xx response from Waaseyaa.
	errClient = errors.New("client error: non-retryable 4xx")
	// errServer indicates a retryable 5xx response from Waaseyaa.
	errServer = errors.New("server error: retryable 5xx")
)

// ErrClientResponse exposes errClient for callers that want to inspect the
// failure class via errors.Is.
func ErrClientResponse() error { return errClient }

// ErrServerResponse exposes errServer for callers that want to inspect the
// failure class via errors.Is.
func ErrServerResponse() error { return errServer }

// SignalBatch is the request body envelope. Signals is opaque to this
// package — the producer (WP05) marshals mapper.Signal values into it.
type SignalBatch struct {
	Signals []any `json:"signals"`
}

// IngestResult is the JSON body returned by Waaseyaa on a 2xx response.
// FR-012 / contracts/signals-post.yaml.
type IngestResult struct {
	Ingested     int `json:"ingested"`
	Skipped      int `json:"skipped"`
	LeadsCreated int `json:"leads_created"`
	LeadsMatched int `json:"leads_matched"`
	Unmatched    int `json:"unmatched"`
}

// WaaseyaaClient is the public surface used by the producer's main loop.
type WaaseyaaClient interface {
	PostSignals(ctx context.Context, batch SignalBatch) (*IngestResult, error)
}

// Config controls Client construction.
type Config struct {
	// BaseURL is the Waaseyaa origin (no trailing slash, no path).
	BaseURL string
	// APIKey is the value sent in the X-Api-Key header.
	APIKey string
	// HTTPClient is optional; nil falls back to a client with
	// DefaultRequestTimeout.
	HTTPClient *http.Client
	// Backoffs overrides the retry schedule. Empty falls back to the
	// production schedule (1s, 5s, 15s — total 21s, within NFR-002's 25s
	// budget). Tests inject a faster schedule.
	Backoffs []time.Duration
	// Logger is required; pass logger.NewNop() in tests.
	Logger infralogger.Logger
}

// Client is the concrete WaaseyaaClient implementation.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	backoffs   []time.Duration
	logger     infralogger.Logger
}

// defaultBackoffs is the production retry schedule. FR-011 / NFR-002.
var defaultBackoffs = []time.Duration{
	1 * time.Second,
	5 * time.Second,
	15 * time.Second,
}

// New constructs a Client. It returns an error if required fields are missing.
func New(cfg Config) (*Client, error) {
	if cfg.BaseURL == "" {
		return nil, errors.New("client: BaseURL is required")
	}
	if cfg.APIKey == "" {
		return nil, errors.New("client: APIKey is required")
	}
	if cfg.Logger == nil {
		return nil, errors.New("client: Logger is required")
	}
	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultRequestTimeout}
	}
	backoffs := cfg.Backoffs
	if len(backoffs) == 0 {
		backoffs = defaultBackoffs
	}
	return &Client{
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		httpClient: httpClient,
		backoffs:   backoffs,
		logger:     cfg.Logger,
	}, nil
}

// PostSignals POSTs the batch to Waaseyaa, retrying on 5xx and transport
// errors per FR-010/FR-011/NFR-002. Returns *IngestResult on 2xx.
func (c *Client) PostSignals(ctx context.Context, batch SignalBatch) (*IngestResult, error) {
	body, err := json.Marshal(batch)
	if err != nil {
		return nil, fmt.Errorf("client: marshal batch: %w", err)
	}
	url := c.baseURL + SignalsEndpointPath

	var result *IngestResult
	op := func(ctx context.Context) error {
		res, attemptErr := c.doOnce(ctx, url, body)
		if attemptErr != nil {
			return attemptErr
		}
		result = res
		return nil
	}
	if err := retry(ctx, c.backoffs, op, c.logger); err != nil {
		return nil, err
	}
	return result, nil
}

// doOnce performs a single HTTP attempt. Returns either the parsed
// IngestResult or a classified error (errClient / errServer / transport).
func (c *Client) doOnce(ctx context.Context, url string, body []byte) (*IngestResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set(HeaderContentType, ContentTypeJSON)
	req.Header.Set(HeaderAPIKey, c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network/transport error — let the retry helper treat as retryable.
		return nil, fmt.Errorf("client: transport: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < httpStatusClientErrorMin:
		return decodeIngestResult(resp.Body)
	case resp.StatusCode >= httpStatusClientErrorMin && resp.StatusCode < httpStatusServerErrorMin:
		return nil, fmt.Errorf("%w: status=%d", errClient, resp.StatusCode)
	case resp.StatusCode >= httpStatusServerErrorMin && resp.StatusCode < httpStatusEnd:
		return nil, fmt.Errorf("%w: status=%d", errServer, resp.StatusCode)
	default:
		return nil, fmt.Errorf("client: unexpected status=%d", resp.StatusCode)
	}
}

// decodeIngestResult parses the 2xx response body.
func decodeIngestResult(r io.Reader) (*IngestResult, error) {
	var out IngestResult
	if decodeErr := json.NewDecoder(r).Decode(&out); decodeErr != nil {
		return nil, fmt.Errorf("client: decode IngestResult: %w", decodeErr)
	}
	return &out, nil
}
