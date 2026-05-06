package rss

import "errors"

// ErrNotModified is returned when the upstream responds 304 Not Modified.
// Callers must treat this as a no-op cycle: do not parse, do not modify
// the catalogue, but DO update poll_checkpoint.last_polled_at.
var ErrNotModified = errors.New("rss: not modified")

// ErrTransient indicates a retry-worthy failure (network, 5xx).
var ErrTransient = errors.New("rss: transient error")

// ErrStructural indicates a non-retryable failure (4xx, malformed feed format).
var ErrStructural = errors.New("rss: structural error")
