package fetcher_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

// testCacheTTL is the cache duration used in tests.
const testCacheTTL = time.Hour

// newTestChecker creates a RobotsChecker for testing.
func newTestChecker(t *testing.T) *fetcher.RobotsChecker {
	t.Helper()

	return fetcher.NewRobotsChecker(
		&http.Client{Timeout: testCacheTTL},
		"TestBot/1.0",
		testCacheTTL,
	)
}

func TestIsAllowed_URLAllowed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
	}))
	defer server.Close()

	checker := newTestChecker(t)

	allowed, err := checker.IsAllowed(context.Background(), server.URL+"/public/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected /public/page to be allowed, got disallowed")
	}
}

func TestIsAllowed_URLDisallowed(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
	}))
	defer server.Close()

	checker := newTestChecker(t)

	allowed, err := checker.IsAllowed(context.Background(), server.URL+"/private/secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if allowed {
		t.Error("expected /private/secret to be disallowed, got allowed")
	}
}

func TestIsAllowed_Missing404(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	checker := newTestChecker(t)

	allowed, err := checker.IsAllowed(context.Background(), server.URL+"/any/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected allow-all when robots.txt returns 404")
	}
}

func TestIsAllowed_ServerError500(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	checker := newTestChecker(t)

	allowed, err := checker.IsAllowed(context.Background(), server.URL+"/any/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !allowed {
		t.Error("expected allow-all when robots.txt returns 500 (graceful degradation)")
	}
}

func TestIsAllowed_CacheHit(t *testing.T) {
	t.Parallel()

	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nAllow: /\n"))
	}))
	defer server.Close()

	checker := newTestChecker(t)

	// First call — fetches from server.
	allowed, err := checker.IsAllowed(context.Background(), server.URL+"/page1")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}

	if !allowed {
		t.Error("expected first call to be allowed")
	}

	// Second call — should use cache, NOT hit server again.
	allowed, err = checker.IsAllowed(context.Background(), server.URL+"/page2")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}

	if !allowed {
		t.Error("expected second call to be allowed")
	}

	expectedRequests := int32(1)
	if actual := requestCount.Load(); actual != expectedRequests {
		t.Errorf("expected %d server request(s), got %d (cache miss)", expectedRequests, actual)
	}
}

func TestCrawlDelay_Extraction(t *testing.T) {
	t.Parallel()

	const expectedDelay = 5 * time.Second

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nCrawl-delay: 5\nDisallow: /private/\n"))
	}))
	defer server.Close()

	checker := newTestChecker(t)

	// Trigger a fetch so the entry is cached.
	_, err := checker.IsAllowed(context.Background(), server.URL+"/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	host := extractTestHost(t, server.URL)
	delay := checker.CrawlDelay(host)

	if delay != expectedDelay {
		t.Errorf("expected crawl delay %v, got %v", expectedDelay, delay)
	}
}

func TestCrawlDelay_NoneSet(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
	}))
	defer server.Close()

	checker := newTestChecker(t)

	// Trigger fetch to cache.
	_, err := checker.IsAllowed(context.Background(), server.URL+"/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	host := extractTestHost(t, server.URL)
	delay := checker.CrawlDelay(host)

	if delay != 0 {
		t.Errorf("expected zero crawl delay, got %v", delay)
	}
}

func TestCrawlDelay_UncachedHost(t *testing.T) {
	t.Parallel()

	checker := fetcher.NewRobotsChecker(
		&http.Client{Timeout: testCacheTTL},
		"TestBot/1.0",
		testCacheTTL,
	)

	delay := checker.CrawlDelay("uncached.example.com")

	if delay != 0 {
		t.Errorf("expected zero crawl delay for uncached host, got %v", delay)
	}
}

// extractTestHost extracts the host:port from an httptest.Server URL.
func extractTestHost(t *testing.T, serverURL string) string {
	t.Helper()

	// httptest URLs are like http://127.0.0.1:PORT
	// We need just the host:port part.
	const schemePrefix = "http://"

	if len(serverURL) <= len(schemePrefix) {
		t.Fatalf("invalid server URL: %s", serverURL)
	}

	return serverURL[len(schemePrefix):]
}
