# Crawler Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add Redis storage, adaptive scheduling, proxy rotation, and depth-3 default to the crawler.

**Architecture:** Four independent features layered onto the existing Colly-based crawler. Redis storage is the foundation (other features benefit from Redis). Config additions follow the existing `env` tag + `yaml` tag pattern. Adaptive scheduling hooks into the scheduler's `handleJobSuccess` reschedule step. Proxy rotation uses Colly's built-in `proxy.RoundRobinProxySwitcher`. Max depth default is a one-line guard in `setupCollector`.

**Tech Stack:** Go 1.25, Colly v2.3.0, gocolly/redisstorage, go-redis/v9

**Design doc:** `docs/plans/2026-02-14-crawler-improvements-design.md`

---

### Task 1: Add Redis Storage Config Fields

**Files:**
- Modify: `crawler/internal/config/crawler/config.go`
- Modify: `crawler/internal/config/crawler/config.go` (add new constants and fields)

**Context:** The crawler config struct at `crawler/internal/config/crawler/config.go` uses `env` and `yaml` struct tags. We need to add Redis storage config fields to it. The existing `RedisConfig` in `crawler/internal/config/config.go` handles the general Redis connection (for events). The new fields are specifically for Colly's Redis storage backend.

**Step 1: Add constants and config fields**

Add to `crawler/internal/config/crawler/config.go`:

```go
// Add new defaults at the top with existing constants
const (
	// DefaultRedisStorageExpires is the default TTL for Colly visited URL keys in Redis
	DefaultRedisStorageExpires = 168 * time.Hour // 7 days
)
```

Add new fields to the `Config` struct (after `HTTPRetryDelay`):

```go
	// RedisStorageEnabled enables Redis-backed Colly storage for visited URLs, cookies, and request queue
	RedisStorageEnabled bool `env:"CRAWLER_REDIS_STORAGE_ENABLED" yaml:"redis_storage_enabled"`
	// RedisStorageExpires is the TTL for visited URL keys in Redis (0 = no expiry)
	RedisStorageExpires time.Duration `env:"CRAWLER_REDIS_STORAGE_EXPIRES" yaml:"redis_storage_expires"`
```

Update `New()` defaults:

```go
	RedisStorageEnabled: false,
	RedisStorageExpires: DefaultRedisStorageExpires,
```

**Step 2: Run linter to verify**

Run: `cd crawler && golangci-lint run ./internal/config/crawler/...`
Expected: PASS (no issues)

**Step 3: Commit**

```bash
git add crawler/internal/config/crawler/config.go
git commit -m "feat(crawler): add Redis storage config fields"
```

---

### Task 2: Add Proxy Config Fields

**Files:**
- Modify: `crawler/internal/config/crawler/config.go`

**Context:** Proxy configuration follows the same pattern as other crawler config. Proxy URLs come from a comma-separated env var. We need an enabled flag and a URL list.

**Step 1: Add proxy config fields to Config struct**

Add after the Redis storage fields:

```go
	// ProxiesEnabled enables round-robin proxy rotation for requests
	ProxiesEnabled bool `env:"CRAWLER_PROXIES_ENABLED" yaml:"proxies_enabled"`
	// ProxyURLs is the list of proxy URLs (HTTP or SOCKS5) for round-robin rotation
	ProxyURLs []string `env:"CRAWLER_PROXY_URLS" yaml:"proxy_urls"`
```

Update `New()` defaults:

```go
	ProxiesEnabled: false,
	ProxyURLs:      nil,
```

**Step 2: Run linter to verify**

Run: `cd crawler && golangci-lint run ./internal/config/crawler/...`
Expected: PASS

**Step 3: Commit**

```bash
git add crawler/internal/config/crawler/config.go
git commit -m "feat(crawler): add proxy rotation config fields"
```

---

### Task 3: Install redisstorage dependency

**Files:**
- Modify: `crawler/go.mod`
- Modify: `crawler/go.sum`
- Modify: `crawler/vendor/`

**Context:** The `gocolly/redisstorage` package provides a Redis-backed storage backend for Colly that implements `colly/storage.Storage` and `queue.Storage`. The crawler already has `go-redis/v9` in go.mod.

**Step 1: Add the dependency**

```bash
cd crawler && go get -u github.com/gocolly/redisstorage
```

**Step 2: Vendor**

```bash
cd /home/fsd42/dev/north-cloud && task vendor
```

**Step 3: Verify it builds**

```bash
cd crawler && go build ./...
```
Expected: PASS

**Step 4: Commit**

```bash
git add crawler/go.mod crawler/go.sum crawler/vendor/
git commit -m "feat(crawler): add gocolly/redisstorage dependency"
```

---

### Task 4: Integrate Redis Storage into Collector Setup

**Files:**
- Modify: `crawler/internal/crawler/crawler.go` (add `redisClient` field)
- Modify: `crawler/internal/crawler/constructor.go` (accept Redis client)
- Modify: `crawler/internal/crawler/collector.go` (set Redis storage on collector)

**Context:** The collector is created in `setupCollector()` at `crawler/internal/crawler/collector.go:120`. After `colly.NewCollector(opts...)`, we need to call `c.SetStorage(storage)` if Redis storage is enabled. The Crawler struct needs a reference to the Redis client (already created in `bootstrap/redis.go`). The `CrawlerParams` struct at `constructor.go:37` is how dependencies are injected.

**Step 1: Add Redis client to Crawler struct**

In `crawler/internal/crawler/crawler.go`, add to the `Crawler` struct:

```go
	redisClient *redis.Client // Redis client for Colly storage (optional)
```

Add import: `"github.com/redis/go-redis/v9"`

**Step 2: Add Redis client to CrawlerParams**

In `crawler/internal/crawler/constructor.go`, add to `CrawlerParams`:

```go
	RedisClient *redis.Client // Redis client for Colly storage (optional)
```

Add import: `"github.com/redis/go-redis/v9"`

Wire it in `NewCrawlerWithParams`:

```go
	c := &Crawler{
		// ... existing fields ...
		redisClient: p.RedisClient,
	}
```

**Step 3: Set Redis storage in setupCollector**

In `crawler/internal/crawler/collector.go`, after `c.collector = colly.NewCollector(opts...)` (line ~120), add:

```go
	// Set Redis storage if enabled
	if storageErr := c.setupRedisStorage(); storageErr != nil {
		c.GetJobLogger().Warn(logs.CategoryLifecycle, "Failed to set Redis storage, using in-memory",
			logs.Err(storageErr),
		)
	}
```

Add a new method to `collector.go`:

```go
// setupRedisStorage configures Redis-backed Colly storage if enabled and available.
func (c *Crawler) setupRedisStorage() error {
	if !c.cfg.RedisStorageEnabled || c.redisClient == nil {
		return nil
	}

	// Get job ID for key prefix isolation
	crawlCtx := c.getCrawlContext()
	prefix := "crawler:default"
	if crawlCtx != nil {
		prefix = "crawler:" + crawlCtx.SourceID
	}

	storage := &redisstorage.Storage{
		Address:  c.redisClient.Options().Addr,
		Password: c.redisClient.Options().Password,
		DB:       c.redisClient.Options().DB,
		Prefix:   prefix,
		Expires:  c.cfg.RedisStorageExpires,
	}

	if err := c.collector.SetStorage(storage); err != nil {
		return fmt.Errorf("failed to set Redis storage: %w", err)
	}

	c.GetJobLogger().Info(logs.CategoryLifecycle, "Redis storage enabled for Colly",
		logs.String("prefix", prefix),
		logs.Duration("expires", c.cfg.RedisStorageExpires),
	)
	return nil
}
```

Add imports: `"github.com/gocolly/redisstorage"`

**Step 4: Wire Redis client through bootstrap**

The bootstrap already creates a Redis client in `bootstrap/redis.go`. Find where `CrawlerParams` is constructed in the bootstrap pipeline and pass the Redis client. Check `crawler/internal/bootstrap/app.go` or similar for where `NewCrawlerWithParams` is called.

**Step 5: Run linter and tests**

```bash
cd crawler && golangci-lint run ./internal/crawler/...
cd crawler && go test ./internal/crawler/...
```
Expected: PASS

**Step 6: Commit**

```bash
git add crawler/internal/crawler/crawler.go crawler/internal/crawler/constructor.go crawler/internal/crawler/collector.go
git commit -m "feat(crawler): integrate Redis storage backend for Colly"
```

---

### Task 5: Add Proxy Rotation to Collector Setup

**Files:**
- Modify: `crawler/internal/crawler/collector.go`

**Context:** Colly provides `proxy.RoundRobinProxySwitcher()` which takes a variadic list of proxy URL strings and returns a `ProxyFunc`. This is set on the collector via `c.SetProxyFunc(rp)`. The proxy func is set after collector creation but before any requests. The response callback already has access to `r.Request.ProxyURL` for logging.

**Step 1: Add proxy setup to setupCollector**

In `crawler/internal/crawler/collector.go`, after the Redis storage setup and before `configureTransport()`, add:

```go
	// Set up proxy rotation if enabled
	if proxyErr := c.setupProxyRotation(); proxyErr != nil {
		return fmt.Errorf("failed to set up proxy rotation: %w", proxyErr)
	}
```

Add a new method:

```go
// setupProxyRotation configures round-robin proxy rotation if enabled.
func (c *Crawler) setupProxyRotation() error {
	if !c.cfg.ProxiesEnabled || len(c.cfg.ProxyURLs) == 0 {
		return nil
	}

	rp, err := proxy.RoundRobinProxySwitcher(c.cfg.ProxyURLs...)
	if err != nil {
		return fmt.Errorf("failed to create proxy switcher: %w", err)
	}

	c.collector.SetProxyFunc(rp)

	c.GetJobLogger().Info(logs.CategoryLifecycle, "Proxy rotation enabled",
		logs.Int("proxy_count", len(c.cfg.ProxyURLs)),
	)
	return nil
}
```

Add import: `"github.com/gocolly/colly/v2/proxy"`

**Step 2: Add proxy URL logging to response callback**

In `responseCallback()`, after the existing `jl.Debug` for "Response received", add:

```go
		if proxyURL := r.Request.ProxyURL; proxyURL != "" {
			jl.Debug(logs.CategoryFetch, "Request via proxy",
				logs.URL(pageURL),
				logs.String("proxy", proxyURL),
			)
		}
```

**Step 3: Run linter**

```bash
cd crawler && golangci-lint run ./internal/crawler/...
```
Expected: PASS

**Step 4: Commit**

```bash
git add crawler/internal/crawler/collector.go
git commit -m "feat(crawler): add round-robin proxy rotation support"
```

---

### Task 6: Max Depth Default Guard

**Files:**
- Modify: `crawler/internal/crawler/collector.go`

**Context:** Currently `setupCollector()` at line 66 reads `maxDepth := source.MaxDepth` and passes it directly to Colly. If `MaxDepth` is 0, Colly treats it as unlimited depth — which is dangerous. We need to default 0 to 3 and warn if depth > 5.

**Step 1: Add constant and depth guard**

Add constant to `collector.go`:

```go
const (
	// defaultMaxDepth is used when source MaxDepth is 0 (unset)
	defaultMaxDepth = 3
	// warnMaxDepth logs a warning when source MaxDepth exceeds this value
	warnMaxDepth = 5
)
```

In `setupCollector()`, replace `maxDepth := source.MaxDepth` with:

```go
	maxDepth := source.MaxDepth
	if maxDepth == 0 {
		maxDepth = defaultMaxDepth
		c.GetJobLogger().Info(logs.CategoryLifecycle, "Using default max depth",
			logs.Int("max_depth", maxDepth),
		)
	}
	if maxDepth > warnMaxDepth {
		c.GetJobLogger().Warn(logs.CategoryLifecycle, "Max depth exceeds recommended limit",
			logs.Int("max_depth", maxDepth),
			logs.Int("recommended_max", warnMaxDepth),
		)
	}
```

**Step 2: Run linter**

```bash
cd crawler && golangci-lint run ./internal/crawler/...
```
Expected: PASS

**Step 3: Commit**

```bash
git add crawler/internal/crawler/collector.go
git commit -m "feat(crawler): default max depth to 3 with depth-5 warning"
```

---

### Task 7: Add adaptive_scheduling Field to Job Model

**Files:**
- Create: `crawler/migrations/013_add_adaptive_scheduling.up.sql`
- Create: `crawler/migrations/013_add_adaptive_scheduling.down.sql`
- Modify: `crawler/internal/domain/job.go`

**Context:** The Job struct at `crawler/internal/domain/job.go` uses `db` struct tags for sqlx. Migration files follow the pattern `NNN_description.up.sql` / `NNN_description.down.sql`. The next migration number is 013.

**Step 1: Create migration files**

`crawler/migrations/013_add_adaptive_scheduling.up.sql`:
```sql
ALTER TABLE jobs ADD COLUMN adaptive_scheduling BOOLEAN NOT NULL DEFAULT true;
```

`crawler/migrations/013_add_adaptive_scheduling.down.sql`:
```sql
ALTER TABLE jobs DROP COLUMN IF EXISTS adaptive_scheduling;
```

**Step 2: Add field to Job struct**

In `crawler/internal/domain/job.go`, add after `BackoffUntil`:

```go
	// Adaptive scheduling (adjusts interval based on content change detection)
	AdaptiveScheduling bool `db:"adaptive_scheduling" json:"adaptive_scheduling"`
```

**Step 3: Run migration locally**

```bash
cd crawler && go run cmd/migrate/main.go up
```
Expected: Migration 013 applied successfully

**Step 4: Commit**

```bash
git add crawler/migrations/013_add_adaptive_scheduling.up.sql crawler/migrations/013_add_adaptive_scheduling.down.sql crawler/internal/domain/job.go
git commit -m "feat(crawler): add adaptive_scheduling field to jobs table"
```

---

### Task 8: Page Hash Tracker (Redis-backed)

**Files:**
- Create: `crawler/internal/adaptive/hash_tracker.go`
- Create: `crawler/internal/adaptive/hash_tracker_test.go`

**Context:** This component stores and compares SHA-256 hashes of start URL content in Redis. It uses the existing Redis client from `crawler/internal/bootstrap/redis.go`. Redis key pattern: `crawler:adaptive:{source_id}` stores a JSON hash with `last_hash`, `last_change_at`, `unchanged_count`, `current_interval`.

**Step 1: Write the test**

Create `crawler/internal/adaptive/hash_tracker_test.go`:

```go
package adaptive

import (
	"testing"
	"time"
)

func TestComputeHash(t *testing.T) {
	t.Helper()

	hash := ComputeHash([]byte("<html><body>Hello World</body></html>"))
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}

	// Same input should produce same hash
	hash2 := ComputeHash([]byte("<html><body>Hello World</body></html>"))
	if hash != hash2 {
		t.Fatalf("expected same hash for same input: %s != %s", hash, hash2)
	}

	// Different input should produce different hash
	hash3 := ComputeHash([]byte("<html><body>Different</body></html>"))
	if hash == hash3 {
		t.Fatal("expected different hash for different input")
	}
}

func TestCalculateAdaptiveInterval(t *testing.T) {
	t.Helper()

	baseline := 30 * time.Minute
	maxInterval := 24 * time.Hour

	tests := []struct {
		name           string
		unchangedCount int
		expected       time.Duration
	}{
		{"changed (0 unchanged)", 0, baseline},
		{"1 unchanged", 1, 60 * time.Minute},
		{"2 unchanged", 2, 2 * time.Hour},
		{"3 unchanged", 3, 4 * time.Hour},
		{"7+ unchanged caps at max", 7, maxInterval},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Helper()
			result := CalculateAdaptiveInterval(baseline, maxInterval, tt.unchangedCount)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

```bash
cd crawler && go test ./internal/adaptive/... -v
```
Expected: FAIL (package doesn't exist yet)

**Step 3: Write the implementation**

Create `crawler/internal/adaptive/hash_tracker.go`:

```go
package adaptive

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/redis/go-redis/v9"
)

// Adaptive scheduling constants
const (
	maxAdaptiveInterval = 24 * time.Hour
	keyPrefix           = "crawler:adaptive:"
	exponentialBase     = 2.0
)

// HashState holds the adaptive scheduling state for a source.
type HashState struct {
	LastHash       string        `json:"last_hash"`
	LastChangeAt   time.Time     `json:"last_change_at"`
	UnchangedCount int           `json:"unchanged_count"`
	CurrentInterval time.Duration `json:"current_interval"`
}

// HashTracker stores and compares content hashes in Redis for adaptive scheduling.
type HashTracker struct {
	client *redis.Client
}

// NewHashTracker creates a new hash tracker.
func NewHashTracker(client *redis.Client) *HashTracker {
	return &HashTracker{client: client}
}

// ComputeHash returns the hex-encoded SHA-256 of content.
func ComputeHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// CalculateAdaptiveInterval computes the next crawl interval based on unchanged count.
// Formula: baseline * 2^(unchangedCount), capped at maxInterval.
func CalculateAdaptiveInterval(baseline, maxInterval time.Duration, unchangedCount int) time.Duration {
	if unchangedCount <= 0 {
		return baseline
	}
	multiplier := math.Pow(exponentialBase, float64(unchangedCount))
	interval := time.Duration(float64(baseline) * multiplier)
	if interval > maxInterval {
		return maxInterval
	}
	return interval
}

// CompareAndUpdate compares a new hash against the stored hash for a source.
// Returns the updated state and whether the content changed.
func (ht *HashTracker) CompareAndUpdate(ctx context.Context, sourceID, newHash string, baseline time.Duration) (*HashState, bool, error) {
	key := keyPrefix + sourceID

	// Get existing state
	var state HashState
	data, err := ht.client.Get(ctx, key).Bytes()
	if err != nil && err != redis.Nil {
		return nil, false, fmt.Errorf("failed to get hash state: %w", err)
	}
	if err == nil {
		if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
			return nil, false, fmt.Errorf("failed to unmarshal hash state: %w", unmarshalErr)
		}
	}

	changed := state.LastHash != newHash
	now := time.Now()

	if changed {
		state.LastHash = newHash
		state.LastChangeAt = now
		state.UnchangedCount = 0
		state.CurrentInterval = baseline
	} else {
		state.UnchangedCount++
		state.CurrentInterval = CalculateAdaptiveInterval(baseline, maxAdaptiveInterval, state.UnchangedCount)
	}

	// Store updated state
	stateBytes, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		return nil, false, fmt.Errorf("failed to marshal hash state: %w", marshalErr)
	}
	if setErr := ht.client.Set(ctx, key, stateBytes, 0).Err(); setErr != nil {
		return nil, false, fmt.Errorf("failed to set hash state: %w", setErr)
	}

	return &state, changed, nil
}

// GetState retrieves the current hash state for a source.
func (ht *HashTracker) GetState(ctx context.Context, sourceID string) (*HashState, error) {
	key := keyPrefix + sourceID
	data, err := ht.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get hash state: %w", err)
	}

	var state HashState
	if unmarshalErr := json.Unmarshal(data, &state); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal hash state: %w", unmarshalErr)
	}
	return &state, nil
}
```

**Step 4: Run tests**

```bash
cd crawler && go test ./internal/adaptive/... -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/adaptive/
git commit -m "feat(crawler): add hash tracker for adaptive scheduling"
```

---

### Task 9: Capture Start URL Hashes During Crawl

**Files:**
- Modify: `crawler/internal/crawler/crawler.go` (add hash map and hash tracker)
- Modify: `crawler/internal/crawler/collector.go` (capture hash in response callback)
- Modify: `crawler/internal/crawler/constructor.go` (inject hash tracker)

**Context:** During a crawl, when the response for a start URL comes back, we compute its SHA-256 and store it on the Crawler struct. After the crawl completes, the scheduler reads these hashes and updates the adaptive state. Start URLs are defined in the source config as `source.StartURLs` or `source.URL`.

**Step 1: Add start URL hash tracking to Crawler struct**

In `crawler/internal/crawler/crawler.go`, add to `Crawler` struct:

```go
	// Adaptive scheduling: stores hashes of start URL responses
	startURLHashes   map[string]string // URL -> SHA-256 hash
	startURLHashesMu sync.RWMutex
	hashTracker      *adaptive.HashTracker // Redis-backed hash tracker (optional)
```

Add import: `"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"`

**Step 2: Wire hash tracker through constructor**

In `constructor.go`, add to `CrawlerParams`:

```go
	HashTracker *adaptive.HashTracker // For adaptive scheduling (optional)
```

Wire in `NewCrawlerWithParams`:

```go
	hashTracker: p.HashTracker,
```

**Step 3: Initialize hash map in Start**

In `start.go`, in the `Start()` method, after `c.lifecycle.Reset()`:

```go
	// Reset start URL hash map for this execution
	c.startURLHashesMu.Lock()
	c.startURLHashes = make(map[string]string)
	c.startURLHashesMu.Unlock()
```

**Step 4: Capture hash in response callback**

In `collector.go`, in `responseCallback()`, before the archiver block, add:

```go
		// Capture hash for start URLs (adaptive scheduling)
		c.captureStartURLHash(pageURL, r.Body)
```

Add method:

```go
// captureStartURLHash stores the SHA-256 hash of a start URL's response body.
func (c *Crawler) captureStartURLHash(pageURL string, body []byte) {
	crawlCtx := c.getCrawlContext()
	if crawlCtx == nil || crawlCtx.Source == nil {
		return
	}

	// Check if this URL is a start URL
	isStartURL := pageURL == crawlCtx.Source.URL
	if !isStartURL {
		for _, u := range crawlCtx.Source.StartURLs {
			if pageURL == u {
				isStartURL = true
				break
			}
		}
	}
	if !isStartURL {
		return
	}

	hash := adaptive.ComputeHash(body)
	c.startURLHashesMu.Lock()
	c.startURLHashes[pageURL] = hash
	c.startURLHashesMu.Unlock()
}
```

**Step 5: Add getter for scheduler access**

Add to `crawler.go`:

```go
// GetStartURLHashes returns the hashes captured during the last crawl.
func (c *Crawler) GetStartURLHashes() map[string]string {
	c.startURLHashesMu.RLock()
	defer c.startURLHashesMu.RUnlock()
	result := make(map[string]string, len(c.startURLHashes))
	for k, v := range c.startURLHashes {
		result[k] = v
	}
	return result
}

// GetHashTracker returns the hash tracker for adaptive scheduling.
func (c *Crawler) GetHashTracker() *adaptive.HashTracker {
	return c.hashTracker
}
```

Add these methods to the `Interface` interface:

```go
	// GetStartURLHashes returns the hashes captured during the last crawl
	GetStartURLHashes() map[string]string
	// GetHashTracker returns the hash tracker for adaptive scheduling
	GetHashTracker() *adaptive.HashTracker
```

**Step 6: Run linter and tests**

```bash
cd crawler && golangci-lint run ./internal/crawler/...
cd crawler && go test ./...
```
Expected: PASS

**Step 7: Commit**

```bash
git add crawler/internal/crawler/
git commit -m "feat(crawler): capture start URL hashes for adaptive scheduling"
```

---

### Task 10: Integrate Adaptive Scheduling into Scheduler

**Files:**
- Modify: `crawler/internal/scheduler/interval_scheduler.go`

**Context:** After a job completes successfully in `handleJobSuccess()`, the scheduler currently calculates `nextRun` using `calculateNextRun(job)` which uses the fixed interval. We need to check if `job.AdaptiveScheduling` is true, and if so, use the hash tracker to determine the adaptive interval instead.

**Step 1: Add adaptive scheduling to handleJobSuccess**

In `interval_scheduler.go`, in `handleJobSuccess()`, replace the block at lines ~787-791:

```go
	// If recurring, schedule next run
	if job.IntervalMinutes != nil && job.ScheduleEnabled {
		job.Status = "scheduled"
		nextRun := s.calculateNextRun(job)
		job.NextRunAt = &nextRun
	}
```

With:

```go
	// If recurring, schedule next run
	if job.IntervalMinutes != nil && job.ScheduleEnabled {
		job.Status = "scheduled"
		nextRun := s.calculateAdaptiveOrFixedNextRun(jobExec, job)
		job.NextRunAt = &nextRun
	}
```

Add the new method:

```go
// calculateAdaptiveOrFixedNextRun calculates the next run time.
// If adaptive scheduling is enabled and hash data is available, uses content change detection.
// Otherwise falls back to the fixed interval.
func (s *IntervalScheduler) calculateAdaptiveOrFixedNextRun(jobExec *JobExecution, job *domain.Job) time.Time {
	if !job.AdaptiveScheduling {
		return s.calculateNextRun(job)
	}

	hashTracker := s.crawler.GetHashTracker()
	if hashTracker == nil {
		return s.calculateNextRun(job)
	}

	hashes := s.crawler.GetStartURLHashes()
	if len(hashes) == 0 {
		return s.calculateNextRun(job)
	}

	// Use the first start URL hash (primary content indicator)
	var firstHash string
	for _, h := range hashes {
		firstHash = h
		break
	}

	baseline := getIntervalDuration(job)
	state, changed, err := hashTracker.CompareAndUpdate(jobExec.Context, job.SourceID, firstHash, baseline)
	if err != nil {
		s.logger.Warn("Adaptive scheduling hash comparison failed, using fixed interval",
			infralogger.String("job_id", job.ID),
			infralogger.Error(err),
		)
		return s.calculateNextRun(job)
	}

	s.logger.Info("Adaptive scheduling decision",
		infralogger.String("job_id", job.ID),
		infralogger.Bool("content_changed", changed),
		infralogger.Int("unchanged_count", state.UnchangedCount),
		infralogger.Duration("adaptive_interval", state.CurrentInterval),
		infralogger.Duration("baseline_interval", baseline),
	)

	return time.Now().Add(state.CurrentInterval)
}
```

**Step 2: Run linter and tests**

```bash
cd crawler && golangci-lint run ./internal/scheduler/...
cd crawler && go test ./internal/scheduler/...
```
Expected: PASS

**Step 3: Commit**

```bash
git add crawler/internal/scheduler/interval_scheduler.go
git commit -m "feat(crawler): integrate adaptive scheduling into job completion"
```

---

### Task 11: Wire Everything Through Bootstrap

**Files:**
- Modify: `crawler/internal/bootstrap/app.go` (or wherever `NewCrawlerWithParams` is called)

**Context:** The bootstrap creates the Redis client in `bootstrap/redis.go`. It creates the crawler in `NewCrawlerWithParams`. We need to pass the Redis client and create the hash tracker, then pass both to the crawler params.

**Step 1: Find and modify the bootstrap wiring**

Look in `crawler/internal/bootstrap/app.go` for where `CrawlerParams` is constructed. Add:

```go
// Create hash tracker for adaptive scheduling if Redis is available
var hashTracker *adaptive.HashTracker
if redisClient != nil {
	hashTracker = adaptive.NewHashTracker(redisClient)
}
```

Then in the `CrawlerParams` construction:

```go
	RedisClient: redisClient,
	HashTracker: hashTracker,
```

**Step 2: Update .env.example with new env vars**

Add to `.env.example`:

```bash
# Crawler Redis Storage (Colly visited URLs persistence)
CRAWLER_REDIS_STORAGE_ENABLED=false
CRAWLER_REDIS_STORAGE_EXPIRES=168h

# Crawler Proxy Rotation
CRAWLER_PROXIES_ENABLED=false
CRAWLER_PROXY_URLS=
```

**Step 3: Run full test suite**

```bash
cd crawler && go test ./...
```
Expected: PASS

**Step 4: Run linter on full crawler**

```bash
cd crawler && golangci-lint run
```
Expected: PASS

**Step 5: Commit**

```bash
git add crawler/internal/bootstrap/ .env.example
git commit -m "feat(crawler): wire Redis storage, proxy, and adaptive scheduling through bootstrap"
```

---

### Task 12: Update CLAUDE.md Documentation

**Files:**
- Modify: `crawler/CLAUDE.md`

**Context:** The crawler CLAUDE.md needs to document the new features: Redis storage, proxy rotation, adaptive scheduling, and the max depth default.

**Step 1: Add sections**

Add after the "Common Gotchas" section:

```markdown
## Redis Storage (Colly)

Enabled via `CRAWLER_REDIS_STORAGE_ENABLED=true`. Persists Colly's visited URLs, cookies, and request queue in Redis.

- Key prefix: `crawler:{source_id}:`
- TTL: 7 days (configurable via `CRAWLER_REDIS_STORAGE_EXPIRES`)
- Falls back to in-memory if Redis unavailable

## Proxy Rotation

Enabled via `CRAWLER_PROXIES_ENABLED=true`. Uses Colly's `RoundRobinProxySwitcher`.

- `CRAWLER_PROXY_URLS`: Comma-separated list of proxy URLs (HTTP or SOCKS5)
- Global to all sources, round-robin rotation
- Proxy URL logged per request at debug level

## Adaptive Scheduling

Jobs with `adaptive_scheduling: true` (default) adjust their crawl interval based on content changes.

- After each crawl, SHA-256 of start URL content is compared to previous hash
- **Changed**: Reset to baseline interval
- **Unchanged**: Exponential backoff (`baseline × 2^unchanged_count`, max 24h)
- State stored in Redis: `crawler:adaptive:{source_id}`
- Jobs with `adaptive_scheduling: false` use fixed intervals

## Max Depth

Default max depth is **3** when source config `max_depth` is 0 (unset).
Sources with `max_depth > 5` trigger a startup warning.
```

**Step 2: Commit**

```bash
git add crawler/CLAUDE.md
git commit -m "docs(crawler): document Redis storage, proxy, adaptive scheduling, and depth defaults"
```

---

## Implementation Order Summary

| Task | Component | Depends On |
|------|-----------|------------|
| 1 | Redis storage config | - |
| 2 | Proxy config | - |
| 3 | Install redisstorage | - |
| 4 | Redis storage integration | Tasks 1, 3 |
| 5 | Proxy rotation | Task 2 |
| 6 | Max depth default | - |
| 7 | adaptive_scheduling DB field | - |
| 8 | Hash tracker | Task 3 |
| 9 | Start URL hash capture | Tasks 4, 8 |
| 10 | Scheduler adaptive logic | Tasks 7, 9 |
| 11 | Bootstrap wiring | Tasks 4, 5, 9 |
| 12 | Documentation | Tasks 1-11 |
