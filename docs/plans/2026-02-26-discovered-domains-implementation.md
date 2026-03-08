# Discovered Domains Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a domain-centric source discovery pipeline that replaces the flat discovered links page with grouped domain aggregation, quality scoring, and operator lifecycle management.

**Architecture:** Extend the crawler's `discovered_links` table with `domain`, `http_status`, `content_type` columns. Add a `discovered_domain_states` table for operator state. New backend API endpoints aggregate by domain with quality scoring. New Vue dashboard views replace the current flat list. Source-manager integration checks existing sources.

**Tech Stack:** Go 1.26+ (crawler service), PostgreSQL (migrations), Vue 3 + TypeScript (dashboard), TanStack Vue Query, Tailwind CSS v4, Gin HTTP framework.

**Design Doc:** `docs/plans/2026-02-26-discovered-domains-design.md`

---

## Task 1: Database Migration — Extend discovered_links + Create domain states table

**Files:**
- Create: `crawler/migrations/020_discovered_domains.up.sql`
- Create: `crawler/migrations/020_discovered_domains.down.sql`

**Step 1: Write the up migration**

```sql
-- 020_discovered_domains.up.sql

-- Add domain, http_status, content_type to discovered_links
ALTER TABLE discovered_links
  ADD COLUMN domain VARCHAR(255),
  ADD COLUMN http_status SMALLINT,
  ADD COLUMN content_type VARCHAR(100);

-- Backfill domain from existing URLs (extract host, strip www.)
UPDATE discovered_links
SET domain = regexp_replace(substring(url FROM '://([^/]+)'), '^www\.', '');

-- Make domain NOT NULL after backfill
ALTER TABLE discovered_links
  ALTER COLUMN domain SET NOT NULL;

-- Index for GROUP BY domain aggregation
CREATE INDEX idx_discovered_links_domain ON discovered_links(domain);

-- Composite index for domain + status filtering
CREATE INDEX idx_discovered_links_domain_status ON discovered_links(domain, status);

-- Domain operator state table
CREATE TABLE discovered_domain_states (
    domain VARCHAR(255) PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    notes TEXT,
    ignored_at TIMESTAMP WITH TIME ZONE,
    ignored_by VARCHAR(100),
    promoted_at TIMESTAMP WITH TIME ZONE,
    promoted_source_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_discovered_domain_states_updated_at
    BEFORE UPDATE ON discovered_domain_states
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

**Step 2: Write the down migration**

```sql
-- 020_discovered_domains.down.sql

DROP TRIGGER IF EXISTS update_discovered_domain_states_updated_at ON discovered_domain_states;
DROP TABLE IF EXISTS discovered_domain_states;

DROP INDEX IF EXISTS idx_discovered_links_domain_status;
DROP INDEX IF EXISTS idx_discovered_links_domain;

ALTER TABLE discovered_links
  DROP COLUMN IF EXISTS content_type,
  DROP COLUMN IF EXISTS http_status,
  DROP COLUMN IF EXISTS domain;
```

**Step 3: Run the migration locally**

Run: `cd crawler && go run cmd/migrate/main.go up`
Expected: Migration 020 applied successfully.

**Step 4: Verify schema**

Run: `docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler -c "\d discovered_links"` and `docker exec -it north-cloud-postgres-crawler psql -U postgres -d crawler -c "\d discovered_domain_states"`
Expected: Both tables show the new columns/structure.

**Step 5: Commit**

```
feat(crawler): add discovered domains migration 020

Extends discovered_links with domain, http_status, content_type columns.
Creates discovered_domain_states table for operator lifecycle management.
```

---

## Task 2: Domain Model — Add domain fields to DiscoveredLink + new DomainState model

**Files:**
- Modify: `crawler/internal/domain/discovered_link.go`
- Create: `crawler/internal/domain/discovered_domain.go`

**Step 1: Extend DiscoveredLink struct**

Add three fields to the existing struct in `crawler/internal/domain/discovered_link.go`:

```go
type DiscoveredLink struct {
	ID           string    `db:"id"            json:"id"`
	SourceID     string    `db:"source_id"     json:"source_id"`
	SourceName   string    `db:"source_name"   json:"source_name"`
	URL          string    `db:"url"           json:"url"`
	ParentURL    *string   `db:"parent_url"    json:"parent_url,omitempty"`
	Depth        int       `db:"depth"         json:"depth"`
	Domain       string    `db:"domain"        json:"domain"`
	HTTPStatus   *int16    `db:"http_status"   json:"http_status,omitempty"`
	ContentType  *string   `db:"content_type"  json:"content_type,omitempty"`
	DiscoveredAt time.Time `db:"discovered_at" json:"discovered_at"`
	QueuedAt     time.Time `db:"queued_at"     json:"queued_at"`
	Status       string    `db:"status"        json:"status"`
	Priority     int       `db:"priority"      json:"priority"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}
```

**Step 2: Create DomainAggregate and DomainState models**

Create `crawler/internal/domain/discovered_domain.go`:

```go
package domain

import (
	"net/url"
	"strings"
	"time"
)

// DomainState represents operator state for a discovered domain.
type DomainState struct {
	Domain           string    `db:"domain"             json:"domain"`
	Status           string    `db:"status"             json:"status"`
	Notes            *string   `db:"notes"              json:"notes,omitempty"`
	IgnoredAt        *time.Time `db:"ignored_at"        json:"ignored_at,omitempty"`
	IgnoredBy        *string   `db:"ignored_by"         json:"ignored_by,omitempty"`
	PromotedAt       *time.Time `db:"promoted_at"       json:"promoted_at,omitempty"`
	PromotedSourceID *string   `db:"promoted_source_id" json:"promoted_source_id,omitempty"`
	CreatedAt        time.Time `db:"created_at"         json:"created_at"`
	UpdatedAt        time.Time `db:"updated_at"         json:"updated_at"`
}

// DomainAggregate holds aggregated stats for a discovered domain.
// Populated by the repository aggregation query, quality score computed in Go.
type DomainAggregate struct {
	Domain     string  `db:"domain"       json:"domain"`
	Status     string  `db:"status"       json:"status"`
	LinkCount  int     `db:"link_count"   json:"link_count"`
	SourceCount int    `db:"source_count" json:"source_count"`
	AvgDepth   float64 `db:"avg_depth"    json:"avg_depth"`
	FirstSeen  time.Time `db:"first_seen" json:"first_seen"`
	LastSeen   time.Time `db:"last_seen"  json:"last_seen"`
	OKRatio    *float64 `db:"ok_ratio"    json:"ok_ratio"`
	HTMLRatio  *float64 `db:"html_ratio"  json:"html_ratio"`
	Notes      *string  `db:"notes"       json:"notes,omitempty"`

	// Computed in Go (not from DB)
	QualityScore     int      `json:"quality_score"`
	ReferringSources []string `json:"referring_sources"`
	IsExistingSource bool     `json:"is_existing_source"`
}

// PathCluster represents a group of URLs sharing a common path prefix.
type PathCluster struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
}

// Domain status constants.
const (
	DomainStatusActive    = "active"
	DomainStatusIgnored   = "ignored"
	DomainStatusReviewing = "reviewing"
	DomainStatusPromoted  = "promoted"
)

// Quality score weight constants.
const (
	qualityWeightOK       = 30
	qualityWeightHTML     = 30
	qualityWeightSources  = 20
	qualityWeightRecency  = 20
	sourceCountCap        = 5
	recencyDecayDays      = 30
	maxQualityScore       = 100
)

// ComputeQualityScore calculates the quality score for a domain aggregate.
func (d *DomainAggregate) ComputeQualityScore() {
	score := 0.0

	if d.OKRatio != nil {
		score += *d.OKRatio * float64(qualityWeightOK)
	}
	if d.HTMLRatio != nil {
		score += *d.HTMLRatio * float64(qualityWeightHTML)
	}

	// Source count normalized (cap at 5)
	sourceNorm := float64(d.SourceCount) / float64(sourceCountCap)
	if sourceNorm > 1.0 {
		sourceNorm = 1.0
	}
	score += sourceNorm * float64(qualityWeightSources)

	// Recency normalized (decays over 30 days)
	daysSince := time.Since(d.LastSeen).Hours() / 24 //nolint:mnd // 24 hours per day
	recencyNorm := 1.0 - daysSince/float64(recencyDecayDays)
	if recencyNorm < 0 {
		recencyNorm = 0
	}
	score += recencyNorm * float64(qualityWeightRecency)

	d.QualityScore = int(score)
	if d.QualityScore > maxQualityScore {
		d.QualityScore = maxQualityScore
	}
}

// ExtractDomain extracts a normalized domain from a URL, stripping www. prefix.
func ExtractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	hostname := u.Hostname()
	return strings.TrimPrefix(hostname, "www.")
}
```

**Step 3: Run tests**

Run: `cd crawler && go build ./...`
Expected: Build succeeds with no errors.

**Step 4: Commit**

```
feat(crawler): add discovered domain models and quality score computation
```

---

## Task 3: Domain State Repository

**Files:**
- Create: `crawler/internal/database/domain_state_repository.go`
- Create: `crawler/internal/database/domain_state_repository_test.go`

**Step 1: Write the repository test**

Create `crawler/internal/database/domain_state_repository_test.go`:

```go
package database_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestDomainStateUpsertFields(t *testing.T) {
	t.Helper()
	// Verify domain state model has required fields
	state := domain.DomainState{
		Domain: "example.com",
		Status: domain.DomainStatusIgnored,
	}
	if state.Domain != "example.com" {
		t.Errorf("expected domain example.com, got %s", state.Domain)
	}
	if state.Status != "ignored" {
		t.Errorf("expected status ignored, got %s", state.Status)
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd crawler && go test ./internal/database/ -run TestDomainState -v`
Expected: PASS

**Step 3: Write the repository**

Create `crawler/internal/database/domain_state_repository.go`:

```go
package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// DomainStateRepository handles database operations for discovered domain states.
type DomainStateRepository struct {
	db *sqlx.DB
}

// NewDomainStateRepository creates a new domain state repository.
func NewDomainStateRepository(db *sqlx.DB) *DomainStateRepository {
	return &DomainStateRepository{db: db}
}

// Upsert creates or updates a domain state record.
func (r *DomainStateRepository) Upsert(ctx context.Context, domainName, status string, notes *string) error {
	now := time.Now()

	query := `
		INSERT INTO discovered_domain_states (domain, status, notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (domain)
		DO UPDATE SET
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			updated_at = NOW()
	`

	_, err := r.db.ExecContext(ctx, query, domainName, status, notes, now)
	if err != nil {
		return fmt.Errorf("upsert domain state: %w", err)
	}

	// Set timestamp fields based on status
	if status == domain.DomainStatusIgnored {
		_, err = r.db.ExecContext(ctx,
			`UPDATE discovered_domain_states SET ignored_at = NOW() WHERE domain = $1`,
			domainName)
		if err != nil {
			return fmt.Errorf("set ignored_at: %w", err)
		}
	} else if status == domain.DomainStatusPromoted {
		_, err = r.db.ExecContext(ctx,
			`UPDATE discovered_domain_states SET promoted_at = NOW() WHERE domain = $1`,
			domainName)
		if err != nil {
			return fmt.Errorf("set promoted_at: %w", err)
		}
	}

	return nil
}

// BulkUpsert updates states for multiple domains in a single transaction.
func (r *DomainStateRepository) BulkUpsert(ctx context.Context, domains []string, status string, notes *string) (int, error) {
	if len(domains) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Build batch insert with ON CONFLICT
	placeholders := make([]string, 0, len(domains))
	args := make([]any, 0, len(domains)*3) //nolint:mnd // 3 args per domain
	argIdx := 1
	for _, d := range domains {
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, NOW(), NOW())", argIdx, argIdx+1, argIdx+2))
		args = append(args, d, status, notes)
		argIdx += 3 //nolint:mnd // 3 args per domain
	}

	query := fmt.Sprintf(`
		INSERT INTO discovered_domain_states (domain, status, notes, created_at, updated_at)
		VALUES %s
		ON CONFLICT (domain)
		DO UPDATE SET
			status = EXCLUDED.status,
			notes = EXCLUDED.notes,
			updated_at = NOW()
	`, strings.Join(placeholders, ", "))

	result, execErr := tx.ExecContext(ctx, query, args...)
	if execErr != nil {
		return 0, fmt.Errorf("bulk upsert: %w", execErr)
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return 0, fmt.Errorf("commit: %w", commitErr)
	}

	affected, _ := result.RowsAffected()
	return int(affected), nil
}

// GetByDomain returns a domain state by domain name, or nil if not found.
func (r *DomainStateRepository) GetByDomain(ctx context.Context, domainName string) (*domain.DomainState, error) {
	var state domain.DomainState
	query := `SELECT * FROM discovered_domain_states WHERE domain = $1`

	err := r.db.GetContext(ctx, &state, query, domainName)
	if err != nil {
		return nil, nil // Not found is not an error — default state is "active"
	}

	return &state, nil
}
```

**Step 4: Run build**

Run: `cd crawler && go build ./...`
Expected: Build succeeds.

**Step 5: Commit**

```
feat(crawler): add domain state repository with upsert and bulk operations
```

---

## Task 4: Domain Aggregate Repository

**Files:**
- Create: `crawler/internal/database/domain_aggregate_repository.go`

**Step 1: Write the repository**

Create `crawler/internal/database/domain_aggregate_repository.go`:

```go
package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

const (
	defaultDomainLimit     = 25
	defaultDomainSortField = "link_count"
)

// DomainAggregateRepository handles domain-level aggregate queries.
type DomainAggregateRepository struct {
	db *sqlx.DB
}

// NewDomainAggregateRepository creates a new domain aggregate repository.
func NewDomainAggregateRepository(db *sqlx.DB) *DomainAggregateRepository {
	return &DomainAggregateRepository{db: db}
}

// DomainListFilters represents filtering options for listing domains.
type DomainListFilters struct {
	Status   string // Filter by domain state
	Search   string // ILIKE on domain name
	MinScore int    // Post-query filter (applied in Go)
	SortBy   string // link_count, last_seen, source_count, domain
	SortOrder string // asc, desc
	Limit    int
	Offset   int
}

// ListAggregates returns domain-level aggregates with optional filtering.
func (r *DomainAggregateRepository) ListAggregates(
	ctx context.Context,
	filters DomainListFilters,
) ([]*domain.DomainAggregate, error) {
	whereClauses, havingClauses, args, argIdx := buildDomainWhereClauses(filters)

	whereStr := ""
	if len(whereClauses) > 0 {
		whereStr = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	havingStr := ""
	if len(havingClauses) > 0 {
		havingStr = "HAVING " + strings.Join(havingClauses, " AND ")
	}

	sortBy, sortOrder := normalizeDomainSort(filters.SortBy, filters.SortOrder)

	limit := filters.Limit
	if limit <= 0 {
		limit = defaultDomainLimit
	}
	offset := filters.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT
			dl.domain,
			COALESCE(ds.status, 'active') AS status,
			COUNT(*) AS link_count,
			COUNT(DISTINCT dl.source_id) AS source_count,
			AVG(dl.depth)::float8 AS avg_depth,
			MIN(dl.discovered_at) AS first_seen,
			MAX(dl.discovered_at) AS last_seen,
			CASE WHEN COUNT(dl.http_status) > 0
				THEN COUNT(CASE WHEN dl.http_status BETWEEN 200 AND 299 THEN 1 END)::float8
					/ COUNT(dl.http_status)::float8
				ELSE NULL
			END AS ok_ratio,
			CASE WHEN COUNT(dl.content_type) > 0
				THEN COUNT(CASE WHEN dl.content_type LIKE 'text/html%%' THEN 1 END)::float8
					/ COUNT(dl.content_type)::float8
				ELSE NULL
			END AS html_ratio,
			ds.notes
		FROM discovered_links dl
		LEFT JOIN discovered_domain_states ds ON dl.domain = ds.domain
		%s
		GROUP BY dl.domain, ds.status, ds.notes
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, whereStr, havingStr, sortBy, sortOrder, argIdx, argIdx+1)

	args = append(args, limit, offset)

	var results []*domain.DomainAggregate
	if err := r.db.SelectContext(ctx, &results, query, args...); err != nil {
		return nil, fmt.Errorf("list domain aggregates: %w", err)
	}

	if results == nil {
		results = []*domain.DomainAggregate{}
	}

	return results, nil
}

// CountAggregates returns the total number of distinct domains matching the filters.
func (r *DomainAggregateRepository) CountAggregates(
	ctx context.Context,
	filters DomainListFilters,
) (int, error) {
	whereClauses, havingClauses, args, _ := buildDomainWhereClauses(filters)

	whereStr := ""
	if len(whereClauses) > 0 {
		whereStr = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	havingStr := ""
	if len(havingClauses) > 0 {
		havingStr = "HAVING " + strings.Join(havingClauses, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) FROM (
			SELECT dl.domain
			FROM discovered_links dl
			LEFT JOIN discovered_domain_states ds ON dl.domain = ds.domain
			%s
			GROUP BY dl.domain, ds.status
			%s
		) sub
	`, whereStr, havingStr)

	var count int
	if err := r.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, fmt.Errorf("count domain aggregates: %w", err)
	}

	return count, nil
}

// GetReferringSources returns distinct source names for a given domain.
func (r *DomainAggregateRepository) GetReferringSources(
	ctx context.Context,
	domainName string,
) ([]string, error) {
	var sources []string
	query := `SELECT DISTINCT source_name FROM discovered_links WHERE domain = $1 ORDER BY source_name`

	if err := r.db.SelectContext(ctx, &sources, query, domainName); err != nil {
		return nil, fmt.Errorf("get referring sources: %w", err)
	}

	if sources == nil {
		sources = []string{}
	}

	return sources, nil
}

// ListLinksByDomain returns paginated links for a specific domain.
func (r *DomainAggregateRepository) ListLinksByDomain(
	ctx context.Context,
	domainName string,
	limit, offset int,
) ([]*domain.DiscoveredLink, int, error) {
	if limit <= 0 {
		limit = defaultDomainLimit
	}

	var links []*domain.DiscoveredLink
	query := `
		SELECT id, source_id, source_name, url, parent_url, depth, domain,
		       http_status, content_type, discovered_at, queued_at, status,
		       priority, created_at, updated_at
		FROM discovered_links
		WHERE domain = $1
		ORDER BY discovered_at DESC
		LIMIT $2 OFFSET $3
	`

	if err := r.db.SelectContext(ctx, &links, query, domainName, limit, offset); err != nil {
		return nil, 0, fmt.Errorf("list links by domain: %w", err)
	}

	if links == nil {
		links = []*domain.DiscoveredLink{}
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM discovered_links WHERE domain = $1`
	if err := r.db.GetContext(ctx, &total, countQuery, domainName); err != nil {
		return nil, 0, fmt.Errorf("count links by domain: %w", err)
	}

	return links, total, nil
}

// buildDomainWhereClauses builds WHERE and HAVING clauses for domain queries.
func buildDomainWhereClauses(filters DomainListFilters) (where, having []string, args []any, nextArgIdx int) {
	argIdx := 1

	if filters.Search != "" {
		where = append(where, fmt.Sprintf("dl.domain ILIKE $%d", argIdx))
		args = append(args, "%"+filters.Search+"%")
		argIdx++
	}

	if filters.Status != "" {
		if filters.Status == "active" {
			// Active means no row in domain_states or status = 'active'
			having = append(having, fmt.Sprintf("COALESCE(ds.status, 'active') = $%d", argIdx))
		} else {
			having = append(having, fmt.Sprintf("ds.status = $%d", argIdx))
		}
		args = append(args, filters.Status)
		argIdx++
	}

	return where, having, args, argIdx
}

// normalizeDomainSort validates and normalizes sort parameters.
func normalizeDomainSort(sortBy, sortOrder string) (string, string) {
	allowedSorts := map[string]string{
		"link_count":   "link_count",
		"source_count": "source_count",
		"last_seen":    "last_seen",
		"domain":       "dl.domain",
	}

	column, ok := allowedSorts[sortBy]
	if !ok {
		column = allowedSorts[defaultDomainSortField]
	}

	order := strings.ToUpper(sortOrder)
	if order != "ASC" && order != "DESC" {
		order = "DESC"
	}

	return column, order
}
```

**Step 2: Run build**

Run: `cd crawler && go build ./...`
Expected: Build succeeds.

**Step 3: Commit**

```
feat(crawler): add domain aggregate repository with SQL grouping and filtering
```

---

## Task 5: Wire Repositories into Bootstrap

**Files:**
- Modify: `crawler/internal/bootstrap/database.go`

**Step 1: Add new repos to DatabaseComponents**

Add `DomainStateRepo` and `DomainAggregateRepo` to the `DatabaseComponents` struct in `crawler/internal/bootstrap/database.go`:

```go
type DatabaseComponents struct {
	DB                    *sqlx.DB
	JobRepo               *database.JobRepository
	ExecutionRepo         *database.ExecutionRepository
	DiscoveredLinkRepo    *database.DiscoveredLinkRepository
	ProcessedEventsRepo   *database.ProcessedEventsRepository
	FrontierRepo          *database.FrontierRepository
	HostStateRepo         *database.HostStateRepository
	FeedStateRepo         *database.FeedStateRepository
	CandidateRepo         *database.CandidateRepository
	DecisionLogRepo       *database.DecisionLogRepository
	DomainStateRepo       *database.DomainStateRepository
	DomainAggregateRepo   *database.DomainAggregateRepository
}
```

**Step 2: Initialize repos in SetupDatabase**

Add after the existing `setupDiscoveryRepositories` call:

```go
domainStateRepo, domainAggregateRepo := setupDomainRepositories(db)
```

And add to the return struct:
```go
DomainStateRepo:     domainStateRepo,
DomainAggregateRepo: domainAggregateRepo,
```

**Step 3: Add the setup function**

```go
func setupDomainRepositories(db *sqlx.DB) (
	domainStateRepo *database.DomainStateRepository,
	domainAggregateRepo *database.DomainAggregateRepository,
) {
	return database.NewDomainStateRepository(db), database.NewDomainAggregateRepository(db)
}
```

**Step 4: Run build**

Run: `cd crawler && go build ./...`
Expected: Build succeeds.

**Step 5: Commit**

```
feat(crawler): wire domain repositories into bootstrap
```

---

## Task 6: Discovered Domains HTTP Handler

**Files:**
- Create: `crawler/internal/api/discovered_domains_handler.go`

**Step 1: Write the handler**

Create `crawler/internal/api/discovered_domains_handler.go`:

```go
package api

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	defaultDomainsLimit  = 25
	defaultDomainsOffset = 0
	maxBulkDomains       = 100
)

// DiscoveredDomainsHandler handles discovered domain HTTP requests.
type DiscoveredDomainsHandler struct {
	aggregateRepo *database.DomainAggregateRepository
	stateRepo     *database.DomainStateRepository
	sourcesURL    string // Source-manager base URL for existing source check
	log           infralogger.Logger
}

// NewDiscoveredDomainsHandler creates a new discovered domains handler.
func NewDiscoveredDomainsHandler(
	aggregateRepo *database.DomainAggregateRepository,
	stateRepo *database.DomainStateRepository,
	sourcesURL string,
	log infralogger.Logger,
) *DiscoveredDomainsHandler {
	return &DiscoveredDomainsHandler{
		aggregateRepo: aggregateRepo,
		stateRepo:     stateRepo,
		sourcesURL:    sourcesURL,
		log:           log,
	}
}

// ListDomains handles GET /api/v1/discovered-domains
func (h *DiscoveredDomainsHandler) ListDomains(c *gin.Context) {
	limit, offset := parseLimitOffset(c, defaultDomainsLimit, defaultDomainsOffset)

	filters := database.DomainListFilters{
		Status:    c.Query("status"),
		Search:    c.Query("search"),
		SortBy:    c.DefaultQuery("sort", "link_count"),
		SortOrder: c.DefaultQuery("order", "desc"),
		Limit:     limit,
		Offset:    offset,
	}

	if minScore := c.Query("min_score"); minScore != "" {
		score, err := strconv.Atoi(minScore)
		if err == nil {
			filters.MinScore = score
		}
	}

	domains, err := h.aggregateRepo.ListAggregates(c.Request.Context(), filters)
	if err != nil {
		h.log.Error("Failed to list domain aggregates", infralogger.Error(err))
		respondInternalError(c, "Failed to retrieve discovered domains")
		return
	}

	total, err := h.aggregateRepo.CountAggregates(c.Request.Context(), filters)
	if err != nil {
		h.log.Error("Failed to count domain aggregates", infralogger.Error(err))
		respondInternalError(c, "Failed to get total count")
		return
	}

	// Compute quality scores and enrich with referring sources
	for _, d := range domains {
		d.ComputeQualityScore()
		sources, srcErr := h.aggregateRepo.GetReferringSources(c.Request.Context(), d.Domain)
		if srcErr == nil {
			d.ReferringSources = sources
		}
	}

	// Post-filter by min_score if requested (must be done after score computation)
	if filters.MinScore > 0 {
		filtered := make([]*domain.DomainAggregate, 0, len(domains))
		for _, d := range domains {
			if d.QualityScore >= filters.MinScore {
				filtered = append(filtered, d)
			}
		}
		domains = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"domains": domains,
		"total":   total,
	})
}

// GetDomain handles GET /api/v1/discovered-domains/:domain
func (h *DiscoveredDomainsHandler) GetDomain(c *gin.Context) {
	domainName := c.Param("domain")

	filters := database.DomainListFilters{
		Search: domainName,
		Limit:  1,
	}

	domains, err := h.aggregateRepo.ListAggregates(c.Request.Context(), filters)
	if err != nil || len(domains) == 0 {
		respondNotFound(c, "Domain")
		return
	}

	d := domains[0]
	// Only return exact match
	if d.Domain != domainName {
		respondNotFound(c, "Domain")
		return
	}

	d.ComputeQualityScore()
	sources, srcErr := h.aggregateRepo.GetReferringSources(c.Request.Context(), d.Domain)
	if srcErr == nil {
		d.ReferringSources = sources
	}

	c.JSON(http.StatusOK, d)
}

// ListDomainLinks handles GET /api/v1/discovered-domains/:domain/links
func (h *DiscoveredDomainsHandler) ListDomainLinks(c *gin.Context) {
	domainName := c.Param("domain")
	limit, offset := parseLimitOffset(c, defaultDomainsLimit, defaultDomainsOffset)

	links, total, err := h.aggregateRepo.ListLinksByDomain(
		c.Request.Context(), domainName, limit, offset,
	)
	if err != nil {
		h.log.Error("Failed to list domain links", infralogger.Error(err))
		respondInternalError(c, "Failed to retrieve domain links")
		return
	}

	// Compute path clusters
	clusters := computePathClusters(links)

	// Add path field to each link response
	type linkWithPath struct {
		*domain.DiscoveredLink
		Path string `json:"path"`
	}
	enriched := make([]linkWithPath, 0, len(links))
	for _, link := range links {
		path := extractPath(link.URL)
		enriched = append(enriched, linkWithPath{
			DiscoveredLink: link,
			Path:           path,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"links":         enriched,
		"path_clusters": clusters,
		"total":         total,
	})
}

// UpdateDomainStateRequest represents the request body for updating domain state.
type UpdateDomainStateRequest struct {
	Status string  `json:"status" binding:"required"`
	Notes  *string `json:"notes"`
}

// UpdateDomainState handles PATCH /api/v1/discovered-domains/:domain/state
func (h *DiscoveredDomainsHandler) UpdateDomainState(c *gin.Context) {
	domainName := c.Param("domain")

	var req UpdateDomainStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if !isValidDomainStatus(req.Status) {
		respondBadRequest(c, "Invalid status: must be active, ignored, reviewing, or promoted")
		return
	}

	if err := h.stateRepo.Upsert(c.Request.Context(), domainName, req.Status, req.Notes); err != nil {
		h.log.Error("Failed to update domain state", infralogger.Error(err))
		respondInternalError(c, "Failed to update domain state")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Domain state updated",
		"domain":  domainName,
		"status":  req.Status,
	})
}

// BulkUpdateDomainStateRequest represents the request body for bulk domain state updates.
type BulkUpdateDomainStateRequest struct {
	Domains []string `json:"domains" binding:"required"`
	Status  string   `json:"status"  binding:"required"`
	Notes   *string  `json:"notes"`
}

// BulkUpdateDomainState handles POST /api/v1/discovered-domains/bulk-state
func (h *DiscoveredDomainsHandler) BulkUpdateDomainState(c *gin.Context) {
	var req BulkUpdateDomainStateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, "Invalid request: "+err.Error())
		return
	}

	if len(req.Domains) == 0 {
		respondBadRequest(c, "At least one domain required")
		return
	}
	if len(req.Domains) > maxBulkDomains {
		respondBadRequest(c, "Maximum 100 domains per bulk operation")
		return
	}

	if !isValidDomainStatus(req.Status) {
		respondBadRequest(c, "Invalid status: must be active, ignored, reviewing, or promoted")
		return
	}

	count, err := h.stateRepo.BulkUpsert(c.Request.Context(), req.Domains, req.Status, req.Notes)
	if err != nil {
		h.log.Error("Failed to bulk update domain states", infralogger.Error(err))
		respondInternalError(c, "Failed to bulk update domain states")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Domain states updated",
		"updated": count,
	})
}

// isValidDomainStatus checks if a domain status is valid.
func isValidDomainStatus(status string) bool {
	switch status {
	case domain.DomainStatusActive, domain.DomainStatusIgnored,
		domain.DomainStatusReviewing, domain.DomainStatusPromoted:
		return true
	default:
		return false
	}
}

// computePathClusters groups URLs by their first path segment.
func computePathClusters(links []*domain.DiscoveredLink) []domain.PathCluster {
	counts := make(map[string]int)
	for _, link := range links {
		pattern := extractPathPattern(link.URL)
		counts[pattern]++
	}

	clusters := make([]domain.PathCluster, 0, len(counts))
	for pattern, count := range counts {
		clusters = append(clusters, domain.PathCluster{
			Pattern: pattern,
			Count:   count,
		})
	}

	// Sort by count descending (simple bubble sort for small sets)
	for i := range clusters {
		for j := i + 1; j < len(clusters); j++ {
			if clusters[j].Count > clusters[i].Count {
				clusters[i], clusters[j] = clusters[j], clusters[i]
			}
		}
	}

	return clusters
}

// extractPathPattern extracts a path pattern from a URL.
// "/news/article/123" -> "/news/*"
// "/" -> "/"
func extractPathPattern(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "/"
	}

	path := u.Path
	if path == "" || path == "/" {
		return "/"
	}

	// Get first path segment
	segments := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(segments) == 0 {
		return "/"
	}

	if len(segments) == 1 {
		return "/" + segments[0]
	}

	return "/" + segments[0] + "/*"
}

// extractPath extracts just the path from a URL.
func extractPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "/"
	}
	if u.Path == "" {
		return "/"
	}
	return u.Path
}
```

**Step 2: Run build**

Run: `cd crawler && go build ./...`
Expected: Build succeeds.

**Step 3: Commit**

```
feat(crawler): add discovered domains HTTP handler with CRUD and bulk ops
```

---

## Task 7: Register Routes + Wire Handler in Bootstrap

**Files:**
- Modify: `crawler/internal/api/api.go`
- Modify: `crawler/internal/bootstrap/services.go`
- Modify: `crawler/internal/bootstrap/server.go` (if exists, or wherever `NewServer` is called)

**Step 1: Add route registration function**

Add to `crawler/internal/api/api.go` after `setupDiscoveredLinksRoutes`:

```go
// setupDiscoveredDomainsRoutes configures discovered domains endpoints
func setupDiscoveredDomainsRoutes(v1 *gin.RouterGroup, domainsHandler *DiscoveredDomainsHandler) {
	if domainsHandler != nil {
		v1.GET("/discovered-domains", domainsHandler.ListDomains)
		v1.GET("/discovered-domains/:domain", domainsHandler.GetDomain)
		v1.GET("/discovered-domains/:domain/links", domainsHandler.ListDomainLinks)
		v1.PATCH("/discovered-domains/:domain/state", domainsHandler.UpdateDomainState)
		v1.POST("/discovered-domains/bulk-state", domainsHandler.BulkUpdateDomainState)
	}
}
```

**Step 2: Add domainsHandler parameter to NewServer and setupCrawlerRoutes**

Add `domainsHandler *DiscoveredDomainsHandler` parameter to both `NewServer` and `setupCrawlerRoutes` in `api.go`. Call `setupDiscoveredDomainsRoutes(v1, domainsHandler)` after the discovered links routes in `setupCrawlerRoutes`.

**Step 3: Create handler in bootstrap/services.go**

In `SetupServices`, after the discovered links handler creation:

```go
smCfg := deps.Config.GetSourceManagerConfig()
domainsHandler := api.NewDiscoveredDomainsHandler(
	db.DomainAggregateRepo, db.DomainStateRepo, smCfg.URL, deps.Logger,
)
```

Add `DiscoveredDomainsHandler *api.DiscoveredDomainsHandler` to `ServiceComponents` and include it in the return value.

**Step 4: Pass handler in server.go**

In `bootstrap/server.go` (wherever `api.NewServer` is called), pass `svc.DiscoveredDomainsHandler`.

**Step 5: Run build and lint**

Run: `cd crawler && go build ./... && golangci-lint run`
Expected: Build and lint pass.

**Step 6: Commit**

```
feat(crawler): register discovered domains API routes
```

---

## Task 8: Update Crawler Link Handler — Set Domain on Save

**Files:**
- Modify: `crawler/internal/crawler/link_handler.go`
- Modify: `crawler/internal/database/discovered_link_repository.go`

**Step 1: Update saveLinkToQueue to set domain**

In `link_handler.go`, update `saveLinkToQueue` to set the domain field:

```go
func (h *LinkHandler) saveLinkToQueue(ctx context.Context, linkURL, parentURL string, depth int) error {
	cc := h.crawler.getCrawlContext()
	if cc == nil || cc.Source == nil {
		h.crawler.logger.Debug("Cannot save link - no crawl context", infralogger.String("url", linkURL))
		return errors.New("no crawl context")
	}

	discoveredLink := &domain.DiscoveredLink{
		SourceID:   cc.SourceID,
		SourceName: cc.Source.Name,
		URL:        linkURL,
		ParentURL:  &parentURL,
		Depth:      depth + 1,
		Domain:     domain.ExtractDomain(linkURL),
		Status:     "pending",
		Priority:   0,
	}

	return h.linkRepo.CreateOrUpdate(ctx, discoveredLink)
}
```

**Step 2: Update CreateOrUpdate to include new columns**

In `discovered_link_repository.go`, update the INSERT query to include `domain`, `http_status`, `content_type`:

```sql
INSERT INTO discovered_links (id, source_id, source_name, url, parent_url, depth,
                          domain, http_status, content_type,
                          discovered_at, queued_at, status, priority)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (source_id, url)
DO UPDATE SET
    parent_url = EXCLUDED.parent_url,
    depth = EXCLUDED.depth,
    domain = EXCLUDED.domain,
    http_status = COALESCE(EXCLUDED.http_status, discovered_links.http_status),
    content_type = COALESCE(EXCLUDED.content_type, discovered_links.content_type),
    priority = EXCLUDED.priority,
    updated_at = NOW()
RETURNING created_at, updated_at
```

Update the `QueryRowContext` call to pass the three new fields. Also update the SELECT lists in `List`, `GetByID`, `GetPendingBySource` to include `domain, http_status, content_type`.

**Step 3: Run build and tests**

Run: `cd crawler && go build ./... && go test ./internal/...`
Expected: Build and tests pass.

**Step 4: Commit**

```
feat(crawler): set domain on discovered links and update repository queries
```

---

## Task 9: Dashboard API Client — Discovered Domains

**Files:**
- Modify: `dashboard/src/api/client.ts`

**Step 1: Add discoveredDomains namespace to crawlerApi**

In `dashboard/src/api/client.ts`, add after the `discoveredLinks` section:

```typescript
discoveredDomains: {
  list: (params?: Record<string, unknown>) => crawlerClient.get('/discovered-domains', { params }),
  get: (domain: string) => crawlerClient.get(`/discovered-domains/${encodeURIComponent(domain)}`),
  getLinks: (domain: string, params?: Record<string, unknown>) =>
    crawlerClient.get(`/discovered-domains/${encodeURIComponent(domain)}/links`, { params }),
  updateState: (domain: string, data: { status: string; notes?: string }) =>
    crawlerClient.patch(`/discovered-domains/${encodeURIComponent(domain)}/state`, data),
  bulkUpdateState: (data: { domains: string[]; status: string; notes?: string }) =>
    crawlerClient.post('/discovered-domains/bulk-state', data),
},
```

**Step 2: Commit**

```
feat(dashboard): add discovered domains API client
```

---

## Task 10: Dashboard Types — Discovered Domains

**Files:**
- Create: `dashboard/src/features/intake/api/discoveredDomains.ts`

**Step 1: Create types and fetch function**

```typescript
import { crawlerApi } from '@/api/client'
import type { FetchParams } from '@/types/table'

export interface DiscoveredDomain {
  domain: string
  status: string
  link_count: number
  source_count: number
  referring_sources: string[]
  ok_ratio: number | null
  html_ratio: number | null
  avg_depth: number
  first_seen: string
  last_seen: string
  quality_score: number
  is_existing_source: boolean
  notes: string | null
}

export interface DiscoveredDomainLink {
  id: string
  url: string
  path: string
  http_status: number | null
  content_type: string | null
  depth: number
  source_id: string
  source_name: string
  discovered_at: string
  status: string
}

export interface PathCluster {
  pattern: string
  count: number
}

export interface DiscoveredDomainFilters {
  search?: string
  status?: string
  min_score?: number
  hide_existing?: boolean
}

export interface DomainLinksResponse {
  links: DiscoveredDomainLink[]
  path_clusters: PathCluster[]
  total: number
}

export const discoveredDomainsKeys = {
  all: ['discovered-domains'] as const,
  lists: () => [...discoveredDomainsKeys.all, 'list'] as const,
  list: (filters: Record<string, unknown>) =>
    [...discoveredDomainsKeys.lists(), filters] as const,
  details: () => [...discoveredDomainsKeys.all, 'detail'] as const,
  detail: (domain: string) => [...discoveredDomainsKeys.details(), domain] as const,
  links: (domain: string) => [...discoveredDomainsKeys.all, 'links', domain] as const,
}

export async function fetchDiscoveredDomainsPaginated(
  params: FetchParams<DiscoveredDomainFilters>,
) {
  const queryParams: Record<string, unknown> = {
    limit: params.limit,
    offset: params.offset,
    sort: params.sortBy,
    order: params.sortOrder,
  }

  if (params.filters.search) queryParams.search = params.filters.search
  if (params.filters.status) queryParams.status = params.filters.status
  if (params.filters.min_score) queryParams.min_score = params.filters.min_score
  if (params.filters.hide_existing) queryParams.hide_existing = true

  const response = await crawlerApi.discoveredDomains.list(queryParams)
  return {
    items: response.data.domains as DiscoveredDomain[],
    total: response.data.total as number,
  }
}

export async function fetchDomainDetail(domain: string) {
  const response = await crawlerApi.discoveredDomains.get(domain)
  return response.data as DiscoveredDomain
}

export async function fetchDomainLinks(
  domain: string,
  params: { limit: number; offset: number },
) {
  const response = await crawlerApi.discoveredDomains.getLinks(domain, params)
  return response.data as DomainLinksResponse
}
```

**Step 2: Commit**

```
feat(dashboard): add discovered domains types and API functions
```

---

## Task 11: Dashboard Composable — useDiscoveredDomainsTable

**Files:**
- Create: `dashboard/src/features/intake/composables/useDiscoveredDomainsTable.ts`

**Step 1: Create the composable**

```typescript
import { computed } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import {
  fetchDiscoveredDomainsPaginated,
  type DiscoveredDomain,
  type DiscoveredDomainFilters,
} from '../api/discoveredDomains'

export function useDiscoveredDomainsTable() {
  const table = useServerPaginatedTable<DiscoveredDomain, DiscoveredDomainFilters>({
    fetchFn: fetchDiscoveredDomainsPaginated,
    queryKeyPrefix: 'discovered-domains',
    defaultLimit: 25,
    defaultSortBy: 'link_count',
    defaultSortOrder: 'desc',
    allowedSortFields: ['link_count', 'source_count', 'last_seen', 'domain'],
    allowedPageSizes: [10, 25, 50, 100],
  })

  const hasActiveFilters = computed(() => {
    const f = table.filters.value
    return !!(f.search || f.status || f.min_score || f.hide_existing)
  })

  const activeFilterCount = computed(() => {
    const f = table.filters.value
    let count = 0
    if (f.search) count++
    if (f.status) count++
    if (f.min_score) count++
    if (f.hide_existing) count++
    return count
  })

  return {
    ...table,
    hasActiveFilters,
    activeFilterCount,
  }
}
```

**Step 2: Commit**

```
feat(dashboard): add discovered domains table composable
```

---

## Task 12: Dashboard View — DiscoveredDomainsView.vue

**Files:**
- Create: `dashboard/src/views/intake/DiscoveredDomainsView.vue`
- Modify: `dashboard/src/router/index.ts`
- Modify: `dashboard/src/config/navigation.ts`

**Step 1: Create the primary view component**

Create `dashboard/src/views/intake/DiscoveredDomainsView.vue`. This is a large component — follow the exact pattern from `DiscoveredLinksView.vue` but with domain-level data.

Key structure:
- Header with "Discovered Domains" title + refresh button
- Filter bar: search input, status dropdown (All/Active/Reviewing/Ignored/Promoted), min score input, hide existing toggle
- Table with checkbox column for bulk selection, domain, score badge, link count, source count, HTML %, last seen, status badge, actions dropdown
- Bulk action bar (appears when items selected): "X selected" + Ignore + Mark Reviewing buttons
- Pagination controls
- Row click navigates to `/intake/discovered-links/:domain`

Use `useDiscoveredDomainsTable` composable for all data. Use `crawlerApi.discoveredDomains.updateState` and `crawlerApi.discoveredDomains.bulkUpdateState` for actions. Invalidate `['discovered-domains']` query key after mutations.

**Step 2: Update router**

In `dashboard/src/router/index.ts`, replace the discovered-links route and add the detail route:

```typescript
{
  path: '/intake/discovered-links',
  name: 'intake-discovered-domains',
  component: () => import('../views/intake/DiscoveredDomainsView.vue'),
  meta: { title: 'Discovered Domains', section: 'intake', requiresAuth: true },
},
{
  path: '/intake/discovered-links/:domain',
  name: 'intake-discovered-domain-detail',
  component: () => import('../views/intake/DiscoveredDomainDetailView.vue'),
  meta: { title: 'Domain Detail', section: 'intake', requiresAuth: true },
},
```

**Step 3: Update navigation**

In `dashboard/src/config/navigation.ts`, change "Discovered Links" to "Discovered Domains" (keep same path).

**Step 4: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors.

**Step 5: Commit**

```
feat(dashboard): add discovered domains list view with bulk operations
```

---

## Task 13: Dashboard View — DiscoveredDomainDetailView.vue

**Files:**
- Create: `dashboard/src/views/intake/DiscoveredDomainDetailView.vue`

**Step 1: Create the detail view**

Create `dashboard/src/views/intake/DiscoveredDomainDetailView.vue`:

Key structure:
- Back link to discovered domains list
- Domain name heading + quality score badge + status badge
- Action buttons: "Create Source" (primary, navigates to `/sources/new?domain=X&name=Y`), Ignore, Mark Reviewing
- Summary cards row: Total Links, OK Rate, HTML Rate, Avg Depth, Discovered From X Sources
- Path clusters table: Pattern | Count (sorted by count desc)
- URLs table (paginated): Path, HTTP Status badge, Content Type, Depth, Source, Discovered date

Use `useQuery` with `fetchDomainDetail` and `fetchDomainLinks` for data. Use `crawlerApi.discoveredDomains.updateState` for actions.

**Step 2: Update source form to read query params**

In `dashboard/src/views/scheduling/SourceFormView.vue`, add query param reading on mount:

```typescript
onMounted(() => {
  loadSource()
  // Pre-fill from query params (from discovered domains "Create Source")
  if (!isEdit.value) {
    const domain = route.query.domain as string | undefined
    const name = route.query.name as string | undefined
    if (domain) {
      form.value.url = `https://${domain}`
    }
    if (name) {
      form.value.name = name
    }
  }
})
```

**Step 3: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors.

**Step 4: Commit**

```
feat(dashboard): add domain detail view and source form pre-fill
```

---

## Task 14: Integration Testing — Backend API

**Step 1: Start services**

Run: `task docker:dev:up`
Expected: Services start (crawler, postgres, etc.).

**Step 2: Run migration**

Run: `cd crawler && go run cmd/migrate/main.go up`
Expected: Migration 020 applies.

**Step 3: Test list endpoint**

Run: `curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8060/api/v1/discovered-domains | jq`
Expected: `{"domains": [], "total": 0}` (empty list is fine — confirms endpoint works).

**Step 4: Test domain state update**

Run: `curl -s -X PATCH -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"status":"ignored"}' http://localhost:8060/api/v1/discovered-domains/test.com/state | jq`
Expected: `{"message": "Domain state updated", "domain": "test.com", "status": "ignored"}`

**Step 5: Test bulk update**

Run: `curl -s -X POST -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"domains":["a.com","b.com"],"status":"reviewing"}' http://localhost:8060/api/v1/discovered-domains/bulk-state | jq`
Expected: `{"message": "Domain states updated", "updated": 2}`

**Step 6: Run linter**

Run: `task lint:crawler`
Expected: No linting errors.

**Step 7: Commit any fixes**

```
fix(crawler): address integration test findings
```

---

## Task 15: Final Lint, Test, and Cleanup

**Step 1: Run full backend tests**

Run: `cd crawler && go test ./...`
Expected: All tests pass.

**Step 2: Run full backend lint**

Run: `task lint:force`
Expected: No linting errors.

**Step 3: Run frontend lint**

Run: `cd dashboard && npm run lint`
Expected: No linting errors.

**Step 4: Manual smoke test**

Start the dashboard (`cd dashboard && npm run dev`), navigate to `/intake/discovered-links`, verify:
- Page loads with "Discovered Domains" title
- Filter bar renders with search, status, min score, hide existing
- Table renders (empty state if no data)
- Clicking a domain row navigates to detail view
- Detail view renders with back link, summary cards, path clusters table, URLs table
- "Create Source" button navigates to `/sources/new?domain=X`

**Step 5: Final commit**

```
chore: final cleanup for discovered domains feature
```
