package frontier_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
)

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Scheme and host normalization
		{"lowercase scheme", "HTTP://Example.com/Path", "https://example.com/Path", false},
		{"lowercase host", "https://EXAMPLE.COM/path", "https://example.com/path", false},
		{"upgrade http to https", "http://example.com/path", "https://example.com/path", false},

		// Port handling
		{"remove default https port", "https://example.com:443/path", "https://example.com/path", false},
		{"remove default http port", "http://example.com:80/path", "https://example.com/path", false},
		{"keep non-default port", "https://example.com:8080/path", "https://example.com:8080/path", false},

		// Path normalization
		{"remove trailing slash", "https://example.com/path/", "https://example.com/path", false},
		{"keep root slash", "https://example.com/", "https://example.com/", false},
		{"path only no query", "https://example.com/news/article-123", "https://example.com/news/article-123", false},
		{"resolve dot segments", "https://example.com/a/b/../c", "https://example.com/a/c", false},
		{"resolve current dir segments", "https://example.com/a/./b", "https://example.com/a/b", false},

		// Fragment removal
		{"remove fragment", "https://example.com/path#section", "https://example.com/path", false},

		// Query parameter handling
		{"sort query params", "https://example.com/path?z=1&a=2", "https://example.com/path?a=2&z=1", false},
		{"strip utm params", "https://example.com/path?utm_source=twitter&id=1", "https://example.com/path?id=1", false},
		{"strip fbclid", "https://example.com/path?fbclid=abc123&id=1", "https://example.com/path?id=1", false},
		{"strip gclid", "https://example.com/path?gclid=xyz&page=2", "https://example.com/path?page=2", false},
		{
			"strip all tracking params",
			"https://example.com/?utm_source=a&utm_medium=b&utm_campaign=c" +
				"&utm_term=d&utm_content=e&fbclid=f&gclid=g&gclsrc=h&dclid=i&msclkid=j&keep=yes",
			"https://example.com/?keep=yes",
			false,
		},
		{"empty query after stripping", "https://example.com/path?utm_source=x", "https://example.com/path", false},

		// Error cases
		{"empty string", "", "", true},
		{"invalid url", "://not-a-url", "", true},
		{"missing scheme", "example.com/path", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := frontier.NormalizeURL(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("NormalizeURL(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("NormalizeURL(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestURLHash_EquivalentURLs(t *testing.T) {
	hash1, err := frontier.URLHash("HTTP://Example.com/path?b=2&a=1")
	if err != nil {
		t.Fatalf("URLHash() unexpected error: %v", err)
	}

	hash2, err := frontier.URLHash("https://example.com/path?a=1&b=2")
	if err != nil {
		t.Fatalf("URLHash() unexpected error: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("expected identical hashes for equivalent URLs, got %q and %q", hash1, hash2)
	}
}

func TestURLHash_Length(t *testing.T) {
	const sha256HexLength = 64

	hash, err := frontier.URLHash("https://example.com")
	if err != nil {
		t.Fatalf("URLHash() unexpected error: %v", err)
	}

	if len(hash) != sha256HexLength {
		t.Errorf("expected hash length %d, got %d", sha256HexLength, len(hash))
	}

	for _, c := range hash {
		if !strings.ContainsRune("0123456789abcdef", c) {
			t.Errorf("hash contains non-hex character: %c", c)
			break
		}
	}
}

func TestURLHash_DifferentURLs(t *testing.T) {
	hash1, err := frontier.URLHash("https://example.com/page-a")
	if err != nil {
		t.Fatalf("URLHash() unexpected error: %v", err)
	}

	hash2, err := frontier.URLHash("https://example.com/page-b")
	if err != nil {
		t.Fatalf("URLHash() unexpected error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes for different URLs")
	}
}

func TestURLHash_Errors(t *testing.T) {
	t.Run("empty string", func(t *testing.T) {
		_, err := frontier.URLHash("")
		if err == nil {
			t.Error("URLHash(\"\") expected error, got nil")
		}
	})

	t.Run("invalid URL", func(t *testing.T) {
		_, err := frontier.URLHash("://bad")
		if err == nil {
			t.Error("URLHash(\"://bad\") expected error, got nil")
		}
	})
}

func TestExtractHost(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"simple", "https://example.com/path", "example.com", false},
		{"with port", "https://example.com:8080/path", "example.com", false},
		{"with www", "https://www.example.com/path", "www.example.com", false},
		{"uppercase host", "https://EXAMPLE.COM/path", "example.com", false},
		{"empty string", "", "", true},
		{"invalid url", "://bad", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := frontier.ExtractHost(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractHost(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ExtractHost(%q) unexpected error: %v", tt.input, err)
				return
			}

			if got != tt.want {
				t.Errorf("ExtractHost(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
