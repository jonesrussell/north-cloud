package elasticsearch //nolint:testpackage // testing unexported normalizeSourceName function

import (
	"testing"
)

// --- normalizeSourceName ---

func TestNormalizeSourceName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple domain", "example.com", "example_com"},
		{"with protocol http", "http://example.com", "example_com"},
		{"with protocol https", "https://example.com", "example_com"},
		{"with hyphens", "my-news-site.com", "my_news_site_com"},
		{"already normalized", "example_com", "example_com"},
		{"uppercase", "Example.COM", "example_com"},
		{"multiple dots", "sub.domain.example.com", "sub_domain_example_com"},
		{"mixed hyphens and dots", "my-site.example.com", "my_site_example_com"},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeSourceName(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeSourceName(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// --- NewIndexManager ---

func TestNewIndexManager(t *testing.T) {
	t.Helper()

	client := &Client{}
	mgr := NewIndexManager(client)

	if mgr == nil {
		t.Fatal("NewIndexManager() returned nil")
	}
	if mgr.client != client {
		t.Error("NewIndexManager() did not set client")
	}
}
