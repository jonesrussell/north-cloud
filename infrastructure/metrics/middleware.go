// Package metrics provides Prometheus-style metrics middleware for HTTP services.
package metrics

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Metrics tracks HTTP request metrics
type Metrics struct {
	mu                sync.RWMutex
	requestCount      map[string]int64      // method + path -> count
	requestDuration   map[string]time.Duration // method + path -> total duration
	requestErrors     map[string]int64      // method + path -> error count
	activeRequests    int64                 // Currently active requests
}

// NewMetrics creates a new metrics tracker
func NewMetrics() *Metrics {
	return &Metrics{
		requestCount:    make(map[string]int64),
		requestDuration: make(map[string]time.Duration),
		requestErrors:   make(map[string]int64),
	}
}

// Middleware returns HTTP middleware that tracks request metrics
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		key := r.Method + " " + r.URL.Path

		// Increment active requests
		m.mu.Lock()
		m.activeRequests++
		m.mu.Unlock()

		// Wrap response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call next handler
		next.ServeHTTP(rw, r)

		// Record metrics
		duration := time.Since(start)
		m.mu.Lock()
		m.requestCount[key]++
		m.requestDuration[key] += duration
		if rw.statusCode >= 400 {
			m.requestErrors[key]++
		}
		m.activeRequests--
		m.mu.Unlock()
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// GetRequestCount returns the request count for a given method and path
func (m *Metrics) GetRequestCount(method, path string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestCount[method+" "+path]
}

// GetRequestDuration returns the total request duration for a given method and path
func (m *Metrics) GetRequestDuration(method, path string) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestDuration[method+" "+path]
}

// GetRequestErrors returns the error count for a given method and path
func (m *Metrics) GetRequestErrors(method, path string) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.requestErrors[method+" "+path]
}

// GetActiveRequests returns the number of currently active requests
func (m *Metrics) GetActiveRequests() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.activeRequests
}

// GetAllMetrics returns all metrics as a map
func (m *Metrics) GetAllMetrics() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]any)
	result["active_requests"] = m.activeRequests

	counts := make(map[string]int64)
	for k, v := range m.requestCount {
		counts[k] = v
	}
	result["request_counts"] = counts

	durations := make(map[string]string)
	for k, v := range m.requestDuration {
		durations[k] = v.String()
	}
	result["request_durations"] = durations

	errors := make(map[string]int64)
	for k, v := range m.requestErrors {
		errors[k] = v
	}
	result["request_errors"] = errors

	return result
}

// Reset resets all metrics
func (m *Metrics) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCount = make(map[string]int64)
	m.requestDuration = make(map[string]time.Duration)
	m.requestErrors = make(map[string]int64)
	m.activeRequests = 0
}

// PrometheusHandler returns an HTTP handler that exposes metrics in Prometheus format
func (m *Metrics) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		
		m.mu.RLock()
		defer m.mu.RUnlock()

		// Write active requests
		w.Write([]byte("# HELP http_active_requests Number of currently active HTTP requests\n"))
		w.Write([]byte("# TYPE http_active_requests gauge\n"))
		w.Write([]byte("http_active_requests " + strconv.FormatInt(m.activeRequests, 10) + "\n\n"))

		// Write request counts
		w.Write([]byte("# HELP http_requests_total Total number of HTTP requests\n"))
		w.Write([]byte("# TYPE http_requests_total counter\n"))
		for key, count := range m.requestCount {
			w.Write([]byte(`http_requests_total{method="` + extractMethod(key) + `",path="` + extractPath(key) + `"} ` + strconv.FormatInt(count, 10) + "\n"))
		}
		w.Write([]byte("\n"))

		// Write request durations
		w.Write([]byte("# HELP http_request_duration_seconds Total HTTP request duration in seconds\n"))
		w.Write([]byte("# TYPE http_request_duration_seconds counter\n"))
		for key, duration := range m.requestDuration {
			seconds := duration.Seconds()
			w.Write([]byte(`http_request_duration_seconds{method="` + extractMethod(key) + `",path="` + extractPath(key) + `"} ` + strconv.FormatFloat(seconds, 'f', 6, 64) + "\n"))
		}
		w.Write([]byte("\n"))

		// Write request errors
		w.Write([]byte("# HELP http_request_errors_total Total number of HTTP request errors\n"))
		w.Write([]byte("# TYPE http_request_errors_total counter\n"))
		for key, count := range m.requestErrors {
			w.Write([]byte(`http_request_errors_total{method="` + extractMethod(key) + `",path="` + extractPath(key) + `"} ` + strconv.FormatInt(count, 10) + "\n"))
		}
	}
}

// extractMethod extracts the HTTP method from a key
func extractMethod(key string) string {
	for i, char := range key {
		if char == ' ' {
			return key[:i]
		}
	}
	return key
}

// extractPath extracts the path from a key
func extractPath(key string) string {
	for i, char := range key {
		if char == ' ' {
			return key[i+1:]
		}
	}
	return ""
}

