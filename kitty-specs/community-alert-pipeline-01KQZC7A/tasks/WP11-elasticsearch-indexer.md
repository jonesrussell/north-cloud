---
work_package_id: WP11
title: Elasticsearch Indexer
dependencies:
- WP05
- WP06
requirement_refs:
- C-004
- C-006
- FR-004
- FR-007
- FR-011
- FR-012
planning_base_branch: main
merge_target_branch: main
branch_strategy: Planning artifacts for this feature were generated on main. During /spec-kitty.implement this WP may branch from a dependency-specific base, but completed changes must merge back into main unless the human explicitly redirects the landing branch.
subtasks:
- T045
- T046
- T047
- T048
phase: B
agent: "claude:opus:reviewer:reviewer"
shell_pid: "261986"
history:
- at: '2026-05-06T20:51:29Z'
  event: created
  by: spec-kitty.tasks
authoritative_surface: alert-crawler/internal/elasticsearch/
execution_mode: code_change
mission_id: 01KQZC7A7SJJZ6EKHZ9JW3AZJG
mission_slug: community-alert-pipeline-01KQZC7A
owned_files:
- alert-crawler/internal/elasticsearch/**
priority: P1
tags: []
---

# WP11 — Elasticsearch Indexer

## Objective

Persist alerts to Elasticsearch index `community_alerts` via raw `net/http.Client` (no Go ES SDK). Mapping owned in-service per the resolved charter tension; idempotent `EnsureIndex` (HEAD then PUT on 404) on startup. Single-doc operations (`Index`, partial update `MarkRescinded`); bulk path retained only for backfill subcommand.

## Context

- Spec §3 FR-004, FR-007, FR-011, FR-012, §5 C-004, C-006
- Plan §Component Design (Elasticsearch indexer), §TC-006, §1 Charter Check (in-service mapping ownership)
- Contracts: `contracts/es-index-mapping.json`
- Research R-003 (rfp-ingestor reference pattern)

## Branch Strategy

Standard. Parallel-safe.

## Subtasks

### T045 — Create `internal/elasticsearch/mapping.go`

**Purpose**: Embed the `community_alerts` mapping JSON and expose it for `EnsureIndex`.

**Steps**:
1. Create `alert-crawler/internal/elasticsearch/mapping.go`:
   ```go
   package elasticsearch

   import _ "embed"

   //go:embed mapping.json
   var indexMapping []byte

   // CommunityAlertsMapping returns the JSON mapping for the community_alerts index.
   // Source: kitty-specs/community-alert-pipeline-01KQZC7A/contracts/es-index-mapping.json
   func CommunityAlertsMapping() []byte { return indexMapping }
   ```
2. Copy `kitty-specs/community-alert-pipeline-01KQZC7A/contracts/es-index-mapping.json` to `alert-crawler/internal/elasticsearch/mapping.json`.
3. Strip the `_comment` fields from the embedded JSON (ES rejects unknown top-level keys at PUT time). Keep them only in the contracts source.

**Files**:
- `alert-crawler/internal/elasticsearch/mapping.go` (new, ~20 lines).
- `alert-crawler/internal/elasticsearch/mapping.json` (new, copied + cleaned).

**Validation**:
- The mapping JSON is valid (parses with `json.Unmarshal` to a generic map).
- `_comment` keys absent from the embedded copy.

### T046 — Create `internal/elasticsearch/indexer.go`

**Purpose**: Single-doc Index, partial-update MarkRescinded, EnsureIndex, raw HTTP client.

**Steps**:
1. Create `alert-crawler/internal/elasticsearch/indexer.go`:
   ```go
   package elasticsearch

   import (
       "bytes"
       "context"
       "encoding/json"
       "fmt"
       "net/http"
       "time"

       "github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
   )

   type Indexer struct {
       baseURL string
       index   string
       http    *http.Client
   }

   type Config struct {
       BaseURL string
       Index   string
   }

   func New(cfg Config) *Indexer {
       return &Indexer{
           baseURL: cfg.BaseURL,
           index:   cfg.Index,
           http:    &http.Client{Timeout: 10 * time.Second},
       }
   }

   // EnsureIndex creates the index if it doesn't exist. Idempotent.
   func (i *Indexer) EnsureIndex(ctx context.Context) error { /* HEAD; PUT mapping if 404 */ }

   // Index writes a single Alert document with deterministic _id.
   func (i *Indexer) Index(ctx context.Context, alert domain.Alert) error { /* PUT /{index}/_doc/{id} */ }

   // MarkRescinded performs a partial update setting lifecycle_state=rescinded
   // and appending a revision_history entry. Idempotent.
   func (i *Indexer) MarkRescinded(ctx context.Context, alertID string, at time.Time, summary string) error {
       /* POST /{index}/_update/{id} with a doc-as-upsert payload */
   }

   // QueryActive returns currently-active alerts (used by catalogue rebuild).
   func (i *Indexer) QueryActive(ctx context.Context, sourceID string) ([]domain.Alert, error) { /* search query */ }

   // BulkIndex (used only by backfill subcommand) writes many alerts in one _bulk request.
   func (i *Indexer) BulkIndex(ctx context.Context, alerts []domain.Alert) error { /* /_bulk NDJSON */ }
   ```
2. Use `errors.Is` with sentinel errors for "index not found" so callers can react.
3. Set `Content-Type: application/json` on PUT/POST; `Content-Type: application/x-ndjson` on `_bulk`.

**Files**:
- `alert-crawler/internal/elasticsearch/indexer.go` (new, ~250 lines).

### T047 — Idempotent `EnsureIndex`

**Purpose**: HEAD `/{index}` → if 200, no-op; if 404, PUT with mapping.

**Steps**:
1. Implementation pattern (within `EnsureIndex`):
   ```go
   func (i *Indexer) EnsureIndex(ctx context.Context) error {
       url := i.baseURL + "/" + i.index
       headReq, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
       resp, err := i.http.Do(headReq)
       if err != nil {
           return fmt.Errorf("HEAD index: %w", err)
       }
       resp.Body.Close()
       switch resp.StatusCode {
       case http.StatusOK:
           return nil
       case http.StatusNotFound:
           putReq, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(indexMapping))
           putReq.Header.Set("Content-Type", "application/json")
           putResp, err := i.http.Do(putReq)
           if err != nil {
               return fmt.Errorf("PUT index: %w", err)
           }
           defer putResp.Body.Close()
           if putResp.StatusCode >= 400 {
               body, _ := io.ReadAll(putResp.Body)
               return fmt.Errorf("PUT index returned %d: %s", putResp.StatusCode, string(body))
           }
           return nil
       default:
           return fmt.Errorf("HEAD index returned unexpected %d", resp.StatusCode)
       }
   }
   ```
2. Note: race condition — two services could PUT simultaneously. ES handles this gracefully (returns 400 with `resource_already_exists_exception`). Treat that as success.

**Files**:
- `alert-crawler/internal/elasticsearch/indexer.go` (already from T046).

**Validation**:
- First call against an empty cluster creates the index.
- Subsequent calls are no-ops.
- Concurrent PUT race is handled.

### T048 — Unit tests with `httptest.Server`

**Purpose**: Exhaustive coverage of ES operations.

**Steps**:
1. Create `alert-crawler/internal/elasticsearch/indexer_test.go` with:
   - **TestEnsureIndex_NotFoundCreates**: HEAD returns 404, PUT returns 200; assert PUT was called with the mapping body.
   - **TestEnsureIndex_ExistsNoOp**: HEAD returns 200; assert no PUT was made.
   - **TestEnsureIndex_RaceCondition**: HEAD returns 404, PUT returns 400 with "resource_already_exists_exception"; assert returns nil.
   - **TestIndex_Roundtrip**: PUT a document; capture request body; assert it's the marshaled Alert.
   - **TestMarkRescinded_PartialUpdate**: POST /_update/{id}; assert payload includes `lifecycle_state: "rescinded"` and `revision_history` append script.
   - **TestQueryActive_BuildsQuery**: assert search query JSON matches expectations.
   - **TestBulkIndex_NDJSON**: assert request body is correct NDJSON shape.
2. Use `httptest.NewServer` with a routing mux to dispatch by path/method.
3. `t.Helper()`. Coverage ≥80%.

**Files**:
- `alert-crawler/internal/elasticsearch/indexer_test.go` (new, ~280 lines).

**Validation**:
- All cases pass.
- Coverage ≥80%.

## Definition of Done

- All four subtasks complete.
- Indexer wraps every ES operation needed by runner and backfill.
- EnsureIndex is idempotent.
- Mapping is embedded from the contracts source.
- Coverage ≥80%.

## Risks

- **Charter tension** (resolved): in-service mapping ownership. Reviewer should reconfirm reasoning vs. routing through `index-manager`.
- **Mapping drift**: if the embedded `mapping.json` drifts from `contracts/es-index-mapping.json`, behavior diverges from spec. Mitigation: a CI check (or a `task lint:alert-crawler` step) that diffs the two files and fails on mismatch. Add this as a follow-up if not already covered.
- **ES error handling**: 4xx from ES (mapping conflict, malformed query) should be surfaced as structured errors, not silently retried.

## Reviewer Guidance

- Verify the embedded mapping matches `contracts/es-index-mapping.json` (modulo `_comment` stripping).
- Verify EnsureIndex handles the race condition.
- Verify all HTTP requests use context-aware constructors.
- Verify deterministic doc `_id` assignment (matches `Alert.ID`).

## Implementation Command

```bash
spec-kitty agent action implement WP11 --agent <name>
```

Depends on WP05, WP06.

## Activity Log

- 2026-05-06T23:01:14Z – claude:sonnet:implementer:implementer – shell_pid=257956 – Started implementation via action command
- 2026-05-06T23:08:57Z – claude:sonnet:implementer:implementer – shell_pid=257956 – ES indexer + EnsureIndex + Bulk + QueryActive (lint + 81.7% cov)
- 2026-05-06T23:09:33Z – claude:opus:reviewer:reviewer – shell_pid=261986 – Started review via action command
