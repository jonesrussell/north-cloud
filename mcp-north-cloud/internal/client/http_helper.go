package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// HTTPHelper provides common HTTP request/response handling
type HTTPHelper struct {
	client  *http.Client
	baseURL string
}

// NewHTTPHelper creates a new HTTP helper
func NewHTTPHelper(client *http.Client, baseURL string) *HTTPHelper {
	return &HTTPHelper{
		client:  client,
		baseURL: baseURL,
	}
}

// DoJSONRequest performs a JSON request and unmarshals the response
func (h *HTTPHelper) DoJSONRequest(
	ctx context.Context,
	method, endpoint string,
	requestBody any,
	successCodes []int,
	errorPrefix string,
) ([]byte, error) {
	var bodyReader io.Reader
	if requestBody != nil {
		body, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if status code is in success list
	isSuccess := false
	for _, code := range successCodes {
		if resp.StatusCode == code {
			isSuccess = true
			break
		}
	}

	if !isSuccess {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("%s error: %s", errorPrefix, errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// DoJSONRequestWithResponse performs a JSON request and unmarshals the response into a struct
func DoJSONRequestWithResponse[T any](
	ctx context.Context,
	client *http.Client,
	method, endpoint string,
	requestBody any,
	successCodes []int,
	errorPrefix string,
) (*T, error) {
	var bodyReader io.Reader
	if requestBody != nil {
		body, err := json.Marshal(requestBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if status code is in success list
	isSuccess := false
	for _, code := range successCodes {
		if resp.StatusCode == code {
			isSuccess = true
			break
		}
	}

	if !isSuccess {
		var errorResp struct {
			Error string `json:"error"`
		}
		if jsonErr := json.Unmarshal(respBody, &errorResp); jsonErr == nil && errorResp.Error != "" {
			return nil, fmt.Errorf("%s error: %s", errorPrefix, errorResp.Error)
		}
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(respBody))
	}

	var result T
	if err = json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &result, nil
}

// DoJSONRequestNoBody performs a JSON request with no body and unmarshals the response
func DoJSONRequestNoBody[T any](
	ctx context.Context,
	client *http.Client,
	method, endpoint string,
	successCodes []int,
	errorPrefix string,
) (*T, error) {
	return DoJSONRequestWithResponse[T](ctx, client, method, endpoint, nil, successCodes, errorPrefix)
}
