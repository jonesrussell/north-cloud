# Leadership & Contact Page Scraper Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a CLI command to the crawler that discovers, fetches, and extracts leadership/contact data from community websites, storing results via source-manager API.

**Architecture:** Standalone CLI subcommand in crawler binary using `flag` package. New `internal/scraper/` package handles orchestration with a worker pool. Source-manager gets one new migration (`last_scraped_at`), one new PATCH endpoint, and a community list endpoint filtered by `has_source=true`.

**Tech Stack:** Go 1.26, net/http, `colly` (for page fetching), existing `internal/leadership/` extraction package.

**Spec:** `docs/superpowers/specs/2026-03-10-leadership-contact-scraper-design.md`

---

## File Map

### Source-Manager Changes

| File | Action | Responsibility |
|------|--------|----------------|
| `source-manager/migrations/014_add_last_scraped_at.up.sql` | Create | Add `last_scraped_at TIMESTAMPTZ` to communities |
| `source-manager/migrations/014_add_last_scraped_at.down.sql` | Create | Drop column |
| `source-manager/internal/models/community.go` | Modify | Add `LastScrapedAt` field |
| `source-manager/internal/repository/community.go` | Modify | Add `LastScrapedAt` to scan/insert/update, add `UpdateLastScrapedAt()`, add `ListWithSource()` |
| `source-manager/internal/handlers/community_handler.go` | Create | `UpdateScrapedAt` handler, `ListWithSource` handler |
| `source-manager/internal/api/router.go` | Modify | Register new routes |

### Crawler Changes

| File | Action | Responsibility |
|------|--------|----------------|
| `crawler/main.go` | Modify | Add CLI subcommand dispatch |
| `crawler/internal/scraper/scraper.go` | Create | Orchestrator: worker pool, discovery, extraction |
| `crawler/internal/scraper/scraper_test.go` | Create | Unit tests with mocked HTTP |
| `crawler/internal/scraper/client.go` | Create | Source-manager API client for communities/people/band-offices |
| `crawler/internal/scraper/client_test.go` | Create | Client unit tests |
| `crawler/internal/scraper/pagefetcher.go` | Create | Fetch page HTML, extract links and text |
| `crawler/internal/scraper/pagefetcher_test.go` | Create | Page fetcher tests |

---

## Chunk 1: Source-Manager Migration and Model

### Task 1: Add `last_scraped_at` migration

**Files:**
- Create: `source-manager/migrations/014_add_last_scraped_at.up.sql`
- Create: `source-manager/migrations/014_add_last_scraped_at.down.sql`

- [ ] **Step 1: Create up migration**

```sql
ALTER TABLE communities ADD COLUMN last_scraped_at TIMESTAMPTZ;
```

- [ ] **Step 2: Create down migration**

```sql
ALTER TABLE communities DROP COLUMN last_scraped_at;
```

- [ ] **Step 3: Run migration**

Run: `task migrate:up` (from source-manager directory)
Expected: Migration 014 applied successfully.

- [ ] **Step 4: Commit**

```bash
git add source-manager/migrations/014_add_last_scraped_at.up.sql source-manager/migrations/014_add_last_scraped_at.down.sql
git commit -m "feat(source-manager): add last_scraped_at column to communities (#273)"
```

### Task 2: Add `LastScrapedAt` to Community model

**Files:**
- Modify: `source-manager/internal/models/community.go`

- [ ] **Step 1: Add field to Community struct**

Add after the `UpdatedAt` field (line 43):

```go
LastScrapedAt *time.Time `db:"last_scraped_at" json:"last_scraped_at,omitempty"`
```

- [ ] **Step 2: Verify tests still pass**

Run: `cd source-manager && GOWORK=off go test ./...`
Expected: All tests pass (model change alone won't break anything yet).

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/models/community.go
git commit -m "feat(source-manager): add LastScrapedAt field to Community model (#273)"
```

### Task 3: Update `scanCommunity` and queries for `last_scraped_at`

**Files:**
- Modify: `source-manager/internal/repository/community.go`

The `scanCommunity` helper and all SELECT/INSERT/UPDATE queries must include the new column. This task updates:

1. `scanCommunity` — add `&c.LastScrapedAt` to scan destinations (becomes 26 fields, update `communityColumnCount`)
2. All SELECT queries — add `last_scraped_at` to column lists (in `GetByID`, `GetBySlug`, `ListPaginated`, `ListUnlinked`, `FindNearby`)
3. `Create` — add `last_scraped_at` to INSERT (param $26)
4. `upsertCommunity` — add `last_scraped_at` to INSERT and ON CONFLICT SET

- [ ] **Step 1: Update `scanCommunity`**

Change `communityColumnCount` from 25 to 26. Add `&c.LastScrapedAt` after `&c.UpdatedAt` in the dest slice.

- [ ] **Step 2: Update all SELECT column lists**

Add `last_scraped_at` after `updated_at` in every SELECT query in the file: `GetByID`, `GetBySlug`, `ListPaginated`, `ListUnlinked`, and `FindNearby` CTE.

- [ ] **Step 3: Update Create INSERT**

Add `last_scraped_at` to the INSERT column list and add `$26` placeholder. Add `c.LastScrapedAt` to the args.

- [ ] **Step 4: Update upsertCommunity**

Add `last_scraped_at` to the INSERT column list, add `$26` placeholder, and add `last_scraped_at = EXCLUDED.last_scraped_at` to the ON CONFLICT SET clause. Add `c.LastScrapedAt` to the args.

- [ ] **Step 5: Run tests and linter**

Run: `cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run ./...`
Expected: All tests pass, 0 lint issues.

- [ ] **Step 6: Commit**

```bash
git add source-manager/internal/repository/community.go
git commit -m "feat(source-manager): include last_scraped_at in all community queries (#273)"
```

### Task 4: Add `UpdateLastScrapedAt` repository method

**Files:**
- Modify: `source-manager/internal/repository/community.go`

- [ ] **Step 1: Write failing test**

Create a test in `source-manager/internal/repository/community_test.go` (or the appropriate test file) that calls `UpdateLastScrapedAt(ctx, communityID, timestamp)` and expects it to update the row.

Note: If repository tests require a running database (integration tests), write the test but mark with a build tag. If there's a mock pattern, follow it.

- [ ] **Step 2: Implement `UpdateLastScrapedAt`**

```go
// UpdateLastScrapedAt sets the last_scraped_at timestamp for a community.
func (r *CommunityRepository) UpdateLastScrapedAt(ctx context.Context, id string, scrapedAt time.Time) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE communities SET last_scraped_at = $2 WHERE id = $1`,
		id, scrapedAt,
	)
	if err != nil {
		return fmt.Errorf("update last_scraped_at: %w", err)
	}

	rows, rowsErr := result.RowsAffected()
	if rowsErr != nil {
		return fmt.Errorf("update last_scraped_at rows affected: %w", rowsErr)
	}
	if rows == 0 {
		return errors.New("update last_scraped_at: community not found")
	}

	return nil
}
```

- [ ] **Step 3: Run tests and linter**

Run: `cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run ./...`
Expected: Pass.

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/repository/community.go
git commit -m "feat(source-manager): add UpdateLastScrapedAt repository method (#273)"
```

### Task 5: Add `ListWithSource` repository method

**Files:**
- Modify: `source-manager/internal/repository/community.go`

- [ ] **Step 1: Implement `ListWithSource`**

Returns communities where `source_id IS NOT NULL` and `website IS NOT NULL`:

```go
// ListWithSource returns communities that have a linked source and a website URL.
func (r *CommunityRepository) ListWithSource(ctx context.Context) ([]Community, error) {
	query := `SELECT id, name, slug, community_type, province, region,
		inac_id, statcan_csd, osm_relation_id, wikidata_qid, latitude, longitude,
		nation, treaty, language_group, reserve_name, population, population_year,
		website, feed_url, data_source, source_id, enabled, created_at, updated_at, last_scraped_at
		FROM communities
		WHERE source_id IS NOT NULL AND website IS NOT NULL
		ORDER BY name ASC`
	// ... standard rows loop with scanCommunity
}
```

- [ ] **Step 2: Run tests and linter**

Run: `cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run ./...`

- [ ] **Step 3: Commit**

```bash
git add source-manager/internal/repository/community.go
git commit -m "feat(source-manager): add ListWithSource repository method (#273)"
```

---

## Chunk 2: Source-Manager API Endpoints

### Task 6: Create community handler with PATCH and list-with-source endpoints

**Files:**
- Create: `source-manager/internal/handlers/community_handler.go`

- [ ] **Step 1: Create handler struct and constructor**

Follow the pattern from `source-manager/internal/handlers/person.go`. The handler needs:
- `CommunityRepository` dependency
- `Logger` dependency

```go
type CommunityHandler struct {
	repo   *repository.CommunityRepository
	logger infralogger.Logger
}

func NewCommunityHandler(repo *repository.CommunityRepository, log infralogger.Logger) *CommunityHandler {
	return &CommunityHandler{repo: repo, logger: log}
}
```

- [ ] **Step 2: Implement `UpdateScrapedAt` handler**

```go
// UpdateScrapedAt handles PATCH /api/v1/communities/:id/scraped
func (h *CommunityHandler) UpdateScrapedAt(c *gin.Context) {
	id := c.Param("id")

	var body struct {
		LastScrapedAt time.Time `json:"last_scraped_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "last_scraped_at is required"})
		return
	}

	if err := h.repo.UpdateLastScrapedAt(c.Request.Context(), id, body.LastScrapedAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}
```

- [ ] **Step 3: Implement `ListWithSource` handler**

```go
// ListWithSource handles GET /api/v1/communities/with-source
func (h *CommunityHandler) ListWithSource(c *gin.Context) {
	communities, err := h.repo.ListWithSource(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"communities": communities,
		"count":       len(communities),
	})
}
```

- [ ] **Step 4: Run linter**

Run: `cd source-manager && GOWORK=off golangci-lint run ./...`

- [ ] **Step 5: Commit**

```bash
git add source-manager/internal/handlers/community_handler.go
git commit -m "feat(source-manager): add community handler with scraped-at and list-with-source (#273)"
```

### Task 7: Register routes and wire handler in bootstrap

**Files:**
- Modify: `source-manager/internal/api/router.go`
- Modify: `source-manager/internal/bootstrap/` (wherever handlers are wired)

- [ ] **Step 1: Wire CommunityHandler in bootstrap**

In the bootstrap setup where other handlers are created, add:
```go
communityHandler := handlers.NewCommunityHandler(communityRepo, log)
```
Pass it to `setupServiceRoutes`.

- [ ] **Step 2: Register routes in router.go**

Add to the public communities group:
```go
publicCommunities.GET("/with-source", communityHandler.ListWithSource)
```

Add to the protected communities group:
```go
communities.PATCH("/:id/scraped", communityHandler.UpdateScrapedAt)
```

- [ ] **Step 3: Run tests and linter**

Run: `cd source-manager && GOWORK=off go test ./... && GOWORK=off golangci-lint run ./...`

- [ ] **Step 4: Commit**

```bash
git add source-manager/internal/api/router.go source-manager/internal/bootstrap/
git commit -m "feat(source-manager): register community handler routes (#273)"
```

---

## Chunk 3: Crawler Source-Manager Client

### Task 8: Create source-manager client for scraper

**Files:**
- Create: `crawler/internal/scraper/client.go`
- Create: `crawler/internal/scraper/client_test.go`

This client talks to source-manager's API for communities, people, and band offices.

- [ ] **Step 1: Write failing test for `ListCommunitiesWithSource`**

```go
func TestClient_ListCommunitiesWithSource(t *testing.T) {
	t.Helper()
	// httptest server returning JSON with communities array
	// Call client.ListCommunitiesWithSource(ctx)
	// Assert correct count and fields
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v -run TestClient_ListCommunitiesWithSource`
Expected: FAIL (package doesn't exist yet)

- [ ] **Step 3: Implement client struct and `ListCommunitiesWithSource`**

```go
package scraper

// Community represents a community from source-manager with a linked source.
type Community struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Website   *string `json:"website,omitempty"`
	SourceID  *string `json:"source_id,omitempty"`
}

// Client calls source-manager API endpoints for the leadership scraper.
type Client struct {
	baseURL    string
	httpClient *http.Client
	jwtToken   string
}

func NewClient(baseURL, jwtToken string) *Client { ... }

func (c *Client) ListCommunitiesWithSource(ctx context.Context) ([]Community, error) {
	// GET {baseURL}/api/v1/communities/with-source
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v -run TestClient_ListCommunitiesWithSource`
Expected: PASS

- [ ] **Step 5: Write test and implement `ListPeople`**

```go
func (c *Client) ListPeople(ctx context.Context, communityID string) ([]Person, error) {
	// GET {baseURL}/api/v1/communities/{communityID}/people
}
```

- [ ] **Step 6: Write test and implement `CreatePerson`**

```go
func (c *Client) CreatePerson(ctx context.Context, communityID string, p PersonInput) error {
	// POST {baseURL}/api/v1/communities/{communityID}/people
	// Sets verified=false, data_source="crawler", source_url=p.SourceURL
}
```

- [ ] **Step 7: Write test and implement `GetBandOffice`**

```go
func (c *Client) GetBandOffice(ctx context.Context, communityID string) (*BandOffice, error) {
	// GET {baseURL}/api/v1/communities/{communityID}/band-office
	// Returns nil, nil if 404
}
```

- [ ] **Step 8: Write test and implement `UpsertBandOffice`**

```go
func (c *Client) UpsertBandOffice(ctx context.Context, communityID string, b BandOfficeInput) error {
	// POST {baseURL}/api/v1/communities/{communityID}/band-office
	// Sets verified=false, data_source="crawler", source_url=b.SourceURL
}
```

- [ ] **Step 9: Write test and implement `UpdateScrapedAt`**

```go
func (c *Client) UpdateScrapedAt(ctx context.Context, communityID string, scrapedAt time.Time) error {
	// PATCH {baseURL}/api/v1/communities/{communityID}/scraped
	// Body: {"last_scraped_at": "2026-01-01T12:00:00Z"}
}
```

- [ ] **Step 10: Run all client tests and linter**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v && GOWORK=off golangci-lint run ./...`
Expected: All pass, 0 lint issues.

- [ ] **Step 11: Commit**

```bash
git add crawler/internal/scraper/client.go crawler/internal/scraper/client_test.go
git commit -m "feat(crawler): add source-manager API client for leadership scraper (#273)"
```

---

## Chunk 4: Page Fetcher

### Task 9: Create page fetcher

**Files:**
- Create: `crawler/internal/scraper/pagefetcher.go`
- Create: `crawler/internal/scraper/pagefetcher_test.go`

The page fetcher downloads a page, extracts all links, and returns the page text.

- [ ] **Step 1: Write failing test**

```go
func TestPageFetcher_FetchLinks(t *testing.T) {
	t.Helper()
	// httptest server with HTML containing <a href="/chief-and-council">Chief and Council</a>
	// Call fetcher.FetchLinks(ctx, serverURL)
	// Assert returns slice of leadership.Link with correct href and text
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v -run TestPageFetcher`
Expected: FAIL

- [ ] **Step 3: Implement PageFetcher**

```go
package scraper

// PageFetcher downloads pages and extracts links and text.
type PageFetcher struct {
	httpClient *http.Client
}

// FetchResult holds the extracted data from a fetched page.
type FetchResult struct {
	Links []leadership.Link
	Text  string
}

// FetchLinks fetches a URL and returns all anchor links found on the page.
func (f *PageFetcher) FetchLinks(ctx context.Context, pageURL string) ([]leadership.Link, error) { ... }

// FetchText fetches a URL and returns the visible text content.
func (f *PageFetcher) FetchText(ctx context.Context, pageURL string) (string, error) { ... }
```

Use `golang.org/x/net/html` for parsing (already in vendor via colly dependency), or `github.com/PuerkitoBio/goquery` (already available via colly).

- [ ] **Step 4: Run test to verify it passes**

- [ ] **Step 5: Write test for `FetchText`**

Test that `FetchText` returns visible text from an HTML page (strips tags).

- [ ] **Step 6: Run all tests and linter**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v && GOWORK=off golangci-lint run ./...`

- [ ] **Step 7: Commit**

```bash
git add crawler/internal/scraper/pagefetcher.go crawler/internal/scraper/pagefetcher_test.go
git commit -m "feat(crawler): add page fetcher for leadership scraper (#273)"
```

---

## Chunk 5: Scraper Orchestrator

### Task 10: Create scraper orchestrator

**Files:**
- Create: `crawler/internal/scraper/scraper.go`
- Create: `crawler/internal/scraper/scraper_test.go`

- [ ] **Step 1: Define Scraper struct and config**

```go
package scraper

// Config holds scraper configuration.
type Config struct {
	SourceManagerURL string
	JWTToken         string
	Workers          int
	DryRun           bool
}

// Result tracks what happened for a single community.
type Result struct {
	CommunityID   string
	CommunityName string
	PeopleAdded   int
	PeopleSkipped int
	OfficeUpdated bool
	OfficeSkipped bool
	Error         error
}

// Scraper orchestrates leadership/contact page scraping.
type Scraper struct {
	client  *Client
	fetcher *PageFetcher
	config  Config
	logger  infralogger.Logger
}
```

- [ ] **Step 2: Write failing test for single-community scrape**

```go
func TestScraper_ScrapeCommunity(t *testing.T) {
	t.Helper()
	// Mock: source-manager API (communities, people, band-office endpoints)
	// Mock: community website with leadership + contact pages
	// Call scraper.ScrapeCommunity(ctx, community)
	// Assert: correct people created, band office upserted
}
```

- [ ] **Step 3: Implement `ScrapeCommunity`**

Single-community logic:
1. Fetch homepage → extract links
2. `leadership.DiscoverPages(baseURL, links)` → discovered pages
3. For leadership pages: fetch text → `leadership.ExtractLeaders(text)` → compare with existing → create new
4. For contact pages: fetch text → `leadership.ExtractContact(text)` → compare with existing → upsert
5. Update `last_scraped_at`
6. Return Result

- [ ] **Step 4: Run test**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v -run TestScraper_ScrapeCommunity`
Expected: PASS

- [ ] **Step 5: Write test for skip-if-unchanged logic**

```go
func TestScraper_SkipsUnchangedPeople(t *testing.T) {
	t.Helper()
	// Mock: existing people match extracted people
	// Assert: no POST calls made, PeopleSkipped > 0
}
```

- [ ] **Step 6: Implement comparison logic**

Compare extracted `(name, role)` tuples against existing people. For band offices, compare phone/email/postal_code fields. Skip if identical.

- [ ] **Step 7: Run test**

Expected: PASS

- [ ] **Step 8: Implement `Run` method with worker pool**

```go
// Run scrapes all communities with linked sources using a worker pool.
func (s *Scraper) Run(ctx context.Context) ([]Result, error) {
	communities, err := s.client.ListCommunitiesWithSource(ctx)
	// ...
	jobs := make(chan Community, len(communities))
	results := make(chan Result, len(communities))

	// Start workers
	var wg sync.WaitGroup
	for range s.config.Workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for community := range jobs {
				results <- s.ScrapeCommunity(ctx, community)
			}
		}()
	}

	// Send jobs
	for _, c := range communities {
		jobs <- c
	}
	close(jobs)

	// Collect results
	go func() { wg.Wait(); close(results) }()

	var allResults []Result
	for r := range results {
		allResults = append(allResults, r)
	}
	return allResults, nil
}
```

- [ ] **Step 9: Write test for dry-run mode**

Assert that in dry-run mode, no POST/PUT/PATCH calls are made.

- [ ] **Step 10: Run all tests and linter**

Run: `cd crawler && GOWORK=off go test ./internal/scraper/... -v && GOWORK=off golangci-lint run ./...`

- [ ] **Step 11: Commit**

```bash
git add crawler/internal/scraper/scraper.go crawler/internal/scraper/scraper_test.go
git commit -m "feat(crawler): add leadership scraper orchestrator with worker pool (#273)"
```

---

## Chunk 6: CLI Entry Point

### Task 11: Add CLI subcommand to crawler

**Files:**
- Modify: `crawler/main.go`

Currently `main.go` just calls `bootstrap.Start()`. We need to add subcommand dispatch:
- No args or `serve` → existing `bootstrap.Start()` behavior
- `scrape-leadership` → run the scraper

- [ ] **Step 1: Modify `main.go` to support subcommands**

```go
func main() {
	if len(os.Args) > 1 && os.Args[1] == "scrape-leadership" {
		if err := runScrapeLeadership(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Default: start HTTP server
	if err := bootstrap.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runScrapeLeadership(args []string) error {
	fs := flag.NewFlagSet("scrape-leadership", flag.ExitOnError)
	communityID := fs.String("community-id", "", "Scrape a single community by ID")
	dryRun := fs.Bool("dry-run", false, "Print extracted data without saving")
	workers := fs.Int("workers", 4, "Number of concurrent workers")
	sourceManagerURL := fs.String("source-manager-url", "", "Source-manager base URL (overrides config)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load config for source-manager URL and JWT
	cfg := loadScraperConfig(*sourceManagerURL)

	// Create logger
	log := createLogger()

	// Create and run scraper
	s := scraper.New(scraper.Config{
		SourceManagerURL: cfg.SourceManagerURL,
		JWTToken:         cfg.JWTToken,
		Workers:          *workers,
		DryRun:           *dryRun,
		CommunityID:      *communityID,
	}, log)

	results, err := s.Run(context.Background())
	if err != nil {
		return err
	}

	// Print summary
	printSummary(results)
	return nil
}
```

- [ ] **Step 2: Implement `loadScraperConfig` helper**

Load source-manager URL and JWT secret from environment or config file. Follow the existing pattern in `crawler/internal/config/config.go` — the crawler config already has `SourceManagerAPIURL` and `Auth.JWTSecret`.

- [ ] **Step 3: Run linter**

Run: `cd crawler && GOWORK=off golangci-lint run ./...`

- [ ] **Step 4: Commit**

```bash
git add crawler/main.go
git commit -m "feat(crawler): add scrape-leadership CLI subcommand (#273)"
```

### Task 12: End-to-end manual test

- [ ] **Step 1: Start source-manager and run migration**

```bash
task docker:dev:up
task migrate:up
```

- [ ] **Step 2: Verify new endpoints**

```bash
# List communities with linked sources
curl http://localhost:8050/api/v1/communities/with-source | jq '.count'

# Test PATCH scraped-at (requires JWT)
curl -X PATCH http://localhost:8050/api/v1/communities/{id}/scraped \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"last_scraped_at": "2026-03-10T12:00:00Z"}'
```

- [ ] **Step 3: Run scraper in dry-run mode**

```bash
cd crawler && go run . scrape-leadership --dry-run --workers 2
```

Expected: JSON output of discovered pages and extracted data, no API writes.

- [ ] **Step 4: Run scraper for a single community**

```bash
cd crawler && go run . scrape-leadership --community-id={id} --dry-run
```

- [ ] **Step 5: Run full linter**

```bash
task lint:force
```

- [ ] **Step 6: Final commit**

```bash
git commit -m "feat(crawler): leadership and contact page scraper (#273)

Closes #273"
```
