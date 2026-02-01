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
