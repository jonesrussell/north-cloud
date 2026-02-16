//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"testing"
)

func TestTruncateWords(t *testing.T) {
	t.Helper()

	tests := []struct {
		name  string
		input string
		limit int
		want  string
	}{
		{
			name:  "short title under limit",
			input: "Police arrest suspect",
			limit: titleExcerptWordLimit,
			want:  "Police arrest suspect",
		},
		{
			name:  "exact limit",
			input: "one two three four five six seven eight nine ten",
			limit: titleExcerptWordLimit,
			want:  "one two three four five six seven eight nine ten",
		},
		{
			name:  "over limit truncated with ellipsis",
			input: "one two three four five six seven eight nine ten eleven twelve",
			limit: titleExcerptWordLimit,
			want:  "one two three four five six seven eight nine ten...",
		},
		{
			name:  "empty string",
			input: "",
			limit: titleExcerptWordLimit,
			want:  "",
		},
		{
			name:  "single word",
			input: "headline",
			limit: titleExcerptWordLimit,
			want:  "headline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			got := truncateWords(tt.input, tt.limit)
			if got != tt.want {
				t.Errorf("truncateWords(%q, %d) = %q, want %q", tt.input, tt.limit, got, tt.want)
			}
		})
	}
}

func TestClassifyErrorType(t *testing.T) {
	t.Helper()

	tests := []struct {
		name   string
		errMsg string
		want   string
	}{
		{
			name:   "deadline exceeded is timeout",
			errMsg: "context deadline exceeded",
			want:   "timeout",
		},
		{
			name:   "timeout keyword",
			errMsg: "request timeout after 5s",
			want:   "timeout",
		},
		{
			name:   "5xx server error",
			errMsg: "ml service returned 503",
			want:   "5xx",
		},
		{
			name:   "4xx client error",
			errMsg: "ml service returned 400",
			want:   "4xx",
		},
		{
			name:   "connection refused",
			errMsg: "connection refused",
			want:   "connection",
		},
		{
			name:   "dial tcp error",
			errMsg: "dial tcp 127.0.0.1:8076: connect: connection refused",
			want:   "connection",
		},
		{
			name:   "no such host",
			errMsg: "dial tcp: lookup crime-ml: no such host",
			want:   "connection",
		},
		{
			name:   "decode response EOF",
			errMsg: "decode response: unexpected EOF",
			want:   "decode",
		},
		{
			name:   "unmarshal error",
			errMsg: "json: cannot unmarshal string into Go value",
			want:   "decode",
		},
		{
			name:   "unknown error",
			errMsg: "something unknown happened",
			want:   "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()

			got := classifyErrorType(tt.errMsg)
			if got != tt.want {
				t.Errorf("classifyErrorType(%q) = %q, want %q", tt.errMsg, got, tt.want)
			}
		})
	}
}
