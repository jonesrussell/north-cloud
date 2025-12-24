package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/gosources/internal/logger"
	"github.com/jonesrussell/gosources/internal/models"
)

type SourceRepository struct {
	db     *sql.DB
	logger logger.Logger
}

func NewSourceRepository(db *sql.DB, log logger.Logger) *SourceRepository {
	return &SourceRepository{
		db:     db,
		logger: log,
	}
}

func (r *SourceRepository) Create(ctx context.Context, source *models.Source) error {
	source.ID = uuid.New().String()
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()

	selectorsJSON, err := json.Marshal(source.Selectors)
	if err != nil {
		return fmt.Errorf("marshal selectors: %w", err)
	}

	timeJSON, err := json.Marshal(source.Time)
	if err != nil {
		return fmt.Errorf("marshal time: %w", err)
	}

	query := `
		INSERT INTO sources (
			id, name, url, rate_limit, max_depth,
			time, selectors, enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err = r.db.ExecContext(ctx,
		query,
		source.ID,
		source.Name,
		source.URL,
		source.RateLimit,
		source.MaxDepth,
		timeJSON,
		selectorsJSON,
		source.Enabled,
		source.CreatedAt,
		source.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert source: %w", err)
	}

	return nil
}

func (r *SourceRepository) GetByID(ctx context.Context, id string) (*models.Source, error) {
	var source models.Source
	var selectorsJSON, timeJSON []byte

	query := `
		SELECT id, name, url, rate_limit, max_depth,
		       time, selectors, enabled, created_at, updated_at
		FROM sources
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&source.ID,
		&source.Name,
		&source.URL,
		&source.RateLimit,
		&source.MaxDepth,
		&timeJSON,
		&selectorsJSON,
		&source.Enabled,
		&source.CreatedAt,
		&source.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("source not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("query source: %w", err)
	}

	if unmarshalErr := json.Unmarshal(selectorsJSON, &source.Selectors); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal selectors: %w", unmarshalErr)
	}

	if unmarshalErr := json.Unmarshal(timeJSON, &source.Time); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal time: %w", unmarshalErr)
	}

	// Merge selectors with defaults
	source.Selectors = source.Selectors.MergeWithDefaults()

	return &source, nil
}

func (r *SourceRepository) List(ctx context.Context) ([]models.Source, error) {
	query := `
		SELECT id, name, url, rate_limit, max_depth,
		       time, selectors, enabled, created_at, updated_at
		FROM sources
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	var sources []models.Source
	for rows.Next() {
		var source models.Source
		var selectorsJSON, timeJSON []byte

		scanErr := rows.Scan(
			&source.ID,
			&source.Name,
			&source.URL,
			&source.RateLimit,
			&source.MaxDepth,
			&timeJSON,
			&selectorsJSON,
			&source.Enabled,
			&source.CreatedAt,
			&source.UpdatedAt,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("scan source: %w", scanErr)
		}

		if unmarshalErr := json.Unmarshal(selectorsJSON, &source.Selectors); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshal selectors: %w", unmarshalErr)
		}

		if unmarshalErr := json.Unmarshal(timeJSON, &source.Time); unmarshalErr != nil {
			return nil, fmt.Errorf("unmarshal time: %w", unmarshalErr)
		}

		// Merge selectors with defaults
		source.Selectors = source.Selectors.MergeWithDefaults()

		sources = append(sources, source)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate sources: %w", rowsErr)
	}

	return sources, nil
}

func (r *SourceRepository) Update(ctx context.Context, source *models.Source) error {
	source.UpdatedAt = time.Now()

	selectorsJSON, err := json.Marshal(source.Selectors)
	if err != nil {
		return fmt.Errorf("marshal selectors: %w", err)
	}

	timeJSON, err := json.Marshal(source.Time)
	if err != nil {
		return fmt.Errorf("marshal time: %w", err)
	}

	query := `
		UPDATE sources
		SET name = $2, url = $3, rate_limit = $4, max_depth = $5, time = $6, selectors = $7,
		    enabled = $8, updated_at = $9
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx,
		query,
		source.ID,
		source.Name,
		source.URL,
		source.RateLimit,
		source.MaxDepth,
		timeJSON,
		selectorsJSON,
		source.Enabled,
		source.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("update source: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("source not found")
	}

	return nil
}

func (r *SourceRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM sources WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete source: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return errors.New("source not found")
	}

	return nil
}

func (r *SourceRepository) GetCities(ctx context.Context) ([]models.City, error) {
	query := `
		SELECT name as source_name
		FROM sources
		WHERE enabled = true
		ORDER BY name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query cities: %w", err)
	}
	defer rows.Close()

	var cities []models.City
	for rows.Next() {
		var city models.City
		var sourceName string

		scanErr := rows.Scan(&sourceName)
		if scanErr != nil {
			return nil, fmt.Errorf("scan city: %w", scanErr)
		}

		// Use source name as city name
		city.Name = sourceName
		// Derive index name from source name
		city.Index = deriveClassifiedContentIndex(sourceName)

		cities = append(cities, city)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate cities: %w", rowsErr)
	}

	return cities, nil
}

var (
	// invalidIndexNameChars matches all characters that are invalid in Elasticsearch index names.
	// Invalid characters: space, ", *, ,, /, <, >, ?, \, |
	invalidIndexNameChars = regexp.MustCompile(`[\s"*,/<>?\\|]`)
	// consecutiveUnderscores matches two or more consecutive underscores.
	consecutiveUnderscores = regexp.MustCompile(`_{2,}`)
)

// deriveClassifiedContentIndex derives an Elasticsearch index name for classified content
// from a source name. Format: {normalized_source_name}_classified_content
func deriveClassifiedContentIndex(sourceName string) string {
	if sourceName == "" {
		return "unknown_classified_content"
	}

	// Normalize source name for Elasticsearch index
	normalized := sanitizeIndexName(sourceName)
	return fmt.Sprintf("%s_classified_content", normalized)
}

// sanitizeIndexName sanitizes a source name for use in Elasticsearch index names.
// Elasticsearch index names cannot contain: space, ", *, ,, /, <, >, ?, \, |
// This function replaces invalid characters with underscores, normalizes dots/dashes,
// removes leading/trailing underscores, and collapses consecutive underscores.
func sanitizeIndexName(sourceName string) string {
	if sourceName == "" {
		return "unknown"
	}

	// Convert to lowercase first
	normalized := strings.ToLower(sourceName)

	// Replace invalid Elasticsearch index name characters with underscores in one pass
	normalized = invalidIndexNameChars.ReplaceAllString(normalized, "_")

	// Replace dots and dashes with underscores (existing behavior)
	normalized = strings.ReplaceAll(normalized, ".", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")

	// Collapse consecutive underscores into a single underscore
	normalized = consecutiveUnderscores.ReplaceAllString(normalized, "_")

	// Remove leading and trailing underscores
	normalized = strings.Trim(normalized, "_")

	// Handle edge case: if all characters were invalid, return fallback
	if normalized == "" {
		return "unknown"
	}

	return normalized
}
