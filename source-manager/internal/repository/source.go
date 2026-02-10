package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

type SourceRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

func NewSourceRepository(db *sql.DB, log infralogger.Logger) *SourceRepository {
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

	// Normalize rate_limit so API always returns parseable duration (e.g. "10" -> "10s")
	source.RateLimit = models.NormalizeRateLimit(source.RateLimit)

	// Merge selectors with defaults
	source.Selectors = source.Selectors.MergeWithDefaults()

	return &source, nil
}

// ListFilter holds pagination and filter params for ListPaginated.
type ListFilter struct {
	Limit     int
	Offset    int
	SortBy    string // name, url, enabled, created_at
	SortOrder string // asc, desc
	Search    string // ILIKE on name or url
	Enabled   *bool  // nil = all, true = enabled only, false = disabled only
}

// Count returns the total number of sources matching the filter (ignores Limit/Offset/Sort).
func (r *SourceRepository) Count(ctx context.Context, filter ListFilter) (int, error) {
	whereClause, args := buildListWhere(filter)
	query := `SELECT COUNT(*) FROM sources WHERE 1=1` + whereClause

	var count int
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count sources: %w", err)
	}
	return count, nil
}

const (
	limitParamIdx  = 1
	offsetParamIdx = 2
)

// ListPaginated returns sources with pagination, sorting, and filtering.
func (r *SourceRepository) ListPaginated(ctx context.Context, filter ListFilter) ([]models.Source, error) {
	whereClause, whereArgs := buildListWhere(filter)
	orderClause := buildListOrder(filter)
	argCount := len(whereArgs)
	limitPlaceholder := strconv.Itoa(argCount + limitParamIdx)
	offsetPlaceholder := strconv.Itoa(argCount + offsetParamIdx)
	// whereClause and orderClause use whitelisted column names; limit/offset are integers
	// #nosec G202 -- SQL string built from validated filter, column names from whitelist
	query := `
		SELECT id, name, url, rate_limit, max_depth,
		       time, selectors, enabled, created_at, updated_at
		FROM sources
		WHERE 1=1` + whereClause + orderClause + `
		LIMIT $` + limitPlaceholder + ` OFFSET $` + offsetPlaceholder

	args := append(append([]any{}, whereArgs...), filter.Limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query sources: %w", err)
	}
	defer rows.Close()

	sources, scanErr := scanSourceRows(rows)
	if scanErr != nil {
		return nil, scanErr
	}
	return sources, nil
}

func scanSourceRows(rows *sql.Rows) ([]models.Source, error) {
	sources := make([]models.Source, 0)
	for rows.Next() {
		source, err := scanSourceRow(rows)
		if err != nil {
			return nil, err
		}
		source.RateLimit = models.NormalizeRateLimit(source.RateLimit)
		source.Selectors = source.Selectors.MergeWithDefaults()
		sources = append(sources, *source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sources: %w", err)
	}
	return sources, nil
}

func scanSourceRow(rows *sql.Rows) (*models.Source, error) {
	var source models.Source
	var selectorsJSON, timeJSON []byte
	if err := rows.Scan(
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
	); err != nil {
		return nil, fmt.Errorf("scan source: %w", err)
	}
	if unmarshalErr := json.Unmarshal(selectorsJSON, &source.Selectors); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal selectors: %w", unmarshalErr)
	}
	if unmarshalErr := json.Unmarshal(timeJSON, &source.Time); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal time: %w", unmarshalErr)
	}
	return &source, nil
}

func buildListWhere(filter ListFilter) (whereClause string, args []any) {
	var clauses []string
	args = make([]any, 0)
	pos := 1

	if filter.Search != "" {
		clauses = append(clauses, fmt.Sprintf("(name ILIKE $%d OR url ILIKE $%d)", pos, pos))
		args = append(args, "%"+filter.Search+"%")
		pos++
	}
	if filter.Enabled != nil {
		clauses = append(clauses, fmt.Sprintf("enabled = $%d", pos))
		args = append(args, *filter.Enabled)
	}

	if len(clauses) == 0 {
		return "", args
	}
	whereClause = " AND " + strings.Join(clauses, " AND ")
	return whereClause, args
}

func buildListOrder(filter ListFilter) string {
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "name"
	}
	validSort := map[string]bool{
		"name": true, "url": true, "enabled": true, "created_at": true,
	}
	if !validSort[sortBy] {
		sortBy = "name"
	}
	order := strings.ToUpper(filter.SortOrder)
	if order != "ASC" && order != "DESC" {
		order = "ASC"
	}
	return fmt.Sprintf(" ORDER BY %s %s", sortBy, order)
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

	return scanSourceRows(rows)
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

// UpsertSourcesTx upserts multiple sources in a single transaction.
// Returns the count of created and updated sources.
// If any upsert fails, the entire transaction is rolled back.
func (r *SourceRepository) UpsertSourcesTx(ctx context.Context, sources []*models.Source) (created, updated int, err error) {
	if len(sources) == 0 {
		return 0, 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				r.logger.Error("failed to rollback transaction",
					infralogger.Error(rbErr),
				)
			}
		}
	}()

	for _, source := range sources {
		isCreated, upsertErr := r.UpsertSource(ctx, tx, source)
		if upsertErr != nil {
			err = fmt.Errorf("upsert source %q: %w", source.Name, upsertErr)
			return 0, 0, err
		}
		if isCreated {
			created++
		} else {
			updated++
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit transaction: %w", commitErr)
		return 0, 0, err
	}

	return created, updated, nil
}

// UpsertSource inserts or updates a source within an existing transaction.
// Returns true if the source was created (new), false if updated (existed).
// Uses PostgreSQL's ON CONFLICT with xmax trick to determine insert vs update.
func (r *SourceRepository) UpsertSource(ctx context.Context, tx *sql.Tx, source *models.Source) (bool, error) {
	now := time.Now()

	// Generate new ID if not set (will be overwritten if exists)
	if source.ID == "" {
		source.ID = uuid.New().String()
	}
	source.CreatedAt = now
	source.UpdatedAt = now

	selectorsJSON, err := json.Marshal(source.Selectors)
	if err != nil {
		return false, fmt.Errorf("marshal selectors: %w", err)
	}

	timeJSON, err := json.Marshal(source.Time)
	if err != nil {
		return false, fmt.Errorf("marshal time: %w", err)
	}

	// Use ON CONFLICT to upsert, and xmax = 0 trick to determine if inserted
	query := `
		INSERT INTO sources (
			id, name, url, rate_limit, max_depth,
			time, selectors, enabled, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (name) DO UPDATE SET
			url = EXCLUDED.url,
			rate_limit = EXCLUDED.rate_limit,
			max_depth = EXCLUDED.max_depth,
			time = EXCLUDED.time,
			selectors = EXCLUDED.selectors,
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
		RETURNING id, (xmax = 0) AS is_insert
	`

	var returnedID string
	var isInsert bool
	err = tx.QueryRowContext(ctx,
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
	).Scan(&returnedID, &isInsert)

	if err != nil {
		return false, fmt.Errorf("upsert source: %w", err)
	}

	// Update the source ID (may have changed if it was an update)
	source.ID = returnedID

	return isInsert, nil
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

	cities := make([]models.City, 0)
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
