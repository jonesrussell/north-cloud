// Package database provides database access for the publisher service.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"

	"github.com/jonesrussell/north-cloud/publisher/internal/domain"
)

// outboxSelectList is the column list for SELECT/RETURNING on classified_outbox (single source for schema changes)
const outboxSelectList = `id, content_id, source_name, index_name, content_type, topics,
			quality_score, is_crime_related, crime_subcategory,
			title, body, url, published_date, status, retry_count,
			max_retries, error_message, created_at, updated_at,
			published_at, next_retry_at`

// OutboxRepository manages the transactional outbox in PostgreSQL
type OutboxRepository struct {
	db *sql.DB
}

// NewOutboxRepository creates a new repository
func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// FetchPending returns pending entries ready for publishing.
// Uses FOR UPDATE SKIP LOCKED for concurrent worker safety.
// Prioritizes crime-related content.
func (r *OutboxRepository) FetchPending(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	query := `
		UPDATE classified_outbox
		SET status = 'publishing', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM classified_outbox
			WHERE status = 'pending'
			ORDER BY
				is_crime_related DESC,
				created_at ASC
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING ` + outboxSelectList + `
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch pending: %w", err)
	}
	defer rows.Close()

	return scanOutboxEntries(rows)
}

// FetchRetryable returns failed entries ready for retry
func (r *OutboxRepository) FetchRetryable(ctx context.Context, limit int) ([]domain.OutboxEntry, error) {
	query := `
		UPDATE classified_outbox
		SET status = 'publishing', updated_at = NOW()
		WHERE id IN (
			SELECT id FROM classified_outbox
			WHERE status = 'failed'
			  AND retry_count < max_retries
			  AND (next_retry_at IS NULL OR next_retry_at <= NOW())
			ORDER BY next_retry_at ASC NULLS FIRST
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING ` + outboxSelectList + `
	`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("fetch retryable: %w", err)
	}
	defer rows.Close()

	return scanOutboxEntries(rows)
}

// execExpectOneRow runs an exec and returns domain.ErrNotFound when no row was affected
func (r *OutboxRepository) execExpectOneRow(ctx context.Context, query string, args ...any) error {
	result, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("get affected rows: %w", rowsErr)
	}
	if rows == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// MarkPublished marks an entry as successfully published
func (r *OutboxRepository) MarkPublished(ctx context.Context, id string) error {
	query := `
		UPDATE classified_outbox
		SET status = 'published',
		    published_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1`
	if err := r.execExpectOneRow(ctx, query, id); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return fmt.Errorf("mark published: %w", err)
	}
	return nil
}

// MarkFailed marks an entry as failed with retry scheduling
func (r *OutboxRepository) MarkFailed(ctx context.Context, id, errorMsg string) error {
	// Exponential backoff: 1min, 2min, 4min, 8min, 16min
	query := `
		UPDATE classified_outbox
		SET status = 'failed',
		    error_message = $2,
		    retry_count = retry_count + 1,
		    next_retry_at = NOW() + (INTERVAL '1 minute' * POWER(2, retry_count)),
		    updated_at = NOW()
		WHERE id = $1`
	if err := r.execExpectOneRow(ctx, query, id, errorMsg); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return err
		}
		return fmt.Errorf("mark failed: %w", err)
	}
	return nil
}

// ResetToPending resets stale "publishing" entries back to "pending".
// This handles entries that were claimed but worker crashed before completing.
func (r *OutboxRepository) ResetToPending(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		UPDATE classified_outbox
		SET status = 'pending', updated_at = NOW()
		WHERE status = 'publishing'
		  AND updated_at < NOW() - $1::interval`

	result, err := r.db.ExecContext(ctx, query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("reset to pending: %w", err)
	}

	return result.RowsAffected()
}

// CleanupPublished removes old published entries
func (r *OutboxRepository) CleanupPublished(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `
		DELETE FROM classified_outbox
		WHERE status = 'published'
		  AND published_at < NOW() - $1::interval`

	result, err := r.db.ExecContext(ctx, query, olderThan.String())
	if err != nil {
		return 0, fmt.Errorf("cleanup published: %w", err)
	}

	return result.RowsAffected()
}

// GetStats returns outbox statistics
func (r *OutboxRepository) GetStats(ctx context.Context) (*domain.OutboxStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'publishing') as publishing,
			COUNT(*) FILTER (WHERE status = 'published') as published,
			COUNT(*) FILTER (WHERE status = 'failed' AND retry_count < max_retries) as failed_retryable,
			COUNT(*) FILTER (WHERE status = 'failed' AND retry_count >= max_retries) as failed_exhausted,
			COALESCE(AVG(EXTRACT(EPOCH FROM (published_at - created_at)))
				FILTER (WHERE status = 'published' AND published_at > NOW() - INTERVAL '1 hour'), 0) as avg_publish_lag_seconds
		FROM classified_outbox`

	var stats domain.OutboxStats
	err := r.db.QueryRowContext(ctx, query).Scan(
		&stats.Pending,
		&stats.Publishing,
		&stats.Published,
		&stats.FailedRetryable,
		&stats.FailedExhausted,
		&stats.AvgPublishLagSeconds,
	)
	if err != nil {
		return nil, fmt.Errorf("get outbox stats: %w", err)
	}
	return &stats, nil
}

// GetByID retrieves a single outbox entry by ID
func (r *OutboxRepository) GetByID(ctx context.Context, id string) (*domain.OutboxEntry, error) {
	query := `SELECT ` + outboxSelectList + `
		FROM classified_outbox
		WHERE id = $1`

	var e domain.OutboxEntry
	var topics pq.StringArray

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&e.ID, &e.ContentID, &e.SourceName, &e.IndexName, &e.ContentType, &topics,
		&e.QualityScore, &e.IsCrimeRelated, &e.CrimeSubcategory,
		&e.Title, &e.Body, &e.URL, &e.PublishedDate, &e.Status, &e.RetryCount,
		&e.MaxRetries, &e.ErrorMessage, &e.CreatedAt, &e.UpdatedAt,
		&e.PublishedAt, &e.NextRetryAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get by id: %w", err)
	}
	e.Topics = topics
	return &e, nil
}

// Count returns the total number of entries by status
func (r *OutboxRepository) Count(ctx context.Context, status domain.OutboxStatus) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM classified_outbox WHERE status = $1`,
		status).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count: %w", err)
	}
	return count, nil
}

// initialOutboxCapacity is a reasonable default for batch operations
const initialOutboxCapacity = 100

func scanOutboxEntries(rows *sql.Rows) ([]domain.OutboxEntry, error) {
	entries := make([]domain.OutboxEntry, 0, initialOutboxCapacity)
	for rows.Next() {
		var e domain.OutboxEntry
		var topics pq.StringArray

		err := rows.Scan(
			&e.ID, &e.ContentID, &e.SourceName, &e.IndexName, &e.ContentType, &topics,
			&e.QualityScore, &e.IsCrimeRelated, &e.CrimeSubcategory,
			&e.Title, &e.Body, &e.URL, &e.PublishedDate, &e.Status, &e.RetryCount,
			&e.MaxRetries, &e.ErrorMessage, &e.CreatedAt, &e.UpdatedAt,
			&e.PublishedAt, &e.NextRetryAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan outbox entry: %w", err)
		}
		e.Topics = topics
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
