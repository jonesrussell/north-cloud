// Package pipeline_test provides tests for the pipeline client library.
package pipeline_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/north-cloud/infrastructure/pipeline"
)

func TestClient_Emit_Success(t *testing.T) {
	t.Helper()

	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)

		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/events" {
			t.Errorf("path = %s, want /api/v1/events", r.URL.Path)
		}

		var req map[string]any
		if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
			t.Errorf("decode error: %v", decodeErr)
		}

		if req["stage"] != "crawled" {
			t.Errorf("stage = %v, want crawled", req["stage"])
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := pipeline.NewClient(server.URL, "test-service")
	ctx := context.Background()

	emitErr := client.Emit(ctx, pipeline.Event{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       "crawled",
		OccurredAt:  time.Now().UTC(),
		ServiceName: "test-service",
	})

	if emitErr != nil {
		t.Errorf("Emit() error = %v", emitErr)
	}

	if !received.Load() {
		t.Error("expected server to receive the event")
	}
}

func TestClient_Emit_NoopWhenURLEmpty(t *testing.T) {
	t.Helper()

	client := pipeline.NewClient("", "test-service")
	ctx := context.Background()

	emitErr := client.Emit(ctx, pipeline.Event{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       "crawled",
		OccurredAt:  time.Now().UTC(),
		ServiceName: "test-service",
	})

	if emitErr != nil {
		t.Errorf("Emit() should be no-op when URL is empty, got error: %v", emitErr)
	}
}

func TestClient_EmitBatch_SingleRequest(t *testing.T) {
	t.Helper()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		if r.URL.Path != "/api/v1/events/batch" {
			t.Errorf("path = %s, want /api/v1/events/batch", r.URL.Path)
		}

		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := pipeline.NewClient(server.URL, "test-service")
	ctx := context.Background()

	events := []pipeline.Event{
		{ArticleURL: "https://example.com/1", SourceName: "test", Stage: "crawled", OccurredAt: time.Now().UTC(), ServiceName: "test"},
		{ArticleURL: "https://example.com/2", SourceName: "test", Stage: "crawled", OccurredAt: time.Now().UTC(), ServiceName: "test"},
		{ArticleURL: "https://example.com/3", SourceName: "test", Stage: "crawled", OccurredAt: time.Now().UTC(), ServiceName: "test"},
	}

	batchErr := client.EmitBatch(ctx, events)
	if batchErr != nil {
		t.Errorf("EmitBatch() error = %v", batchErr)
	}

	if requestCount.Load() != 1 {
		t.Errorf("EmitBatch() made %d requests, want 1", requestCount.Load())
	}
}

func TestClient_Emit_ClientError(t *testing.T) {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := pipeline.NewClient(server.URL, "test-service")
	ctx := context.Background()

	emitErr := client.Emit(ctx, pipeline.Event{
		ArticleURL: "https://example.com/article",
		SourceName: "example_com",
		Stage:      "crawled",
		OccurredAt: time.Now().UTC(),
	})

	if emitErr == nil {
		t.Error("Emit() should return error for 401 response")
	}
}

func TestClient_CircuitBreaker_Opens(t *testing.T) {
	t.Helper()

	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := pipeline.NewClient(server.URL, "test-service")
	ctx := context.Background()

	event := pipeline.Event{
		ArticleURL:  "https://example.com/article",
		SourceName:  "example_com",
		Stage:       "crawled",
		OccurredAt:  time.Now().UTC(),
		ServiceName: "test-service",
	}

	// Trigger enough failures to trip the circuit breaker
	const tripThreshold = 6
	for i := range tripThreshold {
		_ = i
		_ = client.Emit(ctx, event)
	}

	requestsBefore := requestCount.Load()

	// Next call should be blocked by circuit breaker (no HTTP request)
	_ = client.Emit(ctx, event)

	if requestCount.Load() != requestsBefore {
		t.Error("expected circuit breaker to block the request")
	}
}
