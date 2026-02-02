package main

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestCacheEntryMetadataSerialization(t *testing.T) {
	t.Helper()
	metadata := &CacheEntryMetadata{
		Request: CachedRequest{
			Method: "GET",
			URL:    "https://example.com/article",
			Headers: map[string]string{
				"User-Agent":      "NorthCloud-Crawler/1.0",
				"Accept-Language": "en-US",
			},
		},
		Response: CachedResponse{
			Status:        200,
			Headers:       map[string]string{"Content-Type": "text/html; charset=utf-8"},
			WasCompressed: false,
		},
		RecordedAt: time.Date(2026, 2, 1, 14, 30, 0, 0, time.UTC),
		CacheKey:   "GET_abc123",
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded CacheEntryMetadata
	if unmarshalErr := json.Unmarshal(data, &decoded); unmarshalErr != nil {
		t.Fatalf("failed to unmarshal: %v", unmarshalErr)
	}

	if decoded.Request.Method != http.MethodGet {
		t.Errorf("expected method GET, got %s", decoded.Request.Method)
	}
	if decoded.Response.Status != 200 {
		t.Errorf("expected status 200, got %d", decoded.Response.Status)
	}
	if decoded.CacheKey != "GET_abc123" {
		t.Errorf("expected cache key GET_abc123, got %s", decoded.CacheKey)
	}
}

func TestCacheEntryFilePaths(t *testing.T) {
	t.Helper()
	entry := &CacheEntry{
		Domain:   "example-com",
		CacheKey: "GET_abc123",
		BaseDir:  "/app/fixtures",
	}

	metaPath := entry.MetadataPath()
	bodyPath := entry.BodyPath()

	expectedMeta := "/app/fixtures/example-com/GET_abc123.json"
	expectedBody := "/app/fixtures/example-com/GET_abc123.body"

	if metaPath != expectedMeta {
		t.Errorf("expected metadata path %q, got %q", expectedMeta, metaPath)
	}
	if bodyPath != expectedBody {
		t.Errorf("expected body path %q, got %q", expectedBody, bodyPath)
	}
}
