// Package domain contains the core domain models for the classifier service.
package domain

import (
	"errors"
	"fmt"
	"time"
)

// ErrNotFound is returned when an entity is not found in the database.
var ErrNotFound = errors.New("entity not found")

// ErrInvalidDeadLetterEntry is returned when creating a DLQ entry with invalid fields.
var ErrInvalidDeadLetterEntry = errors.New("invalid dead letter entry")

// ErrorCode categorizes DLQ errors for filtering and alerting
type ErrorCode string

const (
	ErrorCodeESTimeout       ErrorCode = "ES_TIMEOUT"
	ErrorCodeESUnavailable   ErrorCode = "ES_UNAVAILABLE"
	ErrorCodeESIndexNotFound ErrorCode = "ES_INDEX_NOT_FOUND"
	ErrorCodeRulePanic       ErrorCode = "RULE_PANIC"
	ErrorCodeQualityError    ErrorCode = "QUALITY_ERROR"
	ErrorCodeContentType     ErrorCode = "CONTENT_TYPE_ERROR"
	ErrorCodeIndexingFailed  ErrorCode = "INDEXING_FAILED"
	ErrorCodeUnknown         ErrorCode = "UNKNOWN"
)

const (
	defaultMaxRetries     = 5
	baseRetryDelaySeconds = 60
	maxRetryDelaySeconds  = 3600 // Cap at 1 hour
)

// DeadLetterEntry represents a failed classification awaiting retry
type DeadLetterEntry struct {
	ID            string    `db:"id"`
	ContentID     string    `db:"content_id"`
	SourceName    string    `db:"source_name"`
	IndexName     string    `db:"index_name"`
	ErrorMessage  string    `db:"error_message"`
	ErrorCode     ErrorCode `db:"error_code"`
	RetryCount    int       `db:"retry_count"`
	MaxRetries    int       `db:"max_retries"`
	NextRetryAt   time.Time `db:"next_retry_at"`
	CreatedAt     time.Time `db:"created_at"`
	LastAttemptAt time.Time `db:"last_attempt_at"`
}

// NewDeadLetterEntry creates a new DLQ entry with exponential backoff.
// Returns an error if required fields (contentID, sourceName, indexName) are empty.
func NewDeadLetterEntry(contentID, sourceName, indexName, errorMsg string, errorCode ErrorCode) (*DeadLetterEntry, error) {
	if contentID == "" {
		return nil, fmt.Errorf("%w: content_id is required", ErrInvalidDeadLetterEntry)
	}
	if sourceName == "" {
		return nil, fmt.Errorf("%w: source_name is required", ErrInvalidDeadLetterEntry)
	}
	if indexName == "" {
		return nil, fmt.Errorf("%w: index_name is required", ErrInvalidDeadLetterEntry)
	}

	now := time.Now()
	return &DeadLetterEntry{
		ContentID:     contentID,
		SourceName:    sourceName,
		IndexName:     indexName,
		ErrorMessage:  errorMsg,
		ErrorCode:     errorCode,
		RetryCount:    0,
		MaxRetries:    defaultMaxRetries,
		NextRetryAt:   now.Add(time.Duration(baseRetryDelaySeconds) * time.Second),
		CreatedAt:     now,
		LastAttemptAt: now,
	}, nil
}

// MustNewDeadLetterEntry is like NewDeadLetterEntry but panics on validation error.
// Use only when you're certain the inputs are valid (e.g., in tests).
func MustNewDeadLetterEntry(contentID, sourceName, indexName, errorMsg string, errorCode ErrorCode) *DeadLetterEntry {
	entry, err := NewDeadLetterEntry(contentID, sourceName, indexName, errorMsg, errorCode)
	if err != nil {
		panic(err)
	}
	return entry
}

// NextRetryDelay calculates exponential backoff with cap
// Delays: 1min, 2min, 4min, 8min, 16min (capped at 1hr)
func (d *DeadLetterEntry) NextRetryDelay() time.Duration {
	multiplier := 1 << d.RetryCount // 2^retryCount
	delaySeconds := min(baseRetryDelaySeconds*multiplier, maxRetryDelaySeconds)
	return time.Duration(delaySeconds) * time.Second
}

// ShouldRetry returns true if retries remain
func (d *DeadLetterEntry) ShouldRetry() bool {
	return d.RetryCount < d.MaxRetries
}

// IsExhausted returns true if all retries have been used
func (d *DeadLetterEntry) IsExhausted() bool {
	return d.RetryCount >= d.MaxRetries
}

// IncrementRetry updates retry state for next attempt
func (d *DeadLetterEntry) IncrementRetry(newError string) {
	d.RetryCount++
	d.LastAttemptAt = time.Now()
	d.ErrorMessage = newError
	d.NextRetryAt = time.Now().Add(d.NextRetryDelay())
}

// String returns a debug representation
func (d *DeadLetterEntry) String() string {
	return fmt.Sprintf("DLQ[%s] content=%s source=%s retries=%d/%d next=%s error=%s",
		d.ID, d.ContentID, d.SourceName, d.RetryCount, d.MaxRetries,
		d.NextRetryAt.Format(time.RFC3339), d.ErrorCode)
}

// DLQStats holds dead-letter queue statistics
type DLQStats struct {
	Pending     int64      `json:"pending"`
	Exhausted   int64      `json:"exhausted"`
	Ready       int64      `json:"ready"`
	AvgRetries  float64    `json:"avg_retries"`
	OldestEntry *time.Time `json:"oldest_entry,omitempty"`
}

// DLQSourceCount holds counts grouped by source
type DLQSourceCount struct {
	SourceName string `json:"source_name"`
	Count      int64  `json:"count"`
}

// DLQErrorCount holds counts grouped by error code
type DLQErrorCount struct {
	ErrorCode ErrorCode `json:"error_code"`
	Count     int64     `json:"count"`
}

// ClassifyError maps an error to an ErrorCode
func ClassifyError(err error) ErrorCode {
	if err == nil {
		return ErrorCodeUnknown
	}

	errStr := err.Error()

	// Check for common error patterns
	switch {
	case contains(errStr, "timeout"):
		return ErrorCodeESTimeout
	case contains(errStr, "connection refused", "no such host", "connection reset"):
		return ErrorCodeESUnavailable
	case contains(errStr, "index_not_found"):
		return ErrorCodeESIndexNotFound
	case contains(errStr, "panic"):
		return ErrorCodeRulePanic
	case contains(errStr, "quality"):
		return ErrorCodeQualityError
	case contains(errStr, "content_type", "content type"):
		return ErrorCodeContentType
	case contains(errStr, "indexing", "bulk"):
		return ErrorCodeIndexingFailed
	default:
		return ErrorCodeUnknown
	}
}

// contains checks if s contains any of the substrings
func contains(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if sub != "" && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
