package callback

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testAPIKey = "test-callback-secret"

func TestSendEnrichmentPostsJSONWithAPIKey(t *testing.T) {
	t.Parallel()

	result := validResult(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want %s", r.Method, http.MethodPost)
		}
		if got := r.Header.Get(HeaderAPIKey); got != testAPIKey {
			t.Fatalf("api key header = %q, want %q", got, testAPIKey)
		}
		if got := r.Header.Get(contentTypeHeader); got != jsonContentType {
			t.Fatalf("content type = %q, want %q", got, jsonContentType)
		}

		var payload EnrichmentResult
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if payload.LeadID != result.LeadID || payload.Type != result.Type {
			t.Fatalf("payload = %#v, want lead/type from %#v", payload, result)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	client := New(Config{Backoffs: noBackoff(t)})
	if err := client.SendEnrichment(context.Background(), server.URL, testAPIKey, result); err != nil {
		t.Fatalf("send enrichment: %v", err)
	}
}

func TestSendEnrichmentRetriesServerErrors(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if hits.Add(1) == 1 {
			http.Error(w, "temporary", http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(server.Close)

	client := New(Config{Backoffs: []time.Duration{time.Millisecond}})
	if err := client.SendEnrichment(context.Background(), server.URL, testAPIKey, validResult(t)); err != nil {
		t.Fatalf("send enrichment: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("hits = %d, want 2", got)
	}
}

func TestSendEnrichmentDoesNotRetryClientErrors(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "bad callback", http.StatusUnauthorized)
	}))
	t.Cleanup(server.Close)

	client := New(Config{Backoffs: []time.Duration{time.Millisecond, time.Millisecond}})
	err := client.SendEnrichment(context.Background(), server.URL, testAPIKey, validResult(t))
	if err == nil {
		t.Fatal("send enrichment returned nil error, want 4xx error")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("hits = %d, want 1", got)
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatal("error leaked callback API key")
	}
}

func TestSendEnrichmentRetriesNetworkErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	callbackURL := server.URL
	server.Close()

	client := New(Config{
		HTTPClient: server.Client(),
		Backoffs:   []time.Duration{time.Millisecond},
	})
	err := client.SendEnrichment(context.Background(), callbackURL, testAPIKey, validResult(t))
	if err == nil {
		t.Fatal("send enrichment returned nil error, want network error")
	}
	if !strings.Contains(err.Error(), "post callback") {
		t.Fatalf("error = %q, want callback post context", err.Error())
	}
}

func TestSendEnrichmentStopsBackoffOnContextCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		http.Error(w, "temporary", http.StatusServiceUnavailable)
		cancel()
	}))
	t.Cleanup(server.Close)

	client := New(Config{Backoffs: []time.Duration{time.Hour}})
	err := client.SendEnrichment(ctx, server.URL, testAPIKey, validResult(t))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("hits = %d, want 1", got)
	}
}

func TestSendEnrichmentRejectsInvalidCallbackURLWithoutSecretLeak(t *testing.T) {
	t.Parallel()

	client := New(Config{Backoffs: noBackoff(t)})
	err := client.SendEnrichment(context.Background(), "https://user:pass@example .com/callback", testAPIKey, validResult(t))
	if err == nil {
		t.Fatal("send enrichment returned nil error, want invalid URL error")
	}
	if strings.Contains(err.Error(), testAPIKey) {
		t.Fatal("error leaked callback API key")
	}
}

func validResult(t *testing.T) EnrichmentResult {
	t.Helper()

	return EnrichmentResult{
		LeadID:     "lead-123",
		Type:       "company_intel",
		Status:     "success",
		Confidence: 0.91,
		Data: map[string]any{
			"summary": "Acme Mining operates in Northern Ontario.",
		},
	}
}

func noBackoff(t *testing.T) []time.Duration {
	t.Helper()

	return []time.Duration{}
}
