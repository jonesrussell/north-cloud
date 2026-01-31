# Test Audit and Coverage Improvement Plan

**Date:** 2026-01-31
**Status:** Approved
**Goal:** Remove dead tests, clean up useless code, increase critical path coverage

## Executive Summary

Analysis of 52 test files across 8 services revealed:
- 25+ tests that provide no value (interface stubs, orphaned benchmarks)
- 3 dead production functions
- Critical coverage gaps in the content pipeline (crawler → classifier → publisher)
- Services with zero meaningful tests (auth, search, index-manager, mcp-north-cloud)

## Current State

| Service | Test Files | Useful Tests | Dead/Orphaned | Coverage |
|---------|-----------|--------------|---------------|----------|
| crawler | 24 | 15 (83 tests) | 9 interface stubs | ~0% |
| source-manager | 7 | 3 | 4 | Mixed |
| classifier | 11 | 9 | 2 | 47-69% |
| publisher | 4 | 1 partial | 3 | ~1% |
| auth | 1 | 0 | 1 | 0% |
| search | 1 | 0 | 1 | 0% |
| index-manager | 0 | 0 | 0 | 0% |
| mcp-north-cloud | 0 | 0 | 0 | 0% |

---

## Phase 1: Cleanup (Remove Dead Weight)

### Files to Delete

#### Crawler
- `internal/database/job_repository_automgmt_test.go` - 6 trivial interface checks
- `internal/database/job_repository_migration_test.go` - 3 trivial interface checks

#### Source-Manager
- `internal/handlers/source_test.go` - All tests skipped, dead mock
- `internal/handlers/handlers_bench_test.go` - Fictional benchmarks

#### Publisher
- `internal/router/router_bench_test.go` - Orphaned, tests fictional Article struct

#### Auth
- `internal/auth/auth_bench_test.go` - Benchmarks inline code, not production

#### Search
- `internal/service/search_bench_test.go` - Benchmarks inline code, not production

### Dead Production Code to Remove

#### Source-Manager
- `internal/config/config.go:parseBool()` - Never called

#### Publisher
- `internal/config/config.go:parseBool()` - Never called
- `internal/domain/outbox.go:ShouldRetry()` - Never called
- `internal/domain/outbox.go:IsExhausted()` - Never called

### Tests to Remove (in files we keep)

#### Source-Manager
- `internal/events/publisher_test.go` - All 4 tests (don't test real behavior)
- `internal/models/source_test.go:TestSource_Validation` - Trivial

#### Publisher
- `internal/worker/outbox_worker_test.go` - Remove tests for ShouldRetry/IsExhausted
- `internal/worker/outbox_worker_test.go` - Remove trivial tests (TestOutboxStats_Fields, TestOutboxStatus_Constants)

---

## Phase 2: Refactor Existing Tests

### Classifier - Extract Shared Utilities

Create `classifier/internal/testhelpers/testhelpers.go`:
```go
// Shared mocks
type MockSourceReputationDB struct { ... }
type MockLogger struct { ... }

// Shared setup
func CreateTestClassifier() *classifier.Classifier { ... }
```

Update files:
- `handlers_test.go`
- `source_reputation_test.go`
- `integration_test.go`

### Classifier - Strengthen Telemetry Tests

`telemetry_test.go` - Replace "should not panic" with actual assertions:
- Verify metrics incremented
- Verify spans created with correct attributes

### Classifier - Fix Benchmarks

`classifier_bench_test.go` - Call production code:
```go
// Before (bad):
for _, keyword := range crimeKeywords {
    if strings.Contains(lowerText, keyword) { ... }
}

// After (good):
result := engine.Match(tc.title, tc.body)
```

### Crawler - Consolidate NoOp Tests

`events/noop_handler_test.go` - Merge 5 identical tests into 1:
```go
func TestNoOpHandler_AllMethods_ReturnNil(t *testing.T) {
    handler := events.NewNoOpHandler(nil)
    // Test all 5 methods in one test
}
```

---

## Phase 3: Critical Path Coverage

### Publisher (Priority 1)

#### `internal/database/outbox_repository_test.go`
Add tests for:
- `FetchPending()` - Critical: Tests `FOR UPDATE SKIP LOCKED` behavior
- `FetchRetryable()` - Critical: Tests retry queue fetching

#### `internal/worker/outbox_worker_test.go`
Add tests for:
- `publishOne()` - The actual publish loop
- `runRecovery()` - Recovery from failures
- `runCleanup()` - Cleanup of old entries

### Auth (Priority 1)

Create `internal/auth/jwt_test.go`:
- `TestGenerateToken` - Valid token generation
- `TestValidateToken` - Valid token validation
- `TestValidateToken_Expired` - Expired token rejection
- `TestValidateToken_InvalidSignature` - Tampered token rejection

Create `internal/api/handlers_test.go`:
- `TestLoginHandler_Success`
- `TestLoginHandler_InvalidCredentials`
- `TestLoginHandler_MissingFields`

### Search (Priority 1)

Create `internal/service/search_service_test.go`:
- `TestSearch_BasicQuery` - Simple text search
- `TestSearch_WithFilters` - Topic/quality filters
- `TestSearch_Pagination` - Page/size handling
- `TestSearch_EmptyResults` - No matches case

### Crawler (Priority 2)

Add integration tests with testcontainers:
- `internal/database/job_repository_test.go` - Real PostgreSQL tests
- `internal/scheduler/interval_scheduler_test.go` - Scheduler integration

---

## Phase 4: Remaining Coverage

### Index-Manager

Create tests for:
- `internal/service/index_service_test.go` - Index CRUD
- `internal/elasticsearch/client_test.go` - ES operations

### MCP-North-Cloud

Create tests for:
- `internal/mcp/tools_test.go` - Tool implementations
- `internal/client/client_test.go` - HTTP client operations

### Testcontainers Setup

Add `testcontainers` for integration tests:
- PostgreSQL container for repository tests
- Redis container for publisher tests
- Elasticsearch container for search/classifier tests

---

## Success Criteria

After all phases:

| Service | Target Coverage |
|---------|----------------|
| crawler | 40%+ |
| source-manager | 60%+ |
| classifier | 70%+ |
| publisher | 50%+ |
| auth | 80%+ |
| search | 50%+ |
| index-manager | 40%+ |
| mcp-north-cloud | 40%+ |

## Dependencies

- Go 1.24+ (all services)
- testcontainers-go for integration tests
- sqlmock for database unit tests

## Risks

1. **Integration test flakiness** - Mitigate with proper container lifecycle management
2. **Time investment** - Phase 3-4 are substantial; can be done incrementally
3. **Breaking changes during cleanup** - Run full test suite after each deletion
