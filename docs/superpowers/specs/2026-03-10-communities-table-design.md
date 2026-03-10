# Communities Table Design (#264)

**Date**: 2026-03-10
**Service**: source-manager
**Issues**: #264 (communities table), prerequisite for #271 (people + band_offices)

## Context

Milestone 10 (People + Band Office Registry) requires a communities table as the authoritative registry for all Canadian communities — First Nations, cities, towns, and settlements. This table is consumed by Minoo, MCP tools, and all NC apps.

## Schema

Migration `009_create_communities_table`:

Up migration (`009_create_communities_table.up.sql`):

```sql
CREATE TABLE communities (
    id              VARCHAR(36)  PRIMARY KEY,
    name            VARCHAR(255) NOT NULL,
    slug            VARCHAR(255) NOT NULL,

    -- Classification
    community_type  VARCHAR(50)  NOT NULL,  -- first_nation, city, town, settlement, region
    province        VARCHAR(2),             -- ISO 3166-2:CA (ON, BC, AB, etc.)
    region          VARCHAR(100),

    -- Authoritative identifiers
    inac_id         VARCHAR(20),            -- CIRNAC Band Number (FN only)
    statcan_csd     VARCHAR(20),            -- StatsCan Census Subdivision code

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

Down migration (`009_create_communities_table.down.sql`):

```sql
DROP TRIGGER IF EXISTS set_communities_updated_at ON communities;
DROP TABLE IF EXISTS communities;
```

**UUID strategy**: generated in Go via `uuid.New().String()` (same as `SourceRepository.Create`), not via SQL default.

## Go Model

`internal/models/community.go` — struct with `db` and `json` tags following existing Source patterns. Nullable fields use pointers.

## Repository

`internal/repository/community.go`:

### Standard CRUD
- `Create(ctx, *Community) error`
- `GetByID(ctx, id) (*Community, error)`
- `GetBySlug(ctx, slug) (*Community, error)`
- `Update(ctx, *Community) error`
- `Delete(ctx, id) error`

### Filtered List
- `Count(ctx, CommunityFilter) (int, error)` — total matching count
- `ListPaginated(ctx, CommunityFilter) ([]Community, error)` — paginated results
- `CommunityFilter`: Type, Province, Search (ILIKE on name), Limit, Offset

### Bulk Upsert
- `UpsertByInacID(ctx, *Community) error` — ON CONFLICT (inac_id) DO UPDATE
- `UpsertByStatCanCSD(ctx, *Community) error` — ON CONFLICT (statcan_csd) DO UPDATE

### Spatial Query (FindNearby)
- `FindNearby(ctx, lat, lon, radiusKm, limit) ([]CommunityWithDistance, error)`
- **Bounding-box prefilter**: converts radiusKm to degree offsets (~111km/degree lat, adjusted for lon at latitude), filters with `WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ?`
- **Haversine formula**: computes accurate distance in a CTE
- **Result**: outer query filters `WHERE distance_km <= radiusKm ORDER BY distance_km LIMIT N`
- Returns `CommunityWithDistance` (embedded Community + `DistanceKm float64`)

This is the standard non-PostGIS spatial pattern (Airbnb, Uber, Yelp).

## Testing

Repository unit tests in `internal/repository/community_test.go`:
- CRUD: create, get by ID, get by slug, update, delete (happy path + not-found)
- List: filter by type, province, search, pagination, empty results
- Upsert: insert + update for both InacID and StatCanCSD
- FindNearby: within radius sorted by distance, excludes outside radius, null lat/lon handling
- All helpers use `t.Helper()`

## Decisions

- **SQL haversine with bounding-box prefilter** over PostGIS — no new dependencies, sufficient for ~1000 communities
- **String types for community_type/data_source** — no Go enums, validated at API layer (comes with #272)
- **No API endpoints in this PR** — schema + model + repository + tests only; endpoints come with #272
- **Reuse existing updated_at trigger** from migration 001
- **UUID generated in Go** (not SQL default) — matches SourceRepository pattern
- **Named constraints** for unique columns — matches sources table pattern
- **`enabled` not `status`** — consistent naming with sources table
- **`Count` + `ListPaginated`** split — matches existing SourceRepository pattern
- **CTE for FindNearby** — `HAVING` on non-aggregated columns is invalid in PostgreSQL

## Implementation Order

1. #264 — communities table (this design) → merge
2. #271 — people + band_offices tables → merge
3. #272–#275 — API endpoints, scraper, verification queue, MCP tools
