---
work_package_id: WP10
title: SQLite Catalogue
dependencies:
- WP05
- WP06
requirement_refs:
- FR-006
- FR-008
- FR-015
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T040
- T041
- T042
- T043
- T044
phase: B
agent: "claude:sonnet:implementer:implementer"
shell_pid: "251290"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/catalogue/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/catalogue/**
priority: P1
tags: []
---

# WP10 — SQLite Catalogue

## Objective

Local state store for poll checkpoints (ETag, Last-Modified, last_polled_at, consecutive_failures per source) and the active-alert catalogue used for rescission detection. Idempotent rebuild from Elasticsearch on startup if the SQLite DB is missing.

## Context

- Spec §3 FR-006, FR-008, FR-015
- Plan §Component Design (Catalogue), §TC-004
- Data model: §6 PollCheckpoint, §7 AlertCatalogEntry
- Research R-002 (signal-crawler SQLite dedup pattern)

## Branch Strategy

Standard. Parallel-safe with other Phase B WPs.

## Subtasks

### T040 — Create `internal/catalogue/schema.sql`

**Purpose**: Schema for the two SQLite tables.

**Steps**:
1. Create `alert-crawler/internal/catalogue/schema.sql`:
   ```sql
   CREATE TABLE IF NOT EXISTS poll_checkpoint (
       source_id            TEXT NOT NULL,
       feed_url             TEXT NOT NULL,
       last_polled_at       TIMESTAMP NOT NULL,
       last_etag            TEXT NOT NULL DEFAULT '',
       last_modified        TEXT NOT NULL DEFAULT '',
       last_status          INTEGER NOT NULL DEFAULT 0,
       consecutive_failures INTEGER NOT NULL DEFAULT 0,
       PRIMARY KEY (source_id, feed_url)
   );

   CREATE TABLE IF NOT EXISTS alert_catalogue (
       source_id     TEXT NOT NULL,
       alert_id      TEXT NOT NULL,
       last_seen_at  TIMESTAMP NOT NULL,
       is_active     BOOLEAN NOT NULL DEFAULT 1,
       content_hash  TEXT NOT NULL,
       PRIMARY KEY (source_id, alert_id)
   );

   CREATE INDEX IF NOT EXISTS idx_alert_catalogue_active
       ON alert_catalogue(source_id, is_active, last_seen_at);
   ```

**Files**:
- `alert-crawler/internal/catalogue/schema.sql` (new, ~25 lines).

### T041 — Migrations directory with unique numeric prefixes

**Purpose**: Match NC convention (golang-migrate crashes on duplicate prefixes per repo CLAUDE.md).

**Steps**:
1. Create `alert-crawler/internal/catalogue/migrations/`.
2. Create `0001_initial_schema.up.sql` with the schema from T040 and `0001_initial_schema.down.sql` with `DROP TABLE` statements.
3. Embed migrations via `go:embed` in the store package.

**Files**:
- `alert-crawler/internal/catalogue/migrations/0001_initial_schema.up.sql` (new)
- `alert-crawler/internal/catalogue/migrations/0001_initial_schema.down.sql` (new)

### T042 — Create `internal/catalogue/store.go`

**Purpose**: Go store interface and implementation.

**Steps**:
1. Create `alert-crawler/internal/catalogue/store.go`:
   ```go
   package catalogue

   import (
       "context"
       "database/sql"
       "time"

       _ "github.com/mattn/go-sqlite3"
   )

   type Store struct {
       db *sql.DB
   }

   type PollCheckpoint struct {
       SourceID            string
       FeedURL             string
       LastPolledAt        time.Time
       LastETag            string
       LastModified        string
       LastStatus          int
       ConsecutiveFailures int
   }

   type CatalogEntry struct {
       SourceID     string
       AlertID      string
       LastSeenAt   time.Time
       IsActive     bool
       ContentHash  string
   }

   func Open(ctx context.Context, path string) (*Store, error) { /* ... */ }

   func (s *Store) LoadCheckpoint(ctx context.Context, sourceID, feedURL string) (*PollCheckpoint, error) { /* ... */ }
   func (s *Store) SaveCheckpoint(ctx context.Context, c PollCheckpoint) error { /* ... */ }
   func (s *Store) IncrementConsecutiveFailures(ctx context.Context, sourceID, feedURL string) error { /* ... */ }
   func (s *Store) ResetConsecutiveFailures(ctx context.Context, sourceID, feedURL string) error { /* ... */ }

   func (s *Store) LookupAlert(ctx context.Context, sourceID, alertID string) (*CatalogEntry, error) { /* ... */ }
   func (s *Store) MarkSeen(ctx context.Context, e CatalogEntry) error { /* ... */ }
   // RescindAbsent returns IDs of currently-active alerts whose last_seen_at < pollStartedAt.
   // Caller is responsible for emitting rescinded events for each returned ID.
   func (s *Store) RescindAbsent(ctx context.Context, sourceID string, pollStartedAt time.Time) ([]string, error) { /* ... */ }
   func (s *Store) MarkRescinded(ctx context.Context, sourceID, alertID string) error { /* ... */ }

   func (s *Store) Close() error { return s.db.Close() }
   ```
2. Use context-aware methods (`QueryRowContext`, `ExecContext`, etc.) per the charter.
3. Embed migrations via `go:embed` and run them on `Open`.

**Files**:
- `alert-crawler/internal/catalogue/store.go` (new, ~250 lines).

### T043 — Catalogue rebuild-from-ES path

**Purpose**: If `data/state.db` is missing or empty, rebuild the active-alert catalogue from Elasticsearch (idempotent recovery on startup).

**Steps**:
1. Add a `RebuildFromES(ctx, esClient elasticsearch.Indexer) error` method.
2. Logic:
   - Query ES for all documents in `community_alerts` where `lifecycle_state == "active"`.
   - For each, insert/upsert a `CatalogEntry` with `is_active=true`, `last_seen_at=now()` (best approximation), `content_hash` derived from the document.
   - The poll_checkpoint table is NOT rebuilt; the next poll cycle re-establishes it from a 200 response.
3. The method is idempotent (using `INSERT OR REPLACE`).
4. The runner (WP15) calls `RebuildFromES` only if the catalogue is empty on startup AND `--rebuild` is passed (CLI flag) or auto-detected from row count.

**Files**:
- `alert-crawler/internal/catalogue/rebuild.go` (new, ~80 lines).

**Validation**:
- Rebuild against a 5-document ES result set creates 5 catalogue entries.
- Rebuild on a non-empty catalogue is a safe no-op (or refresh).

### T044 — Unit tests with in-memory SQLite

**Purpose**: Exhaustive coverage with fast, isolated tests.

**Steps**:
1. Create `alert-crawler/internal/catalogue/store_test.go` with cases:
   - **TestOpen_RunsMigrations**: open against `:memory:`; tables exist.
   - **TestSaveAndLoadCheckpoint**: round-trip a checkpoint.
   - **TestIncrementAndResetFailures**: counter behavior.
   - **TestMarkSeen_Upserts**: same alert ID on subsequent polls updates last_seen_at and content_hash; is_active flips back to true on revival.
   - **TestRescindAbsent**: insert 3 alerts; mark only 2 seen in this poll; assert RescindAbsent returns the 3rd; assert MarkRescinded flips is_active.
   - **TestLookupAlert_NotFound**: returns sql.ErrNoRows or nil.
2. Use `t.Helper()` in fixture builders.

**Files**:
- `alert-crawler/internal/catalogue/store_test.go` (new, ~280 lines).
- `alert-crawler/internal/catalogue/rebuild_test.go` (new, ~80 lines).

**Validation**:
- `task test:alert-crawler` passes.
- Coverage ≥80% on `internal/catalogue/`.

## Definition of Done

- Two-table schema, single migration file with unique numeric prefix.
- All CRUD operations work and are context-aware.
- RescindAbsent correctly identifies absent alerts within a single poll.
- Rebuild-from-ES path is idempotent.
- Coverage ≥80%.

## Risks

- **CGO required**: `go-sqlite3` needs CGO. Alpine builder image already has `gcc` and `musl-dev` (per WP05 Dockerfile).
- **Migration prefix collision**: WP10 owns prefix 0001 in alert-crawler; future migrations must increment.
- **Concurrent access**: SQLite is single-writer. Alert-crawler is oneshot, so concurrency is not an issue, but the design should not assume more.

## Reviewer Guidance

- Verify all SQL uses parameterized queries (no string concatenation).
- Verify migrations are idempotent (`IF NOT EXISTS`).
- Verify RescindAbsent's logic: only `is_active=true` entries with `last_seen_at < pollStartedAt` are returned.
- Verify the rebuild path does not double-write events (the runner emits `created` events; rebuild does NOT, because rebuilt entries already existed).

## Implementation Command

```bash
spec-kitty agent action implement WP10 --agent <name>
```

Depends on WP05, WP06.

## Activity Log

- 2026-05-06T22:51:50Z – claude:sonnet:implementer:implementer – shell_pid=251290 – Started implementation via action command
- 2026-05-06T22:58:38Z – claude:sonnet:implementer:implementer – shell_pid=251290 – SQLite catalogue + migrations + rebuild path
