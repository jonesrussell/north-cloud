# Discovered Domains Test Coverage Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add comprehensive test coverage for the discovered domains feature, introduce a TypeScript union type for domain status, and document the upsert atomicity trade-off.

**Architecture:** Interface-based mocking for handler tests, go-sqlmock for repository tests, table-driven tests for pure logic. Three logical commits: (1) interfaces + domain/handler helper tests, (2) repo + handler tests + upsert comment, (3) TypeScript union type.

**Tech Stack:** Go 1.26, go-sqlmock, Gin test mode, httptest, TypeScript, Vue 3

---

### Task 1: Add Repository Interfaces

**Files:**
- Modify: `crawler/internal/database/interfaces.go` (append after line 61)

**Step 1: Add DomainStateRepositoryInterface and DomainAggregateRepositoryInterface**

Append to `crawler/internal/database/interfaces.go` after the existing `ExecutionRepositoryInterface`:

```go
// DomainStateRepositoryInterface defines the contract for domain state operations.
type DomainStateRepositoryInterface interface {
	Upsert(ctx context.Context, domainName, status string, notes *string) error
	BulkUpsert(ctx context.Context, domains []string, status string, notes *string) (int, error)
	GetByDomain(ctx context.Context, domainName string) (*domain.DomainState, error)
}

// DomainAggregateRepositoryInterface defines the contract for domain aggregate queries.
type DomainAggregateRepositoryInterface interface {
	ListAggregates(ctx context.Context, filters DomainListFilters) ([]*domain.DomainAggregate, error)
	CountAggregates(ctx context.Context, filters DomainListFilters) (int, error)
	GetReferringSources(ctx context.Context, domainName string) ([]string, error)
	ListLinksByDomain(ctx context.Context, domainName string, limit, offset int) ([]*domain.DiscoveredLink, int, error)
}
```

Note: The `domain` package is already imported in this file. `DomainListFilters` is defined in `domain_aggregate_repository.go` in the same `database` package.

**Step 2: Run tests to verify no compile errors**

Run: `cd crawler && go build ./...`
Expected: Clean build, no errors

---

### Task 2: Update Handler to Use Interfaces

**Files:**
- Modify: `crawler/internal/api/discovered_domains_handler.go` (lines 22-38)
- Modify: `crawler/internal/bootstrap/services.go` (line 86-88 — no change needed if concrete types satisfy interfaces)

**Step 1: Change handler struct fields from concrete to interface types**

In `crawler/internal/api/discovered_domains_handler.go`, replace lines 22-38:

```go
// DiscoveredDomainsHandler handles discovered domain HTTP requests.
type DiscoveredDomainsHandler struct {
	aggregateRepo database.DomainAggregateRepositoryInterface
	stateRepo     database.DomainStateRepositoryInterface
	log           infralogger.Logger
}

// NewDiscoveredDomainsHandler creates a new discovered domains handler.
func NewDiscoveredDomainsHandler(
	aggregateRepo database.DomainAggregateRepositoryInterface,
	stateRepo database.DomainStateRepositoryInterface,
	log infralogger.Logger,
) *DiscoveredDomainsHandler {
	return &DiscoveredDomainsHandler{
		aggregateRepo: aggregateRepo,
		stateRepo:     stateRepo,
		log:           log,
	}
}
```

**Step 2: Run tests to verify no breakage**

Run: `cd crawler && go build ./... && go test ./...`
Expected: Clean build, all existing tests pass. The concrete `*DomainStateRepository` and `*DomainAggregateRepository` implicitly satisfy the new interfaces.

---

### Task 3: Write Domain Logic Tests — ComputeQualityScore

**Files:**
- Create: `crawler/internal/domain/discovered_domain_test.go`

**Step 1: Write table-driven tests for ComputeQualityScore**

```go
package domain_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func floatPtr(f float64) *float64 { return &f }

func TestComputeQualityScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		agg      domain.DomainAggregate
		wantMin  int
		wantMax  int
	}{
		{
			name: "nil ratios zero sources old date",
			agg: domain.DomainAggregate{
				OKRatio:     nil,
				HTMLRatio:   nil,
				SourceCount: 0,
				LastSeen:    time.Now().Add(-60 * 24 * time.Hour), // 60 days ago
			},
			wantMin: 0,
			wantMax: 0,
		},
		{
			name: "perfect scores",
			agg: domain.DomainAggregate{
				OKRatio:     floatPtr(1.0),
				HTMLRatio:   floatPtr(1.0),
				SourceCount: 5,
				LastSeen:    time.Now(),
			},
			wantMin: 99, // rounding may cause 99 instead of 100
			wantMax: 100,
		},
		{
			name: "only ok ratio set",
			agg: domain.DomainAggregate{
				OKRatio:     floatPtr(1.0),
				HTMLRatio:   nil,
				SourceCount: 0,
				LastSeen:    time.Now().Add(-60 * 24 * time.Hour),
			},
			wantMin: 30,
			wantMax: 30,
		},
		{
			name: "source count capped at 5",
			agg: domain.DomainAggregate{
				OKRatio:     nil,
				HTMLRatio:   nil,
				SourceCount: 10, // well above cap of 5
				LastSeen:    time.Now().Add(-60 * 24 * time.Hour),
			},
			wantMin: 20, // 20 * min(10/5, 1.0) = 20
			wantMax: 20,
		},
		{
			name: "recency half decay at 15 days",
			agg: domain.DomainAggregate{
				OKRatio:     nil,
				HTMLRatio:   nil,
				SourceCount: 0,
				LastSeen:    time.Now().Add(-15 * 24 * time.Hour),
			},
			wantMin: 9,  // ~10 (20 * 0.5) but with hour rounding
			wantMax: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			agg := tt.agg
			agg.ComputeQualityScore()

			if agg.QualityScore < tt.wantMin || agg.QualityScore > tt.wantMax {
				t.Errorf("QualityScore = %d, want [%d, %d]", agg.QualityScore, tt.wantMin, tt.wantMax)
			}
		})
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd crawler && go test -v ./internal/domain/... -run TestComputeQualityScore`
Expected: PASS (5 sub-tests)

---

### Task 4: Write Domain Logic Tests — ExtractDomain

**Files:**
- Modify: `crawler/internal/domain/discovered_domain_test.go` (append)

**Step 1: Add table-driven tests for ExtractDomain**

Append to `crawler/internal/domain/discovered_domain_test.go`:

```go
func TestExtractDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"strips www prefix", "https://www.example.com/page", "example.com"},
		{"no www", "https://example.com", "example.com"},
		{"preserves subdomain", "http://sub.example.com/path", "sub.example.com"},
		{"empty string", "", ""},
		{"invalid url", "://bad", ""},
		{"just domain no scheme", "example.com", ""}, // url.Parse treats as path, no host
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.ExtractDomain(tt.url)
			if got != tt.want {
				t.Errorf("ExtractDomain(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it passes**

Run: `cd crawler && go test -v ./internal/domain/... -run TestExtractDomain`
Expected: PASS (6 sub-tests)

---

### Task 5: Write Handler Helper Tests

**Files:**
- Create: `crawler/internal/api/discovered_domains_handler_test.go`

Note: This file uses `package api` (not `api_test`) to access unexported functions `isValidDomainStatus`, `extractPathPattern`, `extractPath`, `computePathClusters`. Add the `//nolint:testpackage` directive like `helpers_test.go` and `internal_handler_test.go`.

**Step 1: Write tests for isValidDomainStatus, extractPathPattern, extractPath**

```go
//nolint:testpackage // Testing unexported handler functions
package api

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func TestIsValidDomainStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status string
		want   bool
	}{
		{domain.DomainStatusActive, true},
		{domain.DomainStatusIgnored, true},
		{domain.DomainStatusReviewing, true},
		{domain.DomainStatusPromoted, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			t.Parallel()
			got := isValidDomainStatus(tt.status)
			if got != tt.want {
				t.Errorf("isValidDomainStatus(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestExtractPathPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"multi segment", "https://example.com/news/article/123", "/news/*"},
		{"single segment", "https://example.com/about", "/about"},
		{"root with slash", "https://example.com/", "/"},
		{"root no slash", "https://example.com", "/"},
		{"invalid url", "://bad", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPathPattern(tt.url)
			if got != tt.want {
				t.Errorf("extractPathPattern(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestExtractPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		url  string
		want string
	}{
		{"normal path", "https://example.com/news/article", "/news/article"},
		{"root", "https://example.com", "/"},
		{"root with slash", "https://example.com/", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractPath(tt.url)
			if got != tt.want {
				t.Errorf("extractPath(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run tests**

Run: `cd crawler && go test -v ./internal/api/... -run "TestIsValidDomainStatus|TestExtractPathPattern|TestExtractPath"`
Expected: PASS

---

### Task 6: Write computePathClusters Tests

**Files:**
- Modify: `crawler/internal/api/discovered_domains_handler_test.go` (append)

**Step 1: Add tests for computePathClusters**

Append to the same file:

```go
func TestComputePathClusters(t *testing.T) {
	t.Parallel()

	t.Run("empty input", func(t *testing.T) {
		t.Parallel()
		clusters := computePathClusters(nil)
		if len(clusters) != 0 {
			t.Errorf("expected 0 clusters, got %d", len(clusters))
		}
	})

	t.Run("groups and sorts by count descending", func(t *testing.T) {
		t.Parallel()
		links := []*domain.DiscoveredLink{
			{URL: "https://example.com/news/article1"},
			{URL: "https://example.com/news/article2"},
			{URL: "https://example.com/news/article3"},
			{URL: "https://example.com/about"},
		}

		clusters := computePathClusters(links)

		if len(clusters) != 2 {
			t.Fatalf("expected 2 clusters, got %d", len(clusters))
		}
		// First cluster should be /news/* with count 3 (sorted descending)
		if clusters[0].Pattern != "/news/*" {
			t.Errorf("first cluster pattern = %q, want /news/*", clusters[0].Pattern)
		}
		if clusters[0].Count != 3 {
			t.Errorf("first cluster count = %d, want 3", clusters[0].Count)
		}
		if clusters[1].Pattern != "/about" {
			t.Errorf("second cluster pattern = %q, want /about", clusters[1].Pattern)
		}
		if clusters[1].Count != 1 {
			t.Errorf("second cluster count = %d, want 1", clusters[1].Count)
		}
	})
}
```

Note: This test needs the `domain` import. Add it to the import block:
```go
import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)
```

**Step 2: Run tests**

Run: `cd crawler && go test -v ./internal/api/... -run TestComputePathClusters`
Expected: PASS

---

### Task 7: Commit 1 — Interfaces + Domain Logic + Handler Helpers

**Step 1: Run full test suite**

Run: `cd crawler && go test ./...`
Expected: All tests pass

**Step 2: Run linter**

Run: `cd crawler && golangci-lint run`
Expected: No new violations

**Step 3: Commit**

```bash
git add crawler/internal/database/interfaces.go \
       crawler/internal/api/discovered_domains_handler.go \
       crawler/internal/domain/discovered_domain_test.go \
       crawler/internal/api/discovered_domains_handler_test.go
git commit -m "test: add interfaces and unit tests for discovered domains

Add DomainStateRepositoryInterface and DomainAggregateRepositoryInterface
to enable mock-based testing. Update handler to accept interfaces.

Add tests for ComputeQualityScore, ExtractDomain, isValidDomainStatus,
extractPathPattern, extractPath, and computePathClusters.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 8: Write DomainStateRepository Tests (sqlmock)

**Files:**
- Modify: `crawler/internal/database/domain_state_repository_test.go` (replace existing)

**Step 1: Replace the existing minimal test with sqlmock-based tests**

```go
package database_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

func newStateRepo(t *testing.T) (*database.DomainStateRepository, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	t.Cleanup(func() { mockDB.Close() })

	db := sqlx.NewDb(mockDB, "postgres")

	return database.NewDomainStateRepository(db), mock
}

func stringPtr(s string) *string { return &s }

func TestDomainStateRepository_Upsert(t *testing.T) {
	repo, mock := newStateRepo(t)
	ctx := context.Background()

	// Expect INSERT ON CONFLICT
	mock.ExpectExec("INSERT INTO discovered_domain_states").
		WithArgs("example.com", domain.DomainStatusIgnored, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Expect status timestamp update for "ignored"
	mock.ExpectExec("UPDATE discovered_domain_states SET ignored_at").
		WithArgs("example.com").
		WillReturnResult(sqlmock.NewResult(0, 1))

	notes := stringPtr("spam domain")
	err := repo.Upsert(ctx, "example.com", domain.DomainStatusIgnored, notes)
	if err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainStateRepository_GetByDomain_Found(t *testing.T) {
	repo, mock := newStateRepo(t)
	ctx := context.Background()

	rows := sqlmock.NewRows([]string{
		"domain", "status", "notes", "ignored_at", "ignored_by",
		"promoted_at", "promoted_source_id", "created_at", "updated_at",
	}).AddRow("example.com", "active", nil, nil, nil, nil, nil,
		sqlmock.AnyArg(), sqlmock.AnyArg())

	mock.ExpectQuery("SELECT \\* FROM discovered_domain_states WHERE domain").
		WithArgs("example.com").
		WillReturnRows(rows)

	state, err := repo.GetByDomain(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetByDomain() error = %v", err)
	}

	if state == nil {
		t.Fatal("GetByDomain() returned nil, want state")
	}

	if state.Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", state.Domain, "example.com")
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainStateRepository_GetByDomain_NotFound(t *testing.T) {
	repo, mock := newStateRepo(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT \\* FROM discovered_domain_states WHERE domain").
		WithArgs("nonexistent.com").
		WillReturnRows(sqlmock.NewRows(nil))

	state, err := repo.GetByDomain(ctx, "nonexistent.com")
	if err != nil {
		t.Fatalf("GetByDomain() error = %v", err)
	}

	if state != nil {
		t.Errorf("GetByDomain() = %v, want nil", state)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainStateRepository_BulkUpsert(t *testing.T) {
	repo, mock := newStateRepo(t)
	ctx := context.Background()

	domains := []string{"a.com", "b.com"}
	notes := stringPtr("bulk review")

	// Expect transaction
	mock.ExpectBegin()

	// Expect bulk INSERT
	mock.ExpectExec("INSERT INTO discovered_domain_states").
		WillReturnResult(sqlmock.NewResult(0, 2))

	// Expect bulk timestamp update (status=ignored triggers ignored_at)
	mock.ExpectExec("UPDATE discovered_domain_states SET ignored_at").
		WillReturnResult(sqlmock.NewResult(0, 2))

	mock.ExpectCommit()

	count, err := repo.BulkUpsert(ctx, domains, domain.DomainStatusIgnored, notes)
	if err != nil {
		t.Fatalf("BulkUpsert() error = %v", err)
	}

	if count != 2 {
		t.Errorf("BulkUpsert() count = %d, want 2", count)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}
```

**Step 2: Run tests**

Run: `cd crawler && go test -v ./internal/database/... -run "TestDomainState"`
Expected: PASS (4 tests)

---

### Task 9: Write DomainAggregateRepository Tests (sqlmock)

**Files:**
- Create: `crawler/internal/database/domain_aggregate_repository_test.go`

**Step 1: Write sqlmock tests for aggregate operations**

```go
package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
)

func newAggregateRepo(t *testing.T) (*database.DomainAggregateRepository, sqlmock.Sqlmock) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}

	t.Cleanup(func() { mockDB.Close() })

	db := sqlx.NewDb(mockDB, "postgres")

	return database.NewDomainAggregateRepository(db), mock
}

func TestDomainAggregateRepository_ListAggregates(t *testing.T) {
	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	now := time.Now()
	rows := sqlmock.NewRows([]string{
		"domain", "status", "link_count", "source_count", "avg_depth",
		"first_seen", "last_seen", "ok_ratio", "html_ratio", "notes",
	}).AddRow("example.com", "active", 42, 3, 1.5, now, now, 0.95, 0.80, nil)

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	filters := database.DomainListFilters{Limit: 25}
	results, err := repo.ListAggregates(ctx, filters)
	if err != nil {
		t.Fatalf("ListAggregates() error = %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("ListAggregates() returned %d results, want 1", len(results))
	}

	if results[0].Domain != "example.com" {
		t.Errorf("Domain = %q, want %q", results[0].Domain, "example.com")
	}

	if results[0].LinkCount != 42 {
		t.Errorf("LinkCount = %d, want 42", results[0].LinkCount)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainAggregateRepository_CountAggregates(t *testing.T) {
	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(15))

	count, err := repo.CountAggregates(ctx, database.DomainListFilters{})
	if err != nil {
		t.Fatalf("CountAggregates() error = %v", err)
	}

	if count != 15 {
		t.Errorf("CountAggregates() = %d, want 15", count)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainAggregateRepository_GetReferringSources(t *testing.T) {
	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	mock.ExpectQuery("SELECT DISTINCT source_name").
		WithArgs("example.com").
		WillReturnRows(sqlmock.NewRows([]string{"source_name"}).
			AddRow("cbc.ca").
			AddRow("toronto-star"))

	sources, err := repo.GetReferringSources(ctx, "example.com")
	if err != nil {
		t.Fatalf("GetReferringSources() error = %v", err)
	}

	if len(sources) != 2 {
		t.Fatalf("GetReferringSources() returned %d, want 2", len(sources))
	}

	if sources[0] != "cbc.ca" {
		t.Errorf("sources[0] = %q, want %q", sources[0], "cbc.ca")
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestDomainAggregateRepository_ListLinksByDomain(t *testing.T) {
	repo, mock := newAggregateRepo(t)
	ctx := context.Background()

	now := time.Now()
	var httpStatus int16 = 200

	// Expect links query
	mock.ExpectQuery("SELECT id, source_id").
		WithArgs("example.com", 25, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "source_id", "source_name", "url", "parent_url", "depth",
			"domain", "http_status", "content_type", "discovered_at",
			"queued_at", "status", "priority", "created_at", "updated_at",
		}).AddRow("link-1", "src-1", "CBC News", "https://example.com/page",
			nil, 1, "example.com", httpStatus, "text/html", now,
			now, "pending", 0, now, now))

	// Expect count query
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("example.com").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	links, total, err := repo.ListLinksByDomain(ctx, "example.com", 25, 0)
	if err != nil {
		t.Fatalf("ListLinksByDomain() error = %v", err)
	}

	if len(links) != 1 {
		t.Fatalf("ListLinksByDomain() returned %d links, want 1", len(links))
	}

	if total != 50 {
		t.Errorf("total = %d, want 50", total)
	}

	if links[0].URL != "https://example.com/page" {
		t.Errorf("URL = %q, want %q", links[0].URL, "https://example.com/page")
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}
```

**Step 2: Run tests**

Run: `cd crawler && go test -v ./internal/database/... -run "TestDomainAggregate"`
Expected: PASS (4 tests)

---

### Task 10: Write normalizeDomainSort Tests

**Files:**
- Modify: `crawler/internal/database/domain_aggregate_repository_test.go` (append)

Note: `normalizeDomainSort` is unexported. Either:
- (a) Change the test package to `package database` (internal test), or
- (b) Test it indirectly through `buildDomainAggregateQuery` (also unexported).

Best approach: Create a separate test file in package `database` (not `database_test`) for unexported function tests.

**Step 1: Create internal test file for unexported functions**

Create `crawler/internal/database/domain_aggregate_internal_test.go`:

```go
//nolint:testpackage // Testing unexported normalizeDomainSort
package database

import (
	"testing"
)

func TestNormalizeDomainSort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantCol   string
		wantOrder string
	}{
		{"valid link_count desc", "link_count", "desc", "link_count", "DESC"},
		{"valid source_count asc", "source_count", "asc", "source_count", "ASC"},
		{"valid domain", "domain", "desc", "dl.domain", "DESC"},
		{"valid last_seen", "last_seen", "asc", "last_seen", "ASC"},
		{"invalid sort defaults to link_count", "bogus", "desc", "link_count", "DESC"},
		{"invalid order defaults to DESC", "link_count", "bogus", "link_count", "DESC"},
		{"empty sort defaults to link_count", "", "desc", "link_count", "DESC"},
		{"empty order defaults to DESC", "link_count", "", "link_count", "DESC"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotCol, gotOrder := normalizeDomainSort(tt.sortBy, tt.sortOrder)
			if gotCol != tt.wantCol {
				t.Errorf("column = %q, want %q", gotCol, tt.wantCol)
			}
			if gotOrder != tt.wantOrder {
				t.Errorf("order = %q, want %q", gotOrder, tt.wantOrder)
			}
		})
	}
}
```

**Step 2: Run tests**

Run: `cd crawler && go test -v ./internal/database/... -run TestNormalizeDomainSort`
Expected: PASS (8 sub-tests)

---

### Task 11: Write Handler Tests — Mock Repos + HTTP Tests

**Files:**
- Modify: `crawler/internal/api/discovered_domains_handler_test.go` (append mock types + HTTP tests)

**Step 1: Add mock repo types and HTTP handler tests**

Append to `crawler/internal/api/discovered_domains_handler_test.go`:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// --- Mock types ---

type mockAggregateRepo struct {
	listFn    func(ctx context.Context, f database.DomainListFilters) ([]*domain.DomainAggregate, error)
	countFn   func(ctx context.Context, f database.DomainListFilters) (int, error)
	sourcesFn func(ctx context.Context, d string) ([]string, error)
	linksFn   func(ctx context.Context, d string, l, o int) ([]*domain.DiscoveredLink, int, error)
}

func (m *mockAggregateRepo) ListAggregates(ctx context.Context, f database.DomainListFilters) ([]*domain.DomainAggregate, error) {
	return m.listFn(ctx, f)
}

func (m *mockAggregateRepo) CountAggregates(ctx context.Context, f database.DomainListFilters) (int, error) {
	return m.countFn(ctx, f)
}

func (m *mockAggregateRepo) GetReferringSources(ctx context.Context, d string) ([]string, error) {
	return m.sourcesFn(ctx, d)
}

func (m *mockAggregateRepo) ListLinksByDomain(ctx context.Context, d string, l, o int) ([]*domain.DiscoveredLink, int, error) {
	return m.linksFn(ctx, d, l, o)
}

type mockStateRepo struct {
	upsertFn     func(ctx context.Context, d, s string, n *string) error
	bulkUpsertFn func(ctx context.Context, ds []string, s string, n *string) (int, error)
	getByFn      func(ctx context.Context, d string) (*domain.DomainState, error)
}

func (m *mockStateRepo) Upsert(ctx context.Context, d, s string, n *string) error {
	return m.upsertFn(ctx, d, s, n)
}

func (m *mockStateRepo) BulkUpsert(ctx context.Context, ds []string, s string, n *string) (int, error) {
	return m.bulkUpsertFn(ctx, ds, s, n)
}

func (m *mockStateRepo) GetByDomain(ctx context.Context, d string) (*domain.DomainState, error) {
	return m.getByFn(ctx, d)
}

// --- Test helpers ---

func setupDomainsRouter(aggRepo *mockAggregateRepo, stateRepo *mockStateRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewDiscoveredDomainsHandler(aggRepo, stateRepo, infralogger.NewNop())

	g := r.Group("/api/v1/discovered-domains")
	g.GET("", h.ListDomains)
	g.GET("/:domain", h.GetDomain)
	g.GET("/:domain/links", h.ListDomainLinks)
	g.PATCH("/:domain/state", h.UpdateDomainState)
	g.POST("/bulk-state", h.BulkUpdateDomainState)

	return r
}

func defaultAggregateRepo() *mockAggregateRepo {
	now := time.Now()
	return &mockAggregateRepo{
		listFn: func(_ context.Context, _ database.DomainListFilters) ([]*domain.DomainAggregate, error) {
			return []*domain.DomainAggregate{
				{Domain: "example.com", Status: "active", LinkCount: 10, LastSeen: now},
			}, nil
		},
		countFn: func(_ context.Context, _ database.DomainListFilters) (int, error) {
			return 1, nil
		},
		sourcesFn: func(_ context.Context, _ string) ([]string, error) {
			return []string{"cbc.ca"}, nil
		},
		linksFn: func(_ context.Context, _ string, _, _ int) ([]*domain.DiscoveredLink, int, error) {
			return []*domain.DiscoveredLink{}, 0, nil
		},
	}
}

func defaultStateRepo() *mockStateRepo {
	return &mockStateRepo{
		upsertFn: func(_ context.Context, _, _ string, _ *string) error {
			return nil
		},
		bulkUpsertFn: func(_ context.Context, ds []string, _ string, _ *string) (int, error) {
			return len(ds), nil
		},
		getByFn: func(_ context.Context, _ string) (*domain.DomainState, error) {
			return nil, nil
		},
	}
}

// --- HTTP handler tests ---

func TestListDomains_OK(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discovered-domains?limit=25", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	domains, ok := body["domains"].([]any)
	if !ok || len(domains) != 1 {
		t.Errorf("expected 1 domain in response, got %v", body["domains"])
	}
}

func TestGetDomain_OK(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discovered-domains/example.com", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestGetDomain_NotFound(t *testing.T) {
	aggRepo := defaultAggregateRepo()
	aggRepo.listFn = func(_ context.Context, _ database.DomainListFilters) ([]*domain.DomainAggregate, error) {
		return []*domain.DomainAggregate{}, nil
	}

	router := setupDomainsRouter(aggRepo, defaultStateRepo())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/discovered-domains/missing.com", http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusNotFound, w.Body.String())
	}
}

func TestUpdateDomainState_OK(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"status":"reviewing"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/discovered-domains/example.com/state",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestUpdateDomainState_InvalidStatus(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"status":"bogus"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/discovered-domains/example.com/state",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestBulkUpdateDomainState_OK(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"domains":["a.com","b.com"],"status":"ignored"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovered-domains/bulk-state",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestBulkUpdateDomainState_EmptyDomains(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	body := `{"domains":[],"status":"ignored"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovered-domains/bulk-state",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestBulkUpdateDomainState_ExceedsMax(t *testing.T) {
	router := setupDomainsRouter(defaultAggregateRepo(), defaultStateRepo())

	// Build a JSON array with 101 domains
	domains := make([]string, 101)
	for i := range domains {
		domains[i] = "domain" + string(rune('0'+i%10)) + ".com"
	}
	domainsJSON, _ := json.Marshal(domains)
	body := `{"domains":` + string(domainsJSON) + `,"status":"ignored"}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/discovered-domains/bulk-state",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}
```

Important: The full import block for this file should be:

```go
import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)
```

**Step 2: Run tests**

Run: `cd crawler && go test -v ./internal/api/... -run "TestListDomains|TestGetDomain|TestUpdateDomainState|TestBulkUpdateDomainState"`
Expected: PASS (8 tests)

---

### Task 12: Add Upsert Atomicity Comment

**Files:**
- Modify: `crawler/internal/database/domain_state_repository.go` (line 28)

**Step 1: Update the Upsert doc comment**

Replace the comment at line 28:

```go
// Upsert creates or updates a domain state record.
// The INSERT ... ON CONFLICT is atomic. The follow-up setStatusTimestamp
// is a separate statement; a crash between them would leave the timestamp
// stale but not corrupt state. Accepted trade-off for simplicity.
```

**Step 2: Verify build**

Run: `cd crawler && go build ./...`
Expected: Clean build

---

### Task 13: Commit 2 — Repository Tests + Handler Tests + Upsert Comment

**Step 1: Run full test suite**

Run: `cd crawler && go test ./...`
Expected: All tests pass

**Step 2: Run linter**

Run: `cd crawler && golangci-lint run`
Expected: No new violations

**Step 3: Commit**

```bash
git add crawler/internal/database/domain_state_repository_test.go \
       crawler/internal/database/domain_aggregate_repository_test.go \
       crawler/internal/database/domain_aggregate_internal_test.go \
       crawler/internal/api/discovered_domains_handler_test.go \
       crawler/internal/database/domain_state_repository.go
git commit -m "test: add repository and handler tests for discovered domains

Add sqlmock-based tests for DomainStateRepository (Upsert, GetByDomain,
BulkUpsert) and DomainAggregateRepository (ListAggregates, CountAggregates,
GetReferringSources, ListLinksByDomain, normalizeDomainSort).

Add HTTP handler tests with mock repos for all five endpoints.

Document upsert atomicity trade-off in code comment.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 14: TypeScript DomainStatus Union Type

**Files:**
- Modify: `dashboard/src/features/intake/api/discoveredDomains.ts`

**Step 1: Add DomainStatus type and update interfaces**

At line 8 (before the `DiscoveredDomain` interface), add:

```typescript
export type DomainStatus = 'active' | 'ignored' | 'reviewing' | 'promoted'
```

Update the interfaces:
- `DiscoveredDomain.status`: change `string` → `DomainStatus`
- `DiscoveredDomainLink.status`: change `string` → `DomainStatus`
- `DiscoveredDomainFilters.status`: change `string` → `DomainStatus`

**Step 2: Check for compile errors in dashboard**

Run: `cd dashboard && npx vue-tsc --noEmit 2>&1 | head -20`
Expected: No new errors. If there are errors, they'll be in components comparing status to string literals — those should already be valid `DomainStatus` values.

**Step 3: Run dashboard lint**

Run: `cd dashboard && npm run lint`
Expected: No new violations

---

### Task 15: Commit 3 — TypeScript Union Type

**Step 1: Commit**

```bash
git add dashboard/src/features/intake/api/discoveredDomains.ts
git commit -m "refactor(dashboard): add DomainStatus union type

Replace string type with DomainStatus union ('active' | 'ignored' |
'reviewing' | 'promoted') for DiscoveredDomain, DiscoveredDomainLink,
and DiscoveredDomainFilters interfaces.

Co-Authored-By: Claude Opus 4.6 <noreply@anthropic.com>"
```

---

### Task 16: Final Verification + PR

**Step 1: Run full Go test suite**

Run: `cd crawler && go test ./...`
Expected: All pass

**Step 2: Run linter**

Run: `task lint:crawler`
Expected: Clean

**Step 3: Run dashboard checks**

Run: `cd dashboard && npm run lint`
Expected: Clean

**Step 4: Push branch and create PR**

Use `superpowers:finishing-a-development-branch` skill to handle branch push and PR creation.
