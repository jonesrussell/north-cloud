package database

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// hostStateSelectColumns lists columns for SELECT queries on host_state.
const hostStateSelectColumns = `host, last_fetch_at, min_delay_ms, robots_txt,
	robots_fetched_at, robots_ttl_hours, created_at, updated_at`

// HostStateRepository handles database operations for per-host politeness state.
type HostStateRepository struct {
	db *sqlx.DB
}

// NewHostStateRepository creates a new host state repository.
func NewHostStateRepository(db *sqlx.DB) *HostStateRepository {
	return &HostStateRepository{db: db}
}

// GetOrCreate returns the host state, creating a default entry if none exists.
// Uses INSERT ... ON CONFLICT DO NOTHING then SELECT.
func (r *HostStateRepository) GetOrCreate(ctx context.Context, host string) (*domain.HostState, error) {
	insertQuery := `INSERT INTO host_state (host) VALUES ($1) ON CONFLICT (host) DO NOTHING`

	_, err := r.db.ExecContext(ctx, insertQuery, host)
	if err != nil {
		return nil, fmt.Errorf("failed to insert host state: %w", err)
	}

	selectQuery := `SELECT ` + hostStateSelectColumns + ` FROM host_state WHERE host = $1`

	var state domain.HostState
	if selectErr := r.db.GetContext(ctx, &state, selectQuery, host); selectErr != nil {
		return nil, fmt.Errorf("failed to select host state: %w", selectErr)
	}

	return &state, nil
}

// UpdateLastFetch records a fetch to this host (sets last_fetch_at = NOW()).
func (r *HostStateRepository) UpdateLastFetch(ctx context.Context, host string) error {
	query := `UPDATE host_state SET last_fetch_at = NOW(), updated_at = NOW() WHERE host = $1`

	result, err := r.db.ExecContext(ctx, query, host)
	return execRequireRows(result, err, fmt.Errorf("host state not found: %s", host))
}

// UpdateRobotsTxt caches robots.txt content and optionally updates crawl delay.
// If crawlDelayMs is non-nil, updates min_delay_ms; otherwise keeps existing value.
func (r *HostStateRepository) UpdateRobotsTxt(
	ctx context.Context,
	host string,
	robotsTxt string,
	crawlDelayMs *int,
) error {
	query := `
		UPDATE host_state
		SET robots_txt = $2, robots_fetched_at = NOW(),
			min_delay_ms = COALESCE($3, min_delay_ms), updated_at = NOW()
		WHERE host = $1
	`

	result, err := r.db.ExecContext(ctx, query, host, robotsTxt, crawlDelayMs)
	return execRequireRows(result, err, fmt.Errorf("host state not found: %s", host))
}

// UpdateMinDelay adjusts the per-host minimum delay (e.g., after 429 response).
func (r *HostStateRepository) UpdateMinDelay(ctx context.Context, host string, delayMs int) error {
	query := `UPDATE host_state SET min_delay_ms = $2, updated_at = NOW() WHERE host = $1`

	result, err := r.db.ExecContext(ctx, query, host, delayMs)
	return execRequireRows(result, err, fmt.Errorf("host state not found: %s", host))
}
