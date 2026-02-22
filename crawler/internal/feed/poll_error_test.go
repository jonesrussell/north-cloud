package feed_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/feed"
)

func TestClassifyHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantType   feed.ErrorType
		wantLevel  feed.LogLevel
	}{
		{"429 rate limited", http.StatusTooManyRequests, feed.ErrTypeRateLimited, feed.LevelWarn},
		{"403 forbidden", http.StatusForbidden, feed.ErrTypeForbidden, feed.LevelWarn},
		{"404 not found", http.StatusNotFound, feed.ErrTypeNotFound, feed.LevelWarn},
		{"410 gone", http.StatusGone, feed.ErrTypeGone, feed.LevelWarn},
		{"500 server error", http.StatusInternalServerError, feed.ErrTypeUpstream, feed.LevelWarn},
		{"502 bad gateway", http.StatusBadGateway, feed.ErrTypeUpstream, feed.LevelWarn},
		{"503 service unavailable", http.StatusServiceUnavailable, feed.ErrTypeUpstream, feed.LevelWarn},
		{"599 max server error", 599, feed.ErrTypeUpstream, feed.LevelWarn},
		{"301 redirect", http.StatusMovedPermanently, feed.ErrTypeUnexpected, feed.LevelError},
		{"418 teapot", http.StatusTeapot, feed.ErrTypeUnexpected, feed.LevelError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pollErr := feed.ClassifyHTTPStatus(tt.statusCode, "https://example.com/feed")

			if pollErr.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", pollErr.Type, tt.wantType)
			}
			if pollErr.Level != tt.wantLevel {
				t.Errorf("Level = %d, want %d", pollErr.Level, tt.wantLevel)
			}
			if pollErr.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", pollErr.StatusCode, tt.statusCode)
			}
		})
	}
}

func TestPollError_Error(t *testing.T) {
	t.Parallel()

	pollErr := &feed.PollError{
		Type:       feed.ErrTypeNotFound,
		Level:      feed.LevelWarn,
		StatusCode: http.StatusNotFound,
		URL:        "https://example.com/feed",
		Cause:      errors.New("HTTP 404"),
	}

	msg := pollErr.Error()
	if msg == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestPollError_Unwrap(t *testing.T) {
	t.Parallel()

	cause := errors.New("connection refused")
	pollErr := &feed.PollError{
		Type:  feed.ErrTypeNetwork,
		Level: feed.LevelWarn,
		Cause: cause,
	}

	if !errors.Is(pollErr, cause) {
		t.Error("expected Unwrap to return cause")
	}
}

func TestPollError_ErrorsAs(t *testing.T) {
	t.Parallel()

	pollErr := feed.ClassifyHTTPStatus(http.StatusNotFound, "https://example.com/feed")
	wrapped := fmt.Errorf("poll feed: %w", pollErr)

	var target *feed.PollError
	if !errors.As(wrapped, &target) {
		t.Fatal("expected errors.As to match PollError")
	}
	if target.Type != feed.ErrTypeNotFound {
		t.Errorf("Type = %q, want %q", target.Type, feed.ErrTypeNotFound)
	}
}

func TestClassifyNetworkError(t *testing.T) {
	t.Parallel()

	cause := errors.New("dial tcp: lookup example.com: no such host")
	pollErr := feed.ClassifyNetworkError(cause, "https://example.com/feed")

	if pollErr.Type != feed.ErrTypeNetwork {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeNetwork)
	}
	if pollErr.Level != feed.LevelWarn {
		t.Errorf("Level = %d, want %d", pollErr.Level, feed.LevelWarn)
	}
	if pollErr.StatusCode != 0 {
		t.Errorf("StatusCode = %d, want 0", pollErr.StatusCode)
	}
}

func TestClassifyParseError(t *testing.T) {
	t.Parallel()

	cause := errors.New("invalid XML")
	pollErr := feed.ClassifyParseError(cause, "https://example.com/feed")

	if pollErr.Type != feed.ErrTypeParse {
		t.Errorf("Type = %q, want %q", pollErr.Type, feed.ErrTypeParse)
	}
	if pollErr.Level != feed.LevelWarn {
		t.Errorf("Level = %d, want %d", pollErr.Level, feed.LevelWarn)
	}
}

func TestDisableThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		errType   feed.ErrorType
		threshold int
		exists    bool
	}{
		{feed.ErrTypeNotFound, 3, true},
		{feed.ErrTypeGone, 1, true},
		{feed.ErrTypeForbidden, 5, true},
		{feed.ErrTypeUpstream, 10, true},
		{feed.ErrTypeNetwork, 10, true},
		{feed.ErrTypeParse, 5, true},
		{feed.ErrTypeRateLimited, 0, false},
		{feed.ErrTypeUnexpected, 0, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.errType), func(t *testing.T) {
			t.Parallel()

			threshold, ok := feed.DisableThreshold(tt.errType)
			if ok != tt.exists {
				t.Errorf("DisableThreshold(%q) exists = %v, want %v", tt.errType, ok, tt.exists)
			}
			if ok && threshold != tt.threshold {
				t.Errorf("DisableThreshold(%q) = %d, want %d", tt.errType, threshold, tt.threshold)
			}
		})
	}
}
