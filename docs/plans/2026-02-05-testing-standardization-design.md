# Testing Standardization Design

## Status: Validated
## Date: 2026-02-05

---

## Problem

North Cloud is transitioning from prototype to platform. Multiple services interact across the crawl → classify → publish pipeline, operator-facing tools are in active use, and new services (mining-ml) are joining the pipeline.

The biggest risks are not inconsistent test patterns — they are:

- A pipeline regression that silently corrupts data
- Schema drift between classifier output and publisher expectations
- A classifier update that changes output shape
- A source-manager import that misroutes sites
- A dashboard API that lies to operators

Current state:

| Service | Test Files | Risk Level |
|---------|-----------|------------|
| Crawler | 30 | Well-tested |
| Classifier | 19 | Well-tested |
| Publisher | 9 | Adequate |
| NC-HTTP-Proxy | 7 | Adequate |
| Source-Manager | 5 | Under-tested |
| Auth | 2 | Minimal |
| Search | 2 | Minimal |
| Index-Manager | 1 | Minimal |
| MCP-North-Cloud | 0 | None |
| Mining-ML | 0 | None |
| Dashboard | 0 | None |
| Search-Frontend | 0 | None |

No integration tests exist with real databases. No contract tests protect service boundaries. No coverage thresholds are enforced. Test directory structure is inconsistent across services.

---

## Approach

**Two phases, production safety first:**

- **Phase 1 — Production Safety**: Contract tests at ES boundaries, pipeline integration suite, Tier 1 unit test gaps
- **Phase 2 — Standardization**: Consistent structure, shared utilities, coverage gates, remaining unit test gaps

---

## Phase 1: Production Safety

### Layer 1: Contract Tests

#### Principle

Index-manager already defines Elasticsearch mappings for every index type. Those mappings are the canonical schema. Any service that writes to or reads from ES must prove its documents conform.

#### Shared Contract Package

Index-manager exports mapping definitions as a shared Go package:

```
index-manager/
  pkg/
    contracts/
      contracts.go          # Mapping types and helpers
      raw_content.go        # RawContentMapping()
      classified_content.go # ClassifiedContentMapping()
```

Includes a shared assertion helper:

```go
// AssertFieldsExist validates that all required fields exist in a mapping.
func AssertFieldsExist(t *testing.T, mapping Mapping, fields []string) {
    t.Helper()
    for _, field := range fields {
        _, exists := mapping.Properties[field]
        assert.True(t, exists,
            "required field %q not found in mapping", field)
    }
}
```

#### Contracts

| Producer | Index Pattern | Consumer | What breaks if it drifts |
|----------|--------------|----------|--------------------------|
| Crawler | `*_raw_content` | Classifier | Classifier can't find fields to classify |
| Classifier | `*_classified_content` | Publisher | Publisher silently drops articles |
| Classifier | `*_classified_content` | Search | Search results missing fields |
| Index-manager | mapping definitions | All of the above | Everything |

#### Producer Contract Test (example: classifier)

The classifier builds a document. The contract test validates that every field it writes exists in the canonical mapping and has the correct type:

```go
func TestClassifierProducesValidClassifiedContent(t *testing.T) {
    mapping := contracts.ClassifiedContentMapping()

    producedFields := []string{
        "title", "body", "url", "source",
        "content_type", "quality_score", "topics",
        "crime_detected", "crime_subcategories",
        "classification_status", "classified_at",
    }

    contracts.AssertFieldsExist(t, mapping, producedFields)
}
```

#### Consumer Contract Test (example: publisher)

The publisher reads classified content. The contract test validates that every field it depends on exists in the canonical mapping:

```go
func TestPublisherExpectedClassifiedContentFields(t *testing.T) {
    mapping := contracts.ClassifiedContentMapping()

    requiredFields := []string{
        "title", "body", "url", "source",
        "content_type", "quality_score", "topics",
        "crime_detected", "crime_subcategories",
    }

    contracts.AssertFieldsExist(t, mapping, requiredFields)
}
```

#### Test Location

Each service owns its contract tests:

```
service/tests/contracts/  # or internal/contracts/ until Phase 2 standardization
```

#### CI Integration

Contract tests run on every PR alongside unit tests. They are fast (pure Go assertions, no external dependencies) and slot into the existing `test` job in `.github/workflows/test.yml`.

---

### Layer 2: Pipeline Integration Suite

#### Goal

Boot the full stack, push content through crawl → classify → publish, assert on what arrives in Redis. Catches failures that unit and contract tests cannot: connectivity issues, migration drift, timing bugs, message format mismatches.

#### Infrastructure

- `docker-compose.test.yml` — extends `docker-compose.base.yml` with test-specific config (no volume mounts, no hot reload, deterministic ports)
- nc-http-proxy runs in `replay` mode, serving fixtures from `crawler/fixtures/`
- Go test binary in `/tests/integration/pipeline/` orchestrates the flow

#### Test Sequence

```
1. docker compose up (postgres, ES, Redis, auth, source-manager,
   crawler, classifier, publisher, nc-http-proxy, index-manager)
2. Wait for health checks on all services
3. Seed: create source via source-manager API
4. Seed: create channel + route via publisher API
5. Trigger: create crawler job pointing at fixture URL
6. Wait: poll ES for classified content (with timeout)
7. Wait: subscribe to Redis channel, assert message arrives
8. Verify:
   - raw_content index has document with classification_status
   - classified_content index has quality_score, topics, content_type
   - Redis message has expected fields and values
   - All fields match contract expectations
9. docker compose down
```

#### Fixture Strategy

Start with 2-3 fixture pages covering the main content types:

1. **News article** — should classify as article, pass quality threshold, publish to news channel
2. **Listing page** — should classify as non-article, publisher should skip it
3. **Crime article** — should classify with crime detection, route to crime channel

Fixtures are version-controlled in `crawler/fixtures/`, served by nc-http-proxy in replay mode.

#### CI Integration

New workflow: `.github/workflows/integration.yml`

- Triggers on merge to main only
- Uses `docker compose -f docker-compose.base.yml -f docker-compose.test.yml`
- Timeout: 10 minutes (boot + test + teardown)

#### Local Access

```bash
task test:integration:pipeline
```

Developers run this before pushing multi-service changes.

---

### Tier 1: Unit Test Gaps

#### Source-Manager (currently 5 test files)

Priority coverage areas:

- **Importer logic** — Excel/CSV import with field mapping, header aliases, validation. This path has had recent bugs ("Website Name" header alias) and is fragile.
- **Source CRUD validation** — selector validation, URL normalization, duplicate detection
- **Test crawl endpoint** — the preview-without-saving path that operators rely on

Estimated scope: ~4-6 new test files.

#### Index-Manager (currently 1 test file)

Priority coverage areas:

- **Mapping builder functions** — `getCrimeMapping()`, `getMiningMapping()`, base content mapping. These are now the canonical contracts and must be tested.
- **Index creation/deletion** — verify correct naming patterns (`{source}_raw_content`, `{source}_classified_content`)
- **Migration logic** — index schema migrations are high-risk operations. Use a helper that applies the migration function against an in-memory ES mock and asserts the resulting mapping JSON. Keeps tests fast without needing real ES or Testcontainers.

Estimated scope: ~4-6 new test files.

Approach for both: table-driven tests, inline mocks (consistent with existing patterns), `testify/assert` + `go-sqlmock`.

---

## Phase 2: Standardization

Executed after Phase 1 is complete and the pipeline is protected.

### Test Directory Structure

Standardize across all Go services:

```
service/
  internal/           # unit tests live next to code (keep existing pattern)
  tests/
    contracts/        # contract tests against ES mappings
    integration/      # service-level integration tests (if any)
```

Crawler already has `tests/integration/` and `tests/features/`. Other services adopt the same layout.

### Shared Test Utilities

Small `infrastructure/testutil/` package:

- `AssertFieldsExist(t, mapping, fields)` — contract test helper (from Phase 1)
- `WaitForHealth(t, url, timeout)` — poll a service health endpoint
- `MustSeedSource(t, apiURL, source)` — seed test data via API

Nothing else until a third use case demands it.

### Coverage Thresholds in CI

- Add `task test:cover:check` that fails if coverage drops below threshold
- Start at current coverage per service (don't break existing builds)
- Ratchet up incrementally — never allow coverage to decrease

### Tier 2 Unit Tests (auth, dashboard)

**Auth**:
- Token generation, validation, expiry
- Middleware rejection for invalid/expired tokens
- Password hashing

**Dashboard**:
- Vitest + Vue Test Utils setup
- Auth composable tests
- API interceptor tests
- Route guard tests

### Tier 3 Unit Tests (mcp-north-cloud, mining-ml, search)

**MCP-North-Cloud**:
- Tool handler tests (input validation, error handling)
- Response format tests

**Mining-ML**:
- pytest for classifier accuracy against known samples
- API response shape tests

**Search / Search-Frontend**:
- Query building logic
- Field boosting configuration
- Response format tests

### What Standardization Does NOT Include

- Rewriting existing tests that already work
- Migrating from inline mocks to generated mocks
- Changing assertion libraries
- E2E browser tests (dashboard is operator-facing, not public)
- Testcontainers (Compose suite covers integration needs)

---

## CI Architecture (Final State)

```
PR opened/updated:
  ├── lint (changed services)
  ├── unit tests (changed services)
  ├── contract tests (changed services)
  └── vuln check

Merge to main:
  └── pipeline integration suite (full Compose stack)

Local (developer-triggered):
  └── task test:integration:pipeline
```

---

## Implementation Order

1. Create `index-manager/pkg/contracts/` shared package
2. Write contract tests for classifier (producer) and publisher (consumer)
3. Write contract tests for crawler (producer) and classifier (consumer)
4. Add contract tests to PR CI workflow
5. Create `docker-compose.test.yml`
6. Build pipeline integration test harness (`/tests/integration/pipeline/`)
7. Create fixture pages (news article, listing, crime article)
8. Add integration workflow (`.github/workflows/integration.yml`)
9. Add `task test:integration:pipeline` for local use
10. Source-manager unit tests (importer, CRUD, test crawl)
11. Index-manager unit tests (mappings, index ops, migrations)
12. Phase 2: standardize structure, add coverage gates, Tier 2/3 tests
