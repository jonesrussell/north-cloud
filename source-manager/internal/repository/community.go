package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

const (
	defaultCommunityLimit = 50
	maxCommunityLimit     = 200
	earthRadiusKm         = 6371.0
	kmPerDegreeLat        = 111.0
	degreesPerHalfCircle  = 180.0
)

// CommunityRepository provides CRUD operations for the communities table.
type CommunityRepository struct {
	db     *sql.DB
	logger infralogger.Logger
}

// NewCommunityRepository creates a new CommunityRepository.
func NewCommunityRepository(db *sql.DB, log infralogger.Logger) *CommunityRepository {
	return &CommunityRepository{
		db:     db,
		logger: log,
	}
}

// scanCommunity scans a single row into a Community struct.
func scanCommunity(row interface{ Scan(...any) error }) (*models.Community, error) {
	var c models.Community
	scanErr := row.Scan(
		&c.ID, &c.Name, &c.Slug, &c.CommunityType, &c.Province, &c.Region,
		&c.InacID, &c.StatCanCSD, &c.Latitude, &c.Longitude,
		&c.Nation, &c.Treaty, &c.LanguageGroup, &c.ReserveName, &c.Population, &c.PopulationYear,
		&c.Website, &c.FeedURL, &c.DataSource, &c.SourceID, &c.Enabled, &c.CreatedAt, &c.UpdatedAt,
	)
	if scanErr != nil {
		return nil, fmt.Errorf("scan community: %w", scanErr)
	}
	return &c, nil
}

// Create inserts a new community. ID and timestamps are set automatically.
func (r *CommunityRepository) Create(ctx context.Context, c *models.Community) error {
	c.ID = uuid.New().String()
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()

	query := `
		INSERT INTO communities (
			id, name, slug, community_type, province, region,
			inac_id, statcan_csd, latitude, longitude,
			nation, treaty, language_group, reserve_name, population, population_year,
			website, feed_url, data_source, source_id, enabled, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23
		)`

	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.Name, c.Slug, c.CommunityType, c.Province, c.Region,
		c.InacID, c.StatCanCSD, c.Latitude, c.Longitude,
		c.Nation, c.Treaty, c.LanguageGroup, c.ReserveName, c.Population, c.PopulationYear,
		c.Website, c.FeedURL, c.DataSource, c.SourceID, c.Enabled, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create community: %w", err)
	}

	return nil
}

// GetByID returns a community by ID, or nil if not found.
func (r *CommunityRepository) GetByID(ctx context.Context, id string) (*models.Community, error) {
	query := `SELECT id, name, slug, community_type, province, region,
		inac_id, statcan_csd, latitude, longitude,
		nation, treaty, language_group, reserve_name, population, population_year,
		website, feed_url, data_source, source_id, enabled, created_at, updated_at
		FROM communities WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)

	c, err := scanCommunity(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get community by id: %w", err)
	}

	return c, nil
}

// GetBySlug returns a community by slug, or nil if not found.
func (r *CommunityRepository) GetBySlug(ctx context.Context, slug string) (*models.Community, error) {
	query := `SELECT id, name, slug, community_type, province, region,
		inac_id, statcan_csd, latitude, longitude,
		nation, treaty, language_group, reserve_name, population, population_year,
		website, feed_url, data_source, source_id, enabled, created_at, updated_at
		FROM communities WHERE slug = $1`
	row := r.db.QueryRowContext(ctx, query, slug)

	c, err := scanCommunity(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil //nolint:nilnil // nil,nil = "not found" per interface contract
		}
		return nil, fmt.Errorf("get community by slug: %w", err)
	}

	return c, nil
}

// Update modifies an existing community by ID.
// Note: updated_at is handled by the set_communities_updated_at DB trigger.
func (r *CommunityRepository) Update(ctx context.Context, c *models.Community) error {
	query := `
		UPDATE communities SET
			name = $2, slug = $3, community_type = $4, province = $5, region = $6,
			inac_id = $7, statcan_csd = $8, latitude = $9, longitude = $10,
			nation = $11, treaty = $12, language_group = $13, reserve_name = $14,
			population = $15, population_year = $16,
			website = $17, feed_url = $18, data_source = $19, source_id = $20,
			enabled = $21
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		c.ID, c.Name, c.Slug, c.CommunityType, c.Province, c.Region,
		c.InacID, c.StatCanCSD, c.Latitude, c.Longitude,
		c.Nation, c.Treaty, c.LanguageGroup, c.ReserveName, c.Population, c.PopulationYear,
		c.Website, c.FeedURL, c.DataSource, c.SourceID, c.Enabled,
	)
	if err != nil {
		return fmt.Errorf("update community: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update community rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("update community: not found")
	}

	return nil
}

// Delete removes a community by ID.
func (r *CommunityRepository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM communities WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete community: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("delete community rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("delete community: not found")
	}

	return nil
}

// buildCommunityWhere constructs a WHERE clause and args from a CommunityFilter.
func buildCommunityWhere(filter models.CommunityFilter) (where string, args []any) {
	var conditions []string
	argIdx := 1

	if filter.Type != "" {
		conditions = append(conditions, fmt.Sprintf("community_type = $%d", argIdx))
		args = append(args, filter.Type)
		argIdx++
	}

	if filter.Province != "" {
		conditions = append(conditions, fmt.Sprintf("province = $%d", argIdx))
		args = append(args, filter.Province)
		argIdx++
	}

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE $%d", argIdx))
		args = append(args, "%"+filter.Search+"%")
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

// Count returns the total number of communities matching the filter.
func (r *CommunityRepository) Count(ctx context.Context, filter models.CommunityFilter) (int, error) {
	where, args := buildCommunityWhere(filter)
	query := "SELECT COUNT(*) FROM communities" + where

	var count int
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count communities: %w", err)
	}

	return count, nil
}

// ListPaginated returns communities matching the filter with pagination.
func (r *CommunityRepository) ListPaginated(
	ctx context.Context, filter models.CommunityFilter,
) ([]models.Community, error) {
	where, args := buildCommunityWhere(filter)
	argIdx := len(args) + 1

	limit := filter.Limit
	if limit <= 0 || limit > maxCommunityLimit {
		limit = defaultCommunityLimit
	}

	//nolint:gosec // G201: query uses only constant column names and integer placeholders
	query := fmt.Sprintf(`SELECT id, name, slug, community_type, province, region,
		inac_id, statcan_csd, latitude, longitude,
		nation, treaty, language_group, reserve_name, population, population_year,
		website, feed_url, data_source, source_id, enabled, created_at, updated_at
		FROM communities%s ORDER BY name ASC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, limit, filter.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list communities: %w", err)
	}
	defer rows.Close()

	communities := make([]models.Community, 0, limit)
	for rows.Next() {
		c, scanErr := scanCommunity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		communities = append(communities, *c)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list communities rows: %w", closeErr)
	}

	return communities, nil
}

// upsertCommunity is a shared helper for upsert operations.
func (r *CommunityRepository) upsertCommunity(
	ctx context.Context, c *models.Community, conflictColumn string,
) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()

	//nolint:gosec // G201: conflictColumn is always a hardcoded constant from caller
	query := fmt.Sprintf(`
		INSERT INTO communities (
			id, name, slug, community_type, province, region,
			inac_id, statcan_csd, latitude, longitude,
			nation, treaty, language_group, reserve_name, population, population_year,
			website, feed_url, data_source, source_id, enabled, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16,
			$17, $18, $19, $20, $21, $22, $23
		)
		ON CONFLICT (%s) DO UPDATE SET
			name = EXCLUDED.name, slug = EXCLUDED.slug,
			community_type = EXCLUDED.community_type,
			province = EXCLUDED.province, region = EXCLUDED.region,
			latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude,
			nation = EXCLUDED.nation, treaty = EXCLUDED.treaty,
			language_group = EXCLUDED.language_group,
			reserve_name = EXCLUDED.reserve_name,
			population = EXCLUDED.population,
			population_year = EXCLUDED.population_year,
			website = EXCLUDED.website, feed_url = EXCLUDED.feed_url,
			data_source = EXCLUDED.data_source, source_id = EXCLUDED.source_id,
			enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at
		RETURNING id`, conflictColumn)

	if err := r.db.QueryRowContext(ctx, query,
		c.ID, c.Name, c.Slug, c.CommunityType, c.Province, c.Region,
		c.InacID, c.StatCanCSD, c.Latitude, c.Longitude,
		c.Nation, c.Treaty, c.LanguageGroup, c.ReserveName, c.Population, c.PopulationYear,
		c.Website, c.FeedURL, c.DataSource, c.SourceID, c.Enabled, c.CreatedAt, c.UpdatedAt,
	).Scan(&c.ID); err != nil {
		return fmt.Errorf("upsert community on %s: %w", conflictColumn, err)
	}

	return nil
}

// UpsertByInacID inserts or updates a community by CIRNAC band number.
func (r *CommunityRepository) UpsertByInacID(ctx context.Context, c *models.Community) error {
	if c.InacID == nil {
		return errors.New("upsert by inac_id: inac_id is required")
	}
	return r.upsertCommunity(ctx, c, "inac_id")
}

// UpsertByStatCanCSD inserts or updates a community by StatsCan CSD code.
func (r *CommunityRepository) UpsertByStatCanCSD(ctx context.Context, c *models.Community) error {
	if c.StatCanCSD == nil {
		return errors.New("upsert by statcan_csd: statcan_csd is required")
	}
	return r.upsertCommunity(ctx, c, "statcan_csd")
}

// SetSourceLink sets the source_id and website on a community.
func (r *CommunityRepository) SetSourceLink(ctx context.Context, communityID, sourceID, website string) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE communities SET source_id = $2, website = $3 WHERE id = $1`,
		communityID, sourceID, website,
	)
	if err != nil {
		return fmt.Errorf("set source link: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("set source link rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("set source link: community not found")
	}

	return nil
}

// ListUnlinked returns communities that have no source_id set.
func (r *CommunityRepository) ListUnlinked(ctx context.Context) ([]models.Community, error) {
	query := `SELECT id, name, slug, community_type, province, region,
		inac_id, statcan_csd, latitude, longitude,
		nation, treaty, language_group, reserve_name, population, population_year,
		website, feed_url, data_source, source_id, enabled, created_at, updated_at
		FROM communities WHERE source_id IS NULL ORDER BY name ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list unlinked communities: %w", err)
	}
	defer rows.Close()

	var communities []models.Community
	for rows.Next() {
		c, scanErr := scanCommunity(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		communities = append(communities, *c)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list unlinked rows: %w", closeErr)
	}

	return communities, nil
}

// RegionSummary holds a province/region pair with its community count.
type RegionSummary struct {
	Province string `json:"province"`
	Region   string `json:"region"`
	Count    int    `json:"count"`
}

// ListRegions returns distinct province/region pairs with community counts.
func (r *CommunityRepository) ListRegions(ctx context.Context) ([]RegionSummary, error) {
	query := `SELECT COALESCE(province, ''), COALESCE(region, ''), COUNT(*) AS count
		FROM communities
		WHERE enabled = true
		GROUP BY province, region
		ORDER BY province ASC, region ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list regions: %w", err)
	}
	defer rows.Close()

	var regions []RegionSummary
	for rows.Next() {
		var rs RegionSummary
		if scanErr := rows.Scan(&rs.Province, &rs.Region, &rs.Count); scanErr != nil {
			return nil, fmt.Errorf("scan region: %w", scanErr)
		}
		regions = append(regions, rs)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("list regions rows: %w", closeErr)
	}

	return regions, nil
}

// FindNearby returns communities within radiusKm of the given coordinates,
// sorted by distance ascending. Uses bounding-box prefilter + haversine CTE.
func (r *CommunityRepository) FindNearby(
	ctx context.Context, lat, lon, radiusKm float64, limit int,
) ([]models.CommunityWithDistance, error) {
	if limit <= 0 || limit > maxCommunityLimit {
		limit = defaultCommunityLimit
	}

	latDelta := radiusKm / kmPerDegreeLat
	lonDelta := radiusKm / (kmPerDegreeLat * math.Cos(lat*math.Pi/degreesPerHalfCircle))

	query := fmt.Sprintf(`
		WITH nearby AS (
			SELECT id, name, slug, community_type, province, region,
				inac_id, statcan_csd, latitude, longitude,
				nation, treaty, language_group, reserve_name, population, population_year,
				website, feed_url, data_source, source_id, enabled, created_at, updated_at,
				(%.1f * acos(
					LEAST(1.0, cos(radians($1)) * cos(radians(latitude)) *
					cos(radians(longitude) - radians($2)) +
					sin(radians($1)) * sin(radians(latitude)))
				)) AS distance_km
			FROM communities
			WHERE latitude IS NOT NULL
				AND longitude IS NOT NULL
				AND latitude BETWEEN $3 AND $4
				AND longitude BETWEEN $5 AND $6
		)
		SELECT id, name, slug, community_type, province, region,
			inac_id, statcan_csd, latitude, longitude,
			nation, treaty, language_group, reserve_name, population, population_year,
			website, feed_url, data_source, source_id, enabled, created_at, updated_at,
			distance_km
		FROM nearby
		WHERE distance_km <= $7
		ORDER BY distance_km ASC
		LIMIT $8`, earthRadiusKm)

	rows, err := r.db.QueryContext(ctx, query,
		lat, lon,
		lat-latDelta, lat+latDelta,
		lon-lonDelta, lon+lonDelta,
		radiusKm, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("find nearby communities: %w", err)
	}
	defer rows.Close()

	results := make([]models.CommunityWithDistance, 0, limit)
	for rows.Next() {
		var cwd models.CommunityWithDistance
		scanErr := rows.Scan(
			&cwd.ID, &cwd.Name, &cwd.Slug, &cwd.CommunityType, &cwd.Province, &cwd.Region,
			&cwd.InacID, &cwd.StatCanCSD, &cwd.Latitude, &cwd.Longitude,
			&cwd.Nation, &cwd.Treaty, &cwd.LanguageGroup, &cwd.ReserveName,
			&cwd.Population, &cwd.PopulationYear,
			&cwd.Website, &cwd.FeedURL, &cwd.DataSource, &cwd.SourceID,
			&cwd.Enabled, &cwd.CreatedAt, &cwd.UpdatedAt,
			&cwd.DistanceKm,
		)
		if scanErr != nil {
			return nil, fmt.Errorf("scan nearby community: %w", scanErr)
		}
		results = append(results, cwd)
	}

	if closeErr := rows.Err(); closeErr != nil {
		return nil, fmt.Errorf("find nearby rows: %w", closeErr)
	}

	return results, nil
}
