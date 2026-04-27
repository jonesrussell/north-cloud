package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/signal-producer/internal/client"
)

// fastBackoffs is the test retry schedule. Total sleep ~60ms, keeping the
// suite snappy.
var fastBackoffs = []time.Duration{
	10 * time.Millisecond,
	20 * time.Millisecond,
	30 * time.Millisecond,
}

const testAPIKey = "test-api-key-123"

// newTestClient builds a Client wired to the supplied test server. Tests
// inject fastBackoffs so retries don't make the suite slow.
func newTestClient(t *testing.T, srv *httptest.Server) *client.Client {
	t.Helper()
	c, err := client.New(client.Config{
		BaseURL:    srv.URL,
		APIKey:     testAPIKey,
		HTTPClient: srv.Client(),
		Backoffs:   fastBackoffs,
		Logger:     infralogger.NewNop(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return c
}

// assertRequestShape verifies the producer-to-Waaseyaa wire contract.
func assertRequestShape(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get(client.HeaderAPIKey); got != testAPIKey {
		t.Errorf("X-Api-Key header: got %q, want %q", got, testAPIKey)
	}
	if got := r.Header.Get(client.HeaderContentType); got != client.ContentTypeJSON {
		t.Errorf("Content-Type header: got %q, want %q", got, client.ContentTypeJSON)
	}
	if r.URL.Path != client.SignalsEndpointPath {
		t.Errorf("path: got %q, want %q", r.URL.Path, client.SignalsEndpointPath)
	}
	if r.Method != http.MethodPost {
		t.Errorf("method: got %q, want POST", r.Method)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var envelope struct {
		Signals []any `json:"signals"`
	}
	if unmarshalErr := json.Unmarshal(body, &envelope); unmarshalErr != nil {
		t.Fatalf("decode body: %v: %s", unmarshalErr, string(body))
	}
	if envelope.Signals == nil {
		t.Errorf("body.signals must be present (even empty), got nil")
	}
}

func sampleBatch() client.SignalBatch {
	return client.SignalBatch{Signals: []any{
		map[string]any{"external_id": "x1", "label": "test"},
	}}
}

func TestPostSignals_Success(t *testing.T) {
	t.Parallel()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		assertRequestShape(t, r)
		w.Header().Set(client.HeaderContentType, client.ContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.IngestResult{
			Ingested: 1, Skipped: 0, LeadsCreated: 1, LeadsMatched: 0, Unmatched: 0,
		})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.PostSignals(context.Background(), sampleBatch())
	if err != nil {
		t.Fatalf("PostSignals: %v", err)
	}
	if got == nil {
		t.Fatal("got nil result")
	}
	if got.Ingested != 1 || got.LeadsCreated != 1 {
		t.Errorf("IngestResult parse: got %+v", got)
	}
	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Errorf("server hits: got %d, want 1", h)
	}
}

func TestPostSignals_RetriesOn5xxThenSucceeds(t *testing.T) {
	t.Parallel()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		if n < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(client.IngestResult{Ingested: 5})
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	got, err := c.PostSignals(context.Background(), sampleBatch())
	if err != nil {
		t.Fatalf("PostSignals: %v", err)
	}
	if got.Ingested != 5 {
		t.Errorf("got Ingested=%d, want 5", got.Ingested)
	}
	if h := atomic.LoadInt32(&hits); h != 3 {
		t.Errorf("server hits: got %d, want 3", h)
	}
}

func TestPostSignals_NoRetryOn4xx(t *testing.T) {
	t.Parallel()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.PostSignals(context.Background(), sampleBatch())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, client.ErrClient) {
		t.Errorf("expected ErrClient, got %v", err)
	}
	if h := atomic.LoadInt32(&hits); h != 1 {
		t.Errorf("server hits: got %d, want 1 (no retry on 4xx)", h)
	}
}

func TestPostSignals_RetriesExhausted(t *testing.T) {
	t.Parallel()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.PostSignals(context.Background(), sampleBatch())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, client.ErrServer) {
		t.Errorf("expected ErrServer wrap, got %v", err)
	}
	// 1 initial + 3 retries = 4 hits.
	if h := atomic.LoadInt32(&hits); h != 4 {
		t.Errorf("server hits: got %d, want 4", h)
	}
}

func TestPostSignals_ContextCancelDuringRetrySleep(t *testing.T) {
	t.Parallel()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Use a slower backoff so we can race a cancel into the sleep.
	c, err := client.New(client.Config{
		BaseURL:    srv.URL,
		APIKey:     testAPIKey,
		HTTPClient: srv.Client(),
		Backoffs:   []time.Duration{500 * time.Millisecond, 500 * time.Millisecond, 500 * time.Millisecond},
		Logger:     infralogger.NewNop(),
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		// Cancel after the first failed attempt is in flight, while we're sleeping.
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, callErr := c.PostSignals(ctx, sampleBatch())
	if callErr == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(callErr, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", callErr)
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("cancel was not honored promptly; elapsed=%v", elapsed)
	}
}

func TestNew_ValidatesRequiredFields(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		cfg  client.Config
	}{
		{"missing baseurl", client.Config{APIKey: "k", Logger: infralogger.NewNop()}},
		{"missing apikey", client.Config{BaseURL: "http://x", Logger: infralogger.NewNop()}},
		{"missing logger", client.Config{BaseURL: "http://x", APIKey: "k"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := client.New(tc.cfg); err == nil {
				t.Errorf("expected error for %s", tc.name)
			}
		})
	}
}
