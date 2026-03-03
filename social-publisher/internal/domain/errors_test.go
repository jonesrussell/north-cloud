package domain_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitError_IsRetryable(t *testing.T) {
	err := &domain.RateLimitError{
		RetryAfter: 30 * time.Second,
		Message:    "rate limited",
	}
	assert.True(t, err.IsRetryable())
	assert.Equal(t, 30*time.Second, err.RetryAfter)
	assert.Contains(t, err.Error(), "rate limited")
}

func TestTransientError_IsRetryable(t *testing.T) {
	err := &domain.TransientError{Message: "timeout"}
	assert.True(t, err.IsRetryable())
}

func TestPermanentError_IsNotRetryable(t *testing.T) {
	err := &domain.PermanentError{Message: "invalid content", Code: "400"}
	assert.False(t, err.IsRetryable())
	assert.Equal(t, "400", err.Code)
}

func TestAuthError_IsRetryable(t *testing.T) {
	err := &domain.AuthError{Message: "token expired"}
	assert.True(t, err.IsRetryable())
}

func TestValidationError_IsNotRetryable(t *testing.T) {
	err := &domain.ValidationError{
		Field:   "metadata.subreddit",
		Message: "required field missing",
	}
	assert.False(t, err.IsRetryable())
	assert.Contains(t, err.Error(), "subreddit")
}
