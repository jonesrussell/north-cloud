package mlclient

import "errors"

// Sentinel errors returned by the Client.
var (
	// ErrUnavailable is returned when the circuit breaker is open.
	ErrUnavailable = errors.New("ml module unavailable")

	// ErrUnhealthy is returned when the health endpoint reports an unhealthy state.
	ErrUnhealthy = errors.New("ml module unhealthy")

	// ErrTimeout is returned when a request exceeds the configured timeout.
	ErrTimeout = errors.New("ml module request timed out")

	// ErrSchemaVersion is returned when the response schema version does not match expectations.
	ErrSchemaVersion = errors.New("ml module schema version mismatch")
)
