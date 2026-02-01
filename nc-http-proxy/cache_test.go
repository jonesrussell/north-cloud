package main

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestFixtures(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	domainDir := filepath.Join(dir, "example-com")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatal(err)
	}

	metadataContent := `{
		"request": {
			"method": "GET",
			"url": "https://example.com/article",
			"headers": {"User-Agent": "Test"}
		},
		"response": {
			"status": 200,
			"headers": {"Content-Type": "text/html"},
			"was_compressed": false
		},
		"recorded_at": "2026-02-01T14:30:00Z",
		"cache_key": "GET_abc123"
	}`
	if err := os.WriteFile(filepath.Join(domainDir, "GET_abc123.json"), []byte(metadataContent), 0644); err != nil {
		t.Fatal(err)
	}

	bodyContent := "<html><body>Test</body></html>"
	if err := os.WriteFile(filepath.Join(domainDir, "GET_abc123.body"), []byte(bodyContent), 0644); err != nil {
		t.Fatal(err)
	}

	return dir
}

func TestCacheLookupHit(t *testing.T) {
	t.Helper()
	fixturesDir := setupTestFixtures(t)
	cacheDir := t.TempDir()

	cache := NewCache(fixturesDir, cacheDir)

	entry, source, err := cache.Lookup("example-com", "GET_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected entry, got nil")
	}
	if source != SourceFixtures {
		t.Errorf("expected source fixtures, got %s", source)
	}
	if entry.Metadata.Response.Status != 200 {
		t.Errorf("expected status 200, got %d", entry.Metadata.Response.Status)
	}
	if string(entry.Body) != "<html><body>Test</body></html>" {
		t.Errorf("unexpected body: %s", string(entry.Body))
	}
}

func TestCacheLookupMiss(t *testing.T) {
	t.Helper()
	fixturesDir := t.TempDir()
	cacheDir := t.TempDir()

	cache := NewCache(fixturesDir, cacheDir)

	entry, source, err := cache.Lookup("example-com", "GET_notfound")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry != nil {
		t.Error("expected nil entry for cache miss")
	}
	if source != SourceNone {
		t.Errorf("expected source none, got %s", source)
	}
}

func TestCacheLookupFixturesPriority(t *testing.T) {
	t.Helper()
	fixturesDir := setupTestFixtures(t)
	cacheDir := t.TempDir()

	// Create same entry in cache with different content
	cacheDomainDir := filepath.Join(cacheDir, "example-com")
	if err := os.MkdirAll(cacheDomainDir, 0755); err != nil {
		t.Fatal(err)
	}

	cacheMetadata := `{
		"request": {"method": "GET", "url": "https://example.com/article", "headers": {}},
		"response": {"status": 404, "headers": {}, "was_compressed": false},
		"recorded_at": "2026-02-01T15:00:00Z",
		"cache_key": "GET_abc123"
	}`
	if err := os.WriteFile(filepath.Join(cacheDomainDir, "GET_abc123.json"), []byte(cacheMetadata), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDomainDir, "GET_abc123.body"), []byte("cache body"), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache(fixturesDir, cacheDir)

	entry, source, err := cache.Lookup("example-com", "GET_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Fixtures should take priority
	if source != SourceFixtures {
		t.Errorf("expected source fixtures (priority), got %s", source)
	}
	if entry.Metadata.Response.Status != 200 {
		t.Errorf("expected fixtures status 200, got cache status %d", entry.Metadata.Response.Status)
	}
}

func TestCacheLookupMissingBody(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	domainDir := filepath.Join(dir, "example-com")
	if err := os.MkdirAll(domainDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create only metadata, no body
	metadataContent := `{"request": {}, "response": {"status": 200}, "cache_key": "GET_partial"}`
	if err := os.WriteFile(filepath.Join(domainDir, "GET_partial.json"), []byte(metadataContent), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache(dir, t.TempDir())

	entry, source, err := cache.Lookup("example-com", "GET_partial")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should treat as miss when body is missing
	if entry != nil {
		t.Error("expected nil entry for partial cache (missing body)")
	}
	if source != SourceNone {
		t.Errorf("expected source none for partial cache, got %s", source)
	}
}

func TestCacheStore(t *testing.T) {
	t.Helper()
	cacheDir := t.TempDir()
	cache := NewCache(t.TempDir(), cacheDir)

	entry := &CacheEntry{
		Domain:   "example-com",
		CacheKey: "GET_newentry",
		BaseDir:  cacheDir,
		Metadata: &CacheEntryMetadata{
			Request:  CachedRequest{Method: "GET", URL: "https://example.com/new"},
			Response: CachedResponse{Status: 200},
			CacheKey: "GET_newentry",
		},
		Body: []byte("<html>New content</html>"),
	}

	if err := cache.Store(entry); err != nil {
		t.Fatalf("failed to store: %v", err)
	}

	// Verify files were created
	metaPath := filepath.Join(cacheDir, "example-com", "GET_newentry.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Errorf("metadata file not created: %v", err)
	}

	bodyPath := filepath.Join(cacheDir, "example-com", "GET_newentry.body")
	if _, err := os.Stat(bodyPath); err != nil {
		t.Errorf("body file not created: %v", err)
	}

	// Verify content
	retrieved, source, err := cache.Lookup("example-com", "GET_newentry")
	if err != nil {
		t.Fatalf("lookup error: %v", err)
	}
	if source != SourceCache {
		t.Errorf("expected source cache, got %s", source)
	}
	if string(retrieved.Body) != "<html>New content</html>" {
		t.Errorf("body mismatch: %s", string(retrieved.Body))
	}
}

func TestCacheStats(t *testing.T) {
	t.Helper()
	fixturesDir := setupTestFixtures(t)
	cacheDir := t.TempDir()

	// Add another domain to cache
	domain2Dir := filepath.Join(cacheDir, "other-com")
	if err := os.MkdirAll(domain2Dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domain2Dir, "GET_xyz.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(domain2Dir, "GET_xyz.body"), []byte("body"), 0644); err != nil {
		t.Fatal(err)
	}

	cache := NewCache(fixturesDir, cacheDir)
	stats := cache.Stats()

	if stats.FixturesCount < 1 {
		t.Errorf("expected at least 1 fixture, got %d", stats.FixturesCount)
	}
	if stats.CacheCount < 1 {
		t.Errorf("expected at least 1 cache entry, got %d", stats.CacheCount)
	}
	if len(stats.Domains) < 1 {
		t.Errorf("expected at least 1 domain, got %d", len(stats.Domains))
	}
}
