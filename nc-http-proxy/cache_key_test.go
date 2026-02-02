package main

import (
	"net/http"
	"testing"
)

func TestNormalizeDomain(t *testing.T) {
	t.Helper()
	tests := []struct {
		input    string
		expected string
	}{
		{"www.CalgaryHerald.com", "calgaryherald-com"},
		{"example.com", "example-com"},
		{"WWW.EXAMPLE.COM", "example-com"},
		{"sub.domain.co.uk", "sub-domain-co-uk"},
	}

	for _, tc := range tests {
		result := NormalizeDomain(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestNormalizeURL(t *testing.T) {
	t.Helper()
	tests := []struct {
		input    string
		expected string
	}{
		{"https://Example.com/path", "https://example.com/path"},
		{"https://example.com/path?b=2&a=1", "https://example.com/path?a=1&b=2"},
		{"https://example.com/path?utm_source=fb&a=1", "https://example.com/path?a=1"},
		{"https://example.com/path?fbclid=123&a=1", "https://example.com/path?a=1"},
	}

	for _, tc := range tests {
		result := NormalizeURL(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeURL(%q) = %q, want %q", tc.input, result, tc.expected)
		}
	}
}

func TestGenerateCacheKey(t *testing.T) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, "https://example.com/article", http.NoBody)
	req.Header.Set("User-Agent", "NorthCloud-Crawler/1.0")
	req.Header.Set("Accept-Language", "en-US")

	key := GenerateCacheKey(req)

	// Key format: METHOD_hash[:12]
	minKeyLength := 16 // "GET_" + 12 chars minimum
	if len(key) < minKeyLength {
		t.Errorf("cache key too short: %q", key)
	}
	if key[:4] != "GET_" {
		t.Errorf("cache key should start with 'GET_', got %q", key)
	}
}

func TestGenerateCacheKeyDifferentMethods(t *testing.T) {
	t.Helper()
	getReq, _ := http.NewRequest(http.MethodGet, "https://example.com/page", http.NoBody)
	postReq, _ := http.NewRequest(http.MethodPost, "https://example.com/page", http.NoBody)

	getKey := GenerateCacheKey(getReq)
	postKey := GenerateCacheKey(postReq)

	if getKey == postKey {
		t.Error("GET and POST should have different cache keys")
	}
	if getKey[:4] != "GET_" {
		t.Errorf("GET key should start with 'GET_', got %q", getKey)
	}
	postPrefixLen := 5
	if postKey[:postPrefixLen] != "POST_" {
		t.Errorf("POST key should start with 'POST_', got %q", postKey)
	}
}

func TestGenerateCacheKeyDeterministic(t *testing.T) {
	t.Helper()
	req1, _ := http.NewRequest(http.MethodGet, "https://example.com/article?a=1&b=2", http.NoBody)
	req1.Header.Set("User-Agent", "NorthCloud-Crawler/1.0")

	req2, _ := http.NewRequest(http.MethodGet, "https://example.com/article?b=2&a=1", http.NoBody)
	req2.Header.Set("User-Agent", "NorthCloud-Crawler/1.0")

	key1 := GenerateCacheKey(req1)
	key2 := GenerateCacheKey(req2)

	if key1 != key2 {
		t.Errorf("same URL with reordered params should have same key: %q != %q", key1, key2)
	}
}
