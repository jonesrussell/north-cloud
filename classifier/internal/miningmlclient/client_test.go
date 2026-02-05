//nolint:testpackage // Testing internal client requires same package access
package miningmlclient

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
			Relevance:             "core_mining",
			RelevanceConfidence:   0.93,
			MiningStage:           "exploration",
			MiningStageConfidence: 0.88,
			Commodities:           []string{"gold", "copper"},
			CommodityScores:       map[string]float64{"gold": 0.91, "copper": 0.62},
			Location:              "local_canada",
			LocationConfidence:    0.80,
			ProcessingTimeMs:      34,
			ModelVersion:          "2025-02-01-mining-v1",
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	result, err := client.Classify(context.Background(), "Gold exploration in Ontario", "Drill results...")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Relevance != "core_mining" {
		t.Errorf("expected core_mining, got %s", result.Relevance)
	}

	if result.MiningStage != "exploration" {
		t.Errorf("expected exploration, got %s", result.MiningStage)
	}

	if len(result.Commodities) != 2 {
		t.Errorf("expected 2 commodities, got %d", len(result.Commodities))
	}

	if result.ModelVersion != "2025-02-01-mining-v1" {
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
