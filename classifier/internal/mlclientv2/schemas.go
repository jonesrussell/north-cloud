// Package mlclientv2 provides a unified Go client for ML sidecar services
// that speak the standard response envelope.
package mlclientv2

import "encoding/json"

// StandardResponse is the envelope returned by POST /v1/classify on every ML sidecar.
type StandardResponse struct {
	Module           string          `json:"module"`
	Version          string          `json:"version"`
	SchemaVersion    string          `json:"schema_version"`
	Result           json.RawMessage `json:"result"`
	Relevance        *float64        `json:"relevance"`
	Confidence       *float64        `json:"confidence"`
	ProcessingTimeMs float64         `json:"processing_time_ms"`
	RequestID        string          `json:"request_id"`
}

// StandardError is the envelope returned on error responses from ML sidecars.
type StandardError struct {
	Error     string `json:"error"`
	Message   string `json:"message"`
	Module    string `json:"module"`
	RequestID string `json:"request_id"`
	Timestamp string `json:"timestamp"`
}

// HealthResponse is the envelope returned by GET /v1/health on every ML sidecar.
type HealthResponse struct {
	Status        string          `json:"status"`
	Module        string          `json:"module"`
	Version       string          `json:"version"`
	SchemaVersion string          `json:"schema_version"`
	ModelsLoaded  bool            `json:"models_loaded"`
	UptimeSeconds float64         `json:"uptime_seconds"`
	Checks        map[string]bool `json:"checks,omitempty"`
}

// classifyRequest is the POST body sent to /v1/classify.
type classifyRequest struct {
	Title    string         `json:"title"`
	Body     string         `json:"body"`
	Metadata map[string]any `json:"metadata,omitempty"`
}
