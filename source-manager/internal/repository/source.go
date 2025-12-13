package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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
			id, name, url, article_index, page_index, rate_limit, max_depth,
			time, selectors, city_name, group_id, enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`

	_, err = r.db.ExecContext(ctx,
		query,
		source.ID,
		source.Name,
		source.URL,
		source.ArticleIndex,
		source.PageIndex,
		source.RateLimit,
		source.MaxDepth,
		timeJSON,
		selectorsJSON,
		source.CityName,
		source.GroupID,
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
	var cityName, groupID sql.NullString

	query := `
		SELECT id, name, url, article_index, page_index, rate_limit, max_depth,
		       time, selectors, city_name, group_id, enabled, created_at, updated_at
		FROM sources
		WHERE id = $1
	`

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&source.ID,
		&source.Name,
		&source.URL,
		&source.ArticleIndex,
		&source.PageIndex,
		&source.RateLimit,
		&source.MaxDepth,
		&timeJSON,
		&selectorsJSON,
		&cityName,
		&groupID,
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

	if cityName.Valid {
		source.CityName = &cityName.String
	}
	if groupID.Valid {
		source.GroupID = &groupID.String
	}

	return &source, nil
}

func (r *SourceRepository) List(ctx context.Context) ([]models.Source, error) {
	query := `
		SELECT id, name, url, article_index, page_index, rate_limit, max_depth,
		       time, selectors, city_name, group_id, enabled, created_at, updated_at
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
		var cityName, groupID sql.NullString

		scanErr := rows.Scan(
			&source.ID,
			&source.Name,
			&source.URL,
			&source.ArticleIndex,
			&source.PageIndex,
			&source.RateLimit,
			&source.MaxDepth,
			&timeJSON,
			&selectorsJSON,
			&cityName,
			&groupID,
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

		if cityName.Valid {
			source.CityName = &cityName.String
		}
		if groupID.Valid {
			source.GroupID = &groupID.String
		}

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
		SET name = $2, url = $3, article_index = $4, page_index = $5,
		    rate_limit = $6, max_depth = $7, time = $8, selectors = $9,
		    city_name = $10, group_id = $11, enabled = $12, updated_at = $13
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx,
		query,
		source.ID,
		source.Name,
		source.URL,
		source.ArticleIndex,
		source.PageIndex,
		source.RateLimit,
		source.MaxDepth,
		timeJSON,
		selectorsJSON,
		source.CityName,
		source.GroupID,
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
		SELECT 
			COALESCE(city_name, name) as city_name,
			article_index,
			COALESCE(group_id, '') as group_id
		FROM sources
		WHERE enabled = true AND city_name IS NOT NULL
		ORDER BY city_name
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query cities: %w", err)
	}
	defer rows.Close()

	var cities []models.City
	for rows.Next() {
		var city models.City
		var groupID sql.NullString

		scanErr := rows.Scan(&city.Name, &city.Index, &groupID)
		if scanErr != nil {
			return nil, fmt.Errorf("scan city: %w", scanErr)
		}

		if groupID.Valid && groupID.String != "" {
			city.GroupID = groupID.String
		}

		cities = append(cities, city)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate cities: %w", rowsErr)
	}

	return cities, nil
}
