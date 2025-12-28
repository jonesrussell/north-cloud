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

const (
	defaultMinQualityScore = 50
)

// ====================
// Routes
// ====================

// CreateRoute creates a new route
func (r *Repository) CreateRoute(ctx context.Context, req *models.RouteCreateRequest) (*models.Route, error) {
	route := &models.Route{
		ID:              uuid.New(),
		SourceID:        req.SourceID,
		ChannelID:       req.ChannelID,
		MinQualityScore: defaultMinQualityScore, // Default
		Topics:          pq.StringArray(req.Topics),
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if req.MinQualityScore != nil {
		route.MinQualityScore = *req.MinQualityScore
	}

	if req.Enabled != nil {
		route.Enabled = *req.Enabled
	}

	query := `
		INSERT INTO routes (id, source_id, channel_id, min_quality_score, topics, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, source_id, channel_id, min_quality_score, topics, enabled, created_at, updated_at
	`

	err := r.db.QueryRowxContext(
		ctx, query,
		route.ID, route.SourceID, route.ChannelID, route.MinQualityScore, route.Topics, route.Enabled, route.CreatedAt, route.UpdatedAt,
	).StructScan(route)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			switch pqErr.Code {
			case "23505": // unique_violation
				return nil, models.ErrDuplicateRoute
			case "23503": // foreign_key_violation
				return nil, errors.New("source or channel not found")
			}
		}
		return nil, fmt.Errorf("failed to create route: %w", err)
	}

	return route, nil
}

// GetRouteByID retrieves a route by ID
func (r *Repository) GetRouteByID(ctx context.Context, id uuid.UUID) (*models.Route, error) {
	route := &models.Route{}
	query := `
		SELECT id, source_id, channel_id, min_quality_score, topics, enabled, created_at, updated_at
		FROM routes
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, route, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get route: %w", err)
	}

	return route, nil
}

// GetRouteWithDetails retrieves a route with joined source and channel details
func (r *Repository) GetRouteWithDetails(ctx context.Context, id uuid.UUID) (*models.RouteWithDetails, error) {
	routeDetails := &models.RouteWithDetails{}
	query := `
		SELECT
			r.id, r.source_id, r.channel_id, r.min_quality_score, r.topics, r.enabled, r.created_at, r.updated_at,
			s.name AS source_name,
			s.index_pattern AS source_index_pattern,
			c.name AS channel_name,
			c.description AS channel_description
		FROM routes r
		INNER JOIN sources s ON r.source_id = s.id
		INNER JOIN channels c ON r.channel_id = c.id
		WHERE r.id = $1
	`

	err := r.db.GetContext(ctx, routeDetails, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get route with details: %w", err)
	}

	return routeDetails, nil
}

// ListRoutes retrieves all routes
func (r *Repository) ListRoutes(ctx context.Context, enabledOnly bool) ([]models.Route, error) {
	routes := []models.Route{}
	query := `
		SELECT id, source_id, channel_id, min_quality_score, topics, enabled, created_at, updated_at
		FROM routes
	`

	if enabledOnly {
		query += whereEnabledTrue
	}

	query += " ORDER BY created_at DESC"

	err := r.db.SelectContext(ctx, &routes, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}

	return routes, nil
}

// ListRoutesWithDetails retrieves all routes with joined source and channel details
func (r *Repository) ListRoutesWithDetails(ctx context.Context, enabledOnly bool) ([]models.RouteWithDetails, error) {
	routes := []models.RouteWithDetails{}
	query := `
		SELECT
			r.id, r.source_id, r.channel_id, r.min_quality_score, r.topics, r.enabled, r.created_at, r.updated_at,
			s.name AS source_name,
			s.index_pattern AS source_index_pattern,
			c.name AS channel_name,
			c.description AS channel_description
		FROM routes r
		INNER JOIN sources s ON r.source_id = s.id
		INNER JOIN channels c ON r.channel_id = c.id
	`

	if enabledOnly {
		query += " WHERE r.enabled = true"
	}

	query += " ORDER BY r.created_at DESC"

	err := r.db.SelectContext(ctx, &routes, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes with details: %w", err)
	}

	return routes, nil
}

// UpdateRoute updates a route
func (r *Repository) UpdateRoute(ctx context.Context, id uuid.UUID, req *models.RouteUpdateRequest) (*models.Route, error) {
	updateFields := []string{}
	args := []any{}
	argPos := 1

	if req.MinQualityScore != nil {
		updateFields = append(updateFields, fmt.Sprintf("min_quality_score = $%d", argPos))
		args = append(args, *req.MinQualityScore)
		argPos++
	}

	if req.Topics != nil {
		updateFields = append(updateFields, fmt.Sprintf("topics = $%d", argPos))
		args = append(args, pq.Array(req.Topics))
		argPos++
	}

	if req.Enabled != nil {
		updateFields = append(updateFields, fmt.Sprintf("enabled = $%d", argPos))
		args = append(args, *req.Enabled)
		argPos++
	}

	if len(updateFields) == 0 {
		return nil, models.ErrNoFieldsToUpdate
	}

	updateFields = append(updateFields, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE routes
		SET %s
		WHERE id = $%d
		RETURNING id, source_id, channel_id, min_quality_score, topics, enabled, created_at, updated_at
	`, joinStrings(updateFields, ", "), argPos)

	route := &models.Route{}
	err := r.db.QueryRowxContext(ctx, query, args...).StructScan(route)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to update route: %w", err)
	}

	return route, nil
}

// DeleteRoute deletes a route
func (r *Repository) DeleteRoute(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM routes WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete route: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return models.ErrNotFound
	}

	return nil
}

// GetRoutesBySourceID retrieves all routes for a specific source
func (r *Repository) GetRoutesBySourceID(ctx context.Context, sourceID uuid.UUID) ([]models.RouteWithDetails, error) {
	routes := []models.RouteWithDetails{}
	query := `
		SELECT
			r.id, r.source_id, r.channel_id, r.min_quality_score, r.topics, r.enabled, r.created_at, r.updated_at,
			s.name AS source_name,
			s.index_pattern AS source_index_pattern,
			c.name AS channel_name,
			c.description AS channel_description
		FROM routes r
		INNER JOIN sources s ON r.source_id = s.id
		INNER JOIN channels c ON r.channel_id = c.id
		WHERE r.source_id = $1 AND r.enabled = true
		ORDER BY r.created_at DESC
	`

	err := r.db.SelectContext(ctx, &routes, query, sourceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes by source: %w", err)
	}

	return routes, nil
}

// GetRoutesByChannelID retrieves all routes for a specific channel
func (r *Repository) GetRoutesByChannelID(ctx context.Context, channelID uuid.UUID) ([]models.RouteWithDetails, error) {
	routes := []models.RouteWithDetails{}
	query := `
		SELECT
			r.id, r.source_id, r.channel_id, r.min_quality_score, r.topics, r.enabled, r.created_at, r.updated_at,
			s.name AS source_name,
			s.index_pattern AS source_index_pattern,
			c.name AS channel_name,
			c.description AS channel_description
		FROM routes r
		INNER JOIN sources s ON r.source_id = s.id
		INNER JOIN channels c ON r.channel_id = c.id
		WHERE r.channel_id = $1 AND r.enabled = true
		ORDER BY r.created_at DESC
	`

	err := r.db.SelectContext(ctx, &routes, query, channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get routes by channel: %w", err)
	}

	return routes, nil
}
