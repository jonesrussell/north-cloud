// classifier/internal/mlclient/client_test.go
//
//nolint:testpackage // Testing internal client requires same package access
package mlclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Classify(t *testing.T) {
	t.Helper()

	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" {
			t.Errorf("expected /classify, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := ClassifyResponse{
			Relevance:           "core_street_crime",
			RelevanceConfidence: 0.85,
			CrimeTypes:          []string{"violent_crime"},
			CrimeTypeScores:     map[string]float64{"violent_crime": 0.9},
			Location:            "local_canada",
			LocationConfidence:  0.75,
			ProcessingTimeMs:    15,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Classify(context.Background(), "Man charged with murder", "Police arrested...")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_street_crime" {
		t.Errorf("expected core_street_crime, got %s", result.Relevance)
	}

	if result.RelevanceConfidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %f", result.RelevanceConfidence)
	}
}

func TestClient_Health(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("expected /health, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Health(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_HealthUnhealthy(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	err := client.Health(context.Background())

	if err == nil {
		t.Fatal("expected error for unhealthy service")
	}
}
