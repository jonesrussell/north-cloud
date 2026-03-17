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

func newOKServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		resp := mlclient.StandardResponse{
			Module:        "test",
			Version:       "v1",
			SchemaVersion: "1.0",
			Result:        json.RawMessage(`{}`),
			RequestID:     "ok-1",
		}
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
}

func TestBreakerStartsClosed(t *testing.T) {
	t.Parallel()

	srv := newOKServer(t)
	defer srv.Close()

	client := mlclient.NewClient("test", srv.URL, mlclient.WithCircuitBreaker(3, 100*time.Millisecond))

	_, err := client.Classify(context.Background(), "title", "body")
	if err != nil {
		t.Fatalf("breaker should allow requests when closed: %v", err)
	}
}

func TestBreakerOpensAfterTrips(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := mlclient.NewClient("test", srv.URL,
		mlclient.WithCircuitBreaker(3, 100*time.Millisecond),
		mlclient.WithRetry(0, time.Millisecond),
	)

	// Trip the breaker with 3 failures.
	for range 3 {
		_, _ = client.Classify(context.Background(), "a", "b")
	}

	beforeCount := callCount.Load()

	_, err := client.Classify(context.Background(), "a", "b")
	if err == nil {
		t.Fatal("breaker should be open after 3 failures")
	}

	if callCount.Load() != beforeCount {
		t.Fatal("no network call should be made when breaker is open")
	}
}

func TestBreakerResetsOnSuccess(t *testing.T) {
	t.Parallel()

	var failNext atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if failNext.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}

		resp := mlclient.StandardResponse{
			Module:        "test",
			Version:       "v1",
			SchemaVersion: "1.0",
			Result:        json.RawMessage(`{}`),
			RequestID:     "ok",
		}
		w.Header().Set("Content-Type", "application/json")

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	client := mlclient.NewClient("test", srv.URL,
		mlclient.WithCircuitBreaker(3, 100*time.Millisecond),
		mlclient.WithRetry(0, time.Millisecond),
	)

	// Record 2 failures.
	failNext.Store(true)
	_, _ = client.Classify(context.Background(), "a", "b")
	_, _ = client.Classify(context.Background(), "a", "b")

	// One success should reset the counter.
	failNext.Store(false)

	_, err := client.Classify(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}

	// Two more failures should not open the breaker (counter was reset).
	failNext.Store(true)
	_, _ = client.Classify(context.Background(), "a", "b")
	_, _ = client.Classify(context.Background(), "a", "b")

	// Breaker should still be closed (only 2 failures, threshold is 3).
	failNext.Store(false)

	_, err = client.Classify(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("breaker should be closed after success reset: %v", err)
	}
}

func TestBreakerHalfOpenAfterCooldown(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	var succeedNext atomic.Bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		callCount.Add(1)

		if succeedNext.Load() {
			resp := mlclient.StandardResponse{
				Module:        "test",
				Version:       "v1",
				SchemaVersion: "1.0",
				Result:        json.RawMessage(`{}`),
				RequestID:     "ok",
			}
			w.Header().Set("Content-Type", "application/json")

			if err := json.NewEncoder(w).Encode(resp); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := mlclient.NewClient("test", srv.URL,
		mlclient.WithCircuitBreaker(1, 50*time.Millisecond),
		mlclient.WithRetry(0, time.Millisecond),
	)

	// Trip the breaker.
	_, _ = client.Classify(context.Background(), "a", "b")

	// Should be open immediately — no network call made.
	beforeCount := callCount.Load()

	_, err := client.Classify(context.Background(), "a", "b")
	if err == nil {
		t.Fatal("breaker should be open immediately after trip")
	}

	if callCount.Load() != beforeCount {
		t.Fatal("no network call should be made when breaker is open")
	}

	// Wait for cooldown then try again — breaker should allow a probe (half-open).
	time.Sleep(60 * time.Millisecond)

	// Switch server to succeed so the probe request closes the breaker.
	succeedNext.Store(true)

	_, err = client.Classify(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("breaker should allow probe in half-open state: %v", err)
	}

	// After a successful probe, the breaker should be closed — subsequent calls should succeed.
	_, err = client.Classify(context.Background(), "a", "b")
	if err != nil {
		t.Fatalf("breaker should be closed after successful probe: %v", err)
	}
}
