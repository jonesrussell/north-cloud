package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

const (
	defaultSortByPriority = "priority"
)

// DiscoveredLinkRepository handles database operations for discovered links.
type DiscoveredLinkRepository struct {
	db *sqlx.DB
}

// NewDiscoveredLinkRepository creates a new discovered link repository.
func NewDiscoveredLinkRepository(db *sqlx.DB) *DiscoveredLinkRepository {
	return &DiscoveredLinkRepository{db: db}
}

// CreateOrUpdate creates a new discovered link or updates existing one if URL already exists for source.
func (r *DiscoveredLinkRepository) CreateOrUpdate(ctx context.Context, link *domain.DiscoveredLink) error {
	if link.ID == "" {
		link.ID = uuid.New().String()
	}
	if link.DiscoveredAt.IsZero() {
		link.DiscoveredAt = time.Now()
	}
	if link.QueuedAt.IsZero() {
		link.QueuedAt = time.Now()
	}
	if link.Status == "" {
		link.Status = "pending"
	}

	query := `
		INSERT INTO discovered_links (id, source_id, source_name, url, parent_url, depth, 
		                          discovered_at, queued_at, status, priority)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (source_id, url) 
		DO UPDATE SET 
			parent_url = EXCLUDED.parent_url,
			depth = EXCLUDED.depth,
			priority = EXCLUDED.priority,
			updated_at = NOW()
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRowContext(
		ctx,
		query,
		link.ID,
		link.SourceID,
		link.SourceName,
		link.URL,
		link.ParentURL,
		link.Depth,
		link.DiscoveredAt,
		link.QueuedAt,
		link.Status,
		link.Priority,
	).Scan(&link.CreatedAt, &link.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create or update discovered link: %w", err)
	}

	return nil
}

// ListFilters represents filtering options for listing discovered links.
type ListFilters struct {
	Status     string
	SourceID   string
	SourceName string
	Search     string // URL search
	SortBy     string // priority, queued_at, discovered_at
	SortOrder  string // asc, desc
	Limit      int
	Offset     int
}

// List retrieves discovered links with optional filtering.
func (r *DiscoveredLinkRepository) List(ctx context.Context, filters ListFilters) ([]*domain.DiscoveredLink, error) {
	var links []*domain.DiscoveredLink

	// Build WHERE clause
	whereClauses := []string{}
	args := []any{}
	argIndex := 1

	if filters.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filters.Status)
		argIndex++
	}

	if filters.SourceID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, filters.SourceID)
		argIndex++
	}

	if filters.SourceName != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("source_name = $%d", argIndex))
		args = append(args, filters.SourceName)
		argIndex++
	}

	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("url ILIKE $%d", argIndex))
		args = append(args, "%"+filters.Search+"%")
		argIndex++
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Build ORDER BY clause
	sortBy := filters.SortBy
	if sortBy == "" {
		sortBy = defaultSortByPriority
	}
	if sortBy != defaultSortByPriority && sortBy != "queued_at" && sortBy != "discovered_at" {
		sortBy = defaultSortByPriority
	}

	sortOrder := strings.ToUpper(filters.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}

	// Set defaults for limit/offset
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, source_id, source_name, url, parent_url, depth, discovered_at, 
		       queued_at, status, priority, created_at, updated_at
		FROM discovered_links
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereClause, sortBy, sortOrder, argIndex, argIndex+1)

	args = append(args, limit, offset)

	err := r.db.SelectContext(ctx, &links, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list discovered links: %w", err)
	}

	if links == nil {
		links = []*domain.DiscoveredLink{}
	}

	return links, nil
}

// Count returns the total number of discovered links with optional filtering.
func (r *DiscoveredLinkRepository) Count(ctx context.Context, filters ListFilters) (int, error) {
	var count int

	// Build WHERE clause (same as List)
	whereClauses := []string{}
	args := []any{}
	argIndex := 1

	if filters.Status != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, filters.Status)
		argIndex++
	}

	if filters.SourceID != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, filters.SourceID)
		argIndex++
	}

	if filters.SourceName != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("source_name = $%d", argIndex))
		args = append(args, filters.SourceName)
		argIndex++
	}

	if filters.Search != "" {
		whereClauses = append(whereClauses, fmt.Sprintf("url ILIKE $%d", argIndex))
		args = append(args, "%"+filters.Search+"%")
		// argIndex not used after this in Count function
	}

	whereClause := ""
	if len(whereClauses) > 0 {
		whereClause = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	query := fmt.Sprintf(`SELECT COUNT(*) FROM discovered_links %s`, whereClause)

	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count discovered links: %w", err)
	}

	return count, nil
}

// GetByID retrieves a discovered link by its ID.
func (r *DiscoveredLinkRepository) GetByID(ctx context.Context, id string) (*domain.DiscoveredLink, error) {
	var link domain.DiscoveredLink
	query := `
		SELECT id, source_id, source_name, url, parent_url, depth, discovered_at, 
		       queued_at, status, priority, created_at, updated_at
		FROM discovered_links
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, &link, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("discovered link not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get discovered link: %w", err)
	}

	return &link, nil
}

// GetPendingBySource retrieves pending links for a source, ordered by priority and queued_at.
func (r *DiscoveredLinkRepository) GetPendingBySource(
	ctx context.Context,
	sourceID string,
	limit int,
) ([]*domain.DiscoveredLink, error) {
	var links []*domain.DiscoveredLink

	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT id, source_id, source_name, url, parent_url, depth, discovered_at, 
		       queued_at, status, priority, created_at, updated_at
		FROM discovered_links
		WHERE source_id = $1 AND status = 'pending'
		ORDER BY priority DESC, queued_at ASC
		LIMIT $2
	`

	err := r.db.SelectContext(ctx, &links, query, sourceID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending links: %w", err)
	}

	if links == nil {
		links = []*domain.DiscoveredLink{}
	}

	return links, nil
}

// UpdateStatus updates the status of a discovered link.
func (r *DiscoveredLinkRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE discovered_links SET status = $1, updated_at = NOW() WHERE id = $2`

	result, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("discovered link not found: %s", id)
	}

	return nil
}

// Delete removes a discovered link from the database.
func (r *DiscoveredLinkRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM discovered_links WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete discovered link: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("discovered link not found: %s", id)
	}

	return nil
}

// CountPendingBySource returns the count of pending links for a source.
func (r *DiscoveredLinkRepository) CountPendingBySource(ctx context.Context, sourceID string) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM discovered_links WHERE source_id = $1 AND status = 'pending'`

	err := r.db.GetContext(ctx, &count, query, sourceID)
	if err != nil {
		return 0, fmt.Errorf("failed to count pending links: %w", err)
	}

	return count, nil
}
