package mlclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
)

func newTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	return httptest.NewServer(handler)
}

func float64Ptr(f float64) *float64 {
	return &f
}

func TestClassifySuccess(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := mlclient.StandardResponse{
			Module:           "crime",
			Version:          "v1",
			SchemaVersion:    "1.0",
			Result:           json.RawMessage(`{"crime_types":["assault"]}`),
			Relevance:        float64Ptr(0.9),
			Confidence:       float64Ptr(0.8),
			ProcessingTimeMs: 12.0,
			RequestID:        "req-1",
		}
		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	defer srv.Close()

	client := mlclient.NewClient("crime", srv.URL, mlclient.WithTimeout(2*time.Second))

	resp, err := client.Classify(context.Background(), "Test", "Body")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Module != "crime" {
		t.Fatalf("expected module 'crime', got %q", resp.Module)
	}

	if *resp.Relevance != 0.9 {
		t.Fatalf("expected relevance 0.9, got %f", *resp.Relevance)
	}
}

func TestClassifyTimeout(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(_ http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
	})
	defer srv.Close()

	client := mlclient.NewClient("slow", srv.URL, mlclient.WithTimeout(50*time.Millisecond))

	_, err := client.Classify(context.Background(), "Test", "Body")
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestClassifyCircuitBreaker(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	defer srv.Close()

	client := mlclient.NewClient("test", srv.URL,
		mlclient.WithCircuitBreaker(2, 5*time.Second),
		mlclient.WithRetry(0, 10*time.Millisecond),
	)

	// Trip the breaker with two failures.
	_, _ = client.Classify(context.Background(), "a", "b")
	_, _ = client.Classify(context.Background(), "a", "b")

	// Circuit should be open now — no network call should be made.
	beforeCount := callCount.Load()

	_, err := client.Classify(context.Background(), "a", "b")
	if err == nil {
		t.Fatal("expected ErrUnavailable when circuit is open")
	}

	if callCount.Load() != beforeCount {
		t.Fatal("circuit breaker should prevent network calls when open")
	}
}

func TestHealthSuccess(t *testing.T) {
	t.Parallel()

	srv := newTestServer(t, func(w http.ResponseWriter, _ *http.Request) {
		resp := mlclient.HealthResponse{
			Status:        "healthy",
			Module:        "crime",
			Version:       "v1",
			SchemaVersion: "1.0",
			ModelsLoaded:  true,
			UptimeSeconds: 120.0,
		}
		w.Header().Set("Content-Type", "application/json")

		if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
			http.Error(w, encodeErr.Error(), http.StatusInternalServerError)
		}
	})
	defer srv.Close()

	client := mlclient.NewClient("crime", srv.URL)

	health, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if health.Status != "healthy" {
		t.Fatalf("expected healthy, got %q", health.Status)
	}
}
