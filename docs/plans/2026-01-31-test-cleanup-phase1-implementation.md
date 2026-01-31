# Test Cleanup Phase 1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove dead tests, orphaned benchmark files, and unused production code across all services.

**Architecture:** Systematic deletion of files and code identified as providing no value - interface-check stubs, orphaned benchmarks testing fictional code, and dead production functions.

**Tech Stack:** Go 1.24+, golangci-lint, go test

---

## Pre-Implementation Verification

Before starting, run full test suite to establish baseline:

```bash
cd /home/fsd42/dev/north-cloud
for dir in crawler source-manager classifier publisher auth search; do
  echo "=== $dir ===" && cd $dir && go test ./... 2>&1 | tail -5 && cd ..
done
```

---

### Task 1: Delete Crawler Interface-Check Tests

**Files:**
- Delete: `crawler/internal/database/job_repository_automgmt_test.go`
- Delete: `crawler/internal/database/job_repository_migration_test.go`

**Step 1: Verify files exist and contain only interface checks**

Run: `cat crawler/internal/database/job_repository_automgmt_test.go | head -20`
Expected: Interface signature checks like `var _ interface { ... } = (*database.JobRepository)(nil)`

**Step 2: Delete the files**

```bash
rm crawler/internal/database/job_repository_automgmt_test.go
rm crawler/internal/database/job_repository_migration_test.go
```

**Step 3: Verify tests still pass**

Run: `cd crawler && go test ./internal/database/... -v`
Expected: PASS (remaining tests, if any, should pass)

**Step 4: Commit**

```bash
git add -A crawler/internal/database/
git commit -m "test(crawler): remove trivial interface-check tests

These tests only verified method signatures exist, not actual behavior.
They provided no coverage and passed even if implementations were deleted."
```

---

### Task 2: Delete Source-Manager Dead Handler Tests

**Files:**
- Delete: `source-manager/internal/handlers/source_test.go`
- Delete: `source-manager/internal/handlers/handlers_bench_test.go`

**Step 1: Verify source_test.go is all skipped**

Run: `grep -c "t.Skip" source-manager/internal/handlers/source_test.go`
Expected: Multiple occurrences (all tests skipped)

**Step 2: Verify handlers_bench_test.go tests fictional code**

Run: `head -30 source-manager/internal/handlers/handlers_bench_test.go`
Expected: Test-only `Source` struct not matching production

**Step 3: Delete the files**

```bash
rm source-manager/internal/handlers/source_test.go
rm source-manager/internal/handlers/handlers_bench_test.go
```

**Step 4: Verify tests still pass**

Run: `cd source-manager && go test ./internal/handlers/... -v`
Expected: PASS or "no test files"

**Step 5: Commit**

```bash
git add -A source-manager/internal/handlers/
git commit -m "test(source-manager): remove skipped and fictional handler tests

- source_test.go: All tests were skipped (handlers don't use interfaces)
- handlers_bench_test.go: Benchmarks tested fictional code, not production"
```

---

### Task 3: Delete Source-Manager Publisher Tests

**Files:**
- Delete: `source-manager/internal/events/publisher_test.go`

**Step 1: Verify tests don't test real behavior**

Run: `cat source-manager/internal/events/publisher_test.go`
Expected: Comments like "We can't easily test without Redis" and tests that don't call Publish()

**Step 2: Delete the file**

```bash
rm source-manager/internal/events/publisher_test.go
```

**Step 3: Verify tests still pass**

Run: `cd source-manager && go test ./internal/events/... -v`
Expected: PASS or "no test files"

**Step 4: Commit**

```bash
git add -A source-manager/internal/events/
git commit -m "test(source-manager): remove publisher tests that don't test behavior

Tests only verified nil-receiver handling and struct construction,
not actual Redis publishing logic."
```

---

### Task 4: Remove Source-Manager Dead parseBool Function

**Files:**
- Modify: `source-manager/internal/config/config.go` (remove lines 146-149)
- Modify: `source-manager/internal/config/config_test.go` (remove parseBool tests)

**Step 1: Verify parseBool is not called in production**

Run: `grep -r "parseBool" source-manager --include="*.go" | grep -v "_test.go" | grep -v "func parseBool"`
Expected: No results (function defined but never called)

**Step 2: Find and remove parseBool from config.go**

The function is at lines 146-149:
```go
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}
```

Remove these 4 lines from `source-manager/internal/config/config.go`.

**Step 3: Find and remove parseBool tests from config_test.go**

Run: `grep -n "parseBool\|TestParseBool" source-manager/internal/config/config_test.go`
Remove any test functions that test parseBool.

**Step 4: Verify tests still pass**

Run: `cd source-manager && go test ./internal/config/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add source-manager/internal/config/
git commit -m "refactor(source-manager): remove unused parseBool function

Function was defined but never called in production code.
Boolean parsing is handled by infrastructure config loader."
```

---

### Task 5: Remove Source-Manager Trivial Validation Test

**Files:**
- Modify: `source-manager/internal/models/source_test.go` (remove TestSource_Validation)

**Step 1: Locate the trivial test**

The test is at lines 142-165. It only checks that struct fields aren't empty after being set - not actual validation logic.

**Step 2: Remove TestSource_Validation function**

Remove lines 142-165 from `source-manager/internal/models/source_test.go`:
```go
func TestSource_Validation(t *testing.T) {
	validSource := models.Source{
		ID:   "test-id",
		Name: "Test Source",
		URL:  "https://example.com",
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{
				Title: "h1",
				Body:  ".content",
			},
		},
	}

	// Test that valid source has all required fields
	if validSource.ID == "" {
		t.Error("Source.ID should not be empty")
	}
	if validSource.Name == "" {
		t.Error("Source.Name should not be empty")
	}
	if validSource.URL == "" {
		t.Error("Source.URL should not be empty")
	}
}
```

**Step 3: Verify tests still pass**

Run: `cd source-manager && go test ./internal/models/... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add source-manager/internal/models/source_test.go
git commit -m "test(source-manager): remove trivial validation test

TestSource_Validation only verified struct fields weren't empty after
being set. It tested no actual validation logic."
```

---

### Task 6: Delete Publisher Orphaned Benchmark File

**Files:**
- Delete: `publisher/internal/router/router_bench_test.go`

**Step 1: Verify file tests fictional Article struct**

Run: `head -25 publisher/internal/router/router_bench_test.go`
Expected: Test-only Article struct with different fields than production

**Step 2: Delete the file**

```bash
rm publisher/internal/router/router_bench_test.go
```

**Step 3: Verify tests still pass**

Run: `cd publisher && go test ./internal/router/... -v`
Expected: PASS or "no test files"

**Step 4: Commit**

```bash
git add -A publisher/internal/router/
git commit -m "test(publisher): remove orphaned router benchmarks

Benchmarks tested fictional Article struct and filtering logic
that doesn't exist in production (filtering is done in Elasticsearch)."
```

---

### Task 7: Remove Publisher Dead Domain Methods + Tests

**Files:**
- Modify: `publisher/internal/domain/outbox.go` (remove ShouldRetry and IsExhausted)
- Modify: `publisher/internal/worker/outbox_worker_test.go` (remove tests for dead methods + trivial tests)

**Step 1: Verify ShouldRetry and IsExhausted are not called**

Run: `grep -r "ShouldRetry\|IsExhausted" publisher --include="*.go" | grep -v "_test.go" | grep -v "func (o \*OutboxEntry)"`
Expected: No results (methods defined but never called)

**Step 2: Remove dead methods from outbox.go**

Remove lines 74-82 from `publisher/internal/domain/outbox.go`:
```go
// ShouldRetry returns true if the entry can be retried
func (o *OutboxEntry) ShouldRetry() bool {
	return o.RetryCount < o.MaxRetries
}

// IsExhausted returns true if all retries have been exhausted
func (o *OutboxEntry) IsExhausted() bool {
	return o.RetryCount >= o.MaxRetries
}
```

**Step 3: Remove tests for dead methods and trivial tests from outbox_worker_test.go**

Remove these functions:
- `TestOutboxEntry_ShouldRetry` (lines 238-261)
- `TestOutboxEntry_IsExhausted` (lines 264-287)
- `TestOutboxStats_Fields` (lines 191-215) - trivial
- `TestOutboxStatus_Constants` (lines 217-236) - trivial

**Step 4: Verify tests still pass**

Run: `cd publisher && go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/domain/outbox.go publisher/internal/worker/outbox_worker_test.go
git commit -m "refactor(publisher): remove dead domain methods and trivial tests

- Removed ShouldRetry() and IsExhausted() - never called in production
- Removed tests for dead methods
- Removed trivial tests that only verified struct fields exist"
```

---

### Task 8: Remove Publisher Dead parseBool Function

**Files:**
- Modify: `publisher/internal/config/config.go` (remove lines 235-241)

**Step 1: Verify parseBool is not called in production**

Run: `grep -r "parseBool" publisher --include="*.go" | grep -v "_test.go" | grep -v "func parseBool"`
Expected: No results

**Step 2: Remove parseBool from config.go**

Remove lines 235-241 from `publisher/internal/config/config.go`:
```go
// parseBool parses a string value as a boolean.
// Returns true for "true", "1", "yes" (case-insensitive), false otherwise.
// This function handles common boolean string representations.
func parseBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "true" || s == "1" || s == "yes"
}
```

**Step 3: Check for orphaned imports**

If `strings` package is no longer used after removal, remove it from imports.

**Step 4: Verify tests and lint pass**

Run: `cd publisher && go test ./internal/config/... -v && golangci-lint run ./internal/config/...`
Expected: PASS

**Step 5: Commit**

```bash
git add publisher/internal/config/config.go
git commit -m "refactor(publisher): remove unused parseBool function

Function was defined but never called in production code.
Boolean parsing is handled by infrastructure config loader."
```

---

### Task 9: Delete Auth Orphaned Benchmark File

**Files:**
- Delete: `auth/internal/auth/auth_bench_test.go`

**Step 1: Verify benchmarks don't call production jwt.go**

Run: `grep -E "auth\.(Generate|Validate|New)" auth/internal/auth/auth_bench_test.go`
Expected: No results (benchmarks implement JWT inline, don't call production)

**Step 2: Delete the file**

```bash
rm auth/internal/auth/auth_bench_test.go
```

**Step 3: Verify tests still pass**

Run: `cd auth && go test ./... -v`
Expected: PASS or "no test files"

**Step 4: Commit**

```bash
git add -A auth/internal/auth/
git commit -m "test(auth): remove orphaned JWT benchmarks

Benchmarks implemented JWT generation/validation inline instead of
calling production code in jwt.go. They measured stdlib performance,
not service performance."
```

---

### Task 10: Delete Search Orphaned Benchmark File

**Files:**
- Delete: `search/internal/service/search_bench_test.go`

**Step 1: Verify benchmarks don't call production search_service.go**

Run: `grep -E "service\.(Search|New)" search/internal/service/search_bench_test.go`
Expected: No results (benchmarks build ES queries inline)

**Step 2: Delete the file**

```bash
rm search/internal/service/search_bench_test.go
```

**Step 3: Verify tests still pass**

Run: `cd search && go test ./... -v`
Expected: PASS or "no test files"

**Step 4: Commit**

```bash
git add -A search/internal/service/
git commit -m "test(search): remove orphaned search benchmarks

Benchmarks built Elasticsearch queries inline instead of calling
production code in search_service.go. They measured JSON marshaling,
not actual search performance."
```

---

### Task 11: Final Verification

**Step 1: Run full test suite**

```bash
cd /home/fsd42/dev/north-cloud
for dir in crawler source-manager classifier publisher auth search index-manager mcp-north-cloud; do
  echo "=== $dir ===" && cd $dir && go test ./... 2>&1 | tail -3 && cd ..
done
```
Expected: All PASS

**Step 2: Run linters**

```bash
task lint
```
Expected: No new errors

**Step 3: Create summary commit**

```bash
git log --oneline -12
```

Verify 10 cleanup commits were made.

---

## Summary

| Task | Service | Action | Files Affected |
|------|---------|--------|----------------|
| 1 | crawler | Delete interface stubs | 2 deleted |
| 2 | source-manager | Delete dead handler tests | 2 deleted |
| 3 | source-manager | Delete publisher tests | 1 deleted |
| 4 | source-manager | Remove parseBool | 2 modified |
| 5 | source-manager | Remove trivial test | 1 modified |
| 6 | publisher | Delete router benchmarks | 1 deleted |
| 7 | publisher | Remove dead methods + tests | 2 modified |
| 8 | publisher | Remove parseBool | 1 modified |
| 9 | auth | Delete JWT benchmarks | 1 deleted |
| 10 | search | Delete search benchmarks | 1 deleted |
| 11 | all | Final verification | - |

**Total: 7 files deleted, 6 files modified, ~400 lines of dead code removed**
