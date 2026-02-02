package models

import (
	"strconv"
	"strings"
	"time"
)

// DefaultRateLimit is the default rate limit when empty or invalid (matches DB default).
const DefaultRateLimit = "1s"

// NormalizeRateLimit converts rate_limit to a duration string with unit (e.g. "10" -> "10s").
// Accepts already-valid durations ("10s", "1m") or bare numbers as seconds.
// Returns DefaultRateLimit for empty or invalid input so stored values are always parseable by clients.
func NormalizeRateLimit(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return DefaultRateLimit
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return s
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n > 0 {
			return strconv.Itoa(n) + "s"
		}
		return DefaultRateLimit
	}
	return DefaultRateLimit
}
