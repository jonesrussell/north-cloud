//nolint:testpackage // Testing internal client requires same package access
package coforgemlclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Classify(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/classify" {
			t.Errorf("expected /classify, got %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		response := ClassifyResponse{
			Relevance:           "core_coforge",
			RelevanceConfidence: 0.92,
			Audience:            "hybrid",
			AudienceConfidence:  0.78,
			Topics:              []string{"funding_round", "devtools"},
			TopicScores:         map[string]float64{"funding_round": 0.88, "devtools": 0.71},
			Industries:          []string{"saas", "ai_ml"},
			IndustryScores:      map[string]float64{"saas": 0.82, "ai_ml": 0.65},
			ProcessingTimeMs:    52,
			ModelVersion:        "2026-02-08-coforge-v1",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Classify(context.Background(), "AI startup open-sources SDK", "Fintech company...")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_coforge" {
		t.Errorf("expected core_coforge, got %s", result.Relevance)
	}
	if result.Audience != "hybrid" {
		t.Errorf("expected hybrid, got %s", result.Audience)
	}
	if len(result.Topics) != 2 {
		t.Errorf("expected 2 topics, got %d", len(result.Topics))
	}
	if len(result.Industries) != 2 {
		t.Errorf("expected 2 industries, got %d", len(result.Industries))
	}
	if result.ModelVersion != "2026-02-08-coforge-v1" {
		t.Errorf("expected model_version, got %s", result.ModelVersion)
	}
}

func TestClient_Classify_Non200(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	_, err := client.Classify(context.Background(), "title", "body")

	if err == nil {
		t.Fatal("expected error for non-200 response")
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

func TestClient_Classify_ErrUnavailable(t *testing.T) {
	t.Helper()

	client := NewClient("http://localhost:99999")
	_, err := client.Classify(context.Background(), "title", "body")

	if err == nil {
		t.Fatal("expected error for unreachable service")
	}
	if !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected ErrUnavailable, got %v", err)
	}
}
