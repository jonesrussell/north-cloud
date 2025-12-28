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
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/lib/pq"
)

const (
	whereEnabledTrue = " WHERE enabled = true"
)

// Repository provides database operations for all entities
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new repository instance
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// ====================
// Sources
// ====================

// CreateSource creates a new source
func (r *Repository) CreateSource(ctx context.Context, req *models.SourceCreateRequest) (*models.Source, error) {
	source := &models.Source{
		ID:           uuid.New(),
		Name:         req.Name,
		IndexPattern: req.IndexPattern,
		Enabled:      true, // Default to true
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if req.Enabled != nil {
		source.Enabled = *req.Enabled
	}

	query := `
		INSERT INTO sources (id, name, index_pattern, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, index_pattern, enabled, created_at, updated_at
	`

	err := r.db.QueryRowxContext(
		ctx, query,
		source.ID, source.Name, source.IndexPattern, source.Enabled, source.CreatedAt, source.UpdatedAt,
	).StructScan(source)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" { // unique_violation
			return nil, models.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create source: %w", err)
	}

	return source, nil
}

// GetSourceByID retrieves a source by ID
func (r *Repository) GetSourceByID(ctx context.Context, id uuid.UUID) (*models.Source, error) {
	source := &models.Source{}
	query := `
		SELECT id, name, index_pattern, enabled, created_at, updated_at
		FROM sources
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, source, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return source, nil
}

// GetSourceByName retrieves a source by name
func (r *Repository) GetSourceByName(ctx context.Context, name string) (*models.Source, error) {
	source := &models.Source{}
	query := `
		SELECT id, name, index_pattern, enabled, created_at, updated_at
		FROM sources
		WHERE name = $1
	`

	err := r.db.GetContext(ctx, source, query, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return source, nil
}

// ListSources retrieves all sources
func (r *Repository) ListSources(ctx context.Context, enabledOnly bool) ([]models.Source, error) {
	sources := []models.Source{}
	query := `
		SELECT id, name, index_pattern, enabled, created_at, updated_at
		FROM sources
	`

	if enabledOnly {
		query += whereEnabledTrue
	}

	query += " ORDER BY name ASC"

	err := r.db.SelectContext(ctx, &sources, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}

	return sources, nil
}

// UpdateSource updates a source
func (r *Repository) UpdateSource(ctx context.Context, id uuid.UUID, req *models.SourceUpdateRequest) (*models.Source, error) {
	updates := make(map[string]any)

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.IndexPattern != nil {
		updates["index_pattern"] = *req.IndexPattern
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	query, args, err := buildUpdateQuery(
		"sources",
		id,
		updates,
		"id, name, index_pattern, enabled, created_at, updated_at",
	)
	if err != nil {
		return nil, err
	}

	source := &models.Source{}
	err = r.db.QueryRowxContext(ctx, query, args...).StructScan(source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, models.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to update source: %w", err)
	}

	return source, nil
}

// DeleteSource deletes a source
func (r *Repository) DeleteSource(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sources WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
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

// ====================
// Channels
// ====================

// CreateChannel creates a new channel
func (r *Repository) CreateChannel(ctx context.Context, req *models.ChannelCreateRequest) (*models.Channel, error) {
	channel := &models.Channel{
		ID:          uuid.New(),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}

	query := `
		INSERT INTO channels (id, name, description, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, enabled, created_at, updated_at
	`

	err := r.db.QueryRowxContext(
		ctx, query,
		channel.ID, channel.Name, channel.Description, channel.Enabled, channel.CreatedAt, channel.UpdatedAt,
	).StructScan(channel)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, models.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	return channel, nil
}

// GetChannelByID retrieves a channel by ID
func (r *Repository) GetChannelByID(ctx context.Context, id uuid.UUID) (*models.Channel, error) {
	channel := &models.Channel{}
	query := `
		SELECT id, name, description, enabled, created_at, updated_at
		FROM channels
		WHERE id = $1
	`

	err := r.db.GetContext(ctx, channel, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return channel, nil
}

// GetChannelByName retrieves a channel by name
func (r *Repository) GetChannelByName(ctx context.Context, name string) (*models.Channel, error) {
	channel := &models.Channel{}
	query := `
		SELECT id, name, description, enabled, created_at, updated_at
		FROM channels
		WHERE name = $1
	`

	err := r.db.GetContext(ctx, channel, query, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	return channel, nil
}

// ListChannels retrieves all channels
func (r *Repository) ListChannels(ctx context.Context, enabledOnly bool) ([]models.Channel, error) {
	channels := []models.Channel{}
	query := `
		SELECT id, name, description, enabled, created_at, updated_at
		FROM channels
	`

	if enabledOnly {
		query += whereEnabledTrue
	}

	query += " ORDER BY name ASC"

	err := r.db.SelectContext(ctx, &channels, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list channels: %w", err)
	}

	return channels, nil
}

// UpdateChannel updates a channel
func (r *Repository) UpdateChannel(ctx context.Context, id uuid.UUID, req *models.ChannelUpdateRequest) (*models.Channel, error) {
	updates := make(map[string]any)

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	query, args, err := buildUpdateQuery(
		"channels",
		id,
		updates,
		"id, name, description, enabled, created_at, updated_at",
	)
	if err != nil {
		return nil, err
	}

	channel := &models.Channel{}
	err = r.db.QueryRowxContext(ctx, query, args...).StructScan(channel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, models.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to update channel: %w", err)
	}

	return channel, nil
}

// DeleteChannel deletes a channel
func (r *Repository) DeleteChannel(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM channels WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete channel: %w", err)
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

// Helper function to join strings
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	var result strings.Builder
	result.WriteString(strs[0])
	for i := 1; i < len(strs); i++ {
		result.WriteString(sep)
		result.WriteString(strs[i])
	}
	return result.String()
}

// buildUpdateQuery builds a dynamic UPDATE query with the given fields
// Returns the query string and args slice, or error if no fields to update
func buildUpdateQuery(table string, id uuid.UUID, updates map[string]any, returningFields string) (string, []any, error) {
	if len(updates) == 0 {
		return "", nil, models.ErrNoFieldsToUpdate
	}

	updateFields := make([]string, 0, len(updates)+1)
	args := make([]any, 0, len(updates)+2)
	argPos := 1

	// Add update fields
	for field, value := range updates {
		updateFields = append(updateFields, fmt.Sprintf("%s = $%d", field, argPos))
		args = append(args, value)
		argPos++
	}

	// Add updated_at
	updateFields = append(updateFields, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	// Add ID for WHERE clause
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE %s
		SET %s
		WHERE id = $%d
		RETURNING %s
	`, table, joinStrings(updateFields, ", "), argPos, returningFields)

	return query, args, nil
}
