// Package database provides database access for the classifier service.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// DeadLetterRepository manages the dead-letter queue in PostgreSQL
type DeadLetterRepository struct {
	db *sql.DB
}

// NewDeadLetterRepository creates a new repository
func NewDeadLetterRepository(db *sql.DB) *DeadLetterRepository {
	return &DeadLetterRepository{db: db}
}

// Enqueue adds a failed document to the DLQ (idempotent via ON CONFLICT)
func (r *DeadLetterRepository) Enqueue(ctx context.Context, entry *domain.DeadLetterEntry) error {
	query := `
		INSERT INTO dead_letter_queue
			(content_id, source_name, index_name, error_message, error_code, next_retry_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (content_id) DO UPDATE SET
			retry_count = dead_letter_queue.retry_count + 1,
			error_message = EXCLUDED.error_message,
			error_code = EXCLUDED.error_code,
			last_attempt_at = NOW(),
			next_retry_at = NOW() + (INTERVAL '1 second' * POWER(2, dead_letter_queue.retry_count + 1) * 60)
		WHERE dead_letter_queue.retry_count < dead_letter_queue.max_retries`

	_, err := r.db.ExecContext(ctx, query,
		entry.ContentID,
		entry.SourceName,
		entry.IndexName,
		entry.ErrorMessage,
		entry.ErrorCode,
		entry.NextRetryAt,
	)
	if err != nil {
		return fmt.Errorf("enqueue DLQ: %w", err)
	}
	return nil
}

// FetchRetryable returns documents ready for retry with row-level locking.
// Uses FOR UPDATE SKIP LOCKED to allow concurrent workers.
func (r *DeadLetterRepository) FetchRetryable(ctx context.Context, limit int) ([]domain.DeadLetterEntry, error) {
	query := `
		SELECT id, content_id, source_name, index_name, error_message, error_code,
		       retry_count, max_retries, next_retry_at, created_at, last_attempt_at
		FROM dead_letter_queue
		WHERE next_retry_at <= NOW()
		  AND retry_count < max_retries
		ORDER BY next_retry_at ASC
		LIMIT $1
		FOR UPDATE SKIP LOCKED`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch retryable: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.DeadLetterEntry, 0, limit)
	for rows.Next() {
		var e domain.DeadLetterEntry
		scanErr := rows.Scan(
			&e.ID, &e.ContentID, &e.SourceName, &e.IndexName, &e.ErrorMessage, &e.ErrorCode,
			&e.RetryCount, &e.MaxRetries, &e.NextRetryAt, &e.CreatedAt, &e.LastAttemptAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("scan DLQ entry: %w", scanErr)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Remove deletes a successfully processed entry
func (r *DeadLetterRepository) Remove(ctx context.Context, contentID string) error {
	result, err := r.db.ExecContext(ctx,
		`DELETE FROM dead_letter_queue WHERE content_id = $1`,
		contentID)
	if err != nil {
		return fmt.Errorf("remove from DLQ: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("DLQ entry not found: %s", contentID)
	}
	return nil
}

// MarkExhausted flags entries that exceeded max retries (for alerting)
func (r *DeadLetterRepository) MarkExhausted(ctx context.Context, contentID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE dead_letter_queue
		SET retry_count = max_retries,
		    last_attempt_at = NOW()
		WHERE content_id = $1`,
		contentID)
	if err != nil {
		return fmt.Errorf("mark exhausted: %w", err)
	}
	return nil
}

// UpdateRetryCount increments the retry count and schedules next retry
func (r *DeadLetterRepository) UpdateRetryCount(ctx context.Context, contentID, errorMsg string) error {
	query := `
		UPDATE dead_letter_queue
		SET retry_count = retry_count + 1,
		    error_message = $2,
		    last_attempt_at = NOW(),
		    next_retry_at = NOW() + (INTERVAL '1 second' * POWER(2, retry_count + 1) * 60)
		WHERE content_id = $1
		  AND retry_count < max_retries`

	_, err := r.db.ExecContext(ctx, query, contentID, errorMsg)
	if err != nil {
		return fmt.Errorf("update retry count: %w", err)
	}
	return nil
}

// GetStats returns DLQ statistics for monitoring
func (r *DeadLetterRepository) GetStats(ctx context.Context) (*domain.DLQStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE retry_count < max_retries) as pending,
			COUNT(*) FILTER (WHERE retry_count >= max_retries) as exhausted,
			COUNT(*) FILTER (WHERE next_retry_at <= NOW() AND retry_count < max_retries) as ready,
			COALESCE(AVG(retry_count), 0) as avg_retries,
			MIN(created_at) as oldest_entry
		FROM dead_letter_queue`

	var stats domain.DLQStats
	var oldestEntry sql.NullTime
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.Pending,
		&stats.Exhausted,
		&stats.Ready,
		&stats.AvgRetries,
		&oldestEntry,
	)
	if err != nil {
		return nil, fmt.Errorf("get DLQ stats: %w", err)
	}
	if oldestEntry.Valid {
		stats.OldestEntry = &oldestEntry.Time
	}
	return &stats, nil
}

// CountBySource returns DLQ counts grouped by source
func (r *DeadLetterRepository) CountBySource(ctx context.Context) ([]domain.DLQSourceCount, error) {
	query := `
		SELECT source_name, COUNT(*)
		FROM dead_letter_queue
		WHERE retry_count < max_retries
		GROUP BY source_name
		ORDER BY COUNT(*) DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by source: %w", err)
	}
	defer rows.Close()

	var result []domain.DLQSourceCount
	for rows.Next() {
		var sc domain.DLQSourceCount
		scanErr := rows.Scan(&sc.SourceName, &sc.Count)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, sc)
	}
	return result, rows.Err()
}

// CountByErrorCode returns DLQ counts grouped by error code
func (r *DeadLetterRepository) CountByErrorCode(ctx context.Context) ([]domain.DLQErrorCount, error) {
	query := `
		SELECT COALESCE(error_code, 'UNKNOWN'), COUNT(*)
		FROM dead_letter_queue
		WHERE retry_count < max_retries
		GROUP BY error_code
		ORDER BY COUNT(*) DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("count by error code: %w", err)
	}
	defer rows.Close()

	var result []domain.DLQErrorCount
	for rows.Next() {
		var ec domain.DLQErrorCount
		scanErr := rows.Scan(&ec.ErrorCode, &ec.Count)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, ec)
	}
	return result, rows.Err()
}

// GetByContentID retrieves a single DLQ entry by content ID
func (r *DeadLetterRepository) GetByContentID(ctx context.Context, contentID string) (*domain.DeadLetterEntry, error) {
	query := `
		SELECT id, content_id, source_name, index_name, error_message, error_code,
		       retry_count, max_retries, next_retry_at, created_at, last_attempt_at
		FROM dead_letter_queue
		WHERE content_id = $1`

	var e domain.DeadLetterEntry
	err := r.db.QueryRowContext(ctx, query, contentID).Scan(
		&e.ID, &e.ContentID, &e.SourceName, &e.IndexName, &e.ErrorMessage, &e.ErrorCode,
		&e.RetryCount, &e.MaxRetries, &e.NextRetryAt, &e.CreatedAt, &e.LastAttemptAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get by content ID: %w", err)
	}
	return &e, nil
}

// CleanupExhausted removes entries that have exhausted all retries and are older than retention period
func (r *DeadLetterRepository) CleanupExhausted(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM dead_letter_queue
		WHERE retry_count >= max_retries
		  AND last_attempt_at < NOW() - $1::interval`

	result, err := r.db.ExecContext(ctx, query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("cleanup exhausted: %w", err)
	}
	return result.RowsAffected()
}

// Count returns the total number of entries in the DLQ
func (r *DeadLetterRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dead_letter_queue`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count DLQ: %w", err)
	}
	return count, nil
}
