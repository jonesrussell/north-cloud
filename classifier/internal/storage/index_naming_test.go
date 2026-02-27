//nolint:testpackage // Testing unexported checkBulkResponse requires same package access
package storage

import (
	"strings"
	"testing"
)

func TestGetClassifiedIndexName(t *testing.T) {
	t.Helper()

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
			got, err := GetClassifiedIndexName(tt.input)
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

func TestSanitizeSourceName(t *testing.T) {
	t.Helper()

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeSourceName(tt.input)
			if got != tt.expected {
				t.Errorf("SanitizeSourceName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestClassifiedIndexForContent(t *testing.T) {
	t.Helper()

	tests := []struct {
		name        string
		sourceIndex string
		sourceName  string
		expected    string
		wantErr     bool
	}{
		{
			name:        "prefers source index",
			sourceIndex: "billboard_raw_content",
			sourceName:  "Billboard",
			expected:    "billboard_classified_content",
		},
		{
			name:        "falls back to sanitized source name",
			sourceIndex: "",
			sourceName:  "Billboard",
			expected:    "billboard_classified_content",
		},
		{
			name:        "fallback with spaces",
			sourceIndex: "",
			sourceName:  "Campbell River Mirror",
			expected:    "campbell_river_mirror_classified_content",
		},
		{
			name:        "both empty",
			sourceIndex: "",
			sourceName:  "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ClassifiedIndexForContent(tt.sourceIndex, tt.sourceName)
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

func TestCheckBulkResponse(t *testing.T) {
	t.Helper()

	tests := []struct {
		name       string
		body       string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:    "no errors",
			body:    `{"errors":false,"items":[{"index":{"_index":"test","_id":"1","status":201}}]}`,
			wantErr: false,
		},
		{
			name: "with item error",
			body: `{"errors":true,"items":[{"index":{` +
				`"_index":"Test_classified","_id":"1","status":400,` +
				`"error":{"type":"invalid_index_name_exception",` +
				`"reason":"must be lowercase"}}}]}`,
			wantErr:    true,
			wantErrMsg: "1 of 1 bulk items failed",
		},
		{
			name: "mixed success and failure",
			body: `{"errors":true,"items":[` +
				`{"index":{"_index":"good","_id":"1","status":201}},` +
				`{"index":{"_index":"Bad","_id":"2","status":400,` +
				`"error":{"type":"invalid_index_name_exception",` +
				`"reason":"must be lowercase"}}}]}`,
			wantErr:    true,
			wantErrMsg: "1 of 2 bulk items failed",
		},
		{
			name:       "invalid json",
			body:       `not json`,
			wantErr:    true,
			wantErrMsg: "failed to parse bulk response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkBulkResponse([]byte(tt.body))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrMsg)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
