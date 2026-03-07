package naming

import (
	"testing"
)

func TestSanitizeSourceName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "already valid", input: "apnews_com", expected: "apnews_com"},
		{name: "uppercase", input: "Billboard", expected: "billboard"},
		{name: "spaces", input: "Campbell River Mirror", expected: "campbell_river_mirror"},
		{name: "mixed case with spaces", input: "Manitoba Keewatinowi Okimakanak", expected: "manitoba_keewatinowi_okimakanak"},
		{name: "parentheses", input: "Awards Circuit (Variety)", expected: "awards_circuit_variety"},
		{name: "multiple spaces", input: "Some  Double  Spaced", expected: "some_double_spaced"},
		{name: "leading trailing spaces", input: "  Billboard  ", expected: "billboard"},
		{name: "special chars", input: "CNET!", expected: "cnet"},
		{name: "dots and hyphens", input: "news.com-au", expected: "news_com_au"},
		{name: "empty string", input: "", expected: ""},
		{name: "all invalid chars", input: "!!!", expected: ""},
		{name: "protocol prefix", input: "http://example.com", expected: "http_example_com"},
		{name: "url with path", input: "https://example.com/news", expected: "https_example_com_news"},
		{name: "numbers", input: "News24", expected: "news24"},
		{name: "unicode", input: "Café Monde", expected: "caf_monde"},
		{name: "consecutive special chars", input: "a--b..c", expected: "a_b_c"},
		{name: "only underscores", input: "___", expected: ""},
		{name: "leading underscore after sanitize", input: "-test-", expected: "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := SanitizeSourceName(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeSourceName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestRawContentIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "normal", input: "Billboard", expected: "billboard_raw_content"},
		{name: "with spaces", input: "Campbell River Mirror", expected: "campbell_river_mirror_raw_content"},
		{name: "dots", input: "example.com", expected: "example_com_raw_content"},
		{name: "empty", input: "", expected: "_raw_content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := RawContentIndex(tt.input)
			if got != tt.expected {
				t.Errorf("RawContentIndex(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestClassifiedContentIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "normal", input: "Billboard", expected: "billboard_classified_content"},
		{name: "with spaces", input: "Campbell River Mirror", expected: "campbell_river_mirror_classified_content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ClassifiedContentIndex(tt.input)
			if got != tt.expected {
				t.Errorf("ClassifiedContentIndex(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestClassifiedIndexFromRaw(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "valid raw index", input: "billboard_raw_content", expected: "billboard_classified_content"},
		{name: "valid with underscores", input: "apnews_com_raw_content", expected: "apnews_com_classified_content"},
		{name: "empty string", input: "", wantErr: true},
		{name: "missing suffix", input: "billboard", wantErr: true},
		{name: "wrong suffix", input: "billboard_classified_content", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ClassifiedIndexFromRaw(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIsRawContentIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
	}{
		{"billboard_raw_content", true},
		{"billboard_classified_content", false},
		{"billboard", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := IsRawContentIndex(tt.input); got != tt.expected {
				t.Errorf("IsRawContentIndex(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsClassifiedContentIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected bool
	}{
		{"billboard_classified_content", true},
		{"billboard_raw_content", false},
		{"billboard", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			if got := IsClassifiedContentIndex(tt.input); got != tt.expected {
				t.Errorf("IsClassifiedContentIndex(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBaseSourceFromIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{name: "raw content", input: "billboard_raw_content", expected: "billboard"},
		{name: "classified content", input: "billboard_classified_content", expected: "billboard"},
		{name: "multi-word source", input: "campbell_river_mirror_classified_content", expected: "campbell_river_mirror"},
		{name: "no suffix", input: "billboard", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := BaseSourceFromIndex(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}
