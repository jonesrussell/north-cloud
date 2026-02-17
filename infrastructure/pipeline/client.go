// Package pipeline provides a shared client library for emitting events to the Pipeline Service.
// It is designed as fire-and-forget: errors are returned but should be logged as warnings, not treated as fatal.
// When the base URL is empty, all methods are no-ops, allowing services to optionally integrate.
package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ErrCircuitBreakerOpen is returned when the circuit breaker is open and blocking requests.
var ErrCircuitBreakerOpen = errors.New("pipeline circuit breaker open")

const (
	defaultTimeout            = 2 * time.Second
	circuitBreakerThreshold   = 5
	circuitBreakerHalfOpenAge = 30 * time.Second
	circuitBreakerCloseAfter  = 2
)

// Event represents a pipeline event to emit.
type Event struct {
	ArticleURL     string         `json:"article_url"`
	SourceName     string         `json:"source_name"`
	Stage          string         `json:"stage"`
	OccurredAt     time.Time      `json:"occurred_at"`
	ServiceName    string         `json:"service_name"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type batchRequest struct {
	Events []Event `json:"events"`
}

type circuitState int

const (
	circuitClosed circuitState = iota
	circuitOpen
	circuitHalfOpen
)

type circuitBreaker struct {
	mu                  sync.Mutex
	state               circuitState
	consecutiveFailures int
	lastFailure         time.Time
	successesSinceOpen  int
}

// Client is a fire-and-forget pipeline event emitter.
type Client struct {
	baseURL     string
	serviceName string
	httpClient  *http.Client
	breaker     *circuitBreaker
}

// NewClient creates a new pipeline client. If baseURL is empty, all methods are no-ops.
func NewClient(baseURL, serviceName string) *Client {
	return &Client{
		baseURL:     baseURL,
		serviceName: serviceName,
		httpClient:  &http.Client{Timeout: defaultTimeout},
		breaker:     &circuitBreaker{},
	}
}

// IsEnabled returns true if the client is configured with a URL.
func (c *Client) IsEnabled() bool {
	return c.baseURL != ""
}

// CircuitOpen returns true if the circuit breaker is open.
func (c *Client) CircuitOpen() bool {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	return c.breaker.state == circuitOpen
}

// Emit sends a single event to the Pipeline Service. Fire-and-forget: errors are returned
// but should be logged as warnings, not treated as fatal.
// The client automatically sets ServiceName on the event from the value passed to NewClient.
func (c *Client) Emit(ctx context.Context, event Event) error {
	if !c.IsEnabled() {
		return nil
	}

	if !c.breakerAllow() {
		return ErrCircuitBreakerOpen
	}

	event.ServiceName = c.serviceName

	body, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		return fmt.Errorf("marshal event: %w", marshalErr)
	}

	return c.doPost(ctx, "/api/v1/events", body)
}

// EmitBatch sends multiple events in a single HTTP request.
// The client automatically sets ServiceName on each event from the value passed to NewClient.
func (c *Client) EmitBatch(ctx context.Context, events []Event) error {
	if !c.IsEnabled() || len(events) == 0 {
		return nil
	}

	if !c.breakerAllow() {
		return ErrCircuitBreakerOpen
	}

	for i := range events {
		events[i].ServiceName = c.serviceName
	}

	batch := batchRequest{Events: events}

	body, marshalErr := json.Marshal(batch)
	if marshalErr != nil {
		return fmt.Errorf("marshal batch: %w", marshalErr)
	}

	return c.doPost(ctx, "/api/v1/events/batch", body)
}

// doPost performs an HTTP POST and records circuit breaker results.
func (c *Client) doPost(ctx context.Context, path string, body []byte) error {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if reqErr != nil {
		c.breakerRecordFailure()

		return fmt.Errorf("create request: %w", reqErr)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, doErr := c.httpClient.Do(req)
	if doErr != nil {
		c.breakerRecordFailure()

		return fmt.Errorf("send request: %w", doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		c.breakerRecordFailure()

		return fmt.Errorf("pipeline service error: status %d", resp.StatusCode)
	}

	c.breakerRecordSuccess()

	return nil
}

func (c *Client) breakerAllow() bool {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	switch c.breaker.state {
	case circuitClosed:
		return true
	case circuitOpen:
		if time.Since(c.breaker.lastFailure) > circuitBreakerHalfOpenAge {
			c.breaker.state = circuitHalfOpen
			c.breaker.successesSinceOpen = 0

			return true
		}

		return false
	case circuitHalfOpen:
		return true
	}

	return true
}

func (c *Client) breakerRecordFailure() {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	c.breaker.consecutiveFailures++
	c.breaker.lastFailure = time.Now()
	c.breaker.successesSinceOpen = 0

	if c.breaker.consecutiveFailures >= circuitBreakerThreshold {
		c.breaker.state = circuitOpen
	}
}

func (c *Client) breakerRecordSuccess() {
	c.breaker.mu.Lock()
	defer c.breaker.mu.Unlock()

	if c.breaker.state == circuitHalfOpen {
		c.breaker.successesSinceOpen++

		if c.breaker.successesSinceOpen >= circuitBreakerCloseAfter {
			c.breaker.state = circuitClosed
			c.breaker.consecutiveFailures = 0
		}
	} else {
		c.breaker.consecutiveFailures = 0
	}
}
