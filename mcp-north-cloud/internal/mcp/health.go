package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// healthCheckTimeout is the per-service timeout for health checks.
const healthCheckTimeout = 5 * time.Second

// serviceHealthResult holds the health check result for a single service.
type serviceHealthResult struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	ResponseTimeMs int64  `json:"response_time_ms"`
	Error          string `json:"error,omitempty"`
}

// handleHealthCheck checks connectivity to all configured backend services.
func (s *Server) handleHealthCheck(ctx context.Context, id any, _ json.RawMessage) *Response {
	if len(s.serviceURLs) == 0 {
		return s.successResponse(id, map[string]any{
			"message": "No service URLs configured",
		})
	}

	results := s.checkAllServices(ctx)

	healthyCount := 0
	for i := range results {
		if results[i].Status == "reachable" {
			healthyCount++
		}
	}

	return s.successResponse(id, map[string]any{
		"services":      results,
		"healthy_count": healthyCount,
		"total_count":   len(results),
		"message":       fmt.Sprintf("%d of %d services are healthy", healthyCount, len(results)),
	})
}

// checkAllServices probes all configured services in parallel.
func (s *Server) checkAllServices(ctx context.Context) []serviceHealthResult {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make([]serviceHealthResult, 0, len(s.serviceURLs))
	)

	for name, baseURL := range s.serviceURLs {
		wg.Add(1)
		go func(name, baseURL string) {
			defer wg.Done()
			result := checkService(ctx, name, baseURL)
			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(name, baseURL)
	}

	wg.Wait()

	// Sort by name for deterministic output
	sort.Slice(results, func(i, j int) bool {
		return results[i].Name < results[j].Name
	})

	return results
}

// checkService probes a single service's health endpoint.
func checkService(ctx context.Context, name, baseURL string) serviceHealthResult {
	ctx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()

	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/health", http.NoBody)
	if err != nil {
		return serviceHealthResult{
			Name:           name,
			Status:         "unreachable",
			ResponseTimeMs: time.Since(start).Milliseconds(),
			Error:          "failed to create request",
		}
	}

	resp, err := http.DefaultClient.Do(req)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		return serviceHealthResult{
			Name:           name,
			Status:         "unreachable",
			ResponseTimeMs: elapsed,
			Error:          "connection failed",
		}
	}
	defer resp.Body.Close()

	status := "reachable"
	var errMsg string

	const maxHealthyStatusCode = 299
	if resp.StatusCode > maxHealthyStatusCode {
		status = "unhealthy"
		errMsg = fmt.Sprintf("status code %d", resp.StatusCode)
	}

	return serviceHealthResult{
		Name:           name,
		Status:         status,
		ResponseTimeMs: elapsed,
		Error:          errMsg,
	}
}
