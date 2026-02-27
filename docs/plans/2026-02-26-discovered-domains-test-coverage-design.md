# Discovered Domains: Test Coverage, TypeScript Union Type, Upsert Documentation

**Date**: 2026-02-26
**PR Scope**: Follow-up to PR #103 — addresses three deferred code review items

## Context

PR #103 shipped the discovered domains feature (aggregation, quality scoring, dashboard views). The code review flagged three items as deferred:

1. **Test coverage** — zero tests for core business logic
2. **DomainStatus union type** — TypeScript uses `string` instead of a union type
3. **Upsert without transaction** — single Upsert() uses two separate statements

## Decisions

- **Testing approach**: Interface mocks (no testcontainers). Define repository interfaces, use manual mock structs in handler tests, use go-sqlmock for repository tests.
- **Repository tests**: Happy paths only — no error/edge case coverage in this PR.
- **Upsert**: Documentation-only change (code comment explaining the atomicity trade-off).

## Design

### 1. Repository Interfaces

Add to `crawler/internal/database/interfaces.go`:

- `DomainStateRepositoryInterface` — Upsert, BulkUpsert, GetByDomain
- `DomainAggregateRepositoryInterface` — ListAggregates, CountAggregates, GetReferringSources, ListLinksByDomain

Update `DiscoveredDomainsHandler` to accept interfaces instead of concrete types. No behavioral change.

### 2. Domain Logic Tests

**File**: `crawler/internal/domain/discovered_domain_test.go`

Table-driven tests for:
- `ComputeQualityScore()` — nil ratios, perfect score, partial ratios, source cap, recency decay
- `ExtractDomain()` — www stripping, subdomains, empty/invalid input

### 3. Handler Helper Tests

**File**: `crawler/internal/api/discovered_domains_handler_test.go`

Table-driven tests for:
- `isValidDomainStatus()` — all four valid statuses + invalid
- `extractPathPattern()` — multi-segment, single-segment, root, invalid
- `extractPath()` — normal path, root
- `computePathClusters()` — empty input, grouping, sort order

### 4. Repository Tests (sqlmock, happy paths)

**File**: `crawler/internal/database/domain_state_repository_test.go`

- `Upsert` — success with "ignored" status
- `GetByDomain` — found, not found (nil,nil)
- `BulkUpsert` — success with 2 domains

**File**: `crawler/internal/database/domain_aggregate_repository_test.go`

- `ListAggregates` — returns results
- `CountAggregates` — returns count
- `GetReferringSources` — returns sources
- `ListLinksByDomain` — returns links + total
- `normalizeDomainSort` — table-driven sort validation

### 5. Handler Tests (mock repos)

Same file as helper tests. Manual mock structs implementing the new interfaces.

Tests using `httptest` + Gin test mode:
- `ListDomains` — 200
- `GetDomain` — 200 exact match, 404 not found
- `UpdateDomainState` — 200 success, 400 invalid status
- `BulkUpdateDomainState` — 200 success, 400 empty domains, 400 exceeds max

### 6. TypeScript DomainStatus Union Type

**File**: `dashboard/src/features/intake/api/discoveredDomains.ts`

Add `DomainStatus` type (`'active' | 'ignored' | 'reviewing' | 'promoted'`), apply to `DiscoveredDomain.status`, `DiscoveredDomainLink.status`, and `DiscoveredDomainFilters.status`.

### 7. Upsert Atomicity Comment

Add code comment to `DomainStateRepository.Upsert()` explaining the two-statement design: INSERT ON CONFLICT is atomic, follow-up timestamp UPDATE is separate, crash between them leaves timestamp stale but not corrupt. Accepted trade-off for simplicity.

## Commit Plan

1. **Commit 1**: Interfaces + domain logic tests + handler helper tests
2. **Commit 2**: Repository tests (sqlmock) + handler tests (mock repos) + upsert comment
3. **Commit 3**: TypeScript DomainStatus union type
