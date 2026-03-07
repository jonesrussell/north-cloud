//nolint:testpackage // testing unexported sanitizeErrorMessage
package mcp

import (
	"strings"
	"testing"
)

func TestSanitizeErrorMessage_StripsURLs(t *testing.T) {
	msg := "Failed to list sources: failed to execute request: Get http://localhost:8050/api/v1/sources: connection refused"
	result := sanitizeErrorMessage(msg)

	if strings.Contains(result, "localhost") {
		t.Errorf("should strip URLs, got: %s", result)
	}
	if strings.Contains(result, "8050") {
		t.Errorf("should strip port numbers in URLs, got: %s", result)
	}
	if !strings.Contains(result, "Failed to list sources") {
		t.Errorf("should preserve error prefix, got: %s", result)
	}
}

func TestSanitizeErrorMessage_StripsResponseBody(t *testing.T) {
	msg := `unexpected status code: 500, body: {"error":"internal server error","stack":"goroutine 1 [running]..."}`
	result := sanitizeErrorMessage(msg)

	if strings.Contains(result, "goroutine") {
		t.Errorf("should strip response body, got: %s", result)
	}
	if strings.Contains(result, "stack") {
		t.Errorf("should strip stack traces, got: %s", result)
	}
}

func TestSanitizeErrorMessage_ReplacesStatusCode(t *testing.T) {
	msg := "unexpected status code: 502, body: Bad Gateway"
	result := sanitizeErrorMessage(msg)

	if strings.Contains(result, "502") {
		t.Errorf("should replace status code details, got: %s", result)
	}
	if !strings.Contains(result, "service returned an error") {
		t.Errorf("should contain generic message, got: %s", result)
	}
}

func TestSanitizeErrorMessage_TruncatesLongMessages(t *testing.T) {
	msg := strings.Repeat("a", maxErrorMessageLength+50)
	result := sanitizeErrorMessage(msg)

	if len(result) > maxErrorMessageLength {
		t.Errorf("result length %d exceeds max %d", len(result), maxErrorMessageLength)
	}
	if !strings.HasSuffix(result, "...") {
		t.Errorf("truncated message should end with '...', got: %s", result)
	}
}

func TestSanitizeErrorMessage_PreservesShortMessages(t *testing.T) {
	msg := "Failed to create source: duplicate name"
	result := sanitizeErrorMessage(msg)

	if result != msg {
		t.Errorf("should preserve short messages without URLs, got: %s", result)
	}
}
