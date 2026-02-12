# Pipeline & Intelligence Dashboard Testing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the tests defined in `docs/TESTING_PIPELINE_CHECKLIST.md` — dashboard problem detection edge-case tests and the crawl→classify→publish pipeline integration smoke test.

**Architecture:** Two independent work streams: (A) add boundary/edge tests to the existing Vitest problem detection suite in `dashboard/src/features/intelligence/problems/`, and (B) build the Layer 2 pipeline integration test as a Go test harness in `tests/integration/pipeline/` that boots the full Docker Compose stack and verifies one article flows from fixture URL to Redis.

**Tech Stack:** Vitest 4 + TypeScript (dashboard tests), Go 1.24+ + testify (pipeline integration), Docker Compose, nc-http-proxy fixtures, Redis pub/sub.

**References:**
- `docs/TESTING_PIPELINE_CHECKLIST.md` — what to assert and what not to
- `docs/plans/2026-02-05-testing-standardization-design.md` — Layer 2 design
- `docs/plans/2026-02-11-intelligence-dashboard-redesign.md` — problem rules spec

---

## Part A: Intelligence Dashboard Problem Detection Tests

### Current state

`dashboard/src/features/intelligence/problems/rules.test.ts` has 11 tests:
- 1 happy path (healthy → empty array)
- 7 per-rule tests (failed-crawls, stale-scheduled-jobs, empty-indexes, inactive-sources-ignored, classification-backlog, inactive-channels, zero-publishing)
- 2 cluster health variants (yellow → warning, red → error)
- 1 service-unreachable per service (3 tests)

Missing per checklist:
- Boundary threshold tests (backlog at exactly 100, at 99, at 101)
- Single-item boundary (1 failed job, 1 stale job)
- Multiple simultaneous problems
- Doc comment on happy-path test clarifying it's "smoke check only"
- Dashboard Taskfile wired to vitest

---

### Task A1: Wire dashboard Taskfile to vitest

**Files:**
- Modify: `dashboard/Taskfile.yml`

**Step 1: Update Taskfile test commands**

Replace the placeholder echo commands with real vitest commands:

```yaml
  test:
    desc: "Run tests"
    cmds:
      - npx vitest run

  test:coverage:
    desc: "Run tests with coverage"
    cmds:
      - npx vitest run --coverage
```

**Step 2: Run the existing tests to verify**

Run: `cd dashboard && npx vitest run`
Expected: 11 tests pass (the existing rules.test.ts suite)

**Step 3: Commit**

```bash
git add dashboard/Taskfile.yml
git commit -m "build(dashboard): wire Taskfile test commands to vitest"
```

---

### Task A2: Add boundary and edge-case tests to rules.test.ts

**Files:**
- Modify: `dashboard/src/features/intelligence/problems/rules.test.ts`

**Step 1: Write the boundary tests**

Add a new `describe('boundary and edge cases')` block after the existing tests:

```typescript
describe('boundary and edge cases', () => {
  it('does not fire classification-backlog at threshold (100)', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'borderline', rawCount: 200, classifiedCount: 100, backlog: 100, delta24h: 5, avgQuality: 70, active: true },
    ]
    const problems = detectProblems(metrics)
    expect(problems.find((p) => p.id === 'classification-backlog')).toBeUndefined()
  })

  it('fires classification-backlog above threshold (101)', () => {
    const metrics = healthyMetrics()
    metrics.indexes!.sources = [
      { source: 'over', rawCount: 201, classifiedCount: 100, backlog: 101, delta24h: 5, avgQuality: 70, active: true },
    ]
    const problems = detectProblems(metrics)
    expect(problems.find((p) => p.id === 'classification-backlog')).toBeDefined()
  })

  it('fires failed-crawls for exactly 1 failed job', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.failedJobs = 1
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'failed-crawls')
    expect(p).toBeDefined()
    expect(p!.title).toBe('1 failed crawl job')
    expect(p!.count).toBe(1)
  })

  it('fires stale-scheduled-jobs for exactly 1 stale job', () => {
    const metrics = healthyMetrics()
    metrics.crawler!.staleJobs = 1
    const problems = detectProblems(metrics)
    const p = problems.find((p) => p.id === 'stale-scheduled-jobs')
    expect(p).toBeDefined()
    expect(p!.title).toBe('1 stale scheduled job')
  })

  it('detects multiple problems simultaneously', () => {
    const metrics: PipelineMetrics = {
      crawler: { failedJobs: 5, staleJobs: 2, failedJobUrls: [] },
      indexes: {
        clusterHealth: 'yellow',
        sources: [
          { source: 'empty', rawCount: 0, classifiedCount: 0, backlog: 0, delta24h: 0, avgQuality: 0, active: true },
        ],
      },
      publisher: { publishedToday: 0, inactiveChannels: 1, inactiveChannelNames: ['Crime Feed'] },
    }
    const problems = detectProblems(metrics)
    const ids = problems.map((p) => p.id)
    expect(ids).toContain('failed-crawls')
    expect(ids).toContain('stale-scheduled-jobs')
    expect(ids).toContain('cluster-health')
    expect(ids).toContain('empty-indexes')
    expect(ids).toContain('zero-publishing')
    expect(ids).toContain('inactive-channels')
  })

  it('detects all three services unreachable', () => {
    const metrics: PipelineMetrics = {
      crawler: null,
      indexes: null,
      publisher: null,
    }
    const problems = detectProblems(metrics)
    expect(problems).toHaveLength(3)
    expect(problems.every((p) => p.kind === 'system')).toBe(true)
    expect(problems.every((p) => p.severity === 'error')).toBe(true)
  })
})
```

**Step 2: Run tests to verify they pass**

Run: `cd dashboard && npx vitest run`
Expected: All 18 tests pass (11 existing + 7 new)

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/problems/rules.test.ts
git commit -m "test(dashboard): add boundary and edge-case tests for problem detection rules"
```

---

### Task A3: Add smoke-check doc comment to happy-path test

**Files:**
- Modify: `dashboard/src/features/intelligence/problems/rules.test.ts`

**Step 1: Add clarifying comment**

Above the existing happy-path test, add:

```typescript
  // Smoke check: verifies rules don't fire when all inputs are nominal.
  // This does NOT prove the pipeline is healthy — it only validates rule logic.
  // See docs/TESTING_PIPELINE_CHECKLIST.md §1 for context.
  it('returns empty array when everything is healthy', () => {
```

**Step 2: Run tests to verify nothing broke**

Run: `cd dashboard && npx vitest run`
Expected: All 18 tests pass

**Step 3: Commit**

```bash
git add dashboard/src/features/intelligence/problems/rules.test.ts
git commit -m "docs(dashboard): clarify happy-path test is smoke check only per testing checklist"
```

---

## Part B: Pipeline Integration Test

### Current state

- No `tests/integration/` directory exists
- No `docker-compose.test.yml` exists
- Fixtures exist in `crawler/fixtures/fixture-news-site-com/` (3 HTTP responses: homepage, article, another page)
- Contract tests exist in `tests/contracts/` (5 test files)
- nc-http-proxy supports replay mode with fixtures

### Design decisions

Per `docs/TESTING_PIPELINE_CHECKLIST.md`:
- **Should assert:** raw_content doc exists, classified_content doc exists with contract fields, Redis message received with required payload fields, same article ID across all three
- **Should NOT assert:** exact quality score, exact topics, volume, full schema, auth, resilience
- **Single purpose:** one fixture, one source, one channel, one route — pipeline connectivity + contract at ES/Redis boundaries
- Test uses nc-http-proxy in replay mode for deterministic fixtures

---

### Task B1: Create docker-compose.test.yml

**Files:**
- Create: `docker-compose.test.yml`

**Step 1: Write the test compose override**

This extends `docker-compose.base.yml` with test-specific config: no volume mounts for hot reload, deterministic env vars, no observability stack.

```yaml
# Test-specific overrides for pipeline integration tests.
# Usage: docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d
#
# Boots: postgres instances, elasticsearch, redis, auth, source-manager,
# crawler, classifier, publisher, index-manager, nc-http-proxy (replay mode).
# No hot reload, no observability, no dashboard.

services:
  # --- Infrastructure (inherit from base, no port exposure) ---

  elasticsearch:
    ports: []

  redis:
    ports: []

  # --- Go services (built from Dockerfile, no dev volumes) ---

  auth:
    build:
      context: ./auth
      dockerfile: Dockerfile
    environment:
      AUTH_USERNAME: ${AUTH_USERNAME:-admin}
      AUTH_PASSWORD: ${AUTH_PASSWORD:-testpass123}
      AUTH_JWT_SECRET: ${AUTH_JWT_SECRET:-test-jwt-secret-for-integration}

  source-manager:
    build:
      context: ./source-manager
      dockerfile: Dockerfile

  crawler:
    build:
      context: ./crawler
      dockerfile: Dockerfile
    environment:
      HTTP_PROXY: http://nc-http-proxy:8055
      HTTPS_PROXY: http://nc-http-proxy:8055

  classifier:
    build:
      context: ./classifier
      dockerfile: Dockerfile

  publisher:
    build:
      context: ./publisher
      dockerfile: Dockerfile

  index-manager:
    build:
      context: ./index-manager
      dockerfile: Dockerfile

  nc-http-proxy:
    build:
      context: ./nc-http-proxy
      dockerfile: Dockerfile
    environment:
      PROXY_MODE: replay
    volumes:
      - ./crawler/fixtures:/app/fixtures:ro
```

**Step 2: Verify compose config parses**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.test.yml config --services`
Expected: Lists all services without errors

**Step 3: Commit**

```bash
git add docker-compose.test.yml
git commit -m "build: add docker-compose.test.yml for pipeline integration tests"
```

---

### Task B2: Create Go module for pipeline integration tests

**Files:**
- Create: `tests/integration/pipeline/go.mod`
- Create: `tests/integration/pipeline/go.sum` (generated)

**Step 1: Initialize Go module**

```bash
mkdir -p tests/integration/pipeline
cd tests/integration/pipeline
go mod init github.com/jonesrussell/north-cloud/tests/integration/pipeline
go get github.com/stretchr/testify
go get github.com/redis/go-redis/v9
```

**Step 2: Verify module**

Run: `cd tests/integration/pipeline && go mod tidy`
Expected: Clean output, go.mod and go.sum created

**Step 3: Commit**

```bash
git add tests/integration/pipeline/go.mod tests/integration/pipeline/go.sum
git commit -m "build: initialize Go module for pipeline integration tests"
```

---

### Task B3: Create test helpers (health check, auth, polling)

**Files:**
- Create: `tests/integration/pipeline/helpers_test.go`

**Step 1: Write the helpers file**

These helpers handle: waiting for services to be healthy, getting an auth token, polling Elasticsearch, and subscribing to Redis. All are test helpers (`t.Helper()`).

```go
package pipeline_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	authURL          = "http://localhost:8040"
	sourceManagerURL = "http://localhost:8050"
	crawlerURL       = "http://localhost:8060"
	publisherURL     = "http://localhost:8070"
	indexManagerURL  = "http://localhost:8090"
	esURL            = "http://localhost:9200"
	redisAddr        = "localhost:6379"

	healthTimeout  = 120 * time.Second
	healthInterval = 2 * time.Second
	pollTimeout    = 120 * time.Second
	pollInterval   = 3 * time.Second
)

// waitForHealth polls a URL until it returns 200 or the timeout expires.
func waitForHealth(t *testing.T, name, url string) {
	t.Helper()
	deadline := time.Now().Add(healthTimeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				t.Logf("%s healthy", name)
				return
			}
		}
		time.Sleep(healthInterval)
	}
	t.Fatalf("%s did not become healthy within %v at %s", name, healthTimeout, url)
}

// getAuthToken obtains a JWT token from the auth service.
func getAuthToken(t *testing.T) string {
	t.Helper()
	body := `{"username":"admin","password":"testpass123"}`
	resp, err := http.Post(
		authURL+"/api/v1/auth/login",
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("auth login request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("auth login returned %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode auth response: %v", err)
	}
	return result.Token
}

// authedRequest creates an HTTP request with the Authorization header set.
func authedRequest(t *testing.T, method, url, token string, body io.Reader) *http.Request {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// doAuthed performs an authenticated HTTP request and returns the response body.
func doAuthed(t *testing.T, method, url, token string, body string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := authedRequest(t, method, url, token, reader)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request to %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}
	return resp.StatusCode, b
}

// esSearchResult represents a minimal ES search response.
type esSearchResult struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			ID     string         `json:"_id"`
			Source map[string]any `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// pollES polls an Elasticsearch index until at least one document matches the
// query or the timeout expires. Returns the first matching document.
func pollES(t *testing.T, index, queryJSON string) (string, map[string]any) {
	t.Helper()
	deadline := time.Now().Add(pollTimeout)
	url := fmt.Sprintf("%s/%s/_search", esURL, index)

	for time.Now().Before(deadline) {
		resp, err := http.Post(url, "application/json", strings.NewReader(queryJSON))
		if err != nil {
			time.Sleep(pollInterval)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusNotFound {
			time.Sleep(pollInterval)
			continue
		}

		var result esSearchResult
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			time.Sleep(pollInterval)
			continue
		}
		if result.Hits.Total.Value > 0 {
			hit := result.Hits.Hits[0]
			return hit.ID, hit.Source
		}
		time.Sleep(pollInterval)
	}
	t.Fatalf("no documents found in %s within %v", index, pollTimeout)
	return "", nil
}

// subscribeRedis subscribes to a Redis channel and waits for the first message
// or until the timeout expires. Returns the parsed message as a map.
func subscribeRedis(t *testing.T, channel string, timeout time.Duration) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	sub := rdb.Subscribe(ctx, channel)
	defer sub.Close()

	// Wait for subscription to be confirmed
	_, err := sub.Receive(ctx)
	if err != nil {
		t.Fatalf("Redis subscribe to %s failed: %v", channel, err)
	}

	ch := sub.Channel()
	select {
	case msg := <-ch:
		var parsed map[string]any
		if err := json.Unmarshal([]byte(msg.Payload), &parsed); err != nil {
			t.Fatalf("failed to parse Redis message: %v", err)
		}
		return parsed
	case <-ctx.Done():
		t.Fatalf("no message received on Redis channel %s within %v", channel, timeout)
		return nil
	}
}
```

**Step 2: Verify it compiles**

Run: `cd tests/integration/pipeline && go vet ./...`
Expected: No errors

**Step 3: Commit**

```bash
git add tests/integration/pipeline/helpers_test.go
git commit -m "test: add helper utilities for pipeline integration test"
```

---

### Task B4: Write the pipeline integration test

**Files:**
- Create: `tests/integration/pipeline/pipeline_test.go`

**Step 1: Write the test**

This is the single-purpose smoke test per checklist. It:
1. Waits for all services to be healthy
2. Creates a source via source-manager
3. Creates a channel + route via publisher
4. Creates a one-time crawler job pointing at the fixture URL
5. Polls ES for raw_content and classified_content
6. Subscribes to Redis and asserts required fields on the message

The test is gated behind `go test -tags=integration` to prevent accidental runs.

```go
//go:build integration

// Package pipeline_test implements the crawl→classify→publish pipeline
// integration smoke test.
//
// This is an end-to-end smoke check: one article flows from fixture URL
// to Redis. It does NOT validate volume, completeness, or SLAs.
// See docs/TESTING_PIPELINE_CHECKLIST.md for the full should/should-not
// assert contract.
package pipeline_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Fixture source — served by nc-http-proxy in replay mode.
	fixtureSourceName = "fixture_news_site_com"
	fixtureSourceURL  = "https://fixture-news-site.com"

	// Redis channel for the test route.
	testChannel = "articles:integration-test"

	// Timeouts for async pipeline stages.
	classifyTimeout = 180 * time.Second
	publishTimeout  = 180 * time.Second
)

func TestPipelineSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// ---- Phase 1: Wait for services ----
	t.Log("Waiting for services to become healthy...")
	waitForHealth(t, "auth", authURL+"/health")
	waitForHealth(t, "source-manager", sourceManagerURL+"/health")
	waitForHealth(t, "crawler", crawlerURL+"/health")
	waitForHealth(t, "classifier", "http://localhost:8071/health")
	waitForHealth(t, "publisher", publisherURL+"/health")
	waitForHealth(t, "index-manager", indexManagerURL+"/health")
	waitForHealth(t, "elasticsearch", esURL+"/_cluster/health")

	// ---- Phase 2: Auth ----
	token := getAuthToken(t)
	require.NotEmpty(t, token, "auth token must not be empty")

	// ---- Phase 3: Seed source ----
	t.Log("Creating test source via source-manager...")
	sourceBody := fmt.Sprintf(`{
		"name": "%s",
		"url": "%s",
		"type": "news",
		"selectors": {"title": "h1", "body": "article"},
		"active": true
	}`, fixtureSourceName, fixtureSourceURL)

	status, respBody := doAuthed(t, "POST", sourceManagerURL+"/api/v1/sources", token, sourceBody)
	require.Equal(t, 201, status, "source creation failed: %s", string(respBody))

	var sourceResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(respBody, &sourceResp))
	sourceID := sourceResp.ID
	require.NotEmpty(t, sourceID)
	t.Logf("Created source: %s", sourceID)

	// ---- Phase 4: Seed channel + route via publisher ----
	t.Log("Creating test channel and route via publisher...")
	channelBody := fmt.Sprintf(`{"name": "%s", "description": "Integration test channel"}`, testChannel)
	status, respBody = doAuthed(t, "POST", publisherURL+"/api/v1/channels", token, channelBody)
	require.Equal(t, 201, status, "channel creation failed: %s", string(respBody))

	var channelResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(respBody, &channelResp))
	channelID := channelResp.ID
	t.Logf("Created channel: %s", channelID)

	// Register the source with the publisher
	pubSourceBody := fmt.Sprintf(`{
		"name": "%s",
		"index_pattern": "%s_classified_content",
		"use_classified_content": true
	}`, fixtureSourceName, fixtureSourceName)
	status, respBody = doAuthed(t, "POST", publisherURL+"/api/v1/sources", token, pubSourceBody)
	require.Equal(t, 201, status, "publisher source creation failed: %s", string(respBody))

	var pubSourceResp struct {
		ID string `json:"id"`
	}
	require.NoError(t, json.Unmarshal(respBody, &pubSourceResp))

	routeBody := fmt.Sprintf(`{
		"source_id": "%s",
		"channel_id": "%s",
		"min_quality_score": 0,
		"active": true
	}`, pubSourceResp.ID, channelID)
	status, respBody = doAuthed(t, "POST", publisherURL+"/api/v1/routes", token, routeBody)
	require.Equal(t, 201, status, "route creation failed: %s", string(respBody))
	t.Log("Created route")

	// ---- Phase 5: Start Redis subscriber BEFORE triggering crawl ----
	t.Log("Subscribing to Redis channel...")
	redisCh := make(chan map[string]any, 1)
	go func() {
		msg := subscribeRedis(t, testChannel, publishTimeout)
		redisCh <- msg
	}()
	// Small delay to ensure subscriber is connected before we trigger the pipeline
	time.Sleep(1 * time.Second)

	// ---- Phase 6: Trigger crawl ----
	t.Log("Creating crawler job...")
	jobBody := fmt.Sprintf(`{
		"source_id": "%s",
		"url": "%s",
		"schedule_enabled": false
	}`, sourceID, fixtureSourceURL)
	status, respBody = doAuthed(t, "POST", crawlerURL+"/api/v1/jobs", token, jobBody)
	require.Equal(t, 201, status, "job creation failed: %s", string(respBody))
	t.Log("Crawler job created, waiting for pipeline...")

	// ---- Phase 7: Poll ES for raw_content ----
	t.Log("Polling raw_content index...")
	rawIndex := fixtureSourceName + "_raw_content"
	rawQuery := `{"query": {"match_all": {}}, "size": 1}`
	rawDocID, rawDoc := pollES(t, rawIndex, rawQuery)
	require.NotEmpty(t, rawDocID, "raw_content document ID must not be empty")
	t.Logf("Found raw_content doc: %s", rawDocID)

	// Assert raw doc has classification_status
	assert.Contains(t, rawDoc, "classification_status",
		"raw_content doc must have classification_status field")

	// ---- Phase 8: Poll ES for classified_content ----
	t.Log("Polling classified_content index...")
	classifiedIndex := fixtureSourceName + "_classified_content"
	classifiedQuery := `{"query": {"match_all": {}}, "size": 1}`
	classifiedDocID, classifiedDoc := pollES(t, classifiedIndex, classifiedQuery)
	require.NotEmpty(t, classifiedDocID, "classified_content document ID must not be empty")
	t.Logf("Found classified_content doc: %s", classifiedDocID)

	// Assert contract-relevant fields per checklist "should assert" list
	requiredFields := []string{
		"content_type", "quality_score", "topics",
		"title", "url",
	}
	for _, field := range requiredFields {
		assert.Contains(t, classifiedDoc, field,
			"classified_content doc must contain field: %s", field)
	}

	// ---- Phase 9: Wait for Redis message ----
	t.Log("Waiting for Redis message...")
	var redisMsg map[string]any
	select {
	case msg := <-redisCh:
		redisMsg = msg
	case <-time.After(publishTimeout):
		t.Fatal("no Redis message received within timeout")
	}

	// Assert required payload fields per checklist
	assert.Contains(t, redisMsg, "id", "Redis message must contain 'id'")
	assert.Contains(t, redisMsg, "title", "Redis message must contain 'title'")
	assert.Contains(t, redisMsg, "content_type", "Redis message must contain 'content_type'")
	assert.Contains(t, redisMsg, "quality_score", "Redis message must contain 'quality_score'")
	assert.Contains(t, redisMsg, "topics", "Redis message must contain 'topics'")

	// Assert publisher metadata
	publisherMeta, ok := redisMsg["publisher"].(map[string]any)
	require.True(t, ok, "Redis message must contain 'publisher' object")
	assert.Contains(t, publisherMeta, "channel", "publisher metadata must contain 'channel'")
	assert.Contains(t, publisherMeta, "published_at", "publisher metadata must contain 'published_at'")

	// ---- Phase 10: End-to-end article identity check ----
	// The article in classified_content should be traceable to the Redis message.
	// We check that the classified doc URL matches the Redis message URL.
	if classifiedURL, ok := classifiedDoc["url"]; ok {
		if redisURL, ok2 := redisMsg["canonical_url"]; ok2 {
			assert.Equal(t, classifiedURL, redisURL,
				"classified_content URL should match Redis message canonical_url")
		}
	}

	t.Log("Pipeline smoke test passed")
}
```

**Step 2: Verify it compiles**

Run: `cd tests/integration/pipeline && go vet -tags=integration ./...`
Expected: No errors (test won't run — no Compose stack up)

**Step 3: Commit**

```bash
git add tests/integration/pipeline/pipeline_test.go
git commit -m "test: add crawl→classify→publish pipeline integration smoke test

End-to-end smoke check per docs/TESTING_PIPELINE_CHECKLIST.md:
one article flows from fixture URL to Redis. Asserts contract
fields on ES documents and Redis message. Does not assert
exact values, volume, or performance."
```

---

### Task B5: Add Taskfile entry for pipeline integration test

**Files:**
- Modify: `Taskfile.yml` (root)

**Step 1: Add test:integration:pipeline task**

Add after the existing `test:contracts` task at the bottom of the root Taskfile:

```yaml
  test:integration:pipeline:
    desc: "Run pipeline integration test (requires full Docker Compose stack)"
    cmds:
      - |
        echo "Starting test stack..."
        docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d --build --wait
        echo "Running pipeline integration test..."
        cd tests/integration/pipeline && GOWORK=off go test -tags=integration -v -timeout=10m ./...
        EXIT_CODE=$?
        echo "Tearing down test stack..."
        docker compose -f docker-compose.base.yml -f docker-compose.test.yml down
        exit $EXIT_CODE
```

**Step 2: Verify task is listed**

Run: `task --list | grep integration`
Expected: `test:integration:pipeline` appears in the list

**Step 3: Commit**

```bash
git add Taskfile.yml
git commit -m "build: add task test:integration:pipeline for pipeline smoke test"
```

---

### Task B6: Create CI workflow for pipeline integration

**Files:**
- Create: `.github/workflows/integration.yml`

**Step 1: Write the workflow**

```yaml
name: Pipeline Integration

on:
  push:
    branches: [main]
  workflow_dispatch:

jobs:
  pipeline-smoke:
    runs-on: ubuntu-latest
    timeout-minutes: 15

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: tests/integration/pipeline/go.mod

      - name: Copy env file
        run: cp .env.example .env

      - name: Start test stack
        run: docker compose -f docker-compose.base.yml -f docker-compose.test.yml up -d --build --wait
        timeout-minutes: 8

      - name: Run pipeline integration test
        run: cd tests/integration/pipeline && GOWORK=off go test -tags=integration -v -timeout=10m ./...

      - name: Collect logs on failure
        if: failure()
        run: docker compose -f docker-compose.base.yml -f docker-compose.test.yml logs --tail=100

      - name: Tear down
        if: always()
        run: docker compose -f docker-compose.base.yml -f docker-compose.test.yml down
```

**Step 2: Verify YAML is valid**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/integration.yml'))"`
Expected: No errors

**Step 3: Commit**

```bash
git add .github/workflows/integration.yml
git commit -m "ci: add pipeline integration workflow (runs on merge to main)"
```

---

### Task B7: Verify docker-compose.test.yml builds (local dry run)

This task validates the compose config before the first real run.

**Step 1: Validate compose config**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.test.yml config > /dev/null`
Expected: No errors

**Step 2: Check all services listed**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.test.yml config --services | sort`
Expected: Lists auth, classifier, crawler, elasticsearch, index-manager, nc-http-proxy, publisher, redis, source-manager (plus postgres instances)

**Step 3: No commit needed — this is a verification step**

---

## Scope Summary

| Task | Area | New/Modify | Purpose |
|------|------|------------|---------|
| A1 | Dashboard | Modify Taskfile | Wire vitest to `task test` |
| A2 | Dashboard | Modify rules.test.ts | Boundary + edge-case tests |
| A3 | Dashboard | Modify rules.test.ts | Doc comment on happy path |
| B1 | Pipeline | Create docker-compose.test.yml | Test Compose override |
| B2 | Pipeline | Create Go module | Test module + deps |
| B3 | Pipeline | Create helpers_test.go | Health, auth, polling utilities |
| B4 | Pipeline | Create pipeline_test.go | The actual smoke test |
| B5 | Pipeline | Modify root Taskfile | `task test:integration:pipeline` |
| B6 | Pipeline | Create integration.yml | CI workflow (merge to main) |
| B7 | Pipeline | None | Dry-run compose validation |

## Out of scope

Per `docs/TESTING_PIPELINE_CHECKLIST.md`:

- **Volume/performance tests** — no assertions on latency, throughput, or backlog
- **Full schema validation** — contract tests in `tests/contracts/` handle that
- **Auth/security tests** — tested elsewhere; pipeline test uses a test token
- **Resilience/chaos** — no "Redis down" or "ES slow" scenarios
- **Exact value assertions** — no specific quality score or exact topic list
- **`near-empty-indexes` rule** — defined in design but not yet implemented in `rules.ts`; add when the rule is implemented
