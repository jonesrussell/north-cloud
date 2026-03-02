package domain

import (
	"fmt"
	"time"
)

// PublishError is the interface all publishing errors must satisfy.
type PublishError interface {
	error
	IsRetryable() bool
}

// RateLimitError indicates the platform has rate-limited the request.
type RateLimitError struct {
	RetryAfter time.Duration
	Message    string
}

func (e *RateLimitError) Error() string     { return fmt.Sprintf("rate limited: %s", e.Message) }
func (e *RateLimitError) IsRetryable() bool { return true }

// TransientError indicates a temporary failure that may succeed on retry.
type TransientError struct {
	Message string
}

func (e *TransientError) Error() string     { return fmt.Sprintf("transient error: %s", e.Message) }
func (e *TransientError) IsRetryable() bool { return true }

// PermanentError indicates a non-recoverable failure.
type PermanentError struct {
	Message  string
	Code     string
	Response string
}

func (e *PermanentError) Error() string {
	return fmt.Sprintf("permanent error [%s]: %s", e.Code, e.Message)
}
func (e *PermanentError) IsRetryable() bool { return false }

// AuthError indicates an authentication or authorization failure.
type AuthError struct {
	Message string
}

func (e *AuthError) Error() string     { return fmt.Sprintf("auth error: %s", e.Message) }
func (e *AuthError) IsRetryable() bool { return true }

// ValidationError indicates invalid content that cannot be published.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}
func (e *ValidationError) IsRetryable() bool { return false }
