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

func TestNormalizeImageField(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "string URL",
			input:    "https://example.com/image.jpg",
			expected: "https://example.com/image.jpg",
		},
		{
			name:     "object with url field",
			input:    map[string]any{"url": "https://example.com/image.jpg", "width": 800},
			expected: "https://example.com/image.jpg",
		},
		{
			name:     "object without url",
			input:    map[string]any{"width": 800},
			expected: nil,
		},
		{
			name:     "array with string",
			input:    []any{"https://example.com/image.jpg"},
			expected: "https://example.com/image.jpg",
		},
		{
			name:     "array with object",
			input:    []any{map[string]any{"url": "https://example.com/image.jpg"}},
			expected: "https://example.com/image.jpg",
		},
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			result := rawcontent.NormalizeImageField(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeImageField(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeObjectToName(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "string name",
			input:    "Publisher Inc",
			expected: "Publisher Inc",
		},
		{
			name:     "object with name",
			input:    map[string]any{"name": "Publisher Inc", "@type": "Organization"},
			expected: "Publisher Inc",
		},
		{
			name:     "object without name",
			input:    map[string]any{"@type": "Organization"},
			expected: nil,
		},
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			result := rawcontent.NormalizeObjectToName(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeObjectToName(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeEntityToURL(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "string URL",
			input:    "https://example.com/article",
			expected: "https://example.com/article",
		},
		{
			name:     "object with @id",
			input:    map[string]any{"@id": "https://example.com/article", "@type": "WebPage"},
			expected: "https://example.com/article",
		},
		{
			name:     "object with url",
			input:    map[string]any{"url": "https://example.com/article"},
			expected: "https://example.com/article",
		},
		{
			name:     "object without @id or url",
			input:    map[string]any{"@type": "WebPage"},
			expected: nil,
		},
		{
			name:     "nil",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			result := rawcontent.NormalizeEntityToURL(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeEntityToURL(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeToString(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "string value",
			input:    "1500",
			expected: "1500",
		},
		{
			name:     "int value",
			input:    1500,
			expected: "1500",
		},
		{
			name:     "float value",
			input:    1500.5,
			expected: "1500.5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			result := rawcontent.NormalizeToString(tt.input)
			if result != tt.expected {
				t.Errorf("NormalizeToString(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestNormalizeJSONLDObject_AllFields(t *testing.T) {
	t.Helper()

	t.Run("normalizes image object to string", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context": "https://schema.org",
			"image":    map[string]any{"url": "https://example.com/img.jpg", "width": 800},
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		img, ok := result["image"].(string)
		if !ok {
			t.Fatalf("expected string image, got %T: %v", result["image"], result["image"])
		}
		if img != "https://example.com/img.jpg" {
			t.Errorf("expected image URL, got %q", img)
		}
	})

	t.Run("normalizes publisher object to string", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context":  "https://schema.org",
			"publisher": map[string]any{"name": "News Corp", "@type": "Organization"},
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		pub, ok := result["publisher"].(string)
		if !ok {
			t.Fatalf("expected string publisher, got %T: %v", result["publisher"], result["publisher"])
		}
		if pub != "News Corp" {
			t.Errorf("expected publisher name, got %q", pub)
		}
	})

	t.Run("normalizes mainEntityOfPage object to string", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context":         "https://schema.org",
			"mainEntityOfPage": map[string]any{"@id": "https://example.com/page", "@type": "WebPage"},
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		me, ok := result["mainEntityOfPage"].(string)
		if !ok {
			t.Fatalf("expected string mainEntityOfPage, got %T: %v", result["mainEntityOfPage"], result["mainEntityOfPage"])
		}
		if me != "https://example.com/page" {
			t.Errorf("expected mainEntityOfPage URL, got %q", me)
		}
	})

	t.Run("normalizes wordCount int to string", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context":  "https://schema.org",
			"wordCount": 1500,
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		wc, ok := result["wordCount"].(string)
		if !ok {
			t.Fatalf("expected string wordCount, got %T: %v", result["wordCount"], result["wordCount"])
		}
		if wc != "1500" {
			t.Errorf("expected '1500', got %q", wc)
		}
	})

	t.Run("removes image when no URL extractable", func(t *testing.T) {
		t.Helper()

		input := map[string]any{
			"@context": "https://schema.org",
			"image":    map[string]any{"width": 800},
		}
		result := rawcontent.NormalizeJSONLDObject(input)
		if _, exists := result["image"]; exists {
			t.Errorf("expected image to be removed, got %v", result["image"])
		}
	})
}
