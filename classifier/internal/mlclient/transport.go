package mlclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// doPost marshals body as JSON, POSTs to path with retries on network errors or 503,
// and returns the raw response bytes, HTTP status code, and any error.
func (c *Client) doPost(ctx context.Context, path string, body any) (respBytes []byte, statusCode int, retErr error) {
	var lastErr error
	var lastStatus int

	attempts := c.opts.retryCount + 1
	for i := range attempts {
		respBody, status, postErr := c.doSinglePost(ctx, path, body)
		if postErr == nil && status != http.StatusServiceUnavailable {
			return respBody, status, nil
		}

		lastErr = postErr
		lastStatus = status

		// Do not sleep after the last attempt.
		if i < attempts-1 {
			backoff := c.backoffDelay(i)
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, 0, fmt.Errorf("request cancelled during retry: %w", ctx.Err())
			case <-timer.C:
			}
		}
	}

	if lastErr != nil {
		return nil, lastStatus, lastErr
	}

	return nil, lastStatus, fmt.Errorf("ml service returned %d", lastStatus)
}

// backoffDelay returns an exponential backoff duration with jitter for the given attempt index.
func (c *Client) backoffDelay(attempt int) time.Duration {
	base := c.opts.retryBaseDelay
	for range attempt {
		base *= 2
	}
	// Add jitter: 50-150% of the base delay.
	jitter := 0.5 + rand.Float64() //nolint:mnd // jitter multiplier range [0.5, 1.5)
	return time.Duration(float64(base) * jitter)
}

// doSinglePost performs a single POST request to path with body marshalled as JSON.
func (c *Client) doSinglePost(ctx context.Context, path string, body any) (respBytes []byte, statusCode int, err error) {
	reqBody, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		return nil, 0, fmt.Errorf("marshal request: %w", marshalErr)
	}

	reqCtx, cancel := context.WithTimeout(ctx, c.opts.timeout)
	defer cancel()

	httpReq, reqErr := http.NewRequestWithContext(
		reqCtx, http.MethodPost, c.baseURL+path, bytes.NewReader(reqBody),
	)
	if reqErr != nil {
		return nil, 0, fmt.Errorf("create request: %w", reqErr)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, doErr := c.httpClient.Do(httpReq)
	if doErr != nil {
		return nil, 0, fmt.Errorf("http request: %w", doErr)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", readErr)
	}

	return respBody, resp.StatusCode, nil
}

// doGet performs a simple GET request to path with timeout.
func (c *Client) doGet(ctx context.Context, path string) (respBytes []byte, statusCode int, err error) {
	reqCtx, cancel := context.WithTimeout(ctx, c.opts.timeout)
	defer cancel()

	httpReq, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, c.baseURL+path, http.NoBody)
	if reqErr != nil {
		return nil, 0, fmt.Errorf("create request: %w", reqErr)
	}

	resp, doErr := c.httpClient.Do(httpReq)
	if doErr != nil {
		return nil, 0, fmt.Errorf("http request: %w", doErr)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response body: %w", readErr)
	}

	return respBody, resp.StatusCode, nil
}
