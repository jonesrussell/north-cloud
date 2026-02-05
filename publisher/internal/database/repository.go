package database

import (
	"context"
	"database/sql"
	"encoding/json"
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
	// updateQueryExtraArgs is the number of additional arguments added to update queries
	// (updated_at timestamp and id for WHERE clause)
	updateQueryExtraArgs = 2
	// maxUpdateFields caps the number of columns in a single UPDATE to avoid allocation-size overflow
	maxUpdateFields = 256
)

// Repository provides database operations for all entities
type Repository struct {
	db *sqlx.DB
}

// NewRepository creates a new repository instance
func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Ping checks the database connection
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// ====================
// Channels
// ====================

// CreateChannel creates a new channel with rules
func (r *Repository) CreateChannel(ctx context.Context, req *models.ChannelCreateRequest) (*models.Channel, error) {
	rulesJSON := []byte("{}")
	if req.Rules != nil {
		var err error
		rulesJSON, err = json.Marshal(req.Rules)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal rules: %w", err)
		}
	}

	channel := &models.Channel{
		ID:           uuid.New(),
		Name:         req.Name,
		Slug:         req.Slug,
		RedisChannel: req.RedisChannel,
		Description:  req.Description,
		RulesJSON:    rulesJSON,
		RulesVersion: 1,
		Enabled:      true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if req.Enabled != nil {
		channel.Enabled = *req.Enabled
	}

	query := `
		INSERT INTO channels (id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at
	`

	err := r.db.QueryRowxContext(
		ctx, query,
		channel.ID, channel.Name, channel.Slug, channel.RedisChannel,
		channel.Description, channel.RulesJSON, channel.RulesVersion,
		channel.Enabled, channel.CreatedAt, channel.UpdatedAt,
	).StructScan(channel)

	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return nil, models.ErrAlreadyExists
		}
		return nil, fmt.Errorf("failed to create channel: %w", err)
	}

	if parseErr := channel.ParseRules(); parseErr != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", parseErr)
	}

	return channel, nil
}

// GetChannelByID retrieves a channel by ID
func (r *Repository) GetChannelByID(ctx context.Context, id uuid.UUID) (*models.Channel, error) {
	channel := &models.Channel{}
	query := `
		SELECT id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at
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

	if parseErr := channel.ParseRules(); parseErr != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", parseErr)
	}

	return channel, nil
}

// GetChannelBySlug retrieves a channel by slug
func (r *Repository) GetChannelBySlug(ctx context.Context, slug string) (*models.Channel, error) {
	channel := &models.Channel{}
	query := `
		SELECT id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at
		FROM channels
		WHERE slug = $1
	`

	err := r.db.GetContext(ctx, channel, query, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get channel: %w", err)
	}

	if parseErr := channel.ParseRules(); parseErr != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", parseErr)
	}

	return channel, nil
}

// ListChannels retrieves all channels
func (r *Repository) ListChannels(ctx context.Context, enabledOnly bool) ([]models.Channel, error) {
	channels := []models.Channel{}
	query := `
		SELECT id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at
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

	// Parse rules for each channel
	for i := range channels {
		if parseErr := channels[i].ParseRules(); parseErr != nil {
			return nil, fmt.Errorf("failed to parse rules for channel %s: %w", channels[i].Slug, parseErr)
		}
	}

	return channels, nil
}

// ListEnabledChannelsWithRules returns all enabled channels with parsed rules
func (r *Repository) ListEnabledChannelsWithRules(ctx context.Context) ([]models.Channel, error) {
	return r.ListChannels(ctx, true)
}

// UpdateChannel updates a channel
func (r *Repository) UpdateChannel(ctx context.Context, id uuid.UUID, req *models.ChannelUpdateRequest) (*models.Channel, error) {
	updates := make(map[string]any)

	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Slug != nil {
		updates["slug"] = *req.Slug
	}
	if req.RedisChannel != nil {
		updates["redis_channel"] = *req.RedisChannel
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Rules != nil {
		rulesJSON, err := json.Marshal(req.Rules)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal rules: %w", err)
		}
		updates["rules"] = rulesJSON
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}

	query, args, err := buildUpdateQuery(
		"channels",
		id,
		updates,
		"id, name, slug, redis_channel, description, rules, rules_version, enabled, created_at, updated_at",
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

	if parseErr := channel.ParseRules(); parseErr != nil {
		return nil, fmt.Errorf("failed to parse rules: %w", parseErr)
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

// buildUpdateQuery builds a dynamic UPDATE query with the given fields
// Returns the query string and args slice, or error if no fields to update
func buildUpdateQuery(table string, id uuid.UUID, updates map[string]any, returningFields string) (query string, args []any, err error) {
	if len(updates) == 0 {
		return "", nil, models.ErrNoFieldsToUpdate
	}
	if len(updates) > maxUpdateFields {
		return "", nil, fmt.Errorf("too many update fields: %d exceeds maximum %d", len(updates), maxUpdateFields)
	}

	updateFields := make([]string, 0, len(updates)+1)
	args = make([]any, 0, len(updates)+updateQueryExtraArgs)
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

	query = fmt.Sprintf(`
		UPDATE %s
		SET %s
		WHERE id = $%d
		RETURNING %s
	`, table, strings.Join(updateFields, ", "), argPos, returningFields)

	return query, args, nil
}
