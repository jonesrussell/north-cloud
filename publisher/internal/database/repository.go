package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/publisher/internal/models"
	"github.com/lib/pq"
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
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" { // unique_violation
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
		query += " WHERE enabled = true"
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
	// Build dynamic update query
	updateFields := []string{}
	args := []interface{}{}
	argPos := 1

	if req.Name != nil {
		updateFields = append(updateFields, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *req.Name)
		argPos++
	}

	if req.IndexPattern != nil {
		updateFields = append(updateFields, fmt.Sprintf("index_pattern = $%d", argPos))
		args = append(args, *req.IndexPattern)
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

	// Add updated_at
	updateFields = append(updateFields, fmt.Sprintf("updated_at = $%d", argPos))
	args = append(args, time.Now())
	argPos++

	// Add ID for WHERE clause
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE sources
		SET %s
		WHERE id = $%d
		RETURNING id, name, index_pattern, enabled, created_at, updated_at
	`, joinStrings(updateFields, ", "), argPos)

	source := &models.Source{}
	err := r.db.QueryRowxContext(ctx, query, args...).StructScan(source)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
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
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
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
		query += " WHERE enabled = true"
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
	updateFields := []string{}
	args := []interface{}{}
	argPos := 1

	if req.Name != nil {
		updateFields = append(updateFields, fmt.Sprintf("name = $%d", argPos))
		args = append(args, *req.Name)
		argPos++
	}

	if req.Description != nil {
		updateFields = append(updateFields, fmt.Sprintf("description = $%d", argPos))
		args = append(args, *req.Description)
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
		UPDATE channels
		SET %s
		WHERE id = $%d
		RETURNING id, name, description, enabled, created_at, updated_at
	`, joinStrings(updateFields, ", "), argPos)

	channel := &models.Channel{}
	err := r.db.QueryRowxContext(ctx, query, args...).StructScan(channel)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
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
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
