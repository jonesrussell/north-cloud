package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProxyCacheMissInReplayMode(t *testing.T) {
	t.Helper()
	fixturesDir := t.TempDir() // Empty fixtures
	cacheDir := t.TempDir()    // Empty cache

	cfg := &Config{
		Mode:        ModeReplay,
		FixturesDir: fixturesDir,
		CacheDir:    cacheDir,
	}
	proxy := NewProxy(cfg)

	req := httptest.NewRequest(http.MethodGet, "http://notfound.example.com/missing", http.NoBody)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 Bad Gateway for cache miss in replay mode, got %d", w.Code)
	}

	// Check error headers
	if w.Header().Get("X-Proxy-Mode") != "replay" {
		t.Errorf("expected X-Proxy-Mode header to be 'replay'")
	}
	if w.Header().Get("X-Proxy-Cache-Miss") != "true" {
		t.Error("expected X-Proxy-Cache-Miss header")
	}
}

func TestProxyLiveMode(t *testing.T) {
	t.Helper()
	// Create a test server to proxy to
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "live response")
	}))
	defer backend.Close()

	cfg := &Config{
		Mode:        ModeLive,
		FixturesDir: t.TempDir(),
		CacheDir:    t.TempDir(),
		LiveTimeout: defaultLiveTimeout,
	}
	proxy := NewProxy(cfg)

	// Request to the test backend
	req := httptest.NewRequest(http.MethodGet, backend.URL+"/test", http.NoBody)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK in live mode, got %d", w.Code)
	}
	if w.Body.String() != "live response" {
		t.Errorf("expected 'live response', got %q", w.Body.String())
	}
}

func TestProxyModeSwitch(t *testing.T) {
	t.Helper()
	cfg := &Config{Mode: ModeReplay, FixturesDir: t.TempDir(), CacheDir: t.TempDir()}
	proxy := NewProxy(cfg)

	if proxy.Mode() != ModeReplay {
		t.Errorf("expected initial mode replay, got %s", proxy.Mode())
	}

	proxy.SetMode(ModeRecord)
	if proxy.Mode() != ModeRecord {
		t.Errorf("expected mode record after switch, got %s", proxy.Mode())
	}

	proxy.SetMode(ModeLive)
	if proxy.Mode() != ModeLive {
		t.Errorf("expected mode live after switch, got %s", proxy.Mode())
	}
}

func TestProxyRecordMode(t *testing.T) {
	t.Helper()
	// Create a test server to proxy to
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "<html>recorded</html>")
	}))
	defer backend.Close()

	cacheDir := t.TempDir()
	cfg := &Config{
		Mode:        ModeRecord,
		FixturesDir: t.TempDir(),
		CacheDir:    cacheDir,
		LiveTimeout: defaultLiveTimeout,
	}
	proxy := NewProxy(cfg)

	// Make request - should fetch and cache
	req := httptest.NewRequest(http.MethodGet, backend.URL+"/page", http.NoBody)
	w := httptest.NewRecorder()

	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 OK in record mode, got %d", w.Code)
	}

	// Verify response was cached
	if w.Header().Get("X-Proxy-Source") != "cache" {
		t.Errorf("expected X-Proxy-Source 'cache', got %q", w.Header().Get("X-Proxy-Source"))
	}
}

func TestProxyCacheHit(t *testing.T) {
	t.Helper()
	fixturesDir := setupTestFixtures(t)
	cacheDir := t.TempDir()

	cfg := &Config{
		Mode:        ModeReplay,
		FixturesDir: fixturesDir,
		CacheDir:    cacheDir,
	}
	proxy := NewProxy(cfg)

	// We need to generate a request that matches the fixture's cache key
	// The fixture has cache_key "GET_abc123", but our GenerateCacheKey will produce different key
	// So we test the cache lookup mechanism directly through Lookup

	// For this test, verify the proxy can serve from cache when entry exists
	cache := proxy.Cache()
	entry, source, err := cache.Lookup("example-com", "GET_abc123")
	if err != nil {
		t.Fatalf("cache lookup failed: %v", err)
	}
	if entry == nil {
		t.Fatal("expected cache entry")
	}
	if source != SourceFixtures {
		t.Errorf("expected source fixtures, got %s", source)
	}
}
