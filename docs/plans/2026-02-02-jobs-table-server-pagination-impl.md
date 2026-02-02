# Jobs Table Server-Side Pagination Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix dashboard jobs table to properly display 320+ jobs with server-side pagination, sorting, and adjustable page size.

**Architecture:** Backend adds sort params to existing List/Count methods. Frontend creates reusable `useServerPaginatedTable` composable that manages pagination state and integrates with TanStack Query. Existing `useJobs` composable delegates to the new table composable.

**Tech Stack:** Go 1.24+ (crawler), Vue 3 + TypeScript (dashboard), TanStack Query, Pinia, PostgreSQL

---

## Phase 1: Backend API Enhancement

### Task 1: Add Sort Parameter Parsing Helper

**Files:**
- Modify: `crawler/internal/api/helpers.go:24` (after `parseLimitOffset`)
- Test: `crawler/internal/api/helpers_test.go` (create)

**Step 1: Write the failing test**

Create `crawler/internal/api/helpers_test.go`:

```go
package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseSortParams(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		query          string
		allowedFields  map[string]string
		defaultSortBy  string
		defaultOrder   string
		wantSortBy     string
		wantSortOrder  string
	}{
		{
			name:          "uses defaults when no params",
			query:         "",
			allowedFields: map[string]string{"created_at": "created_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "created_at",
			wantSortOrder: "desc",
		},
		{
			name:          "accepts valid sort field",
			query:         "?sort_by=next_run_at&sort_order=asc",
			allowedFields: map[string]string{"created_at": "created_at", "next_run_at": "next_run_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "next_run_at",
			wantSortOrder: "asc",
		},
		{
			name:          "falls back to default for invalid field",
			query:         "?sort_by=invalid_column",
			allowedFields: map[string]string{"created_at": "created_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "created_at",
			wantSortOrder: "desc",
		},
		{
			name:          "normalizes invalid sort order",
			query:         "?sort_by=created_at&sort_order=invalid",
			allowedFields: map[string]string{"created_at": "created_at"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "created_at",
			wantSortOrder: "desc",
		},
		{
			name:          "maps external name to internal column",
			query:         "?sort_by=source_name",
			allowedFields: map[string]string{"source_name": "COALESCE(source_name, '')"},
			defaultSortBy: "created_at",
			defaultOrder:  "desc",
			wantSortBy:    "COALESCE(source_name, '')",
			wantSortOrder: "desc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/"+tt.query, nil)

			gotSortBy, gotSortOrder := parseSortParams(c, tt.allowedFields, tt.defaultSortBy, tt.defaultOrder)

			if gotSortBy != tt.wantSortBy {
				t.Errorf("sortBy = %q, want %q", gotSortBy, tt.wantSortBy)
			}
			if gotSortOrder != tt.wantSortOrder {
				t.Errorf("sortOrder = %q, want %q", gotSortOrder, tt.wantSortOrder)
			}
		})
	}
}

func TestClampLimit(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		limit    int
		maxLimit int
		want     int
	}{
		{"normal value", 50, 250, 50},
		{"exceeds max", 500, 250, 250},
		{"zero uses max", 0, 250, 250},
		{"negative uses max", -10, 250, 250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			got := clampLimit(tt.limit, tt.maxLimit)
			if got != tt.want {
				t.Errorf("clampLimit(%d, %d) = %d, want %d", tt.limit, tt.maxLimit, got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/api/... -run "TestParseSortParams|TestClampLimit" -v`
Expected: FAIL with "undefined: parseSortParams"

**Step 3: Write minimal implementation**

Add to `crawler/internal/api/helpers.go` after line 24:

```go
// MaxPageSize is the maximum allowed page size for list endpoints.
const MaxPageSize = 250

// parseSortParams parses sort_by and sort_order query params with validation.
// allowedFields maps external names to internal column names/expressions.
// Returns the internal column name and normalized sort order.
func parseSortParams(
	c *gin.Context,
	allowedFields map[string]string,
	defaultSortBy, defaultOrder string,
) (sortBy, sortOrder string) {
	sortBy = c.DefaultQuery("sort_by", defaultSortBy)
	sortOrder = c.DefaultQuery("sort_order", defaultOrder)

	// Validate sort field against whitelist
	if column, ok := allowedFields[sortBy]; ok {
		sortBy = column
	} else {
		sortBy = allowedFields[defaultSortBy]
		if sortBy == "" {
			sortBy = defaultSortBy
		}
	}

	// Normalize sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = defaultOrder
	}

	return sortBy, sortOrder
}

// clampLimit ensures limit is within valid bounds.
func clampLimit(limit, maxLimit int) int {
	if limit <= 0 || limit > maxLimit {
		return maxLimit
	}
	return limit
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/api/... -run "TestParseSortParams|TestClampLimit" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/api/helpers.go crawler/internal/api/helpers_test.go
git commit -m "feat(crawler): add sort parameter parsing helpers

Add parseSortParams for validating sort_by/sort_order query params
with whitelist support. Add clampLimit for enforcing MaxPageSize.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 2: Update Repository List Method Signature

**Files:**
- Modify: `crawler/internal/database/job_repository.go:152-185`
- Modify: `crawler/internal/database/interfaces.go` (if exists)
- Test: `crawler/internal/database/job_repository_test.go`

**Step 1: Write the failing test**

Add to `crawler/internal/database/job_repository_test.go`:

```go
func TestJobRepository_List_WithSorting(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	// Expect query with ORDER BY next_run_at ASC NULLS LAST
	mock.ExpectQuery("SELECT .+ FROM jobs .+ ORDER BY next_run_at ASC NULLS LAST").
		WithArgs(50, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "status"}).
			AddRow("job-1", "src-1", "scheduled").
			AddRow("job-2", "src-2", "pending"))

	params := database.ListJobsParams{
		Limit:     50,
		Offset:    0,
		SortBy:    "next_run_at",
		SortOrder: "asc",
	}

	jobs, listErr := repo.List(ctx, params)
	if listErr != nil {
		t.Fatalf("List() error = %v", listErr)
	}

	if len(jobs) != 2 {
		t.Errorf("expected 2 jobs, got %d", len(jobs))
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}

func TestJobRepository_List_WithStatusFilter(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT .+ FROM jobs WHERE status = \\$1").
		WithArgs("running", 25, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "source_id", "status"}).
			AddRow("job-1", "src-1", "running"))

	params := database.ListJobsParams{
		Status:    "running",
		Limit:     25,
		Offset:    0,
		SortBy:    "created_at",
		SortOrder: "desc",
	}

	jobs, listErr := repo.List(ctx, params)
	if listErr != nil {
		t.Fatalf("List() error = %v", listErr)
	}

	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/database/... -run "TestJobRepository_List_With" -v`
Expected: FAIL with "undefined: database.ListJobsParams"

**Step 3: Write minimal implementation**

Add `ListJobsParams` struct and update `List` method in `crawler/internal/database/job_repository.go`:

```go
// ListJobsParams contains parameters for listing jobs.
type ListJobsParams struct {
	Status    string // Optional status filter
	SourceID  string // Optional source_id filter
	Search    string // Optional search term (source_name, url)
	SortBy    string // Column to sort by (already validated)
	SortOrder string // "asc" or "desc" (already validated)
	Limit     int
	Offset    int
}

// List retrieves jobs with filtering, sorting, and pagination.
func (r *JobRepository) List(ctx context.Context, params ListJobsParams) ([]*domain.Job, error) {
	var jobs []*domain.Job
	var conditions []string
	var args []any
	argIndex := 1

	// Build WHERE conditions
	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, params.Status)
		argIndex++
	}

	if params.SourceID != "" {
		conditions = append(conditions, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, params.SourceID)
		argIndex++
	}

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(COALESCE(source_name, '') ILIKE $%d OR url ILIKE $%d)",
			argIndex, argIndex,
		))
		args = append(args, "%"+params.Search+"%")
		argIndex++
	}

	// Build query
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Default sort
	sortBy := params.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := params.SortOrder
	if sortOrder == "" {
		sortOrder = "desc"
	}

	query := fmt.Sprintf(`SELECT %s
		FROM jobs
		%s
		ORDER BY %s %s NULLS LAST
		LIMIT $%d OFFSET $%d
	`, jobSelectBase, whereClause, sortBy, strings.ToUpper(sortOrder), argIndex, argIndex+1)

	args = append(args, params.Limit, params.Offset)

	err := r.db.SelectContext(ctx, &jobs, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	if jobs == nil {
		jobs = []*domain.Job{}
	}

	return jobs, nil
}
```

Add `strings` import at top of file:

```go
import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	// ... rest of imports
)
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/database/... -run "TestJobRepository_List_With" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/database/job_repository.go crawler/internal/database/job_repository_test.go
git commit -m "feat(crawler): add ListJobsParams with sorting support

Update List method to accept structured params with:
- sort_by/sort_order with NULLS LAST
- source_id filter
- search filter (ILIKE on source_name, url)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 3: Update Count Method to Match List Filters

**Files:**
- Modify: `crawler/internal/database/job_repository.go:243-262`
- Test: `crawler/internal/database/job_repository_test.go`

**Step 1: Write the failing test**

Add to `crawler/internal/database/job_repository_test.go`:

```go
func TestJobRepository_Count_WithFilters(t *testing.T) {
	t.Helper()

	mockDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer mockDB.Close()

	db := sqlx.NewDb(mockDB, "postgres")
	repo := database.NewJobRepository(db)
	ctx := context.Background()

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM jobs WHERE status = \\$1 AND source_id = \\$2").
		WithArgs("running", "src-123").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

	params := database.CountJobsParams{
		Status:   "running",
		SourceID: "src-123",
	}

	count, countErr := repo.Count(ctx, params)
	if countErr != nil {
		t.Fatalf("Count() error = %v", countErr)
	}

	if count != 42 {
		t.Errorf("expected count=42, got %d", count)
	}

	if checkErr := mock.ExpectationsWereMet(); checkErr != nil {
		t.Errorf("unfulfilled expectations: %v", checkErr)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/database/... -run "TestJobRepository_Count_WithFilters" -v`
Expected: FAIL with "undefined: database.CountJobsParams"

**Step 3: Write minimal implementation**

Add to `crawler/internal/database/job_repository.go`:

```go
// CountJobsParams contains parameters for counting jobs.
type CountJobsParams struct {
	Status   string // Optional status filter
	SourceID string // Optional source_id filter
	Search   string // Optional search term
}

// Count returns the total number of jobs matching the given filters.
func (r *JobRepository) Count(ctx context.Context, params CountJobsParams) (int, error) {
	var count int
	var conditions []string
	var args []any
	argIndex := 1

	if params.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIndex))
		args = append(args, params.Status)
		argIndex++
	}

	if params.SourceID != "" {
		conditions = append(conditions, fmt.Sprintf("source_id = $%d", argIndex))
		args = append(args, params.SourceID)
		argIndex++
	}

	if params.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"(COALESCE(source_name, '') ILIKE $%d OR url ILIKE $%d)",
			argIndex, argIndex,
		))
		args = append(args, "%"+params.Search+"%")
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM jobs %s", whereClause)

	err := r.db.GetContext(ctx, &count, query, args...)
	if err != nil {
		return 0, fmt.Errorf("failed to count jobs: %w", err)
	}

	return count, nil
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/database/... -run "TestJobRepository_Count_WithFilters" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/database/job_repository.go crawler/internal/database/job_repository_test.go
git commit -m "feat(crawler): add CountJobsParams with filter support

Count method now uses same filter logic as List, ensuring
total count matches the filtered results.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 4: Update Jobs Handler to Use New Parameters

**Files:**
- Modify: `crawler/internal/api/jobs_handler.go:48-70`
- Test: `crawler/internal/api/jobs_handler_test.go` (create if needed)

**Step 1: Write the failing test**

Create or update `crawler/internal/api/jobs_handler_test.go`:

```go
package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/crawler/internal/api"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

type mockJobRepo struct {
	listParams  database.ListJobsParams
	countParams database.CountJobsParams
	jobs        []*domain.Job
	total       int
}

func (m *mockJobRepo) List(_ context.Context, params database.ListJobsParams) ([]*domain.Job, error) {
	m.listParams = params
	return m.jobs, nil
}

func (m *mockJobRepo) Count(_ context.Context, params database.CountJobsParams) (int, error) {
	m.countParams = params
	return m.total, nil
}

// Add other interface methods as stubs...

func TestListJobs_SortingParams(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	mockRepo := &mockJobRepo{
		jobs:  []*domain.Job{{ID: "job-1", Status: "scheduled"}},
		total: 100,
	}

	handler := api.NewJobsHandler(mockRepo, nil)

	w := httptest.NewRecorder()
	c, r := gin.CreateTestContext(w)
	r.GET("/jobs", handler.ListJobs)

	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/jobs?sort_by=next_run_at&sort_order=asc&limit=25",
		nil,
	)

	r.ServeHTTP(w, c.Request)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	// Verify params passed to repo
	if mockRepo.listParams.SortBy != "next_run_at" {
		t.Errorf("expected SortBy=next_run_at, got %s", mockRepo.listParams.SortBy)
	}
	if mockRepo.listParams.SortOrder != "asc" {
		t.Errorf("expected SortOrder=asc, got %s", mockRepo.listParams.SortOrder)
	}
	if mockRepo.listParams.Limit != 25 {
		t.Errorf("expected Limit=25, got %d", mockRepo.listParams.Limit)
	}

	// Verify response includes sort params
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["sort_by"] != "next_run_at" {
		t.Errorf("expected response sort_by=next_run_at, got %v", resp["sort_by"])
	}
	if resp["total"].(float64) != 100 {
		t.Errorf("expected total=100, got %v", resp["total"])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd crawler && go test ./internal/api/... -run "TestListJobs_SortingParams" -v`
Expected: FAIL (handler doesn't pass sort params)

**Step 3: Write minimal implementation**

Update `ListJobs` in `crawler/internal/api/jobs_handler.go`:

```go
// allowedJobSortFields maps API field names to database column expressions.
var allowedJobSortFields = map[string]string{
	"created_at":  "created_at",
	"updated_at":  "updated_at",
	"status":      "status",
	"source_name": "COALESCE(source_name, '')",
	"next_run_at": "next_run_at",
	"last_run_at": "started_at", // last_run_at maps to started_at in DB
}

// ListJobs handles GET /api/v1/jobs
func (h *JobsHandler) ListJobs(c *gin.Context) {
	// Parse pagination
	limit, offset := parseLimitOffset(c, defaultLimit, defaultOffset)
	limit = clampLimit(limit, MaxPageSize)

	// Parse sorting
	sortBy, sortOrder := parseSortParams(c, allowedJobSortFields, "created_at", "desc")

	// Parse filters
	status := c.Query("status")
	sourceID := c.Query("source_id")
	search := c.Query("search")

	// Build params
	listParams := database.ListJobsParams{
		Status:    status,
		SourceID:  sourceID,
		Search:    search,
		SortBy:    sortBy,
		SortOrder: sortOrder,
		Limit:     limit,
		Offset:    offset,
	}

	countParams := database.CountJobsParams{
		Status:   status,
		SourceID: sourceID,
		Search:   search,
	}

	// Get jobs from database
	jobs, err := h.repo.List(c.Request.Context(), listParams)
	if err != nil {
		respondInternalError(c, "Failed to retrieve jobs")
		return
	}

	total, err := h.repo.Count(c.Request.Context(), countParams)
	if err != nil {
		respondInternalError(c, "Failed to get total count")
		return
	}

	// Map sortBy back to external name for response
	externalSortBy := c.DefaultQuery("sort_by", "created_at")

	c.JSON(http.StatusOK, gin.H{
		"jobs":       jobs,
		"total":      total,
		"limit":      limit,
		"offset":     offset,
		"sort_by":    externalSortBy,
		"sort_order": sortOrder,
	})
}
```

**Step 4: Run test to verify it passes**

Run: `cd crawler && go test ./internal/api/... -run "TestListJobs_SortingParams" -v`
Expected: PASS

**Step 5: Run full test suite**

Run: `cd crawler && go test ./... -v`
Expected: All tests PASS

**Step 6: Commit**

```bash
git add crawler/internal/api/jobs_handler.go crawler/internal/api/jobs_handler_test.go
git commit -m "feat(crawler): add sorting and filtering to ListJobs endpoint

- Add sort_by, sort_order query params with whitelist validation
- Add source_id, search filters
- Clamp limit to MaxPageSize (250)
- Echo sort params in response for debugging

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 5: Update Repository Interface (if applicable)

**Files:**
- Modify: `crawler/internal/database/interfaces.go` (if exists)

**Step 1: Check if interface file exists**

Run: `ls -la crawler/internal/database/interfaces.go`

If it exists, update the interface to match new signatures. If not, skip this task.

**Step 2: Update interface (if needed)**

```go
type JobRepositoryInterface interface {
	List(ctx context.Context, params ListJobsParams) ([]*domain.Job, error)
	Count(ctx context.Context, params CountJobsParams) (int, error)
	// ... other methods unchanged
}
```

**Step 3: Run linter**

Run: `cd crawler && golangci-lint run`
Expected: No errors

**Step 4: Commit (if changes made)**

```bash
git add crawler/internal/database/
git commit -m "refactor(crawler): update JobRepositoryInterface for new params

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Phase 2: Frontend Composable

### Task 6: Create Table Types

**Files:**
- Create: `dashboard/src/types/table.ts`

**Step 1: Create the types file**

```typescript
/**
 * Server-side paginated table types.
 * Used by useServerPaginatedTable composable.
 */

/**
 * Parameters sent to the fetch function.
 */
export interface FetchParams<F = Record<string, unknown>> {
  limit: number
  offset: number
  sortBy: string
  sortOrder: 'asc' | 'desc'
  filters?: F
}

/**
 * Expected response shape from paginated API endpoints.
 */
export interface PaginatedResponse<T> {
  items: T[]
  total: number
}

/**
 * Configuration options for useServerPaginatedTable.
 */
export interface UseServerPaginatedTableOptions<T, F = Record<string, unknown>> {
  /** Function to fetch data from the server */
  fetchFn: (params: FetchParams<F>) => Promise<PaginatedResponse<T>>

  /** Prefix for TanStack Query cache keys (e.g., 'jobs') */
  queryKeyPrefix: string

  /** Default items per page (default: 25) */
  defaultLimit?: number

  /** Default sort column (default: 'created_at') */
  defaultSortBy?: string

  /** Default sort direction (default: 'desc') */
  defaultSortOrder?: 'asc' | 'desc'

  /** Whitelist of sortable field names */
  allowedSortFields: string[]

  /** Allowed page size options (default: [10, 25, 50, 100]) */
  allowedPageSizes?: number[]

  /** Auto-refresh interval in ms (optional) */
  refetchInterval?: number

  /** Debounce filter changes in ms (optional) */
  debounceMs?: number
}

/**
 * Sort state for a table column.
 */
export interface SortState {
  field: string
  order: 'asc' | 'desc'
}
```

**Step 2: Run TypeScript check**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/types/table.ts
git commit -m "feat(dashboard): add server-side table types

Add FetchParams, PaginatedResponse, and UseServerPaginatedTableOptions
types for the reusable table composable.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 7: Create useServerPaginatedTable Composable

**Files:**
- Create: `dashboard/src/composables/useServerPaginatedTable.ts`
- Modify: `dashboard/src/composables/index.ts`

**Step 1: Create the composable**

```typescript
/**
 * Server-side Paginated Table Composable
 *
 * Manages pagination, sorting, and filtering state for server-driven tables.
 * Integrates with TanStack Query for caching and automatic refetching.
 *
 * @example
 * ```ts
 * const table = useServerPaginatedTable({
 *   fetchFn: fetchJobs,
 *   queryKeyPrefix: 'jobs',
 *   allowedSortFields: ['created_at', 'status', 'next_run_at'],
 * })
 *
 * // In template
 * <tr v-for="item in table.items.value" :key="item.id">
 * <Pagination :page="table.page.value" @change="table.setPage" />
 * ```
 */

import { ref, computed, watch, type Ref, type ComputedRef } from 'vue'
import { useQuery, keepPreviousData } from '@tanstack/vue-query'
import type {
  FetchParams,
  PaginatedResponse,
  UseServerPaginatedTableOptions,
} from '@/types/table'

// ============================================================================
// Constants
// ============================================================================

const DEFAULT_LIMIT = 25
const DEFAULT_SORT_BY = 'created_at'
const DEFAULT_SORT_ORDER = 'desc' as const
const DEFAULT_PAGE_SIZES = [10, 25, 50, 100]

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Remove undefined and empty string values from an object.
 * Prevents cache fragmentation in TanStack Query.
 */
function normalizeFilters<T extends Record<string, unknown>>(obj: T): T {
  return Object.fromEntries(
    Object.entries(obj).filter(([_, v]) => v !== undefined && v !== '')
  ) as T
}

// ============================================================================
// Composable
// ============================================================================

export function useServerPaginatedTable<T, F extends Record<string, unknown> = Record<string, unknown>>(
  options: UseServerPaginatedTableOptions<T, F>
) {
  // ---------------------------------------------------------------------------
  // State: Pagination
  // ---------------------------------------------------------------------------

  const page = ref(1)
  const pageSize = ref(options.defaultLimit ?? DEFAULT_LIMIT)
  const sortBy = ref(options.defaultSortBy ?? DEFAULT_SORT_BY)
  const sortOrder = ref<'asc' | 'desc'>(options.defaultSortOrder ?? DEFAULT_SORT_ORDER)
  const filters = ref<F>({} as F)

  const allowedPageSizes = options.allowedPageSizes ?? DEFAULT_PAGE_SIZES

  // ---------------------------------------------------------------------------
  // Computed: Query Params
  // ---------------------------------------------------------------------------

  const queryParams = computed<FetchParams<F>>(() => ({
    limit: pageSize.value,
    offset: (page.value - 1) * pageSize.value,
    sortBy: sortBy.value,
    sortOrder: sortOrder.value,
    filters: normalizeFilters(filters.value),
  }))

  // ---------------------------------------------------------------------------
  // TanStack Query
  // ---------------------------------------------------------------------------

  const {
    data,
    isLoading,
    isFetching,
    error,
    refetch,
  } = useQuery({
    queryKey: computed(() => [options.queryKeyPrefix, 'list', queryParams.value]),
    queryFn: () => options.fetchFn(queryParams.value),
    staleTime: 10_000,
    refetchInterval: options.refetchInterval,
    placeholderData: keepPreviousData,
  })

  // ---------------------------------------------------------------------------
  // Computed: Derived State
  // ---------------------------------------------------------------------------

  const items = computed<T[]>(() => data.value?.items ?? [])
  const total = computed(() => data.value?.total ?? 0)

  const totalPages = computed(() => {
    if (total.value === 0) return 1
    return Math.ceil(total.value / pageSize.value)
  })

  const hasNextPage = computed(() => page.value < totalPages.value)
  const hasPreviousPage = computed(() => page.value > 1)
  const hasError = computed(() => !!error.value)
  const initialLoadDone = computed(() => !isLoading.value && data.value !== undefined)

  // ---------------------------------------------------------------------------
  // Actions: Pagination
  // ---------------------------------------------------------------------------

  function setPage(newPage: number) {
    const clamped = Math.max(1, Math.min(newPage, totalPages.value || 1))
    if (page.value === clamped) return
    page.value = clamped
  }

  function setPageSize(newSize: number) {
    if (!allowedPageSizes.includes(newSize)) return
    if (pageSize.value === newSize) return
    pageSize.value = newSize
    page.value = 1
  }

  function nextPage() {
    setPage(page.value + 1)
  }

  function prevPage() {
    setPage(page.value - 1)
  }

  // ---------------------------------------------------------------------------
  // Actions: Sorting
  // ---------------------------------------------------------------------------

  function setSort(field: string, order?: 'asc' | 'desc') {
    if (!options.allowedSortFields.includes(field)) {
      if (import.meta.env.DEV) {
        console.warn(`[useServerPaginatedTable] Invalid sort field: ${field}`)
      }
      return
    }

    // Toggle if same field without explicit order
    if (sortBy.value === field && !order) {
      sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
    } else {
      sortBy.value = field
      sortOrder.value = order ?? 'asc'
    }
    page.value = 1
  }

  function toggleSort(field: string) {
    setSort(field)
  }

  // ---------------------------------------------------------------------------
  // Actions: Filtering
  // ---------------------------------------------------------------------------

  function setFilters(newFilters: Partial<F>) {
    const merged = { ...filters.value, ...newFilters }
    filters.value = normalizeFilters(merged) as F
    page.value = 1
  }

  function replaceFilters(next: F) {
    filters.value = normalizeFilters(next) as F
    page.value = 1
  }

  function clearFilters() {
    filters.value = {} as F
    page.value = 1
  }

  // ---------------------------------------------------------------------------
  // Actions: Reset
  // ---------------------------------------------------------------------------

  function reset() {
    page.value = 1
    pageSize.value = options.defaultLimit ?? DEFAULT_LIMIT
    sortBy.value = options.defaultSortBy ?? DEFAULT_SORT_BY
    sortOrder.value = options.defaultSortOrder ?? DEFAULT_SORT_ORDER
    filters.value = {} as F
  }

  // ---------------------------------------------------------------------------
  // Return
  // ---------------------------------------------------------------------------

  return {
    // Data state
    items,
    total,
    isLoading,
    isRefetching: isFetching,
    error,
    hasError,
    initialLoadDone,

    // Pagination state
    page,
    pageSize,
    totalPages,
    hasNextPage,
    hasPreviousPage,
    allowedPageSizes,

    // Sorting state
    sortBy,
    sortOrder,

    // Filter state
    filters,

    // Pagination actions
    setPage,
    setPageSize,
    nextPage,
    prevPage,

    // Sorting actions
    setSort,
    toggleSort,

    // Filter actions
    setFilters,
    replaceFilters,
    clearFilters,

    // Utilities
    reset,
    refetch,

    // Raw query for advanced usage
    query: { data, isLoading, isFetching, error, refetch },
  }
}
```

**Step 2: Export from index**

Add to `dashboard/src/composables/index.ts`:

```typescript
export { useServerPaginatedTable } from './useServerPaginatedTable'
```

**Step 3: Run TypeScript check**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add dashboard/src/composables/useServerPaginatedTable.ts dashboard/src/composables/index.ts
git commit -m "feat(dashboard): add useServerPaginatedTable composable

Reusable composable for server-side paginated tables with:
- Pagination state (page, pageSize, total)
- Sorting with whitelist validation
- Filtering with normalization
- TanStack Query integration with keepPreviousData
- Fail-safe actions (invalid inputs ignored)

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 8: Update Jobs API to Pass Pagination Params

**Files:**
- Modify: `dashboard/src/features/intake/api/jobs.ts:66-93`

**Step 1: Update fetchJobs function**

Replace the existing `fetchJobs` function:

```typescript
import type { FetchParams, PaginatedResponse } from '@/types/table'

// Add new type for jobs list params
export interface JobsListParams extends FetchParams<JobFilters> {}

/**
 * Fetch jobs list with pagination, sorting, and filters.
 * Returns items and total for server-side pagination.
 */
export async function fetchJobs(params: JobsListParams): Promise<PaginatedResponse<Job>> {
  const queryParams: Record<string, unknown> = {
    limit: params.limit,
    offset: params.offset,
    sort_by: params.sortBy,
    sort_order: params.sortOrder,
  }

  // Add filters
  if (params.filters?.status) {
    queryParams.status = Array.isArray(params.filters.status)
      ? params.filters.status.join(',')
      : params.filters.status
  }

  if (params.filters?.source_id) {
    queryParams.source_id = params.filters.source_id
  }

  if (params.filters?.search) {
    queryParams.search = params.filters.search
  }

  const response = await crawlerApi.jobs.list(queryParams)

  return {
    items: response.data?.jobs ?? [],
    total: response.data?.total ?? 0,
  }
}
```

**Step 2: Run TypeScript check**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/features/intake/api/jobs.ts
git commit -m "feat(dashboard): update fetchJobs to pass pagination params

Send limit, offset, sort_by, sort_order to backend API.
Return PaginatedResponse shape for composable compatibility.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 9: Refactor useJobs to Use New Composable

**Files:**
- Modify: `dashboard/src/features/intake/composables/useJobs.ts`

**Step 1: Refactor useJobs**

Update to delegate to `useServerPaginatedTable`:

```typescript
/**
 * Jobs Composable
 *
 * Main composable for the Jobs feature. Uses useServerPaginatedTable
 * for pagination/sorting and adds job-specific mutations.
 */

import { computed, toRef } from 'vue'
import { useServerPaginatedTable } from '@/composables/useServerPaginatedTable'
import { useJobsUIStore } from '../stores/useJobsUIStore'
import { fetchJobs } from '../api/jobs'
import {
  useCreateJobMutation,
  useUpdateJobMutation,
  useDeleteJobMutation,
  usePauseJobMutation,
  useResumeJobMutation,
  useCancelJobMutation,
  useRetryJobMutation,
  useBulkPauseJobsMutation,
  useBulkDeleteJobsMutation,
} from './useJobMutations'
import type { CreateJobRequest, UpdateJobRequest, JobFilters, JobStatus } from '@/types/crawler'

// Allowed sort fields for jobs table
const JOBS_SORT_FIELDS = [
  'created_at',
  'updated_at',
  'status',
  'source_name',
  'next_run_at',
  'last_run_at',
]

/**
 * Main composable for jobs list view
 */
export function useJobs() {
  // Use the server-paginated table composable
  const table = useServerPaginatedTable<Job, JobFilters>({
    fetchFn: fetchJobs,
    queryKeyPrefix: 'jobs',
    defaultLimit: 25,
    defaultSortBy: 'created_at',
    defaultSortOrder: 'desc',
    allowedSortFields: JOBS_SORT_FIELDS,
    allowedPageSizes: [10, 25, 50, 100],
    refetchInterval: 30_000,
  })

  // UI store
  const uiStore = useJobsUIStore()

  // Mutations
  const createMutation = useCreateJobMutation()
  const updateMutation = useUpdateJobMutation()
  const deleteMutation = useDeleteJobMutation()
  const pauseMutation = usePauseJobMutation()
  const resumeMutation = useResumeJobMutation()
  const cancelMutation = useCancelJobMutation()
  const retryMutation = useRetryJobMutation()
  const bulkPauseMutation = useBulkPauseJobsMutation()
  const bulkDeleteMutation = useBulkDeleteJobsMutation()

  // ---------------------------------------------------------------------------
  // Computed: Status Counts (from current page data)
  // ---------------------------------------------------------------------------

  const statusCounts = computed(() => {
    const counts: Record<JobStatus, number> = {
      pending: 0,
      scheduled: 0,
      running: 0,
      paused: 0,
      completed: 0,
      failed: 0,
      cancelled: 0,
    }

    for (const job of table.items.value) {
      if (job.status in counts) {
        counts[job.status]++
      }
    }

    return counts
  })

  const activeJobsCount = computed(() => {
    const counts = statusCounts.value
    return counts.running + counts.scheduled + counts.pending
  })

  const failedJobsCount = computed(() => statusCounts.value.failed)

  // ---------------------------------------------------------------------------
  // Mutation Actions
  // ---------------------------------------------------------------------------

  async function createJob(data: CreateJobRequest) {
    return createMutation.mutateAsync(data)
  }

  async function updateJob(id: string, data: UpdateJobRequest) {
    return updateMutation.mutateAsync({ id, data })
  }

  async function deleteJob(id: string) {
    return deleteMutation.mutateAsync(id)
  }

  async function pauseJob(id: string) {
    return pauseMutation.mutateAsync(id)
  }

  async function resumeJob(id: string) {
    return resumeMutation.mutateAsync(id)
  }

  async function cancelJob(id: string) {
    return cancelMutation.mutateAsync(id)
  }

  async function retryJob(id: string) {
    return retryMutation.mutateAsync(id)
  }

  async function bulkPauseJobs(ids: string[]) {
    return bulkPauseMutation.mutateAsync(ids)
  }

  async function bulkDeleteJobs(ids: string[]) {
    return bulkDeleteMutation.mutateAsync(ids)
  }

  // ---------------------------------------------------------------------------
  // Filter Helpers
  // ---------------------------------------------------------------------------

  function toggleStatusFilter(status: JobStatus) {
    const currentStatus = table.filters.value.status

    if (!currentStatus) {
      table.setFilters({ status: [status] })
    } else if (Array.isArray(currentStatus)) {
      const index = currentStatus.indexOf(status)
      if (index > -1) {
        const newStatus = [...currentStatus]
        newStatus.splice(index, 1)
        table.setFilters({ status: newStatus.length > 0 ? newStatus : undefined })
      } else {
        table.setFilters({ status: [...currentStatus, status] })
      }
    } else if (currentStatus === status) {
      table.setFilters({ status: undefined })
    } else {
      table.setFilters({ status: [currentStatus, status] })
    }
  }

  function isStatusActive(status: JobStatus): boolean {
    const currentStatus = table.filters.value.status
    if (!currentStatus) return false
    if (Array.isArray(currentStatus)) {
      return currentStatus.includes(status)
    }
    return currentStatus === status
  }

  function clearAllFilters() {
    table.clearFilters()
  }

  // ---------------------------------------------------------------------------
  // Return
  // ---------------------------------------------------------------------------

  return {
    // Data (from table composable)
    jobs: table.items,
    totalJobs: table.total,
    isLoading: table.isLoading,
    isFetching: table.isRefetching,
    error: table.error,

    // Pagination (from table composable)
    page: table.page,
    pageSize: table.pageSize,
    totalPages: table.totalPages,
    hasNextPage: table.hasNextPage,
    hasPreviousPage: table.hasPreviousPage,
    allowedPageSizes: table.allowedPageSizes,
    setPage: table.setPage,
    setPageSize: table.setPageSize,
    nextPage: table.nextPage,
    prevPage: table.prevPage,

    // Sorting (from table composable)
    sortBy: table.sortBy,
    sortOrder: table.sortOrder,
    setSort: table.setSort,
    toggleSort: table.toggleSort,

    // Filters
    filters: table.filters,
    setFilters: table.setFilters,
    clearAllFilters,
    toggleStatusFilter,
    isStatusActive,
    hasActiveFilters: computed(() => Object.keys(table.filters.value).length > 0),

    // Status counts (computed from current page)
    statusCounts,
    activeJobsCount,
    failedJobsCount,

    // UI store
    ui: uiStore,

    // Mutations
    createJob,
    updateJob,
    deleteJob,
    pauseJob,
    resumeJob,
    cancelJob,
    retryJob,
    bulkPauseJobs,
    bulkDeleteJobs,

    // Mutation states
    isCreating: computed(() => createMutation.isPending.value),
    isUpdating: computed(() => updateMutation.isPending.value),
    isDeleting: computed(() => deleteMutation.isPending.value),
    isPausing: computed(() => pauseMutation.isPending.value),
    isResuming: computed(() => resumeMutation.isPending.value),
    isCancelling: computed(() => cancelMutation.isPending.value),
    isRetrying: computed(() => retryMutation.isPending.value),

    // Utilities
    refetch: table.refetch,
    reset: table.reset,
  }
}

// Re-export useJobDetail unchanged
export { useJobDetail } from './useJobsQuery'
```

**Step 2: Add Job import**

Add at top of file:

```typescript
import type { Job } from '@/types/crawler'
```

**Step 3: Run TypeScript check**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 4: Commit**

```bash
git add dashboard/src/features/intake/composables/useJobs.ts
git commit -m "refactor(dashboard): use useServerPaginatedTable in useJobs

Delegate pagination/sorting to reusable composable.
Remove client-side paginatedJobs slicing.
Keep job-specific mutations and status helpers.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Phase 3: UI Components

### Task 10: Add Sortable Column Headers to JobsTable

**Files:**
- Modify: `dashboard/src/components/domain/jobs/JobsTable.vue`

**Step 1: Update script section**

Add sort indicator logic and emit sort events:

```typescript
// Add to imports
import { ArrowUp, ArrowDown, ArrowUpDown } from 'lucide-vue-next'

// Add to script setup
const jobs = useJobs()

// Define sortable columns
const sortableColumns = [
  { key: 'source_name', label: 'Source' },
  { key: 'status', label: 'Status' },
  { key: 'next_run_at', label: 'Next Run' },
  { key: 'last_run_at', label: 'Last Run' },
  { key: 'created_at', label: 'Created' },
] as const

function getSortIcon(column: string) {
  if (jobs.sortBy.value !== column) return ArrowUpDown
  return jobs.sortOrder.value === 'asc' ? ArrowUp : ArrowDown
}

function handleSort(column: string) {
  jobs.toggleSort(column)
}
```

**Step 2: Update template header section**

Replace static headers with sortable ones:

```vue
<thead>
  <tr class="border-b bg-muted/50">
    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
      Job ID
    </th>
    <th
      v-for="col in sortableColumns"
      :key="col.key"
      class="px-4 py-3 text-left text-sm font-medium text-muted-foreground cursor-pointer hover:text-foreground transition-colors"
      @click="handleSort(col.key)"
    >
      <div class="flex items-center gap-1">
        {{ col.label }}
        <component
          :is="getSortIcon(col.key)"
          :class="[
            'h-4 w-4',
            jobs.sortBy.value === col.key ? 'text-foreground' : 'text-muted-foreground/50'
          ]"
        />
      </div>
    </th>
    <th class="px-4 py-3 text-left text-sm font-medium text-muted-foreground">
      Schedule
    </th>
    <th
      v-if="showActions"
      class="px-4 py-3 text-right text-sm font-medium text-muted-foreground"
    >
      Actions
    </th>
  </tr>
</thead>
```

**Step 3: Add page size selector to pagination section**

Add after the page numbers:

```vue
<!-- Page Size Selector -->
<div class="flex items-center gap-2 ml-4">
  <span class="text-sm text-muted-foreground">Show:</span>
  <select
    :value="jobs.pageSize.value"
    class="rounded-md border bg-background px-2 py-1 text-sm"
    @change="(e) => jobs.setPageSize(Number((e.target as HTMLSelectElement).value))"
  >
    <option
      v-for="size in jobs.allowedPageSizes"
      :key="size"
      :value="size"
    >
      {{ size }}
    </option>
  </select>
</div>
```

**Step 4: Update pagination text to use server total**

Change from:

```vue
Showing {{ (jobs.page.value - 1) * jobs.pageSize.value + 1 }} to
{{ Math.min(jobs.page.value * jobs.pageSize.value, jobs.jobs.value.length) }}
of {{ jobs.jobs.value.length }} jobs
```

To:

```vue
Showing {{ (jobs.page.value - 1) * jobs.pageSize.value + 1 }} to
{{ Math.min(jobs.page.value * jobs.pageSize.value, jobs.totalJobs.value) }}
of {{ jobs.totalJobs.value }} jobs
```

**Step 5: Update data rows to use jobs.jobs instead of paginatedJobs**

Change:

```vue
<tr
  v-for="job in jobs.paginatedJobs.value"
```

To:

```vue
<tr
  v-for="job in jobs.jobs.value"
```

**Step 6: Run lint and dev server**

Run: `cd dashboard && npm run lint && npm run dev`

Test in browser at `/dashboard/intake/jobs`

**Step 7: Commit**

```bash
git add dashboard/src/components/domain/jobs/JobsTable.vue
git commit -m "feat(dashboard): add sortable columns and page size selector

- Click column headers to sort (with indicators)
- Page size selector (10/25/50/100)
- Show server total instead of client array length
- Remove client-side pagination slicing

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 11: Update JobStatsCard to Use Server Total

**Files:**
- Modify: `dashboard/src/components/domain/jobs/JobStatsCard.vue:20`

**Step 1: Update Total Jobs stat**

Change:

```typescript
{
  label: 'Total Jobs',
  value: jobs.jobs.value.length,
```

To:

```typescript
{
  label: 'Total Jobs',
  value: jobs.totalJobs.value,
```

**Step 2: Run lint**

Run: `cd dashboard && npm run lint`
Expected: No errors

**Step 3: Commit**

```bash
git add dashboard/src/components/domain/jobs/JobStatsCard.vue
git commit -m "fix(dashboard): use server total in JobStatsCard

Display true total from API instead of returned array length.

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

### Task 12: Clean Up Deprecated Code

**Files:**
- Modify: `dashboard/src/features/intake/stores/useJobsQueryStore.ts` (if needed)
- Modify: `dashboard/src/features/intake/composables/useJobsQuery.ts` (if needed)

**Step 1: Review and remove unused code**

Check if these stores/composables have unused pagination logic that can be removed.

The `useJobsQueryStore` can be simplified since pagination is now in `useServerPaginatedTable`.

**Step 2: Run full test**

Run: `cd dashboard && npm run lint && npm run build`
Expected: Build succeeds

**Step 3: Commit cleanup (if any)**

```bash
git add dashboard/src/features/intake/
git commit -m "refactor(dashboard): remove deprecated client-side pagination

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Phase 4: Integration Testing

### Task 13: Manual Integration Test

**Steps:**

1. Start backend: `task docker:dev:up`
2. Start frontend: `cd dashboard && npm run dev`
3. Navigate to `/dashboard/intake/jobs`

**Verify:**

- [ ] Total shows actual count (320+, not 50)
- [ ] Pagination shows correct page count
- [ ] All pages have data (no empty pages after page 2)
- [ ] Clicking column headers sorts data
- [ ] Sort indicator shows current sort
- [ ] Page size selector works
- [ ] Changing page size resets to page 1
- [ ] Filters still work
- [ ] Stats card shows correct total

**Step 1: Document any issues found**

If issues found, fix them before final commit.

**Step 2: Final commit**

```bash
git add .
git commit -m "test: verify jobs table server-side pagination

Manual integration test passed:
- 320+ jobs paginate correctly
- Sorting works on all columns
- Page size selector functional
- Stats show true totals

Co-Authored-By: Claude Opus 4.5 <noreply@anthropic.com>"
```

---

## Summary

| Phase | Tasks | Files Modified |
|-------|-------|----------------|
| Backend | 1-5 | helpers.go, job_repository.go, jobs_handler.go |
| Frontend Composable | 6-9 | table.ts, useServerPaginatedTable.ts, jobs.ts, useJobs.ts |
| UI Components | 10-12 | JobsTable.vue, JobStatsCard.vue |
| Testing | 13 | Manual verification |

**Total estimated tasks:** 13
**Key deliverables:**
- Reusable `useServerPaginatedTable` composable
- Backend sorting/filtering support
- Sortable table headers with indicators
- Accurate pagination for 320+ jobs
