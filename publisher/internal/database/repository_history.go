package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/lib/pq"
)

// ====================
// Publish History
// ====================

// CreatePublishHistory creates a new publish history entry
func (r *Repository) CreatePublishHistory(ctx context.Context, req *models.PublishHistoryCreateRequest) (*models.PublishHistory, error) {
	history := &models.PublishHistory{
		ID:           uuid.New(),
		RouteID:      &req.RouteID,
		ArticleID:    req.ArticleID,
		ArticleTitle: req.ArticleTitle,
		ArticleURL:   req.ArticleURL,
		ChannelName:  req.ChannelName,
		PublishedAt:  time.Now(),
		QualityScore: req.QualityScore,
		Topics:       pq.StringArray(req.Topics),
	}

	query := `
		INSERT INTO publish_history (id, route_id, article_id, article_title, article_url, channel_name, published_at, quality_score, topics)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, route_id, article_id, article_title, article_url, channel_name, published_at, quality_score, topics
	`

	err := r.db.QueryRowxContext(
		ctx, query,
		history.ID, history.RouteID, history.ArticleID, history.ArticleTitle, history.ArticleURL,
		history.ChannelName, history.PublishedAt, history.QualityScore, history.Topics,
	).StructScan(history)

	if err != nil {
		return nil, fmt.Errorf("failed to create publish history: %w", err)
	}

	return history, nil
}

// GetPublishHistoryByID retrieves publish history by ID
func (r *Repository) GetPublishHistoryByID(ctx context.Context, id uuid.UUID) (*models.PublishHistory, error) {
	history := &models.PublishHistory{}
	query := `
		SELECT id, route_id, article_id, article_title, article_url, channel_name, published_at, quality_score, topics
		FROM publish_history
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, history, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get publish history: %w", err)
	}

	return history, nil
}

// ListPublishHistory retrieves publish history with optional filters
func (r *Repository) ListPublishHistory(ctx context.Context, filter *models.PublishHistoryFilter) ([]models.PublishHistory, error) {
	history := []models.PublishHistory{}

	// Default pagination
	if filter.Limit == 0 {
		filter.Limit = 100
	}
	const maxLimit = 1000
	if filter.Limit > maxLimit {
		filter.Limit = maxLimit
	}

	// Build query dynamically
	query := `
		SELECT id, route_id, article_id, article_title, article_url, channel_name, published_at, quality_score, topics
		FROM publish_history
		WHERE 1=1
	`

	args := []any{}
	argPos := 1

	// Apply filters
	if filter.ChannelName != "" {
		query += fmt.Sprintf(" AND channel_name = $%d", argPos)
		args = append(args, filter.ChannelName)
		argPos++
	}

	if filter.ArticleID != "" {
		query += fmt.Sprintf(" AND article_id = $%d", argPos)
		args = append(args, filter.ArticleID)
		argPos++
	}

	if filter.StartDate != nil {
		query += fmt.Sprintf(" AND published_at >= $%d", argPos)
		args = append(args, *filter.StartDate)
		argPos++
	}

	if filter.EndDate != nil {
		query += fmt.Sprintf(" AND published_at <= $%d", argPos)
		args = append(args, *filter.EndDate)
		argPos++
	}
	// argPos is used in the LIMIT/OFFSET clause below

	// Order and pagination
	query += " ORDER BY published_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, filter.Limit, filter.Offset)

	err := r.db.SelectContext(ctx, &history, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list publish history: %w", err)
	}

	return history, nil
}

// GetPublishHistoryByArticleID retrieves all publish history for a specific article
func (r *Repository) GetPublishHistoryByArticleID(ctx context.Context, articleID string) ([]models.PublishHistory, error) {
	history := []models.PublishHistory{}
	query := `
		SELECT id, route_id, article_id, article_title, article_url, channel_name, published_at, quality_score, topics
		FROM publish_history
		WHERE article_id = $1
		ORDER BY published_at DESC
	`

	err := r.db.SelectContext(ctx, &history, query, articleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get publish history by article ID: %w", err)
	}

	return history, nil
}

// CheckArticlePublished checks if an article has been published to a specific channel
func (r *Repository) CheckArticlePublished(ctx context.Context, articleID, channelName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS(
			SELECT 1 FROM publish_history
			WHERE article_id = $1 AND channel_name = $2
		)
	`

	err := r.db.GetContext(ctx, &exists, query, articleID, channelName)
	if err != nil {
		return false, fmt.Errorf("failed to check if article published: %w", err)
	}

	return exists, nil
}

// GetPublishStats retrieves publishing statistics
func (r *Repository) GetPublishStats(ctx context.Context, startDate, endDate *time.Time) (map[string]int, error) {
	query := `
		SELECT channel_name, COUNT(*) as count
		FROM publish_history
		WHERE 1=1
	`

	args := []any{}
	argPos := 1

	if startDate != nil {
		query += fmt.Sprintf(" AND published_at >= $%d", argPos)
		args = append(args, *startDate)
		argPos++
	}

	if endDate != nil {
		query += fmt.Sprintf(" AND published_at <= $%d", argPos)
		args = append(args, *endDate)
		// argPos not needed after this - query is complete
	}

	query += " GROUP BY channel_name ORDER BY count DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get publish stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]int)
	for rows.Next() {
		var channelName string
		var count int
		if scanErr := rows.Scan(&channelName, &count); scanErr != nil {
			return nil, fmt.Errorf("failed to scan row: %w", scanErr)
		}
		stats[channelName] = count
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("row iteration error: %w", rowsErr)
	}

	return stats, nil
}

// GetPublishCountByChannel retrieves the number of articles published to a specific channel
func (r *Repository) GetPublishCountByChannel(ctx context.Context, channelName string, since *time.Time) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM publish_history WHERE channel_name = $1`
	args := []any{channelName}

	if since != nil {
		query += " AND published_at >= $2"
		args = append(args, *since)
	}

	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to get publish count: %w", err)
	}

	return count, nil
}

// GetChannelStats retrieves statistics for all channels including last published date and total count
func (r *Repository) GetChannelStats(ctx context.Context) (map[string]struct {
	TotalPublished int
	LastPublished  *time.Time
}, error) {
	query := `
		SELECT 
			channel_name,
			COUNT(*) as total_published,
			MAX(published_at) as last_published
		FROM publish_history
		GROUP BY channel_name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get channel stats: %w", err)
	}
	defer rows.Close()

	stats := make(map[string]struct {
		TotalPublished int
		LastPublished  *time.Time
	})

	for rows.Next() {
		var channelName string
		var totalPublished int
		var lastPublished sql.NullTime

		if scanErr := rows.Scan(&channelName, &totalPublished, &lastPublished); scanErr != nil {
			return nil, fmt.Errorf("failed to scan row: %w", scanErr)
		}

		var lastPub *time.Time
		if lastPublished.Valid {
			lastPub = &lastPublished.Time
		}

		stats[channelName] = struct {
			TotalPublished int
			LastPublished  *time.Time
		}{
			TotalPublished: totalPublished,
			LastPublished:  lastPub,
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("row iteration error: %w", rowsErr)
	}

	return stats, nil
}

// DeleteAllPublishHistory deletes all publish history entries
func (r *Repository) DeleteAllPublishHistory(ctx context.Context) (int64, error) {
	query := `DELETE FROM publish_history`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete all publish history: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}
	return count, nil
}
