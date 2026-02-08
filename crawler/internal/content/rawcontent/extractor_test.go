package rawcontent_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
)

func TestNormalizeContextField(t *testing.T) {
	t.Helper()

	defaultSchemaOrgURL := "https://schema.org"

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "string context",
			input:    "https://schema.org",
			expected: "https://schema.org",
		},
		{
			name:     "object context with @vocab",
			input:    map[string]any{"@vocab": "https://schema.org/"},
			expected: "https://schema.org/",
		},
		{
			name:     "object context without @vocab",
			input:    map[string]any{"@type": "Person"},
			expected: defaultSchemaOrgURL,
		},
		{
			name:     "array context with string first",
			input:    []any{"https://schema.org", map[string]any{"@vocab": "https://example.com"}},
			expected: "https://schema.org",
		},
		{
			name:     "array context with only objects",
			input:    []any{map[string]any{"@vocab": "https://example.com"}},
			expected: defaultSchemaOrgURL,
		},
		{
			name:     "nil context",
			input:    nil,
			expected: defaultSchemaOrgURL,
		},
		{
			name:     "numeric context",
			input:    42,
			expected: defaultSchemaOrgURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			result := rawcontent.NormalizeContextField(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeContextField(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeJSONLDObject_Context(t *testing.T) {
	t.Helper()

	t.Run("normalizes string context", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context": "https://schema.org",
			"@type":    "NewsArticle",
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		if ctx, ok := result["@context"].(string); !ok || ctx != "https://schema.org" {
			t.Errorf("expected string context, got %T: %v", result["@context"], result["@context"])
		}
	})

	t.Run("normalizes object context to string", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context": map[string]any{"@vocab": "https://schema.org/"},
			"@type":    "NewsArticle",
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		ctx, ok := result["@context"].(string)
		if !ok {
			t.Fatalf("expected string context, got %T: %v", result["@context"], result["@context"])
		}
		if ctx != "https://schema.org/" {
			t.Errorf("expected 'https://schema.org/', got %q", ctx)
		}
	})

	t.Run("normalizes array context to string", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context": []any{"https://schema.org", map[string]any{"@vocab": "https://example.com"}},
			"@type":    "NewsArticle",
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		ctx, ok := result["@context"].(string)
		if !ok {
			t.Fatalf("expected string context, got %T: %v", result["@context"], result["@context"])
		}
		if ctx != "https://schema.org" {
			t.Errorf("expected 'https://schema.org', got %q", ctx)
		}
	})
}
