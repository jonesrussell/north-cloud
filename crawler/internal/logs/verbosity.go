// Package logs provides job logging utilities for the crawler service.
package logs

import (
	"errors"
	"strings"
)

// Verbosity represents the log verbosity level for job logging.
type Verbosity string

// Verbosity level constants.
const (
	// VerbosityQuiet logs only errors and warnings.
	VerbosityQuiet Verbosity = "quiet"
	// VerbosityNormal logs info, warnings, and errors (default).
	VerbosityNormal Verbosity = "normal"
	// VerbosityDebug logs debug messages and above.
	VerbosityDebug Verbosity = "debug"
	// VerbosityTrace logs all messages including trace-level details.
	VerbosityTrace Verbosity = "trace"
)

// ErrInvalidVerbosity is returned when an invalid verbosity string is provided.
var ErrInvalidVerbosity = errors.New("invalid verbosity level")

// ParseVerbosity parses a string into a Verbosity level.
// It is case-insensitive and defaults to VerbosityNormal for empty strings.
func ParseVerbosity(s string) (Verbosity, error) {
	if s == "" {
		return VerbosityNormal, nil
	}

	switch strings.ToLower(s) {
	case "quiet":
		return VerbosityQuiet, nil
	case "normal":
		return VerbosityNormal, nil
	case "debug":
		return VerbosityDebug, nil
	case "trace":
		return VerbosityTrace, nil
	default:
		return "", ErrInvalidVerbosity
	}
}

// AllowsLevel returns true if the verbosity level allows the given log level.
// Log levels: info, warn, error are always allowed for quiet and above.
// Debug level requires VerbosityDebug or VerbosityTrace.
// Trace level requires VerbosityTrace.
func (v Verbosity) AllowsLevel(level string) bool {
	switch strings.ToLower(level) {
	case "info", "warn", "warning", "error":
		// Always allowed at any verbosity level
		return true
	case "debug":
		return v == VerbosityDebug || v == VerbosityTrace
	case "trace":
		return v == VerbosityTrace
	default:
		// Unknown levels default to allowed
		return true
	}
}

// String returns the string representation of the Verbosity level.
func (v Verbosity) String() string {
	return string(v)
}
