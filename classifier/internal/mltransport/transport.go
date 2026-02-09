// Package mltransport provides shared HTTP transport for ML sidecar classify and health.
package mltransport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 5 * time.Second

// ClassifyRequest is the common request body for POST /classify.
type ClassifyRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// healthResponse is the JSON shape returned by GET /health (model_version optional).
type healthResponse struct {
	ModelVersion string `json:"model_version"`
}

// DoClassify sends POST /classify to baseURL with req, decoding the response into respPtr.
// respPtr must be a pointer to a struct that matches the ML service response (e.g. *ClassifyResponse).
func DoClassify(ctx context.Context, baseURL string, req *ClassifyRequest, respPtr any) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/classify", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: defaultTimeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ml service returned %d", resp.StatusCode)
	}

	if decodeErr := json.NewDecoder(resp.Body).Decode(respPtr); decodeErr != nil {
		return fmt.Errorf("decode response: %w", decodeErr)
	}

	return nil
}

// DoHealth calls GET /health at baseURL and returns reachable, latencyMs, model_version, and any error.
func DoHealth(ctx context.Context, baseURL string) (reachable bool, latencyMs int64, modelVersion string, err error) {
	start := time.Now()

	httpReq, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", http.NoBody)
	if reqErr != nil {
		return false, 0, "", fmt.Errorf("create request: %w", reqErr)
	}

	client := &http.Client{Timeout: defaultTimeout}
	resp, doErr := client.Do(httpReq)
	latencyMs = time.Since(start).Milliseconds()
	if doErr != nil {
		return false, latencyMs, "", fmt.Errorf("service unreachable: %w", doErr)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return false, latencyMs, "", fmt.Errorf("unhealthy status: %d", resp.StatusCode)
	}

	reachable = true
	var healthResp healthResponse
	if decodeErr := json.NewDecoder(resp.Body).Decode(&healthResp); decodeErr == nil {
		modelVersion = healthResp.ModelVersion
	}
	return reachable, latencyMs, modelVersion, nil
}
