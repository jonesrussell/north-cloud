//nolint:testpackage // Testing unexported checkBulkResponse requires same package access
package storage

import (
	"strings"
	"testing"
)

func TestClassifiedIndexForContent(t *testing.T) {
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
			name:        "invalid source index",
			sourceIndex: "billboard_content",
			sourceName:  "Billboard",
			wantErr:     true,
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
