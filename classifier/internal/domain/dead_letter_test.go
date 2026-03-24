package domain_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeadLetterEntry_Valid(t *testing.T) {
	t.Helper()

	entry, err := domain.NewDeadLetterEntry(
		"content-123", "cbc", "cbc_raw_content",
		"timeout error", domain.ErrorCodeESTimeout,
	)
	require.NoError(t, err)
	assert.Equal(t, "content-123", entry.ContentID)
	assert.Equal(t, "cbc", entry.SourceName)
	assert.Equal(t, "cbc_raw_content", entry.IndexName)
	assert.Equal(t, "timeout error", entry.ErrorMessage)
	assert.Equal(t, domain.ErrorCodeESTimeout, entry.ErrorCode)
	assert.Equal(t, 0, entry.RetryCount)
	assert.Equal(t, 5, entry.MaxRetries)
	assert.False(t, entry.CreatedAt.IsZero())
	assert.False(t, entry.NextRetryAt.IsZero())
	assert.True(t, entry.NextRetryAt.After(entry.CreatedAt))
}

func TestNewDeadLetterEntry_EmptyContentID(t *testing.T) {
	t.Helper()

	_, err := domain.NewDeadLetterEntry("", "cbc", "cbc_raw_content", "err", domain.ErrorCodeUnknown)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidDeadLetterEntry)
}

func TestNewDeadLetterEntry_EmptySourceName(t *testing.T) {
	t.Helper()

	_, err := domain.NewDeadLetterEntry("id", "", "idx", "err", domain.ErrorCodeUnknown)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidDeadLetterEntry)
}

func TestNewDeadLetterEntry_EmptyIndexName(t *testing.T) {
	t.Helper()

	_, err := domain.NewDeadLetterEntry("id", "src", "", "err", domain.ErrorCodeUnknown)
	require.Error(t, err)
	assert.ErrorIs(t, err, domain.ErrInvalidDeadLetterEntry)
}

func TestMustNewDeadLetterEntry_Panics(t *testing.T) {
	t.Helper()

	assert.Panics(t, func() {
		domain.MustNewDeadLetterEntry("", "src", "idx", "err", domain.ErrorCodeUnknown)
	})
}

func TestMustNewDeadLetterEntry_Success(t *testing.T) {
	t.Helper()

	entry := domain.MustNewDeadLetterEntry("id", "src", "idx", "err", domain.ErrorCodeESTimeout)
	assert.Equal(t, "id", entry.ContentID)
}

func TestDeadLetterEntry_ShouldRetry(t *testing.T) {
	t.Helper()

	entry := domain.MustNewDeadLetterEntry("id", "src", "idx", "err", domain.ErrorCodeUnknown)
	assert.True(t, entry.ShouldRetry())
	assert.False(t, entry.IsExhausted())

	// Exhaust retries
	for range entry.MaxRetries {
		entry.IncrementRetry("new error")
	}

	assert.False(t, entry.ShouldRetry())
	assert.True(t, entry.IsExhausted())
}

func TestDeadLetterEntry_NextRetryDelay(t *testing.T) {
	t.Helper()

	entry := domain.MustNewDeadLetterEntry("id", "src", "idx", "err", domain.ErrorCodeUnknown)

	// RetryCount=0 -> 60s * 2^0 = 60s
	assert.Equal(t, 60*time.Second, entry.NextRetryDelay())

	entry.IncrementRetry("err")
	// RetryCount=1 -> 60s * 2^1 = 120s
	assert.Equal(t, 120*time.Second, entry.NextRetryDelay())

	entry.IncrementRetry("err")
	// RetryCount=2 -> 60s * 2^2 = 240s
	assert.Equal(t, 240*time.Second, entry.NextRetryDelay())
}

func TestDeadLetterEntry_NextRetryDelay_CappedAtOneHour(t *testing.T) {
	t.Helper()

	entry := domain.MustNewDeadLetterEntry("id", "src", "idx", "err", domain.ErrorCodeUnknown)
	// Set retry count high enough that 60 * 2^n > 3600
	// 60 * 2^6 = 3840 > 3600, so retryCount=6 should be capped
	for range 6 {
		entry.IncrementRetry("err")
	}
	assert.Equal(t, 3600*time.Second, entry.NextRetryDelay())
}

func TestDeadLetterEntry_IncrementRetry(t *testing.T) {
	t.Helper()

	entry := domain.MustNewDeadLetterEntry("id", "src", "idx", "original error", domain.ErrorCodeUnknown)
	beforeRetry := time.Now()

	entry.IncrementRetry("new error message")

	assert.Equal(t, 1, entry.RetryCount)
	assert.Equal(t, "new error message", entry.ErrorMessage)
	assert.True(t, entry.LastAttemptAt.After(beforeRetry) || entry.LastAttemptAt.Equal(beforeRetry))
	assert.True(t, entry.NextRetryAt.After(beforeRetry))
}

func TestDeadLetterEntry_String(t *testing.T) {
	t.Helper()

	entry := domain.MustNewDeadLetterEntry("content-1", "cbc", "cbc_raw_content", "timeout", domain.ErrorCodeESTimeout)
	entry.ID = "dlq-123"

	s := entry.String()
	assert.Contains(t, s, "DLQ[dlq-123]")
	assert.Contains(t, s, "content=content-1")
	assert.Contains(t, s, "source=cbc")
	assert.Contains(t, s, "retries=0/5")
	assert.Contains(t, s, "ES_TIMEOUT")
}

func TestClassifyError(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		err      error
		expected domain.ErrorCode
	}{
		{"nil error", nil, domain.ErrorCodeUnknown},
		{"timeout", errors.New("context deadline exceeded: timeout"), domain.ErrorCodeESTimeout},
		{"connection refused", errors.New("dial tcp: connection refused"), domain.ErrorCodeESUnavailable},
		{"no such host", errors.New("no such host"), domain.ErrorCodeESUnavailable},
		{"connection reset", errors.New("connection reset by peer"), domain.ErrorCodeESUnavailable},
		{"index not found", errors.New("index_not_found_exception"), domain.ErrorCodeESIndexNotFound},
		{"panic", errors.New("recovered from panic in rule"), domain.ErrorCodeRulePanic},
		{"quality", errors.New("quality score computation failed"), domain.ErrorCodeQualityError},
		{"content_type", errors.New("unknown content_type"), domain.ErrorCodeContentType},
		{"content type space", errors.New("invalid content type"), domain.ErrorCodeContentType},
		{"indexing", errors.New("indexing failed"), domain.ErrorCodeIndexingFailed},
		{"bulk", errors.New("bulk request error"), domain.ErrorCodeIndexingFailed},
		{"unknown", errors.New("something else entirely"), domain.ErrorCodeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ClassifyError(tt.err)
			assert.Equal(t, tt.expected, got)
		})
	}
}
