# Communities Table Implementation Plan (#264)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `communities` table to source-manager as the authoritative registry for all Canadian communities, with CRUD, filtered list, bulk upsert, and spatial (FindNearby) queries.

**Architecture:** New migration, model, and repository in source-manager following existing patterns (Source model/repository). No API endpoints in this PR — those come with #272. Bootstrap wiring deferred to #272 when handlers are added.

**Tech Stack:** Go 1.26, PostgreSQL, lib/pq, google/uuid, testify, infrastructure/logger

**Spec:** `docs/superpowers/specs/2026-03-10-communities-table-design.md`

---

## Chunk 1: Migration + Model

### Task 1: Create migration files

**Files:**
- Create: `source-manager/migrations/009_create_communities_table.up.sql`
- Create: `source-manager/migrations/009_create_communities_table.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
CREATE TABLE communities (
    id              VARCHAR(36)  PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL,

    -- Classification
    community_type  VARCHAR(50)  NOT NULL,
    province        VARCHAR(2),
    region          VARCHAR(100),

    -- Authoritative identifiers
    inac_id         VARCHAR(20),
    statcan_csd     VARCHAR(20),

    -- Geodata
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,

    -- Metadata
    nation          VARCHAR(255),
    treaty          VARCHAR(255),
    language_group  VARCHAR(255),
    reserve_name    VARCHAR(255),
    population      INTEGER,
    population_year INTEGER,

    -- Digital presence
    website         TEXT,
    feed_url        TEXT,

    -- Source attribution
    data_source     VARCHAR(50)  NOT NULL DEFAULT 'manual',
    source_id       VARCHAR(36)  REFERENCES sources(id),

    -- Lifecycle
    enabled         BOOLEAN      NOT NULL DEFAULT true,
    created_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Named constraints
    CONSTRAINT uq_communities_slug UNIQUE (slug),
    CONSTRAINT uq_communities_inac_id UNIQUE (inac_id),
    CONSTRAINT uq_communities_statcan_csd UNIQUE (statcan_csd)
);

CREATE INDEX idx_communities_type ON communities(community_type);
CREATE INDEX idx_communities_province ON communities(province);
CREATE INDEX idx_communities_inac_id ON communities(inac_id) WHERE inac_id IS NOT NULL;
CREATE INDEX idx_communities_statcan_csd ON communities(statcan_csd) WHERE statcan_csd IS NOT NULL;
CREATE INDEX idx_communities_coords ON communities(latitude, longitude) WHERE latitude IS NOT NULL;

CREATE TRIGGER set_communities_updated_at
    BEFORE UPDATE ON communities
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
```

- [ ] **Step 2: Write the down migration**

```sql
DROP TRIGGER IF EXISTS set_communities_updated_at ON communities;
DROP TABLE IF EXISTS communities;
```

- [ ] **Step 3: Commit**

```bash
git add source-manager/migrations/009_create_communities_table.up.sql source-manager/migrations/009_create_communities_table.down.sql
git commit -m "feat(source-manager): add communities table migration (#264)"
```

---

### Task 2: Create Community model

**Files:**
- Create: `source-manager/internal/models/community.go`

- [ ] **Step 1: Write the Community struct**

Follow the Source struct pattern: `db` tags for database columns, `json` tags for API, pointer types for nullable fields.

```go
package models

import "time"

// Community represents a Canadian community in the authoritative registry.
type Community struct {
	ID            string  `db:"id"              json:"id"`
	Name          string  `db:"name"            json:"name"`
	Slug          string  `db:"slug"            json:"slug"`
	CommunityType string  `db:"community_type"  json:"community_type"`
	Province      *string `db:"province"        json:"province,omitempty"`
	Region        *string `db:"region"          json:"region,omitempty"`

	// Authoritative identifiers
	InacID     *string `db:"inac_id"      json:"inac_id,omitempty"`
	StatCanCSD *string `db:"statcan_csd"  json:"statcan_csd,omitempty"`

	// Geodata
	Latitude  *float64 `db:"latitude"   json:"latitude,omitempty"`
	Longitude *float64 `db:"longitude"  json:"longitude,omitempty"`

	// Metadata
	Nation         *string `db:"nation"          json:"nation,omitempty"`
	Treaty         *string `db:"treaty"          json:"treaty,omitempty"`
	LanguageGroup  *string `db:"language_group"  json:"language_group,omitempty"`
	ReserveName    *string `db:"reserve_name"    json:"reserve_name,omitempty"`
	Population     *int    `db:"population"      json:"population,omitempty"`
	PopulationYear *int    `db:"population_year" json:"population_year,omitempty"`

	// Digital presence
	Website *string `db:"website"   json:"website,omitempty"`
	FeedURL *string `db:"feed_url"  json:"feed_url,omitempty"`

	// Source attribution
	DataSource string  `db:"data_source" json:"data_source"`
	SourceID   *string `db:"source_id"   json:"source_id,omitempty"`

	// Lifecycle
	Enabled   bool      `db:"enabled"    json:"enabled"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// CommunityFilter defines filters for listing communities.
type CommunityFilter struct {
	Type     string
	Province string
	Search   string
	Limit    int
	Offset   int
}

// CommunityWithDistance wraps Community with a computed distance field.
type CommunityWithDistance struct {
	Community
	DistanceKm float64 `db:"distance_km" json:"distance_km"`
}
```

- [ ] **Step 2: Commit**

```bash
git add source-manager/internal/models/community.go
git commit -m "feat(source-manager): add Community model (#264)"
```

---

## Chunk 2: Repository CRUD

### Task 3: Create CommunityRepository with constructor and scan helpers

**Files:**
- Create: `source-manager/internal/repository/community.go`

- [ ] **Step 1: Write constructor and scan helpers**

Follow the SourceRepository pattern: constructor takes `*sql.DB` and `infralogger.Logger`, scan helpers reduce duplication.

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
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

// communityColumns is the column list used in SELECT queries.
const communityColumns = `id, name, slug, community_type, province, region,
	inac_id, statcan_csd, latitude, longitude,
	nation, treaty, language_group, reserve_name, population, population_year,
	website, feed_url, data_source, source_id, enabled, created_at, updated_at`

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
```

- [ ] **Step 2: Commit**

```bash
git add source-manager/internal/repository/community.go
git commit -m "feat(source-manager): add CommunityRepository constructor and scan helpers (#264)"
```

---

### Task 4: Implement Create method with tests

**Files:**
- Modify: `source-manager/internal/repository/community.go`
- Create: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write the failing test**

```go
package repository_test

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	"github.com/jonesrussell/north-cloud/source-manager/internal/repository"
	"github.com/jonesrussell/north-cloud/source-manager/internal/testhelpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCommunityTestDB(t *testing.T) (repo *repository.CommunityRepository, cleanup func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db, dbCleanup := setupTestDB(t)
	logger := testhelpers.NewTestLogger()
	repo = repository.NewCommunityRepository(db, logger)

	cleanup = func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, "TRUNCATE TABLE communities CASCADE")
		dbCleanup()
	}

	return repo, cleanup
}

func newTestCommunity(name, slug, communityType string) *models.Community {
	return &models.Community{
		Name:          name,
		Slug:          slug,
		CommunityType: communityType,
		DataSource:    "manual",
		Enabled:       true,
	}
}

func TestCommunityRepository_Create(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("create valid community", func(t *testing.T) {
		c := newTestCommunity("Sudbury", "sudbury", "city")
		province := "ON"
		c.Province = &province

		err := repo.Create(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, c.ID)
		assert.False(t, c.CreatedAt.IsZero())
	})

	t.Run("duplicate slug returns error", func(t *testing.T) {
		c := newTestCommunity("Sudbury Duplicate", "sudbury", "city")
		err := repo.Create(ctx, c)
		assert.Error(t, err)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/repository/ -run TestCommunityRepository_Create -v`
Expected: FAIL — `Create` method not found

- [ ] **Step 3: Implement Create**

Add to `community.go`:

```go
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
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/repository/ -run TestCommunityRepository_Create -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/community.go source-manager/internal/repository/community_test.go
git commit -m "feat(source-manager): add CommunityRepository.Create with tests (#264)"
```

---

### Task 5: Implement GetByID and GetBySlug with tests

**Files:**
- Modify: `source-manager/internal/repository/community.go`
- Modify: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write failing tests**

Add to `community_test.go`:

```go
func TestCommunityRepository_GetByID(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		c := newTestCommunity("Get Test", "get-test", "city")
		require.NoError(t, repo.Create(ctx, c))

		found, err := repo.GetByID(ctx, c.ID)
		require.NoError(t, err)
		assert.Equal(t, c.ID, found.ID)
		assert.Equal(t, "Get Test", found.Name)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.GetByID(ctx, "nonexistent-id")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestCommunityRepository_GetBySlug(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	c := newTestCommunity("Slug Test", "slug-test", "town")
	require.NoError(t, repo.Create(ctx, c))

	t.Run("found", func(t *testing.T) {
		found, err := repo.GetBySlug(ctx, "slug-test")
		require.NoError(t, err)
		assert.Equal(t, c.ID, found.ID)
	})

	t.Run("not found", func(t *testing.T) {
		found, err := repo.GetBySlug(ctx, "no-such-slug")
		assert.NoError(t, err)
		assert.Nil(t, found)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd source-manager && go test ./internal/repository/ -run "TestCommunityRepository_GetBy" -v`
Expected: FAIL

- [ ] **Step 3: Implement GetByID and GetBySlug**

```go
// GetByID returns a community by ID, or nil if not found.
func (r *CommunityRepository) GetByID(ctx context.Context, id string) (*models.Community, error) {
	query := fmt.Sprintf("SELECT %s FROM communities WHERE id = $1", communityColumns)
	row := r.db.QueryRowContext(ctx, query, id)

	c, err := scanCommunity(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get community by id: %w", err)
	}

	return c, nil
}

// GetBySlug returns a community by slug, or nil if not found.
func (r *CommunityRepository) GetBySlug(ctx context.Context, slug string) (*models.Community, error) {
	query := fmt.Sprintf("SELECT %s FROM communities WHERE slug = $1", communityColumns)
	row := r.db.QueryRowContext(ctx, query, slug)

	c, err := scanCommunity(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get community by slug: %w", err)
	}

	return c, nil
}
```

Note: add `"errors"` to the import block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd source-manager && go test ./internal/repository/ -run "TestCommunityRepository_GetBy" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/community.go source-manager/internal/repository/community_test.go
git commit -m "feat(source-manager): add GetByID and GetBySlug (#264)"
```

---

### Task 6: Implement Update and Delete with tests

**Files:**
- Modify: `source-manager/internal/repository/community.go`
- Modify: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCommunityRepository_Update(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	c := newTestCommunity("Before Update", "before-update", "city")
	require.NoError(t, repo.Create(ctx, c))

	c.Name = "After Update"
	pop := 50000
	c.Population = &pop

	err := repo.Update(ctx, c)
	require.NoError(t, err)

	found, getErr := repo.GetByID(ctx, c.ID)
	require.NoError(t, getErr)
	assert.Equal(t, "After Update", found.Name)
	assert.Equal(t, 50000, *found.Population)
}

func TestCommunityRepository_Delete(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	c := newTestCommunity("Delete Me", "delete-me", "town")
	require.NoError(t, repo.Create(ctx, c))

	err := repo.Delete(ctx, c.ID)
	require.NoError(t, err)

	found, getErr := repo.GetByID(ctx, c.ID)
	require.NoError(t, getErr)
	assert.Nil(t, found)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd source-manager && go test ./internal/repository/ -run "TestCommunityRepository_(Update|Delete)" -v`
Expected: FAIL

- [ ] **Step 3: Implement Update and Delete**

```go
// Update modifies an existing community by ID. Updates all fields except id and created_at.
func (r *CommunityRepository) Update(ctx context.Context, c *models.Community) error {
	c.UpdatedAt = time.Now()

	query := `
		UPDATE communities SET
			name = $2, slug = $3, community_type = $4, province = $5, region = $6,
			inac_id = $7, statcan_csd = $8, latitude = $9, longitude = $10,
			nation = $11, treaty = $12, language_group = $13, reserve_name = $14,
			population = $15, population_year = $16,
			website = $17, feed_url = $18, data_source = $19, source_id = $20,
			enabled = $21, updated_at = $22
		WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query,
		c.ID, c.Name, c.Slug, c.CommunityType, c.Province, c.Region,
		c.InacID, c.StatCanCSD, c.Latitude, c.Longitude,
		c.Nation, c.Treaty, c.LanguageGroup, c.ReserveName, c.Population, c.PopulationYear,
		c.Website, c.FeedURL, c.DataSource, c.SourceID, c.Enabled, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("update community: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update community rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return fmt.Errorf("update community: not found")
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
		return fmt.Errorf("delete community: not found")
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd source-manager && go test ./internal/repository/ -run "TestCommunityRepository_(Update|Delete)" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/community.go source-manager/internal/repository/community_test.go
git commit -m "feat(source-manager): add Update and Delete (#264)"
```

---

## Chunk 3: List, Count, Upsert

### Task 7: Implement Count and ListPaginated with tests

**Files:**
- Modify: `source-manager/internal/repository/community.go`
- Modify: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCommunityRepository_CountAndList(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Seed data
	require.NoError(t, repo.Create(ctx, newTestCommunity("City A", "city-a", "city")))
	fnCommunity := newTestCommunity("FN B", "fn-b", "first_nation")
	province := "ON"
	fnCommunity.Province = &province
	require.NoError(t, repo.Create(ctx, fnCommunity))
	require.NoError(t, repo.Create(ctx, newTestCommunity("Town C", "town-c", "town")))

	t.Run("count all", func(t *testing.T) {
		count, err := repo.Count(ctx, models.CommunityFilter{})
		require.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("count by type", func(t *testing.T) {
		count, err := repo.Count(ctx, models.CommunityFilter{Type: "first_nation"})
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("list paginated", func(t *testing.T) {
		results, err := repo.ListPaginated(ctx, models.CommunityFilter{Limit: 2})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("list by province", func(t *testing.T) {
		results, err := repo.ListPaginated(ctx, models.CommunityFilter{Province: "ON"})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "FN B", results[0].Name)
	})

	t.Run("search by name", func(t *testing.T) {
		results, err := repo.ListPaginated(ctx, models.CommunityFilter{Search: "city"})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		assert.Equal(t, "City A", results[0].Name)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/repository/ -run TestCommunityRepository_CountAndList -v`
Expected: FAIL

- [ ] **Step 3: Implement Count, ListPaginated, and buildCommunityWhere helper**

```go
const (
	defaultCommunityLimit = 50
	maxCommunityLimit     = 200
)

// buildCommunityWhere constructs a WHERE clause and args from a CommunityFilter.
func buildCommunityWhere(filter models.CommunityFilter) (string, []any) {
	var conditions []string
	var args []any
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
		argIdx++
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
func (r *CommunityRepository) ListPaginated(ctx context.Context, filter models.CommunityFilter) ([]models.Community, error) {
	where, args := buildCommunityWhere(filter)
	argIdx := len(args) + 1

	limit := filter.Limit
	if limit <= 0 || limit > maxCommunityLimit {
		limit = defaultCommunityLimit
	}

	query := fmt.Sprintf("SELECT %s FROM communities%s ORDER BY name ASC LIMIT $%d OFFSET $%d",
		communityColumns, where, argIdx, argIdx+1)
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
```

Note: add `"strings"` to the import block.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/repository/ -run TestCommunityRepository_CountAndList -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/community.go source-manager/internal/repository/community_test.go
git commit -m "feat(source-manager): add Count and ListPaginated (#264)"
```

---

### Task 8: Implement UpsertByInacID and UpsertByStatCanCSD with tests

**Files:**
- Modify: `source-manager/internal/repository/community.go`
- Modify: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCommunityRepository_UpsertByInacID(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	inacID := "123"

	t.Run("insert when new", func(t *testing.T) {
		c := newTestCommunity("FN One", "fn-one", "first_nation")
		c.InacID = &inacID
		c.DataSource = "cirnac"

		err := repo.UpsertByInacID(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, c.ID)
	})

	t.Run("update when exists", func(t *testing.T) {
		c := newTestCommunity("FN One Updated", "fn-one", "first_nation")
		c.InacID = &inacID
		c.DataSource = "cirnac"

		err := repo.UpsertByInacID(ctx, c)
		require.NoError(t, err)

		found, getErr := repo.GetBySlug(ctx, "fn-one")
		require.NoError(t, getErr)
		assert.Equal(t, "FN One Updated", found.Name)
	})
}

func TestCommunityRepository_UpsertByStatCanCSD(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()
	csd := "3553005"

	t.Run("insert when new", func(t *testing.T) {
		c := newTestCommunity("Sudbury CSD", "sudbury-csd", "city")
		c.StatCanCSD = &csd
		c.DataSource = "statscan"

		err := repo.UpsertByStatCanCSD(ctx, c)
		require.NoError(t, err)
		assert.NotEmpty(t, c.ID)
	})

	t.Run("update when exists", func(t *testing.T) {
		pop := 165000
		c := newTestCommunity("Greater Sudbury", "sudbury-csd", "city")
		c.StatCanCSD = &csd
		c.Population = &pop
		c.DataSource = "statscan"

		err := repo.UpsertByStatCanCSD(ctx, c)
		require.NoError(t, err)

		found, getErr := repo.GetBySlug(ctx, "sudbury-csd")
		require.NoError(t, getErr)
		assert.Equal(t, "Greater Sudbury", found.Name)
		assert.Equal(t, 165000, *found.Population)
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd source-manager && go test ./internal/repository/ -run "TestCommunityRepository_Upsert" -v`
Expected: FAIL

- [ ] **Step 3: Implement upsert methods**

```go
// upsertCommunity is a shared helper for upsert operations.
func (r *CommunityRepository) upsertCommunity(ctx context.Context, c *models.Community, conflictColumn string) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	c.CreatedAt = time.Now()
	c.UpdatedAt = time.Now()

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
			community_type = EXCLUDED.community_type, province = EXCLUDED.province, region = EXCLUDED.region,
			latitude = EXCLUDED.latitude, longitude = EXCLUDED.longitude,
			nation = EXCLUDED.nation, treaty = EXCLUDED.treaty, language_group = EXCLUDED.language_group,
			reserve_name = EXCLUDED.reserve_name, population = EXCLUDED.population,
			population_year = EXCLUDED.population_year,
			website = EXCLUDED.website, feed_url = EXCLUDED.feed_url,
			data_source = EXCLUDED.data_source, source_id = EXCLUDED.source_id,
			enabled = EXCLUDED.enabled, updated_at = EXCLUDED.updated_at
		RETURNING id`, conflictColumn)

	return r.db.QueryRowContext(ctx, query,
		c.ID, c.Name, c.Slug, c.CommunityType, c.Province, c.Region,
		c.InacID, c.StatCanCSD, c.Latitude, c.Longitude,
		c.Nation, c.Treaty, c.LanguageGroup, c.ReserveName, c.Population, c.PopulationYear,
		c.Website, c.FeedURL, c.DataSource, c.SourceID, c.Enabled, c.CreatedAt, c.UpdatedAt,
	).Scan(&c.ID)
}

// UpsertByInacID inserts or updates a community by CIRNAC band number.
func (r *CommunityRepository) UpsertByInacID(ctx context.Context, c *models.Community) error {
	if c.InacID == nil {
		return fmt.Errorf("upsert by inac_id: inac_id is required")
	}
	return r.upsertCommunity(ctx, c, "inac_id")
}

// UpsertByStatCanCSD inserts or updates a community by StatsCan CSD code.
func (r *CommunityRepository) UpsertByStatCanCSD(ctx context.Context, c *models.Community) error {
	if c.StatCanCSD == nil {
		return fmt.Errorf("upsert by statcan_csd: statcan_csd is required")
	}
	return r.upsertCommunity(ctx, c, "statcan_csd")
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd source-manager && go test ./internal/repository/ -run "TestCommunityRepository_Upsert" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/community.go source-manager/internal/repository/community_test.go
git commit -m "feat(source-manager): add UpsertByInacID and UpsertByStatCanCSD (#264)"
```

---

## Chunk 4: Spatial Query (FindNearby)

### Task 9: Implement FindNearby with tests

**Files:**
- Modify: `source-manager/internal/repository/community.go`
- Modify: `source-manager/internal/repository/community_test.go`

- [ ] **Step 1: Write failing tests**

```go
func TestCommunityRepository_FindNearby(t *testing.T) {
	repo, cleanup := setupCommunityTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Seed: Sudbury (46.49, -81.00), Timmins (48.47, -81.33), Toronto (43.65, -79.38)
	sudbury := newTestCommunity("Sudbury", "sudbury", "city")
	sudLat, sudLon := 46.49, -81.00
	sudbury.Latitude = &sudLat
	sudbury.Longitude = &sudLon
	require.NoError(t, repo.Create(ctx, sudbury))

	timmins := newTestCommunity("Timmins", "timmins", "city")
	timLat, timLon := 48.47, -81.33
	timmins.Latitude = &timLat
	timmins.Longitude = &timLon
	require.NoError(t, repo.Create(ctx, timmins))

	toronto := newTestCommunity("Toronto", "toronto", "city")
	torLat, torLon := 43.65, -79.38
	toronto.Latitude = &torLat
	toronto.Longitude = &torLon
	require.NoError(t, repo.Create(ctx, toronto))

	// No coords — should be excluded
	noCoords := newTestCommunity("No Coords", "no-coords", "town")
	require.NoError(t, repo.Create(ctx, noCoords))

	t.Run("finds within radius sorted by distance", func(t *testing.T) {
		// Search from Sudbury with 250km radius — should find Sudbury and Timmins, not Toronto
		results, err := repo.FindNearby(ctx, 46.49, -81.00, 250, 10)
		require.NoError(t, err)
		assert.Len(t, results, 2)
		assert.Equal(t, "Sudbury", results[0].Name)
		assert.Equal(t, "Timmins", results[1].Name)
		assert.Less(t, results[0].DistanceKm, results[1].DistanceKm)
	})

	t.Run("respects limit", func(t *testing.T) {
		results, err := repo.FindNearby(ctx, 46.49, -81.00, 500, 1)
		require.NoError(t, err)
		assert.Len(t, results, 1)
	})

	t.Run("empty when nothing nearby", func(t *testing.T) {
		// Middle of Atlantic Ocean
		results, err := repo.FindNearby(ctx, 40.00, -40.00, 100, 10)
		require.NoError(t, err)
		assert.Empty(t, results)
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd source-manager && go test ./internal/repository/ -run TestCommunityRepository_FindNearby -v`
Expected: FAIL

- [ ] **Step 3: Implement FindNearby with bounding-box prefilter + haversine CTE**

```go
const (
	// earthRadiusKm is the mean radius of the Earth in kilometers.
	earthRadiusKm = 6371.0
	// kmPerDegreeLat is the approximate km per degree of latitude.
	kmPerDegreeLat = 111.0
)

// FindNearby returns communities within radiusKm of the given coordinates,
// sorted by distance ascending. Uses bounding-box prefilter + haversine CTE.
func (r *CommunityRepository) FindNearby(
	ctx context.Context, lat, lon, radiusKm float64, limit int,
) ([]models.CommunityWithDistance, error) {
	// Bounding-box prefilter: convert radius to degree offsets
	latDelta := radiusKm / kmPerDegreeLat
	lonDelta := radiusKm / (kmPerDegreeLat * math.Cos(lat*math.Pi/180))

	query := fmt.Sprintf(`
		WITH nearby AS (
			SELECT %s,
				(%f * acos(
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
		SELECT %s, distance_km FROM nearby
		WHERE distance_km <= $7
		ORDER BY distance_km ASC
		LIMIT $8`, communityColumns, earthRadiusKm, communityColumns)

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
```

Note: add `"math"` to the import block.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd source-manager && go test ./internal/repository/ -run TestCommunityRepository_FindNearby -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/repository/community.go source-manager/internal/repository/community_test.go
git commit -m "feat(source-manager): add FindNearby with haversine + bounding-box (#264)"
```

---

## Chunk 5: Update test helpers + lint + final verification

### Task 10: Update test helpers to run all migrations

**Files:**
- Modify: `source-manager/internal/testhelpers/database.go`

- [ ] **Step 1: Update RunMigrations to run all .up.sql files in order**

The current `RunMigrations` only runs migration 001. Update to glob and sort all `.up.sql` files:

```go
package testhelpers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	_ "github.com/lib/pq" //nolint:blankimports // PostgreSQL driver
)

// RunMigrations executes all SQL migration files on a database connection.
func RunMigrations(ctx context.Context, db *sql.DB, log infralogger.Logger) error {
	_, filename, _, _ := runtime.Caller(0)
	migrationsPath := filepath.Join(filepath.Dir(filename), "..", "..", "migrations")

	files, globErr := filepath.Glob(filepath.Join(migrationsPath, "*.up.sql"))
	if globErr != nil {
		return fmt.Errorf("glob migrations: %w", globErr)
	}

	sort.Strings(files)

	for _, f := range files {
		sqlBytes, readErr := os.ReadFile(f)
		if readErr != nil {
			return fmt.Errorf("read migration %s: %w", filepath.Base(f), readErr)
		}

		if _, execErr := db.ExecContext(ctx, string(sqlBytes)); execErr != nil {
			return fmt.Errorf("execute migration %s: %w", filepath.Base(f), execErr)
		}
	}

	if log != nil {
		log.Info("Migrations applied successfully",
			infralogger.Int("count", len(files)),
		)
	}

	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add source-manager/internal/testhelpers/database.go
git commit -m "fix(source-manager): run all migrations in test helper (#264)"
```

---

### Task 11: Run linter and full test suite

**Files:** None (verification only)

- [ ] **Step 1: Run linter**

Run: `cd source-manager && golangci-lint run ./...`
Expected: No errors. If there are lint violations, fix them before proceeding.

- [ ] **Step 2: Run full test suite**

Run: `cd source-manager && go test ./... -v`
Expected: All tests pass

- [ ] **Step 3: Fix any issues found, commit if needed**

```bash
git add -A source-manager/
git commit -m "fix(source-manager): address lint issues (#264)"
```

- [ ] **Step 4: Final commit — squash or verify clean history**

Verify: `git log --oneline` shows a clean series of feat commits for #264.
