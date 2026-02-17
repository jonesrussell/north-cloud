package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// feedStateSelectColumns lists columns for SELECT queries on feed_state.
const feedStateSelectColumns = `source_id, feed_url, last_polled_at, last_etag, last_modified,
	last_item_count, consecutive_errors, last_error, created_at, updated_at`

// FeedStateRepository handles database operations for feed polling state.
type FeedStateRepository struct {
	db *sqlx.DB
}

// NewFeedStateRepository creates a new feed state repository.
func NewFeedStateRepository(db *sqlx.DB) *FeedStateRepository {
	return &FeedStateRepository{db: db}
}

// FeedPollResult contains the results of a successful feed poll.
type FeedPollResult struct {
	ETag      *string
	Modified  *string
	ItemCount int
}

// GetOrCreate returns feed state for a source, creating a default entry if none exists.
// Uses INSERT ... ON CONFLICT DO NOTHING then SELECT.
func (r *FeedStateRepository) GetOrCreate(ctx context.Context, sourceID, feedURL string) (*domain.FeedState, error) {
	insertQuery := `INSERT INTO feed_state (source_id, feed_url) VALUES ($1, $2) ON CONFLICT (source_id) DO NOTHING`

	_, err := r.db.ExecContext(ctx, insertQuery, sourceID, feedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to insert feed state: %w", err)
	}

	selectQuery := `SELECT ` + feedStateSelectColumns + ` FROM feed_state WHERE source_id = $1`

	var state domain.FeedState
	if selectErr := r.db.GetContext(ctx, &state, selectQuery, sourceID); selectErr != nil {
		return nil, fmt.Errorf("failed to select feed state: %w", selectErr)
	}

	return &state, nil
}

// UpdateSuccess records a successful feed poll.
// Resets consecutive_errors to 0 and clears last_error.
func (r *FeedStateRepository) UpdateSuccess(ctx context.Context, sourceID string, result FeedPollResult) error {
	query := `
		UPDATE feed_state
		SET last_polled_at = NOW(), last_etag = $2, last_modified = $3,
			last_item_count = $4, consecutive_errors = 0, last_error = NULL, updated_at = NOW()
		WHERE source_id = $1
	`

	execResult, err := r.db.ExecContext(ctx, query, sourceID, result.ETag, result.Modified, result.ItemCount)
	return execRequireRows(execResult, err, fmt.Errorf("feed state not found: %s", sourceID))
}

// UpdateError records a feed poll failure, incrementing consecutive_errors.
func (r *FeedStateRepository) UpdateError(ctx context.Context, sourceID, errMsg string) error {
	query := `
		UPDATE feed_state
		SET last_polled_at = NOW(), consecutive_errors = consecutive_errors + 1,
			last_error = $2, updated_at = NOW()
		WHERE source_id = $1
	`

	result, err := r.db.ExecContext(ctx, query, sourceID, errMsg)
	return execRequireRows(result, err, fmt.Errorf("feed state not found: %s", sourceID))
}

// ListDueForPolling returns feed states that are due for polling.
// A feed is due if last_polled_at IS NULL or last_polled_at + interval <= NOW().
// Results are ordered with never-polled feeds first, then oldest-polled.
func (r *FeedStateRepository) ListDueForPolling(ctx context.Context, defaultIntervalMinutes int) ([]*domain.FeedState, error) {
	query := `
		SELECT ` + feedStateSelectColumns + `
		FROM feed_state
		WHERE last_polled_at IS NULL
		   OR last_polled_at + ($1 * INTERVAL '1 minute') <= NOW()
		ORDER BY last_polled_at ASC NULLS FIRST
	`

	var states []*domain.FeedState
	if err := r.db.SelectContext(ctx, &states, query, defaultIntervalMinutes); err != nil {
		return nil, fmt.Errorf("failed to list feed states due for polling: %w", err)
	}

	if states == nil {
		states = []*domain.FeedState{}
	}

	return states, nil
}
