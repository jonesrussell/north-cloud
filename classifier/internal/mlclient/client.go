package mlclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client communicates with a single ML sidecar module.
type Client struct {
	moduleName string
	baseURL    string
	httpClient *http.Client
	opts       clientOptions
	breaker    *circuitBreaker
}

// NewClient creates a Client for the given ML module.
func NewClient(moduleName, baseURL string, opts ...Option) *Client {
	o := defaultOptions()
	for _, fn := range opts {
		fn(&o)
	}

	return &Client{
		moduleName: moduleName,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: o.timeout},
		opts:       o,
		breaker:    newBreaker(o.breakerTrips, o.breakerCooldown),
	}
}

// Classify sends a classification request to the ML sidecar and returns the standard response.
func (c *Client) Classify(ctx context.Context, title, body string) (*StandardResponse, error) {
	if !c.breaker.allow() {
		return nil, fmt.Errorf("%s: %w", c.moduleName, ErrUnavailable)
	}

	req := classifyRequest{
		Title: title,
		Body:  body,
	}

	respBody, status, postErr := c.doPost(ctx, "/v1/classify", req)
	if postErr != nil {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("%s classify: %w", c.moduleName, postErr)
	}

	if status != http.StatusOK {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("%s classify: service returned %d", c.moduleName, status)
	}

	var resp StandardResponse
	if unmarshalErr := json.Unmarshal(respBody, &resp); unmarshalErr != nil {
		c.breaker.recordFailure()
		return nil, fmt.Errorf("%s classify: decode response: %w", c.moduleName, unmarshalErr)
	}

	c.breaker.recordSuccess()

	return &resp, nil
}

// Health checks the ML sidecar health endpoint. Health does not use the circuit breaker
// so it can be called even when the breaker is open.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	respBody, status, getErr := c.doGet(ctx, "/v1/health")
	if getErr != nil {
		return nil, fmt.Errorf("%s health: %w", c.moduleName, getErr)
	}

	if status != http.StatusOK {
		return nil, fmt.Errorf("%s health: service returned %d", c.moduleName, status)
	}

	var resp HealthResponse
	if unmarshalErr := json.Unmarshal(respBody, &resp); unmarshalErr != nil {
		return nil, fmt.Errorf("%s health: decode response: %w", c.moduleName, unmarshalErr)
	}

	return &resp, nil
}
