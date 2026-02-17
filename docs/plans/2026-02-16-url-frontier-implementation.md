# URL Frontier & Feed-Driven Ingestion Pipeline — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a feed-first content ingestion pipeline with a PostgreSQL-backed URL frontier as the single ingestion queue, replacing manual source/job management for 200+ news sources.

**Architecture:** Feed Poller discovers article URLs from RSS/sitemaps → URL Frontier deduplicates and prioritizes → Fetcher Workers (on a separate DigitalOcean droplet) claim URLs and extract content → Elasticsearch raw_content → existing Classifier → Publisher pipeline unchanged.

**Tech Stack:** Go 1.25, PostgreSQL (`FOR UPDATE SKIP LOCKED`), gofeed (RSS/Atom), goquery (HTML), temoto/robotstxt, DigitalOcean VPC, Docker

**Design Doc:** `docs/plans/2026-02-16-url-frontier-design.md`

---

## Phase 1: Foundation (URL Frontier + Data Model)

### Task 1: Crawler Database Migrations

Create the three new tables in the crawler database.

**Files:**
- Create: `crawler/migrations/014_create_url_frontier.up.sql`
- Create: `crawler/migrations/014_create_url_frontier.down.sql`
- Create: `crawler/migrations/015_create_host_state.up.sql`
- Create: `crawler/migrations/015_create_host_state.down.sql`
- Create: `crawler/migrations/016_create_feed_state.up.sql`
- Create: `crawler/migrations/016_create_feed_state.down.sql`

**Step 1: Write url_frontier migration (014)**

```sql
-- 014_create_url_frontier.up.sql
CREATE TABLE IF NOT EXISTS url_frontier (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url             TEXT NOT NULL,
    url_hash        CHAR(64) NOT NULL,
    host            TEXT NOT NULL,
    source_id       VARCHAR(36) NOT NULL,

    origin          VARCHAR(20) NOT NULL,
    parent_url      TEXT,
    depth           SMALLINT NOT NULL DEFAULT 0,

    priority        SMALLINT NOT NULL DEFAULT 5,
    status          VARCHAR(20) NOT NULL DEFAULT 'pending',
    next_fetch_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    last_fetched_at TIMESTAMP WITH TIME ZONE,
    fetch_count     INTEGER NOT NULL DEFAULT 0,
    content_hash    CHAR(64),
    etag            TEXT,
    last_modified   TEXT,

    retry_count     SMALLINT NOT NULL DEFAULT 0,
    last_error      TEXT,

    discovered_at   TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_frontier_url_hash UNIQUE (url_hash)
);

CREATE INDEX idx_frontier_claimable
    ON url_frontier (priority DESC, next_fetch_at ASC)
    WHERE status = 'pending';

CREATE INDEX idx_frontier_host
    ON url_frontier (host, last_fetched_at DESC);

CREATE INDEX idx_frontier_source_status
    ON url_frontier (source_id, status);

CREATE INDEX idx_frontier_content_hash
    ON url_frontier (content_hash)
    WHERE content_hash IS NOT NULL;

CREATE TRIGGER update_url_frontier_updated_at
    BEFORE UPDATE ON url_frontier
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

```sql
-- 014_create_url_frontier.down.sql
DROP TRIGGER IF EXISTS update_url_frontier_updated_at ON url_frontier;
DROP TABLE IF EXISTS url_frontier;
```

**Step 2: Write host_state migration (015)**

```sql
-- 015_create_host_state.up.sql
CREATE TABLE IF NOT EXISTS host_state (
    host                TEXT PRIMARY KEY,
    last_fetch_at       TIMESTAMP WITH TIME ZONE,
    min_delay_ms        INTEGER NOT NULL DEFAULT 1000,
    robots_txt          TEXT,
    robots_fetched_at   TIMESTAMP WITH TIME ZONE,
    robots_ttl_hours    INTEGER NOT NULL DEFAULT 24,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_host_state_updated_at
    BEFORE UPDATE ON host_state
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

```sql
-- 015_create_host_state.down.sql
DROP TRIGGER IF EXISTS update_host_state_updated_at ON host_state;
DROP TABLE IF EXISTS host_state;
```

**Step 3: Write feed_state migration (016)**

```sql
-- 016_create_feed_state.up.sql
CREATE TABLE IF NOT EXISTS feed_state (
    source_id           VARCHAR(36) PRIMARY KEY,
    feed_url            TEXT NOT NULL,
    last_polled_at      TIMESTAMP WITH TIME ZONE,
    last_etag           TEXT,
    last_modified       TEXT,
    last_item_count     INTEGER NOT NULL DEFAULT 0,
    consecutive_errors  INTEGER NOT NULL DEFAULT 0,
    last_error          TEXT,
    created_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE TRIGGER update_feed_state_updated_at
    BEFORE UPDATE ON feed_state
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
```

```sql
-- 016_create_feed_state.down.sql
DROP TRIGGER IF EXISTS update_feed_state_updated_at ON feed_state;
DROP TABLE IF EXISTS feed_state;
```

**Step 4: Run migrations**

```bash
cd crawler && go run cmd/migrate/main.go up
```

**Step 5: Commit**

```bash
git add crawler/migrations/014_* crawler/migrations/015_* crawler/migrations/016_*
git commit -m "feat(crawler): add url_frontier, host_state, feed_state migrations"
```

---

### Task 2: Source-Manager Migration

Add feed configuration fields to the sources table.

**Files:**
- Create: `source-manager/migrations/004_add_feed_fields.up.sql`
- Create: `source-manager/migrations/004_add_feed_fields.down.sql`
- Modify: `source-manager/internal/models/source.go`
- Modify: `source-manager/internal/repository/source.go` (update queries)
- Modify: `source-manager/internal/handlers/source.go` (accept new fields)

**Step 1: Write migration**

```sql
-- 004_add_feed_fields.up.sql
ALTER TABLE sources ADD COLUMN feed_url TEXT;
ALTER TABLE sources ADD COLUMN sitemap_url TEXT;
ALTER TABLE sources ADD COLUMN ingestion_mode VARCHAR(10) NOT NULL DEFAULT 'spider';
ALTER TABLE sources ADD COLUMN feed_poll_interval_minutes INTEGER NOT NULL DEFAULT 15;

CREATE INDEX idx_sources_ingestion_mode ON sources(ingestion_mode);
```

Note: default is `'spider'` so existing sources keep working unchanged. New sources onboarded with feeds get set to `'feed'`.

```sql
-- 004_add_feed_fields.down.sql
DROP INDEX IF EXISTS idx_sources_ingestion_mode;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_poll_interval_minutes;
ALTER TABLE sources DROP COLUMN IF EXISTS ingestion_mode;
ALTER TABLE sources DROP COLUMN IF EXISTS sitemap_url;
ALTER TABLE sources DROP COLUMN IF EXISTS feed_url;
```

**Step 2: Update source model**

Add to `source-manager/internal/models/source.go`:

```go
type Source struct {
    // ... existing fields ...
    FeedURL                  *string `db:"feed_url"                    json:"feed_url,omitempty"`
    SitemapURL               *string `db:"sitemap_url"                 json:"sitemap_url,omitempty"`
    IngestionMode            string  `db:"ingestion_mode"              json:"ingestion_mode"`
    FeedPollIntervalMinutes  int     `db:"feed_poll_interval_minutes"  json:"feed_poll_interval_minutes"`
}
```

**Step 3: Update repository queries to include new fields in INSERT/UPDATE/SELECT**

The `Create()` and `Update()` methods in `source-manager/internal/repository/source.go` need the new columns in their SQL queries. The `List()` and `GetByID()` methods that use `SELECT *` or explicit column lists also need updating.

**Step 4: Run source-manager migration**

```bash
cd source-manager && go run cmd/migrate/main.go up
```

**Step 5: Run tests and lint**

```bash
cd source-manager && go test ./... && golangci-lint run
```

**Step 6: Commit**

```bash
git add source-manager/migrations/004_* source-manager/internal/models/source.go source-manager/internal/repository/source.go source-manager/internal/handlers/source.go
git commit -m "feat(source-manager): add feed_url, sitemap_url, ingestion_mode fields to sources"
```

---

### Task 3: URL Normalization Package

A pure-function package with no external dependencies. Easy to test thoroughly.

**Files:**
- Create: `crawler/internal/frontier/normalize.go`
- Create: `crawler/internal/frontier/normalize_test.go`

**Step 1: Write the tests first**

```go
// crawler/internal/frontier/normalize_test.go
package frontier_test

import (
    "testing"

    "github.com/jonesrussell/north-cloud/crawler/internal/frontier"
)

func TestNormalizeURL(t *testing.T) {
    t.Helper() // Note: not needed at top-level test func, but required in helpers

    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"lowercase scheme", "HTTP://Example.com/Path", "https://example.com/Path"},
        {"lowercase host", "https://EXAMPLE.COM/path", "https://example.com/path"},
        {"remove default https port", "https://example.com:443/path", "https://example.com/path"},
        {"remove default http port", "http://example.com:80/path", "https://example.com/path"},
        {"keep non-default port", "https://example.com:8080/path", "https://example.com:8080/path"},
        {"remove trailing slash", "https://example.com/path/", "https://example.com/path"},
        {"keep root slash", "https://example.com/", "https://example.com/"},
        {"remove fragment", "https://example.com/path#section", "https://example.com/path"},
        {"sort query params", "https://example.com/path?z=1&a=2", "https://example.com/path?a=2&z=1"},
        {"strip utm params", "https://example.com/path?utm_source=twitter&id=1", "https://example.com/path?id=1"},
        {"strip fbclid", "https://example.com/path?fbclid=abc123&id=1", "https://example.com/path?id=1"},
        {"resolve dot segments", "https://example.com/a/b/../c", "https://example.com/a/c"},
        {"upgrade http to https", "http://example.com/path", "https://example.com/path"},
        {"empty query after stripping", "https://example.com/path?utm_source=x", "https://example.com/path"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := frontier.NormalizeURL(tt.input)
            if err != nil {
                t.Fatalf("NormalizeURL(%q) error = %v", tt.input, err)
            }
            if got != tt.expected {
                t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, got, tt.expected)
            }
        })
    }
}

func TestURLHash(t *testing.T) {
    // Same URL normalized differently should produce same hash
    hash1, err := frontier.URLHash("HTTP://Example.com/path?b=2&a=1")
    if err != nil {
        t.Fatalf("URLHash() error = %v", err)
    }
    hash2, err := frontier.URLHash("https://example.com/path?a=1&b=2")
    if err != nil {
        t.Fatalf("URLHash() error = %v", err)
    }
    if hash1 != hash2 {
        t.Errorf("same URL normalized differently produced different hashes: %s vs %s", hash1, hash2)
    }

    // Hash should be 64 chars (SHA-256 hex)
    if len(hash1) != 64 {
        t.Errorf("hash length = %d, want 64", len(hash1))
    }
}

func TestExtractHost(t *testing.T) {
    tests := []struct {
        name     string
        url      string
        expected string
    }{
        {"simple", "https://example.com/path", "example.com"},
        {"with port", "https://example.com:8080/path", "example.com"},
        {"with www", "https://www.example.com/path", "www.example.com"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := frontier.ExtractHost(tt.url)
            if err != nil {
                t.Fatalf("ExtractHost(%q) error = %v", tt.url, err)
            }
            if got != tt.expected {
                t.Errorf("ExtractHost(%q) = %q, want %q", tt.url, got, tt.expected)
            }
        })
    }
}
```

**Step 2: Run tests to verify they fail**

```bash
cd crawler && go test ./internal/frontier/... -v
```

Expected: compilation error (package doesn't exist yet).

**Step 3: Implement normalize.go**

```go
// crawler/internal/frontier/normalize.go
package frontier

import (
    "crypto/sha256"
    "fmt"
    "net/url"
    "sort"
    "strings"
)

// trackingParams are query parameters stripped during normalization.
var trackingParams = map[string]bool{
    "utm_source":   true,
    "utm_medium":   true,
    "utm_campaign": true,
    "utm_term":     true,
    "utm_content":  true,
    "fbclid":       true,
    "gclid":        true,
    "gclsrc":       true,
    "dclid":        true,
    "msclkid":      true,
    "ref":          true,
}

// NormalizeURL applies standard normalization rules to a URL string.
// Returns the normalized URL string.
func NormalizeURL(rawURL string) (string, error) {
    u, err := url.Parse(rawURL)
    if err != nil {
        return "", fmt.Errorf("parsing URL %q: %w", rawURL, err)
    }

    // Lowercase scheme, upgrade to https
    u.Scheme = "https"

    // Lowercase host
    u.Host = strings.ToLower(u.Host)

    // Remove default ports
    host := u.Hostname()
    port := u.Port()
    if port == "443" || port == "80" {
        u.Host = host
    }

    // Resolve path (handles /../ and /./)
    u.Path = u.ResolvedReference(&url.URL{Path: u.Path}).Path

    // Remove trailing slash (but keep root /)
    if len(u.Path) > 1 && strings.HasSuffix(u.Path, "/") {
        u.Path = strings.TrimRight(u.Path, "/")
    }

    // Remove fragment
    u.Fragment = ""

    // Sort query params and strip tracking
    if u.RawQuery != "" {
        params := u.Query()
        cleaned := url.Values{}
        for key, values := range params {
            if !trackingParams[strings.ToLower(key)] {
                cleaned[key] = values
            }
        }
        // Sort keys for deterministic output
        keys := make([]string, 0, len(cleaned))
        for k := range cleaned {
            keys = append(keys, k)
        }
        sort.Strings(keys)

        parts := make([]string, 0, len(keys))
        for _, k := range keys {
            for _, v := range cleaned[k] {
                parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
            }
        }
        u.RawQuery = strings.Join(parts, "&")
    }

    return u.String(), nil
}

// URLHash returns the SHA-256 hex digest of the normalized URL.
func URLHash(rawURL string) (string, error) {
    normalized, err := NormalizeURL(rawURL)
    if err != nil {
        return "", fmt.Errorf("normalizing URL for hash: %w", err)
    }
    h := sha256.Sum256([]byte(normalized))
    return fmt.Sprintf("%x", h), nil
}

// ExtractHost returns the hostname (without port) from a URL.
func ExtractHost(rawURL string) (string, error) {
    u, err := url.Parse(rawURL)
    if err != nil {
        return "", fmt.Errorf("parsing URL for host extraction: %w", err)
    }
    return u.Hostname(), nil
}
```

**Step 4: Run tests to verify they pass**

```bash
cd crawler && go test ./internal/frontier/... -v
```

**Step 5: Lint**

```bash
cd crawler && golangci-lint run ./internal/frontier/...
```

**Step 6: Commit**

```bash
git add crawler/internal/frontier/
git commit -m "feat(crawler): add URL normalization package for frontier"
```

---

### Task 4: Frontier Domain Models

**Files:**
- Create: `crawler/internal/domain/frontier.go`

**Step 1: Create domain models**

```go
// crawler/internal/domain/frontier.go
package domain

import "time"

// FrontierURL status constants.
const (
    FrontierStatusPending  = "pending"
    FrontierStatusFetching = "fetching"
    FrontierStatusFetched  = "fetched"
    FrontierStatusFailed   = "failed"
    FrontierStatusDead     = "dead"
)

// FrontierURL origin constants.
const (
    FrontierOriginFeed    = "feed"
    FrontierOriginSitemap = "sitemap"
    FrontierOriginSpider  = "spider"
    FrontierOriginManual  = "manual"
)

// Priority bounds.
const (
    FrontierMinPriority = 1
    FrontierMaxPriority = 10
    FrontierDefaultPriority = 5
)

// Origin bonus values for priority calculation.
const (
    FrontierFeedBonus    = 2
    FrontierSitemapBonus = 1
)

// FrontierURL represents a URL in the frontier queue.
type FrontierURL struct {
    ID          string     `db:"id"            json:"id"`
    URL         string     `db:"url"           json:"url"`
    URLHash     string     `db:"url_hash"      json:"url_hash"`
    Host        string     `db:"host"          json:"host"`
    SourceID    string     `db:"source_id"     json:"source_id"`

    Origin      string     `db:"origin"        json:"origin"`
    ParentURL   *string    `db:"parent_url"    json:"parent_url,omitempty"`
    Depth       int        `db:"depth"         json:"depth"`

    Priority    int        `db:"priority"      json:"priority"`
    Status      string     `db:"status"        json:"status"`
    NextFetchAt time.Time  `db:"next_fetch_at" json:"next_fetch_at"`

    LastFetchedAt *time.Time `db:"last_fetched_at" json:"last_fetched_at,omitempty"`
    FetchCount    int        `db:"fetch_count"     json:"fetch_count"`
    ContentHash   *string    `db:"content_hash"    json:"content_hash,omitempty"`
    ETag          *string    `db:"etag"            json:"etag,omitempty"`
    LastModified  *string    `db:"last_modified"   json:"last_modified,omitempty"`

    RetryCount  int     `db:"retry_count"  json:"retry_count"`
    LastError   *string `db:"last_error"   json:"last_error,omitempty"`

    DiscoveredAt time.Time `db:"discovered_at" json:"discovered_at"`
    CreatedAt    time.Time `db:"created_at"    json:"created_at"`
    UpdatedAt    time.Time `db:"updated_at"    json:"updated_at"`
}

// HostState tracks per-host politeness and robots.txt cache.
type HostState struct {
    Host            string     `db:"host"              json:"host"`
    LastFetchAt     *time.Time `db:"last_fetch_at"     json:"last_fetch_at,omitempty"`
    MinDelayMs      int        `db:"min_delay_ms"      json:"min_delay_ms"`
    RobotsTxt       *string    `db:"robots_txt"        json:"robots_txt,omitempty"`
    RobotsFetchedAt *time.Time `db:"robots_fetched_at" json:"robots_fetched_at,omitempty"`
    RobotsTTLHours  int        `db:"robots_ttl_hours"  json:"robots_ttl_hours"`
    CreatedAt       time.Time  `db:"created_at"        json:"created_at"`
    UpdatedAt       time.Time  `db:"updated_at"        json:"updated_at"`
}

// FeedState tracks polling state for a source's feed.
type FeedState struct {
    SourceID          string     `db:"source_id"          json:"source_id"`
    FeedURL           string     `db:"feed_url"           json:"feed_url"`
    LastPolledAt      *time.Time `db:"last_polled_at"     json:"last_polled_at,omitempty"`
    LastETag          *string    `db:"last_etag"          json:"last_etag,omitempty"`
    LastModified      *string    `db:"last_modified"      json:"last_modified,omitempty"`
    LastItemCount     int        `db:"last_item_count"    json:"last_item_count"`
    ConsecutiveErrors int        `db:"consecutive_errors" json:"consecutive_errors"`
    LastError         *string    `db:"last_error"         json:"last_error,omitempty"`
    CreatedAt         time.Time  `db:"created_at"         json:"created_at"`
    UpdatedAt         time.Time  `db:"updated_at"         json:"updated_at"`
}
```

**Step 2: Commit**

```bash
git add crawler/internal/domain/frontier.go
git commit -m "feat(crawler): add frontier domain models"
```

---

### Task 5: Frontier Repository

The core repository with submit (upsert), claim (SKIP LOCKED), and update methods.

**Files:**
- Create: `crawler/internal/database/frontier_repository.go`
- Create: `crawler/internal/database/frontier_repository_test.go`

**Step 1: Write tests for Submit (upsert with dedup)**

Test that:
- Inserting a new URL works
- Inserting a duplicate URL (same url_hash) updates priority to the higher value
- Inserting a duplicate URL that's already fetched does NOT re-queue it

Use `sqlmock` following the pattern in `crawler/internal/database/job_repository_test.go`.

**Step 2: Write tests for Claim (SKIP LOCKED with host politeness)**

Test that:
- Claiming returns a pending URL ordered by priority DESC
- Claiming skips URLs whose host is rate-limited
- Claiming updates status to 'fetching'
- Claiming with empty frontier returns nil (no error)

**Step 3: Write tests for UpdateStatus**

Test status transitions: fetching→fetched, fetching→failed, fetching→dead.

**Step 4: Implement frontier_repository.go**

Key methods:

```go
type FrontierRepository struct {
    db *sqlx.DB
}

func NewFrontierRepository(db *sqlx.DB) *FrontierRepository

// Submit upserts a URL into the frontier. On conflict, updates priority
// to the higher value and next_fetch_at to the earlier time, but only
// if the existing URL is still pending.
func (r *FrontierRepository) Submit(ctx context.Context, params SubmitParams) error

// Claim selects and locks the next fetchable URL, respecting host politeness.
// Returns nil if no URLs are available.
func (r *FrontierRepository) Claim(ctx context.Context) (*domain.FrontierURL, error)

// UpdateFetched marks a URL as successfully fetched with content metadata.
func (r *FrontierRepository) UpdateFetched(ctx context.Context, id string, params FetchedParams) error

// UpdateFailed marks a URL as failed, incrementing retry count.
// If max retries exceeded, marks as dead.
func (r *FrontierRepository) UpdateFailed(ctx context.Context, id string, lastError string, maxRetries int) error

// UpdateDead marks a URL as permanently unfetchable with a reason.
func (r *FrontierRepository) UpdateDead(ctx context.Context, id string, reason string) error

// List returns frontier URLs with filtering and pagination (for dashboard).
func (r *FrontierRepository) List(ctx context.Context, filters FrontierFilters) ([]*domain.FrontierURL, int, error)

// Stats returns aggregate counts by status and source (for dashboard).
func (r *FrontierRepository) Stats(ctx context.Context) (*FrontierStats, error)
```

The `Claim` query (the critical path):

```sql
BEGIN;
SELECT f.id, f.url, f.host, f.source_id, f.etag, f.last_modified, f.fetch_count
FROM url_frontier f
LEFT JOIN host_state h ON h.host = f.host
WHERE f.status = 'pending'
  AND f.next_fetch_at <= NOW()
  AND (h.host IS NULL OR h.last_fetch_at + (h.min_delay_ms * INTERVAL '1 millisecond') <= NOW())
ORDER BY f.priority DESC, f.next_fetch_at ASC
LIMIT 1
FOR UPDATE OF f SKIP LOCKED;

UPDATE url_frontier SET status = 'fetching', updated_at = NOW() WHERE id = $1;
COMMIT;
```

The `Submit` upsert:

```sql
INSERT INTO url_frontier (url, url_hash, host, source_id, origin, parent_url, depth, priority, next_fetch_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (url_hash) DO UPDATE SET
    priority = GREATEST(url_frontier.priority, EXCLUDED.priority),
    next_fetch_at = LEAST(url_frontier.next_fetch_at, EXCLUDED.next_fetch_at),
    updated_at = NOW()
WHERE url_frontier.status = 'pending'
```

**Step 5: Run tests**

```bash
cd crawler && go test ./internal/database/... -run TestFrontier -v
```

**Step 6: Lint**

```bash
cd crawler && golangci-lint run
```

**Step 7: Commit**

```bash
git add crawler/internal/database/frontier_repository.go crawler/internal/database/frontier_repository_test.go
git commit -m "feat(crawler): add frontier repository with submit, claim, and status updates"
```

---

### Task 6: Host State Repository

**Files:**
- Create: `crawler/internal/database/host_state_repository.go`
- Create: `crawler/internal/database/host_state_repository_test.go`

**Key methods:**

```go
type HostStateRepository struct {
    db *sqlx.DB
}

// GetOrCreate returns the host state, creating a default entry if none exists.
func (r *HostStateRepository) GetOrCreate(ctx context.Context, host string) (*domain.HostState, error)

// UpdateLastFetch records a fetch to this host.
func (r *HostStateRepository) UpdateLastFetch(ctx context.Context, host string) error

// UpdateRobotsTxt caches the robots.txt content and extracted crawl delay.
func (r *HostStateRepository) UpdateRobotsTxt(ctx context.Context, host string, robotsTxt string, crawlDelayMs *int) error

// UpdateMinDelay adjusts the per-host minimum delay (e.g., after a 429).
func (r *HostStateRepository) UpdateMinDelay(ctx context.Context, host string, delayMs int) error
```

Follow the same test patterns as Task 5. Test with sqlmock.

**Commit:**

```bash
git add crawler/internal/database/host_state_repository.go crawler/internal/database/host_state_repository_test.go
git commit -m "feat(crawler): add host state repository for politeness tracking"
```

---

### Task 7: Feed State Repository

**Files:**
- Create: `crawler/internal/database/feed_state_repository.go`
- Create: `crawler/internal/database/feed_state_repository_test.go`

**Key methods:**

```go
type FeedStateRepository struct {
    db *sqlx.DB
}

// GetOrCreate returns feed state for a source, creating if needed.
func (r *FeedStateRepository) GetOrCreate(ctx context.Context, sourceID, feedURL string) (*domain.FeedState, error)

// UpdateSuccess records a successful poll with new etag/last-modified.
func (r *FeedStateRepository) UpdateSuccess(ctx context.Context, sourceID string, params FeedPollResult) error

// UpdateError records a poll failure, incrementing consecutive_errors.
func (r *FeedStateRepository) UpdateError(ctx context.Context, sourceID string, errMsg string) error

// ListDueForPolling returns sources whose feed needs polling based on interval.
func (r *FeedStateRepository) ListDueForPolling(ctx context.Context) ([]*domain.FeedState, error)
```

**Commit:**

```bash
git add crawler/internal/database/feed_state_repository.go crawler/internal/database/feed_state_repository_test.go
git commit -m "feat(crawler): add feed state repository for poll tracking"
```

---

### Task 8: Frontier API Handler

Dashboard endpoints to inspect the frontier. Follows the pattern in `crawler/internal/api/discovered_links_handler.go`.

**Files:**
- Create: `crawler/internal/api/frontier_handler.go`
- Modify: `crawler/internal/api/api.go` (register routes)

**Endpoints:**

```
GET  /api/v1/frontier           — List frontier URLs (paginated, filterable by status, source_id, host)
GET  /api/v1/frontier/stats     — Aggregate counts (pending, fetching, fetched, failed, dead by source)
POST /api/v1/frontier/submit    — Manual URL submission (for bulk import)
DELETE /api/v1/frontier/:id     — Remove a URL from the frontier
```

**Route registration** (add to `crawler/internal/api/api.go`):

```go
func setupFrontierRoutes(v1 *gin.RouterGroup, frontierHandler *FrontierHandler) {
    if frontierHandler != nil {
        v1.GET("/frontier", frontierHandler.List)
        v1.GET("/frontier/stats", frontierHandler.Stats)
        v1.POST("/frontier/submit", frontierHandler.Submit)
        v1.DELETE("/frontier/:id", frontierHandler.Delete)
    }
}
```

**Commit:**

```bash
git add crawler/internal/api/frontier_handler.go crawler/internal/api/api.go
git commit -m "feat(crawler): add frontier API endpoints for dashboard"
```

---

### Task 9: Bootstrap Wiring (Phase 1)

Wire the new repositories and handler into the crawler's bootstrap.

**Files:**
- Modify: `crawler/internal/bootstrap/database.go` (create frontier, host_state, feed_state repos)
- Modify: `crawler/internal/bootstrap/server.go` (register frontier handler)

**Step 1: Add repos to database setup**

In `SetupDatabase()`, create the three new repositories alongside existing ones:

```go
frontierRepo := database.NewFrontierRepository(db)
hostStateRepo := database.NewHostStateRepository(db)
feedStateRepo := database.NewFeedStateRepository(db)
```

**Step 2: Add handler to server setup**

In `SetupHTTPServer()`, create and register the frontier handler:

```go
frontierHandler := api.NewFrontierHandler(frontierRepo, log)
setupFrontierRoutes(v1, frontierHandler)
```

**Step 3: Run full test suite**

```bash
cd crawler && go test ./... && golangci-lint run
```

**Step 4: Commit**

```bash
git add crawler/internal/bootstrap/database.go crawler/internal/bootstrap/server.go
git commit -m "feat(crawler): wire frontier repos and handler into bootstrap"
```

---

## Phase 2: Feed Poller

### Task 10: Add gofeed Dependency

**Step 1: Add dependency**

```bash
cd crawler && go get github.com/mmcdole/gofeed
```

**Step 2: Vendor**

```bash
cd /home/fsd42/dev/north-cloud && task vendor
```

**Step 3: Commit**

```bash
git add crawler/go.mod crawler/go.sum
git commit -m "feat(crawler): add gofeed dependency for RSS/Atom parsing"
```

---

### Task 11: Feed Parser

Pure functions for parsing RSS/Atom feeds and extracting article URLs.

**Files:**
- Create: `crawler/internal/feed/parser.go`
- Create: `crawler/internal/feed/parser_test.go`

**Step 1: Write tests with fixture feeds**

Create test fixtures (minimal RSS/Atom XML strings) and test that the parser:
- Extracts article URLs from RSS 2.0 feed
- Extracts article URLs from Atom feed
- Returns empty slice for empty feed
- Handles feed entries with no link gracefully

**Step 2: Implement parser**

```go
// crawler/internal/feed/parser.go
package feed

import (
    "context"
    "fmt"
    "strings"

    "github.com/mmcdole/gofeed"
)

// FeedItem represents a discovered article from a feed.
type FeedItem struct {
    URL         string
    Title       string
    PublishedAt string // RFC3339 or empty
}

// ParseFeed parses an RSS/Atom feed body and returns discovered items.
func ParseFeed(ctx context.Context, body string) ([]FeedItem, error) {
    fp := gofeed.NewParser()
    parsed, err := fp.ParseString(body)
    if err != nil {
        return nil, fmt.Errorf("parsing feed: %w", err)
    }

    items := make([]FeedItem, 0, len(parsed.Items))
    for _, entry := range parsed.Items {
        link := extractLink(entry)
        if link == "" {
            continue
        }
        item := FeedItem{
            URL:   link,
            Title: entry.Title,
        }
        if entry.PublishedParsed != nil {
            item.PublishedAt = entry.PublishedParsed.Format("2006-01-02T15:04:05Z07:00")
        }
        items = append(items, item)
    }

    return items, nil
}

func extractLink(entry *gofeed.Item) string {
    if entry.Link != "" {
        return strings.TrimSpace(entry.Link)
    }
    if entry.GUID != "" && strings.HasPrefix(entry.GUID, "http") {
        return strings.TrimSpace(entry.GUID)
    }
    return ""
}
```

**Step 3: Run tests**

```bash
cd crawler && go test ./internal/feed/... -v
```

**Step 4: Commit**

```bash
git add crawler/internal/feed/
git commit -m "feat(crawler): add RSS/Atom feed parser using gofeed"
```

---

### Task 12: Sitemap Parser

**Files:**
- Create: `crawler/internal/feed/sitemap.go`
- Create: `crawler/internal/feed/sitemap_test.go`

Parses standard sitemap XML and news sitemap XML. Filters by `<lastmod>` to only return recently updated URLs. Uses `encoding/xml` (no external dependency needed).

**Key types:**

```go
type SitemapURL struct {
    Loc     string
    LastMod *time.Time
}

// ParseSitemap parses a sitemap XML body and returns URLs.
// If maxAge is non-zero, only returns URLs with lastmod within that duration.
func ParseSitemap(body string, maxAge time.Duration) ([]SitemapURL, error)

// ParseSitemapIndex parses a sitemap index and returns child sitemap URLs.
func ParseSitemapIndex(body string) ([]string, error)
```

**Commit:**

```bash
git add crawler/internal/feed/sitemap.go crawler/internal/feed/sitemap_test.go
git commit -m "feat(crawler): add sitemap parser with lastmod filtering"
```

---

### Task 13: Feed Poller Service

The main polling loop that ties everything together.

**Files:**
- Create: `crawler/internal/feed/poller.go`
- Create: `crawler/internal/feed/poller_test.go`

**Key design:**

```go
type Poller struct {
    sourceClient   SourceClient       // interface to source-manager API
    feedStateRepo  *database.FeedStateRepository
    frontierRepo   *database.FrontierRepository
    httpClient     *http.Client
    log            infralogger.Logger
    pollInterval   time.Duration      // how often to check for due sources (default 30s)
    maxConcurrency int                // max concurrent feed polls (default 10)
}

// SourceClient is the interface the poller needs from source-manager.
type SourceClient interface {
    ListSources(ctx context.Context) ([]SourceConfig, error)
}

// Start begins the polling loop. Blocks until ctx is cancelled.
func (p *Poller) Start(ctx context.Context) error

// pollSource fetches and parses a single source's feed.
func (p *Poller) pollSource(ctx context.Context, source SourceConfig, state *domain.FeedState) error
```

The `pollSource` method:
1. Builds HTTP request with conditional GET headers from `state`
2. Fetches feed URL
3. If 304 → update `last_polled_at`, return
4. If 200 → parse with `feed.ParseFeed()`
5. For each item → normalize URL, compute hash, submit to frontier
6. Update feed state (etag, last_modified, item count, reset errors)
7. On error → call `feedStateRepo.UpdateError()`

**Testing:** Use httptest server to simulate feed responses (200 with RSS body, 304, 500).

**Commit:**

```bash
git add crawler/internal/feed/poller.go crawler/internal/feed/poller_test.go
git commit -m "feat(crawler): add feed poller service with conditional GET and error backoff"
```

---

### Task 14: Wire Feed Poller into Bootstrap

**Files:**
- Modify: `crawler/internal/bootstrap/services.go` (create and start poller)
- Modify: `crawler/internal/config/config.go` (add feed poller config)

**Step 1: Add config**

```go
type FeedPollerConfig struct {
    Enabled        bool          `env:"FEED_POLLER_ENABLED"        yaml:"enabled"`
    PollInterval   time.Duration `env:"FEED_POLLER_POLL_INTERVAL"  yaml:"poll_interval"`
    MaxConcurrency int           `env:"FEED_POLLER_MAX_CONCURRENCY" yaml:"max_concurrency"`
}
```

**Step 2: Start poller in bootstrap**

In `SetupServices()`, if `cfg.FeedPoller.Enabled`:
1. Create `feed.Poller` with dependencies
2. Start in a goroutine: `go poller.Start(ctx)`
3. Include in graceful shutdown

**Step 3: Test**

```bash
cd crawler && go test ./... && golangci-lint run
```

**Step 4: Commit**

```bash
git add crawler/internal/bootstrap/services.go crawler/internal/config/config.go
git commit -m "feat(crawler): wire feed poller into crawler bootstrap with config"
```

---

## Phase 3: Fetcher Workers

### Task 15: Fetcher Binary Scaffold

Create the fetcher as a separate binary in the crawler module.

**Files:**
- Create: `crawler/cmd/fetcher/main.go`
- Create: `crawler/internal/fetcher/config.go`

**Step 1: Create entry point**

```go
// crawler/cmd/fetcher/main.go
package main

import (
    "fmt"
    "os"

    "github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

func main() {
    if err := fetcher.Run(); err != nil {
        fmt.Fprintf(os.Stderr, "Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Step 2: Create fetcher config**

```go
// crawler/internal/fetcher/config.go
package fetcher

import (
    "time"
)

// Config holds fetcher worker configuration.
type Config struct {
    FrontierDBURL    string        `env:"FRONTIER_DB_URL"`
    ElasticsearchURL string        `env:"ELASTICSEARCH_URL"`
    SourceManagerURL string        `env:"SOURCE_MANAGER_URL"`
    WorkerCount      int           `env:"WORKER_COUNT"`
    UserAgent        string        `env:"USER_AGENT"`
    RequestTimeout   time.Duration `env:"REQUEST_TIMEOUT"`
    ClaimRetryDelay  time.Duration `env:"CLAIM_RETRY_DELAY"`
    MaxRetries       int           `env:"MAX_RETRIES"`
}
```

**Step 3: Create Run() with bootstrap phases**

Follows the standard bootstrap pattern: Config → Logger → DB → ES → Source Client → Worker Pool → Run until interrupt.

**Step 4: Build it**

```bash
cd crawler && go build -o bin/fetcher ./cmd/fetcher
```

**Step 5: Commit**

```bash
git add crawler/cmd/fetcher/ crawler/internal/fetcher/
git commit -m "feat(crawler): scaffold fetcher worker binary"
```

---

### Task 16: Robots.txt Handler

**Files:**
- Create: `crawler/internal/fetcher/robots.go`
- Create: `crawler/internal/fetcher/robots_test.go`

**Step 1: Add dependency**

```bash
cd crawler && go get github.com/temoto/robotstxt
```

**Step 2: Write tests**

Test:
- Parses robots.txt and checks if URL is allowed
- Extracts Crawl-delay
- Handles missing robots.txt (allow all)
- Handles 404 robots.txt (allow all)
- Caches and respects TTL

**Step 3: Implement**

```go
type RobotsChecker struct {
    hostStateRepo *database.HostStateRepository
    httpClient    *http.Client
    userAgent     string
    log           infralogger.Logger
}

// IsAllowed checks if the URL is allowed by robots.txt for our user agent.
// Fetches and caches robots.txt if not cached or stale.
func (r *RobotsChecker) IsAllowed(ctx context.Context, urlStr string) (bool, error)
```

**Step 4: Commit**

```bash
git add crawler/internal/fetcher/robots.go crawler/internal/fetcher/robots_test.go crawler/go.mod crawler/go.sum
git commit -m "feat(crawler): add robots.txt checker with caching"
```

---

### Task 17: Fetcher Worker Pool

The core worker loop: claim → check robots → fetch → extract → index → update frontier.

**Files:**
- Create: `crawler/internal/fetcher/worker.go`
- Create: `crawler/internal/fetcher/worker_test.go`

**Key types:**

```go
type WorkerPool struct {
    frontierRepo  *database.FrontierRepository
    hostStateRepo *database.HostStateRepository
    robots        *RobotsChecker
    extractor     *ContentExtractor  // wraps goquery + source selectors
    indexer       *storage.RawContentIndexer  // reuse existing ES indexer
    sourceClient  SourceClient
    log           infralogger.Logger
    workerCount   int
    userAgent     string
    httpClient    *http.Client
}

// Start launches N workers. Blocks until ctx is cancelled.
func (wp *WorkerPool) Start(ctx context.Context) error

// worker is a single worker goroutine loop.
func (wp *WorkerPool) worker(ctx context.Context, workerID int)
```

The worker loop per iteration:
1. `frontierRepo.Claim(ctx)` → if nil, sleep `claimRetryDelay`, continue
2. `robots.IsAllowed(ctx, url)` → if not, `frontierRepo.UpdateDead(ctx, id, "robots_blocked")`
3. Build HTTP request with User-Agent, conditional GET headers (ETag, If-Modified-Since)
4. Execute request with timeout
5. Handle response code (200, 304, 3xx, 404, 429, 5xx) per design doc
6. On 200: extract content, compute content_hash, index to ES, update frontier as fetched
7. `hostStateRepo.UpdateLastFetch(ctx, host)`

**Testing:** Use httptest for mock target servers. Use sqlmock for DB.

**Commit:**

```bash
git add crawler/internal/fetcher/worker.go crawler/internal/fetcher/worker_test.go
git commit -m "feat(crawler): add fetcher worker pool with claim-fetch-extract-index loop"
```

---

### Task 18: Content Extractor (goquery)

Extracts article content from fetched HTML using source selectors. Reuses the extraction logic from `crawler/internal/content/rawcontent/extractor.go` but adapted for standalone page fetching (no Colly HTMLElement dependency).

**Files:**
- Create: `crawler/internal/fetcher/extractor.go`
- Create: `crawler/internal/fetcher/extractor_test.go`

**Key approach:** The existing `extractor.go` operates on Colly's `HTMLElement`. The fetcher needs the same extraction but on a raw `*goquery.Document`. Refactor the extraction functions that can be shared into a common interface, or create a new extractor that wraps goquery and produces the same `RawContent` output.

```go
type ContentExtractor struct {
    sourceClient SourceClient  // fetch source selectors
    selectorCache map[string]SelectorConfig  // cached per source_id
    log          infralogger.Logger
}

// Extract parses HTML and applies source selectors to produce a RawContent document.
func (e *ContentExtractor) Extract(ctx context.Context, sourceID string, pageURL string, body []byte) (*rawcontent.RawContent, error)
```

The output `RawContent` struct is the same one the Colly pipeline produces — the classifier doesn't care about the origin.

**Testing:** Use HTML fixture strings. Verify title, body text, metadata extraction against known selectors.

**Commit:**

```bash
git add crawler/internal/fetcher/extractor.go crawler/internal/fetcher/extractor_test.go
git commit -m "feat(crawler): add goquery-based content extractor for fetcher workers"
```

---

### Task 19: Fetcher Dockerfile

**Files:**
- Create: `crawler/Dockerfile.fetcher`

**Step 1: Write Dockerfile**

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /fetcher ./cmd/fetcher

# Runtime stage
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /fetcher /usr/local/bin/fetcher
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fetcher"]
```

**Step 2: Build and verify**

```bash
cd crawler && docker build -f Dockerfile.fetcher -t north-cloud/fetcher:latest .
```

**Step 3: Commit**

```bash
git add crawler/Dockerfile.fetcher
git commit -m "feat(crawler): add fetcher worker Dockerfile"
```

---

### Task 20: Droplet Provisioning

**Files:**
- Create: `infrastructure/fetcher/cloud-init.yml`
- Create: `infrastructure/fetcher/docker-compose.yml`
- Create: `infrastructure/fetcher/README.md`

**Step 1: Create cloud-init**

```yaml
# infrastructure/fetcher/cloud-init.yml
#cloud-config
package_update: true
packages:
  - docker-compose-plugin

write_files:
  - path: /opt/fetcher/docker-compose.yml
    content: |
      services:
        fetcher:
          image: ghcr.io/jonesrussell/north-cloud-fetcher:latest
          restart: unless-stopped
          environment:
            FRONTIER_DB_URL: ${FRONTIER_DB_URL}
            ELASTICSEARCH_URL: ${ELASTICSEARCH_URL}
            SOURCE_MANAGER_URL: ${SOURCE_MANAGER_URL}
            WORKER_COUNT: "10"
            USER_AGENT: "NorthCloud-Crawler/1.0 (+https://northcloud.biz/crawler)"
            LOG_LEVEL: info
            LOG_FORMAT: json
          logging:
            driver: json-file
            options:
              max-size: "50m"
              max-file: "3"
```

**Step 2: Document provisioning commands**

```bash
# Create VPC (if not exists)
doctl vpcs create --name north-cloud-vpc --region tor1 --ip-range 10.132.0.0/20

# Create droplet
doctl compute droplet create nc-fetcher-01 \
  --region tor1 \
  --size s-1vcpu-1gb \
  --image docker-24-04 \
  --vpc-uuid <vpc-id> \
  --ssh-keys <key-fingerprint> \
  --tag-names north-cloud,fetcher \
  --user-data-file infrastructure/fetcher/cloud-init.yml

# Create firewall
doctl compute firewall create \
  --name nc-fetcher-fw \
  --tag-names fetcher \
  --inbound-rules "protocol:tcp,ports:22,address:YOUR_IP/32" \
  --outbound-rules "protocol:tcp,ports:all,address:0.0.0.0/0 protocol:udp,ports:all,address:0.0.0.0/0"
```

**Step 3: Commit**

```bash
git add infrastructure/fetcher/
git commit -m "feat(infra): add fetcher droplet provisioning with cloud-init and docker-compose"
```

---

## Phase 4: Spider Integration (Outline)

> Implement after Phases 1-3 are stable and verified with real sources.

### Task 21: Modify link_handler.go to Submit to Frontier

- Change `trySaveLink()` to call `frontierRepo.Submit()` instead of `discoveredLinkRepo.CreateOrUpdate()`
- Add `isArticleURL()` check using `ArticleURLPatterns`
- Article URLs → submit to frontier with `origin=spider`
- External links → submit to frontier with `origin=spider`, lower priority

### Task 22: Remove Content Extraction for Frontier-Enabled Sources

- When source `ingestion_mode` is `spider` or `both`, remove Colly content extraction callbacks
- Spider only registers link-discovery callbacks (`OnHTML("a[href]", ...)`)
- Content extraction is handled by fetcher workers via the frontier

### Task 23: Deprecate discovered_links

- Remove writes to `discovered_links` table
- Add migration to drop the table (or keep as archive with a flag)
- Update dashboard to use frontier endpoints instead

---

## Phase 5: Dashboard & Cleanup (Outline)

> Implement after the pipeline is running in production.

### Task 24: Frontier Dashboard View

- New Vue component at `dashboard/src/views/intake/FrontierView.vue`
- Replace `DiscoveredLinksView.vue` in the navigation
- Server-paginated table with filters: status, source, host, origin
- Stats panel: pending/fetching/fetched/failed/dead counts by source

### Task 25: Source Manager Feed Fields in Dashboard

- Update source create/edit forms to include `feed_url`, `sitemap_url`, `ingestion_mode`, `feed_poll_interval_minutes`
- Show ingestion mode badge on source list

### Task 26: Bulk Import Endpoint

- `POST /api/v1/frontier/submit/bulk` — accepts array of URLs with source_id
- For migrating from spreadsheet workflows
- Validates, normalizes, and deduplicates in batch

---

## Verification Checklist

After each phase, verify:

- [ ] `cd crawler && go test ./... && golangci-lint run` passes
- [ ] `cd source-manager && go test ./... && golangci-lint run` passes
- [ ] Migrations run cleanly up and down
- [ ] No `interface{}`, magic numbers, unchecked JSON errors
- [ ] All test helpers use `t.Helper()`
- [ ] Functions under 100 lines, cognitive complexity under 20
- [ ] Error messages use `fmt.Errorf("context: %w", err)` pattern

After Phase 3 specifically:
- [ ] End-to-end: configure a source with RSS feed → feed poller discovers URLs → frontier populated → fetcher worker claims and fetches → raw_content in ES → classifier processes → publisher routes
- [ ] Content extracted by fetcher matches what the Colly spider would have produced
- [ ] robots.txt respected, per-host politeness enforced
- [ ] Duplicate URLs deduplicated by url_hash
- [ ] Duplicate content detected by content_hash
