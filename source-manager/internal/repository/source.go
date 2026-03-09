package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/naming"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
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

	extractionProfileJSON := marshalExtractionProfile(source.ExtractionProfile)

	query := `
		INSERT INTO sources (
			id, name, url, rate_limit, max_depth,
			time, selectors, enabled,
			feed_url, sitemap_url, ingestion_mode, feed_poll_interval_minutes,
			allow_source_discovery, identity_key, extraction_profile, template_hint,
			render_mode, indigenous_region, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
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
		source.FeedURL,
		source.SitemapURL,
		source.IngestionMode,
		source.FeedPollIntervalMinutes,
		source.AllowSourceDiscovery,
		source.IdentityKey,
		extractionProfileJSON,
		source.TemplateHint,
		source.RenderMode,
		source.IndigenousRegion,
		source.CreatedAt,
		source.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("insert source: %w", err)
	}

	return nil
}

// marshalExtractionProfile returns bytes for JSONB storage; nil if empty.
func marshalExtractionProfile(e *models.ExtractionProfileJSON) []byte {
	if e == nil || len(*e) == 0 {
		return nil
	}
	return []byte(*e)
}

func (r *SourceRepository) GetByID(ctx context.Context, id string) (*models.Source, error) {
	var source models.Source
	var selectorsJSON, timeJSON []byte

	query := `
		SELECT id, name, url, rate_limit, max_depth,
		       time, selectors, enabled,
		       feed_url, sitemap_url, ingestion_mode, feed_poll_interval_minutes,
		       feed_disabled_at, feed_disable_reason,
		       allow_source_discovery, identity_key, extraction_profile, template_hint,
		       render_mode, indigenous_region, created_at, updated_at
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
		&source.FeedURL,
		&source.SitemapURL,
		&source.IngestionMode,
		&source.FeedPollIntervalMinutes,
		&source.FeedDisabledAt,
		&source.FeedDisableReason,
		&source.AllowSourceDiscovery,
		&source.IdentityKey,
		&source.ExtractionProfile,
		&source.TemplateHint,
		&source.RenderMode,
		&source.IndigenousRegion,
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

// GetByIdentityKey returns a source with the given identity_key (exact match).
// Used by the Source Identity Resolver. Returns nil, nil when no source is found.
func (r *SourceRepository) GetByIdentityKey(ctx context.Context, identityKey string) (*models.Source, error) {
	if identityKey == "" {
		return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
	}
	query := `
		SELECT id, name, url, rate_limit, max_depth,
		       time, selectors, enabled,
		       feed_url, sitemap_url, ingestion_mode, feed_poll_interval_minutes,
		       feed_disabled_at, feed_disable_reason,
		       allow_source_discovery, identity_key, extraction_profile, template_hint,
		       render_mode, indigenous_region, created_at, updated_at
		FROM sources
		WHERE identity_key = $1
		ORDER BY created_at ASC
		LIMIT 1
	`
	rows, err := r.db.QueryContext(ctx, query, identityKey)
	if err != nil {
		return nil, fmt.Errorf("query source by identity_key: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
	}
	source, scanErr := scanSourceRow(rows)
	if scanErr != nil {
		return nil, scanErr
	}
	source.RateLimit = models.NormalizeRateLimit(source.RateLimit)
	source.Selectors = source.Selectors.MergeWithDefaults()
	return source, nil
}

// ListFilter holds pagination and filter params for ListPaginated.
type ListFilter struct {
	Limit          int
	Offset         int
	SortBy         string // name, url, enabled, created_at
	SortOrder      string // asc, desc
	Search         string // ILIKE on name or url
	Enabled        *bool  // nil = all, true = enabled only, false = disabled only
	FeedActive     *bool  // nil = all, true = feeds that are active or past cooldown
	IndigenousOnly bool   // true = only sources with indigenous_region IS NOT NULL
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
		       time, selectors, enabled,
		       feed_url, sitemap_url, ingestion_mode, feed_poll_interval_minutes,
		       feed_disabled_at, feed_disable_reason,
		       allow_source_discovery, identity_key, extraction_profile, template_hint,
		       render_mode, indigenous_region, created_at, updated_at
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
		&source.FeedURL,
		&source.SitemapURL,
		&source.IngestionMode,
		&source.FeedPollIntervalMinutes,
		&source.FeedDisabledAt,
		&source.FeedDisableReason,
		&source.AllowSourceDiscovery,
		&source.IdentityKey,
		&source.ExtractionProfile,
		&source.TemplateHint,
		&source.RenderMode,
		&source.IndigenousRegion,
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

	if filter.FeedActive != nil && *filter.FeedActive {
		// Include sources where:
		// 1. feed is not disabled (feed_disabled_at IS NULL), OR
		// 2. cooldown has expired (based on reason-specific durations)
		clauses = append(clauses, `(
			feed_disabled_at IS NULL
			OR feed_disabled_at + (CASE feed_disable_reason
				WHEN 'not_found'        THEN INTERVAL '48 hours'
				WHEN 'gone'             THEN INTERVAL '72 hours'
				WHEN 'forbidden'        THEN INTERVAL '24 hours'
				WHEN 'upstream_failure'  THEN INTERVAL '6 hours'
				WHEN 'network'          THEN INTERVAL '12 hours'
				WHEN 'parse_error'      THEN INTERVAL '24 hours'
				ELSE INTERVAL '24 hours'
			END) <= NOW()
		)`)
	}

	if filter.IndigenousOnly {
		clauses = append(clauses, "indigenous_region IS NOT NULL")
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
		       time, selectors, enabled,
		       feed_url, sitemap_url, ingestion_mode, feed_poll_interval_minutes,
		       feed_disabled_at, feed_disable_reason,
		       allow_source_discovery, identity_key, extraction_profile, template_hint,
		       render_mode, indigenous_region, created_at, updated_at
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

	extractionProfileJSON := marshalExtractionProfile(source.ExtractionProfile)

	query := `
		UPDATE sources
		SET name = $2, url = $3, rate_limit = $4, max_depth = $5, time = $6, selectors = $7,
		    enabled = $8,
		    feed_url = $9, sitemap_url = $10, ingestion_mode = $11, feed_poll_interval_minutes = $12,
		    allow_source_discovery = $13, identity_key = $14, extraction_profile = $15, template_hint = $16,
		    render_mode = $17, indigenous_region = $18, updated_at = $19
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
		source.FeedURL,
		source.SitemapURL,
		source.IngestionMode,
		source.FeedPollIntervalMinutes,
		source.AllowSourceDiscovery,
		source.IdentityKey,
		extractionProfileJSON,
		source.TemplateHint,
		source.RenderMode,
		source.IndigenousRegion,
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
// Returns slices of sources that were created and updated (by pointer into the input slice).
// If any upsert fails, the entire transaction is rolled back.
func (r *SourceRepository) UpsertSourcesTx(ctx context.Context, sources []*models.Source) (created, updated []*models.Source, err error) {
	if len(sources) == 0 {
		return nil, nil, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("begin transaction: %w", err)
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

	created = make([]*models.Source, 0, len(sources))
	updated = make([]*models.Source, 0, len(sources))

	for _, source := range sources {
		isCreated, upsertErr := r.UpsertSource(ctx, tx, source)
		if upsertErr != nil {
			err = fmt.Errorf("upsert source %q: %w", source.Name, upsertErr)
			return nil, nil, err
		}
		if isCreated {
			created = append(created, source)
		} else {
			updated = append(updated, source)
		}
	}

	if commitErr := tx.Commit(); commitErr != nil {
		err = fmt.Errorf("commit transaction: %w", commitErr)
		return nil, nil, err
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
			time, selectors, enabled,
			feed_url, sitemap_url, ingestion_mode, feed_poll_interval_minutes,
			render_mode, indigenous_region, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (name) DO UPDATE SET
			url = EXCLUDED.url,
			rate_limit = EXCLUDED.rate_limit,
			max_depth = EXCLUDED.max_depth,
			time = EXCLUDED.time,
			selectors = EXCLUDED.selectors,
			enabled = EXCLUDED.enabled,
			feed_url = EXCLUDED.feed_url,
			sitemap_url = EXCLUDED.sitemap_url,
			ingestion_mode = EXCLUDED.ingestion_mode,
			feed_poll_interval_minutes = EXCLUDED.feed_poll_interval_minutes,
			render_mode = EXCLUDED.render_mode,
			indigenous_region = EXCLUDED.indigenous_region,
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
		source.FeedURL,
		source.SitemapURL,
		source.IngestionMode,
		source.FeedPollIntervalMinutes,
		source.RenderMode,
		source.IndigenousRegion,
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

// DisableFeed marks a source's feed as disabled with a reason.
func (r *SourceRepository) DisableFeed(ctx context.Context, id, reason string) error {
	query := `
		UPDATE sources
		SET feed_disabled_at = NOW(), feed_disable_reason = $2, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id, reason)
	if err != nil {
		return fmt.Errorf("disable feed: %w", err)
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

// EnableFeed clears a source's feed disabled state.
func (r *SourceRepository) EnableFeed(ctx context.Context, id string) error {
	query := `
		UPDATE sources
		SET feed_disabled_at = NULL, feed_disable_reason = NULL, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("enable feed: %w", err)
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

// deriveClassifiedContentIndex derives an Elasticsearch index name for classified content
// from a source name. Falls back to "unknown" prefix when source name is empty.
func deriveClassifiedContentIndex(sourceName string) string {
	if sourceName == "" {
		return "unknown" + naming.ClassifiedContentSuffix
	}
	return naming.ClassifiedContentIndex(sourceName)
}
