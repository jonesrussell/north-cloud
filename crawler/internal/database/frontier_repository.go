package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// ErrNoURLAvailable is returned when Claim finds no fetchable URLs in the frontier.
// Callers should check with errors.Is().
var ErrNoURLAvailable = errors.New("no URL available in frontier")

// Frontier repository constants.
const (
	defaultFrontierLimit  = 50
	defaultFrontierSortBy = "priority"

	// frontierSelectColumns lists columns for SELECT queries on url_frontier (aliased as f).
	frontierSelectColumns = `f.id, f.url, f.url_hash, f.host, f.source_id, f.origin, f.parent_url, f.depth,
		f.priority, f.status, f.next_fetch_at, f.last_fetched_at, f.fetch_count,
		f.content_hash, f.etag, f.last_modified, f.retry_count, f.last_error,
		f.discovered_at, f.created_at, f.updated_at`
)

// FrontierRepository handles database operations for the URL frontier.
type FrontierRepository struct {
	db *sqlx.DB
}

// NewFrontierRepository creates a new frontier repository.
func NewFrontierRepository(db *sqlx.DB) *FrontierRepository {
	return &FrontierRepository{db: db}
}

// SubmitParams contains the parameters for submitting a URL to the frontier.
type SubmitParams struct {
	URL       string
	URLHash   string
	Host      string
	SourceID  string
	Origin    string
	ParentURL *string
	Depth     int
	Priority  int
}

// Submit upserts a URL into the frontier. On conflict (same url_hash),
// updates priority to the higher value and next_fetch_at to the earlier time,
// but only for pending URLs (don't re-queue fetched/dead URLs).
func (r *FrontierRepository) Submit(ctx context.Context, params SubmitParams) error {
	query := `
		INSERT INTO url_frontier (url, url_hash, host, source_id, origin, parent_url, depth, priority, next_fetch_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
		ON CONFLICT (url_hash) DO UPDATE SET
			priority = GREATEST(url_frontier.priority, EXCLUDED.priority),
			next_fetch_at = LEAST(url_frontier.next_fetch_at, EXCLUDED.next_fetch_at),
			updated_at = NOW()
		WHERE url_frontier.status = 'pending'
	`

	_, err := r.db.ExecContext(
		ctx, query,
		params.URL, params.URLHash, params.Host, params.SourceID,
		params.Origin, params.ParentURL, params.Depth, params.Priority,
	)
	if err != nil {
		return fmt.Errorf("failed to submit URL to frontier: %w", err)
	}

	return nil
}

// Claim selects and locks the highest-priority fetchable URL, respecting
// per-host politeness via host_state. Returns ErrNoURLAvailable if no URLs are fetchable.
func (r *FrontierRepository) Claim(ctx context.Context) (*domain.FrontierURL, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin claim transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is a no-op

	url, selectErr := claimSelect(ctx, tx)
	if selectErr != nil {
		return nil, selectErr
	}

	if updateErr := claimUpdateStatus(ctx, tx, url.ID); updateErr != nil {
		return nil, updateErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return nil, fmt.Errorf("failed to commit claim transaction: %w", commitErr)
	}

	url.Status = domain.FrontierStatusFetching
	return url, nil
}

// claimSelect selects and locks the highest-priority fetchable URL within a transaction.
func claimSelect(ctx context.Context, tx *sqlx.Tx) (*domain.FrontierURL, error) {
	query := `
		SELECT ` + frontierSelectColumns + `
		FROM url_frontier f
		LEFT JOIN host_state h ON h.host = f.host
		WHERE f.status = 'pending'
		  AND f.next_fetch_at <= NOW()
		  AND (h.host IS NULL OR h.last_fetch_at + (h.min_delay_ms * INTERVAL '1 millisecond') <= NOW())
		ORDER BY f.priority DESC, f.next_fetch_at ASC
		LIMIT 1
		FOR UPDATE OF f SKIP LOCKED
	`

	var url domain.FrontierURL
	err := tx.GetContext(ctx, &url, query)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoURLAvailable
		}
		return nil, fmt.Errorf("failed to select claimable URL: %w", err)
	}

	return &url, nil
}

// claimUpdateStatus updates a frontier URL's status to fetching within a transaction.
func claimUpdateStatus(ctx context.Context, tx *sqlx.Tx, id string) error {
	query := `UPDATE url_frontier SET status = 'fetching', updated_at = NOW() WHERE id = $1`

	_, err := tx.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update claimed URL status: %w", err)
	}

	return nil
}

// FetchedParams contains the parameters for marking a URL as successfully fetched.
type FetchedParams struct {
	ContentHash  *string
	ETag         *string
	LastModified *string
}

// UpdateFetched marks a frontier URL as successfully fetched.
func (r *FrontierRepository) UpdateFetched(ctx context.Context, id string, params FetchedParams) error {
	query := `
		UPDATE url_frontier
		SET status = 'fetched',
			last_fetched_at = NOW(),
			fetch_count = fetch_count + 1,
			content_hash = $1,
			etag = $2,
			last_modified = $3,
			retry_count = 0,
			updated_at = NOW()
		WHERE id = $4
	`

	result, execErr := r.db.ExecContext(ctx, query, params.ContentHash, params.ETag, params.LastModified, id)
	return execRequireRows(result, execErr, fmt.Errorf("frontier URL not found: %s", id))
}

// UpdateFailed increments retry_count. If retries >= maxRetries, marks as dead.
// Otherwise, sets status back to pending with exponential backoff on next_fetch_at.
func (r *FrontierRepository) UpdateFailed(ctx context.Context, id, lastError string, maxRetries int) error {
	query := `
		UPDATE url_frontier
		SET retry_count = retry_count + 1,
			last_error = $1,
			status = CASE
				WHEN retry_count + 1 >= $2 THEN 'dead'
				ELSE 'pending'
			END,
			next_fetch_at = CASE
				WHEN retry_count + 1 >= $2 THEN next_fetch_at
				ELSE NOW() + (POWER(2, retry_count) * INTERVAL '1 minute')
			END,
			updated_at = NOW()
		WHERE id = $3
	`

	result, execErr := r.db.ExecContext(ctx, query, lastError, maxRetries, id)
	return execRequireRows(result, execErr, fmt.Errorf("frontier URL not found: %s", id))
}

// UpdateDead marks a frontier URL as dead with a reason.
func (r *FrontierRepository) UpdateDead(ctx context.Context, id, reason string) error {
	query := `
		UPDATE url_frontier
		SET status = 'dead',
			last_error = $1,
			updated_at = NOW()
		WHERE id = $2
	`

	result, execErr := r.db.ExecContext(ctx, query, reason, id)
	return execRequireRows(result, execErr, fmt.Errorf("frontier URL not found: %s", id))
}

// FrontierFilters represents filtering options for listing frontier URLs.
type FrontierFilters struct {
	Status   string
	SourceID string
	Host     string
	Origin   string
	Search   string // URL contains
	SortBy   string
	Limit    int
	Offset   int
}

// List returns frontier URLs with pagination and filtering (for dashboard API).
func (r *FrontierRepository) List(ctx context.Context, filters FrontierFilters) ([]*domain.FrontierURL, int, error) {
	whereClause, args := buildFrontierWhere(filters)

	count, countErr := r.countFrontier(ctx, whereClause, args)
	if countErr != nil {
		return nil, 0, countErr
	}

	urls, listErr := r.selectFrontier(ctx, filters, whereClause, args)
	if listErr != nil {
		return nil, 0, listErr
	}

	return urls, count, nil
}

// buildFrontierWhere builds the WHERE clause and args for frontier queries.
func buildFrontierWhere(filters FrontierFilters) (whereClause string, args []any) {
	var conditions []string
	args = []any{}
	argIndex := 1

	if filters.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filters.Status)
		argIndex++
	}

	if filters.SourceID != "" {
		conditions = append(conditions, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, filters.SourceID)
		argIndex++
	}

	if filters.Host != "" {
		conditions = append(conditions, fmt.Sprintf("host = $%d", argIndex))
		args = append(args, filters.Host)
		argIndex++
	}

	if filters.Origin != "" {
		conditions = append(conditions, fmt.Sprintf("origin = $%d", argIndex))
		args = append(args, filters.Origin)
		argIndex++
	}

	if filters.Search != "" {
		conditions = append(conditions, fmt.Sprintf("url ILIKE $%d", argIndex))
		args = append(args, "%"+filters.Search+"%")
	}

	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}
	return whereClause, args
}

// countFrontier returns the total count of frontier URLs matching the WHERE clause.
func (r *FrontierRepository) countFrontier(ctx context.Context, whereClause string, args []any) (int, error) {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM url_frontier %s", whereClause)

	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count frontier URLs: %w", err)
	}

	return count, nil
}

// selectFrontier returns frontier URLs with sorting and pagination.
func (r *FrontierRepository) selectFrontier(
	ctx context.Context,
	filters FrontierFilters,
	whereClause string,
	args []any,
) ([]*domain.FrontierURL, error) {
	argIndex := len(args) + 1

	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = defaultFrontierSortBy
	}
	if sortBy != defaultFrontierSortBy && sortBy != "next_fetch_at" && sortBy != "created_at" {
		sortBy = defaultFrontierSortBy
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = defaultFrontierLimit
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT %s
		FROM url_frontier f
		%s
		ORDER BY %s DESC
		LIMIT $%d OFFSET $%d
	`, frontierSelectColumns, whereClause, sortBy, argIndex, argIndex+1)

	args = append(args, limit, offset)

	var urls []*domain.FrontierURL
	err := r.db.SelectContext(ctx, &urls, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list frontier URLs: %w", err)
	}

	if urls == nil {
		urls = []*domain.FrontierURL{}
	}

	return urls, nil
}

// FrontierStats contains aggregate counts by status for the frontier.
type FrontierStats struct {
	TotalPending  int `json:"total_pending"`
	TotalFetching int `json:"total_fetching"`
	TotalFetched  int `json:"total_fetched"`
	TotalFailed   int `json:"total_failed"`
	TotalDead     int `json:"total_dead"`
}

// Stats returns aggregate counts of frontier URLs grouped by status.
func (r *FrontierRepository) Stats(ctx context.Context) (*FrontierStats, error) {
	query := `SELECT status, COUNT(*) FROM url_frontier GROUP BY status`

	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query frontier stats: %w", err)
	}
	defer rows.Close()

	stats := &FrontierStats{}
	for rows.Next() {
		var status string
		var count int
		if scanErr := rows.Scan(&status, &count); scanErr != nil {
			return nil, fmt.Errorf("failed to scan frontier stats row: %w", scanErr)
		}
		assignStatCount(stats, status, count)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("failed to iterate frontier stats: %w", rowsErr)
	}

	return stats, nil
}

// Delete removes a frontier URL by ID. Returns an error if the URL does not exist.
func (r *FrontierRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM url_frontier WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)

	return execRequireRows(result, err, fmt.Errorf("frontier URL not found: %s", id))
}

// assignStatCount assigns a count to the appropriate FrontierStats field by status.
func assignStatCount(stats *FrontierStats, status string, count int) {
	switch status {
	case domain.FrontierStatusPending:
		stats.TotalPending = count
	case domain.FrontierStatusFetching:
		stats.TotalFetching = count
	case domain.FrontierStatusFetched:
		stats.TotalFetched = count
	case domain.FrontierStatusFailed:
		stats.TotalFailed = count
	case domain.FrontierStatusDead:
		stats.TotalDead = count
	}
}
