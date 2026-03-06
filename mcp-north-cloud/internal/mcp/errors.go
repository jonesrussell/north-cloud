package mcp

import (
	"regexp"
	"strings"
)

// maxErrorMessageLength caps sanitized error messages to prevent leaking large response bodies.
const maxErrorMessageLength = 200

// urlPattern matches http:// and https:// URLs.
var urlPattern = regexp.MustCompile(`https?://[^\s"']+`)

// sanitizeErrorMessage removes internal implementation details from error messages
// sent to the LLM client. It strips URLs, raw response bodies, and status code details,
// while preserving the high-level error context (e.g. "Failed to list sources").
func sanitizeErrorMessage(msg string) string {
	// Strip URLs to avoid exposing internal service addresses
	msg = urlPattern.ReplaceAllString(msg, "<service>")

	// Handle "unexpected status code" pattern (most specific, check first).
	// This also covers the "body:" suffix since both often appear together.
	if strings.Contains(msg, "unexpected status code") {
		parts := strings.SplitN(msg, "unexpected status code", 2)
		msg = strings.TrimSpace(parts[0]) + " service returned an error"
	} else if idx := strings.Index(msg, "body:"); idx >= 0 {
		// Strip raw response bodies after "body:" markers
		msg = strings.TrimSpace(msg[:idx]) + " (service returned an error)"
	}

	// Truncate to max length
	if len(msg) > maxErrorMessageLength {
		msg = msg[:maxErrorMessageLength-3] + "..."
	}

	return msg
}
