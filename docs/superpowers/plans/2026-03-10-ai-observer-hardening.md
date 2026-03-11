# AI Observer Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reduce AI Observer noise, fix JSON parse failures, tune drift thresholds, and add insight retention — making the `ai_insights` index actionable.

**Architecture:** Four independent code tasks targeting the `ai-observer/` service, plus one config-only production change. Each task is self-contained with its own tests. No cross-service changes needed.

**Tech Stack:** Go 1.26+, Elasticsearch 8, Anthropic Haiku API

**GitHub Issues:** #308, #309, #310, #312 (M3: Observability Hardening)

---

## Chunk 1: Fix Truncated LLM JSON Responses (#309)

### Task 1: Increase maxResponseTokens and improve error handling

**Files:**
- Modify: `ai-observer/internal/category/classifier/analyzer.go:35` (maxResponseTokens constant)
- Test: `ai-observer/internal/category/classifier/analyzer_test.go`

- [ ] **Step 1: Write test for truncated JSON handling**

Add to `analyzer_test.go`:

```go
func TestParseInsights_TruncatedJSON(t *testing.T) {
	t.Helper()
	// Simulates LLM output cut off mid-JSON
	truncated := `[{"severity":"high","summary":"Some finding","details":{},"suggested_actions":["act`
	insights, err := parseInsights(truncated, 500, "test-model")
	if err == nil {
		t.Error("expected error for truncated JSON")
	}
	if insights != nil {
		t.Error("expected nil insights for truncated JSON")
	}
}

func TestParseInsights_ValidMultiInsight(t *testing.T) {
	t.Helper()
	// Ensure we can handle responses that need >1000 tokens
	content := `[
		{"severity":"high","summary":"Finding one with a longer description that uses more tokens","details":{"domain":"example.com","rate":0.95},"suggested_actions":["action 1","action 2"]},
		{"severity":"medium","summary":"Finding two about another domain","details":{"domain":"other.com","rate":0.75},"suggested_actions":["action 3"]},
		{"severity":"low","summary":"Minor observation","details":{},"suggested_actions":["monitor"]}
	]`
	insights, err := parseInsights(content, 900, "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedInsights := 3
	if len(insights) != expectedInsights {
		t.Errorf("expected %d insights, got %d", expectedInsights, len(insights))
	}
}
```

- [ ] **Step 2: Run test to verify it compiles and the truncated test fails as expected**

Run: `cd ai-observer && GOWORK=off go test ./internal/category/classifier/ -run TestParseInsights -v`
Expected: Both tests should pass (truncated correctly returns error, multi-insight parses fine)

- [ ] **Step 3: Increase maxResponseTokens from 1000 to 2048**

In `analyzer.go`, change line 35:

```go
maxResponseTokens = 2048
```

- [ ] **Step 4: Run all analyzer tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/category/classifier/ -v`
Expected: All PASS

- [ ] **Step 5: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/category/classifier/`
Expected: No issues

- [ ] **Step 6: Commit**

```bash
git add ai-observer/internal/category/classifier/analyzer.go ai-observer/internal/category/classifier/analyzer_test.go
git commit -m "fix(ai-observer): increase maxResponseTokens to 2048 (#309)

LLM responses were being truncated at 1000 tokens, causing JSON parse
errors in production. Doubled the limit to handle multi-domain analyses."
```

---

## Chunk 2: Insight Deduplication (#308)

### Task 2: Add deduplication check to insight writer

The writer should check ES for recent similar insights before writing. A "similar" insight matches on `category` + `severity` and was created within the cooldown window.

**Files:**
- Create: `ai-observer/internal/insights/dedup.go`
- Create: `ai-observer/internal/insights/dedup_test.go`
- Modify: `ai-observer/internal/insights/writer.go`
- Modify: `ai-observer/internal/insights/writer_test.go`
- Modify: `ai-observer/internal/bootstrap/config.go`
- Modify: `ai-observer/internal/bootstrap/config_test.go`
- Modify: `ai-observer/internal/bootstrap/app.go` (wire cooldown config to writer)

- [ ] **Step 1: Add cooldown config**

In `config.go`, add to `ObserverConfig`:

```go
// ObserverConfig holds polling and budget config.
type ObserverConfig struct {
	Enabled              bool
	DryRun               bool
	IntervalSeconds      int
	MaxTokensPerInterval int
	InsightCooldownHours int
	Categories           CategoriesConfig
}
```

Add default constant:

```go
defaultInsightCooldownHours = 6
```

In `LoadConfig()`, load the new field:

```go
cooldownHours, err := envInt("AI_OBSERVER_INSIGHT_COOLDOWN_HOURS", defaultInsightCooldownHours)
if err != nil {
	return Config{}, err
}
```

And set it on the config struct:

```go
InsightCooldownHours: cooldownHours,
```

- [ ] **Step 2: Write config test**

Add to `config_test.go`:

```go
func TestLoadConfig_CooldownDefault(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedCooldown = 6
	if cfg.Observer.InsightCooldownHours != expectedCooldown {
		t.Errorf("expected cooldown %d, got %d", expectedCooldown, cfg.Observer.InsightCooldownHours)
	}
}
```

- [ ] **Step 3: Run config tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/bootstrap/ -v`
Expected: All PASS

- [ ] **Step 4: Create dedup.go**

```go
// Package insights handles writing AI-generated insights to Elasticsearch.
package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
)

// Deduplicator checks for recent similar insights to avoid writing duplicates.
type Deduplicator struct {
	esClient       *es.Client
	cooldownWindow time.Duration
}

// NewDeduplicator creates a Deduplicator with the given cooldown window.
func NewDeduplicator(esClient *es.Client, cooldownHours int) *Deduplicator {
	return &Deduplicator{
		esClient:       esClient,
		cooldownWindow: time.Duration(cooldownHours) * time.Hour,
	}
}

// IsDuplicate checks if a similar insight (same category + severity) exists
// within the cooldown window.
func (d *Deduplicator) IsDuplicate(ctx context.Context, category, severity string) (bool, error) {
	if d == nil || d.cooldownWindow == 0 {
		return false, nil
	}

	cutoff := time.Now().UTC().Add(-d.cooldownWindow).Format(time.RFC3339)

	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"filter": []any{
					map[string]any{"term": map[string]any{"category": category}},
					map[string]any{"term": map[string]any{"severity": severity}},
					map[string]any{"range": map[string]any{
						"created_at": map[string]any{"gte": cutoff},
					}},
				},
			},
		},
		"size": 0,
	}

	body, err := json.Marshal(query)
	if err != nil {
		return false, fmt.Errorf("marshal dedup query: %w", err)
	}

	res, searchErr := d.esClient.Search(
		d.esClient.Search.WithContext(ctx),
		d.esClient.Search.WithIndex(insightsIndex),
		d.esClient.Search.WithBody(bytes.NewReader(body)),
	)
	if searchErr != nil {
		return false, fmt.Errorf("dedup search: %w", searchErr)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		// If the index doesn't exist yet, treat as no duplicate.
		return false, nil
	}

	var result struct {
		Hits struct {
			Total struct {
				Value int `json:"value"`
			} `json:"total"`
		} `json:"hits"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return false, fmt.Errorf("decode dedup response: %w", decodeErr)
	}

	return result.Hits.Total.Value > 0, nil
}
```

- [ ] **Step 5: Create dedup_test.go**

```go
package insights_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
)

func TestNewDeduplicator(t *testing.T) {
	t.Helper()

	cooldownHours := 6
	d := insights.NewDeduplicator(nil, cooldownHours)

	if d == nil {
		t.Fatal("expected non-nil deduplicator")
	}
}

func TestIsDuplicate_NilDeduplicator(t *testing.T) {
	t.Helper()

	var d *insights.Deduplicator
	isDup, err := d.IsDuplicate(t.Context(), "classifier", "high")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("nil deduplicator should return false")
	}
}
```

- [ ] **Step 6: Run dedup tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/insights/ -v`
Expected: All PASS

- [ ] **Step 7: Wire deduplicator into Writer**

Modify `writer.go` — add deduplicator field and filtering in `WriteAll`:

```go
// Writer writes insights to the ai_insights ES index.
type Writer struct {
	esClient        *es.Client
	observerVersion string
	dedup           *Deduplicator
}

// NewWriter creates a new insight Writer.
func NewWriter(esClient *es.Client, observerVersion string, dedup *Deduplicator) *Writer {
	return &Writer{esClient: esClient, observerVersion: observerVersion, dedup: dedup}
}

// WriteAll indexes all provided insights, skipping duplicates. Each is indexed
// independently; all errors are joined and returned together.
func (w *Writer) WriteAll(ctx context.Context, insightList []category.Insight) error {
	var errs []error
	for _, ins := range insightList {
		if w.dedup != nil {
			isDup, dedupErr := w.dedup.IsDuplicate(ctx, ins.Category, ins.Severity)
			if dedupErr != nil {
				errs = append(errs, fmt.Errorf("dedup check: %w", dedupErr))
				continue
			}
			if isDup {
				continue
			}
		}
		if err := w.write(ctx, ins); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
```

- [ ] **Step 8: Update NewWriter call sites**

Find where `NewWriter` is called (in `bootstrap/app.go`) and pass the deduplicator:

```go
dedup := insights.NewDeduplicator(esClient, cfg.Observer.InsightCooldownHours)
writer := insights.NewWriter(esClient, cfg.Service.Version, dedup)
```

- [ ] **Step 9: Run all ai-observer tests**

Run: `cd ai-observer && GOWORK=off go test ./... -v`
Expected: All PASS

- [ ] **Step 10: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`
Expected: No issues

- [ ] **Step 11: Commit**

```bash
git add ai-observer/internal/insights/dedup.go ai-observer/internal/insights/dedup_test.go \
  ai-observer/internal/insights/writer.go ai-observer/internal/insights/writer_test.go \
  ai-observer/internal/bootstrap/config.go ai-observer/internal/bootstrap/config_test.go \
  ai-observer/internal/bootstrap/app.go
git commit -m "feat(ai-observer): add insight deduplication with cooldown window (#308)

Skip writing insights when a similar one (same category + severity) was
written within the cooldown window (default 6h). Reduces noise by ~70-80%
and saves Haiku token spend on repeated analyses."
```

---

## Chunk 3: Tune Drift Detection Thresholds (#310)

### Task 3: Raise default KL divergence threshold

The severity tiering logic already exists in `drift/severity.go` (medium for <2x threshold, high for >=2x). The problem is the default KL threshold of 0.15 is too sensitive for a diverse news corpus — natural daily variation triggers breaches.

**Files:**
- Modify: `ai-observer/internal/bootstrap/config.go:66` (default constant)
- Modify: `ai-observer/internal/bootstrap/config_test.go` (update expected value)

- [ ] **Step 1: Update default KL threshold from 0.15 to 0.30**

In `config.go`, change line 66:

```go
defaultDriftKLThreshold = 0.30
```

- [ ] **Step 2: Update config test to match new default**

In `config_test.go`, update `TestLoadConfig_DriftDefaults`:

```go
const expectedKLThreshold = 0.30
```

- [ ] **Step 3: Run config tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/bootstrap/ -v`
Expected: All PASS

- [ ] **Step 4: Run drift tests to ensure nothing breaks**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v`
Expected: All PASS (drift tests use explicit thresholds, not defaults)

- [ ] **Step 5: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`
Expected: No issues

- [ ] **Step 6: Commit**

```bash
git add ai-observer/internal/bootstrap/config.go ai-observer/internal/bootstrap/config_test.go
git commit -m "feat(ai-observer): raise default KL divergence threshold to 0.30 (#310)

The previous 0.15 threshold was too sensitive for a diverse news corpus.
Natural daily topic variation caused values of 0.32-0.35, triggering
false HIGH alerts. The existing severity tiering (medium <2x, high >=2x)
works correctly with the new threshold."
```

### Task 4: Update production .env threshold

This is a production config change, not a code change. After deploying the code with the new default, the production `.env` override (if any) should be removed so the new default takes effect.

- [ ] **Step 1: Check if production overrides the KL threshold**

```bash
ssh jones@northcloud.one "grep AI_OBSERVER_DRIFT_KL_THRESHOLD /opt/north-cloud/.env"
```

If it exists, remove the line so the new code default (0.30) takes effect.
If it doesn't exist, no action needed — the new default will apply on next deploy.

- [ ] **Step 2: Document in commit or PR description**

Note: This step is done during deployment, not in the code commit.

---

## Chunk 4: Insight Retention Policy (#312)

### Task 5: Add periodic cleanup of old insights

Add a cleanup function that runs on the slow ticker (every 6h alongside drift) to delete insights older than the retention period. This is simpler than ILM and doesn't require ES ILM to be enabled.

**Files:**
- Create: `ai-observer/internal/insights/cleanup.go`
- Create: `ai-observer/internal/insights/cleanup_test.go`
- Modify: `ai-observer/internal/bootstrap/config.go` (add retention config)
- Modify: `ai-observer/internal/bootstrap/config_test.go`
- Modify: `ai-observer/internal/scheduler/scheduler.go` (add cleanup call)

- [ ] **Step 1: Add retention config**

In `config.go`, add to `ObserverConfig`:

```go
InsightRetentionDays int
```

Add default constant:

```go
defaultInsightRetentionDays = 30
```

Load it in `LoadConfig()`:

```go
retentionDays, err := envInt("AI_OBSERVER_INSIGHT_RETENTION_DAYS", defaultInsightRetentionDays)
if err != nil {
	return Config{}, err
}
```

Set on config:

```go
InsightRetentionDays: retentionDays,
```

- [ ] **Step 2: Write config test**

Add to `config_test.go`:

```go
func TestLoadConfig_RetentionDefault(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")

	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const expectedRetention = 30
	if cfg.Observer.InsightRetentionDays != expectedRetention {
		t.Errorf("expected retention %d, got %d", expectedRetention, cfg.Observer.InsightRetentionDays)
	}
}
```

- [ ] **Step 3: Run config tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/bootstrap/ -v`
Expected: All PASS

- [ ] **Step 4: Create cleanup.go**

```go
package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
)

// Cleaner deletes insights older than the retention period.
type Cleaner struct {
	esClient      *es.Client
	retentionDays int
}

// NewCleaner creates a Cleaner with the given retention period.
func NewCleaner(esClient *es.Client, retentionDays int) *Cleaner {
	return &Cleaner{esClient: esClient, retentionDays: retentionDays}
}

// DeleteOldInsights removes insights older than the retention window.
// Returns the number of deleted documents.
func (c *Cleaner) DeleteOldInsights(ctx context.Context) (int, error) {
	if c == nil || c.retentionDays <= 0 {
		return 0, nil
	}

	cutoff := time.Now().UTC().AddDate(0, 0, -c.retentionDays).Format(time.RFC3339)

	query := map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"created_at": map[string]any{"lt": cutoff},
			},
		},
	}

	body, err := json.Marshal(query)
	if err != nil {
		return 0, fmt.Errorf("marshal cleanup query: %w", err)
	}

	res, deleteErr := c.esClient.DeleteByQuery(
		[]string{insightsIndex},
		bytes.NewReader(body),
		c.esClient.DeleteByQuery.WithContext(ctx),
	)
	if deleteErr != nil {
		return 0, fmt.Errorf("delete old insights: %w", deleteErr)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return 0, fmt.Errorf("delete old insights error: %s", res.String())
	}

	var result struct {
		Deleted int `json:"deleted"`
	}
	if decodeErr := json.NewDecoder(res.Body).Decode(&result); decodeErr != nil {
		return 0, fmt.Errorf("decode cleanup response: %w", decodeErr)
	}

	return result.Deleted, nil
}
```

- [ ] **Step 5: Create cleanup_test.go**

```go
package insights_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/insights"
)

func TestNewCleaner(t *testing.T) {
	t.Helper()

	retentionDays := 30
	c := insights.NewCleaner(nil, retentionDays)
	if c == nil {
		t.Fatal("expected non-nil cleaner")
	}
}

func TestDeleteOldInsights_NilCleaner(t *testing.T) {
	t.Helper()

	var c *insights.Cleaner
	deleted, err := c.DeleteOldInsights(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}

func TestDeleteOldInsights_ZeroRetention(t *testing.T) {
	t.Helper()

	c := insights.NewCleaner(nil, 0)
	deleted, err := c.DeleteOldInsights(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("expected 0 deleted, got %d", deleted)
	}
}
```

- [ ] **Step 6: Run cleanup tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/insights/ -v`
Expected: All PASS

- [ ] **Step 7: Wire cleanup into scheduler**

Add cleanup to the `Scheduler` struct and call it on the slow ticker. In `scheduler.go`:

Add to `Scheduler` struct:

```go
cleaner *insights.Cleaner
```

Add to `New` constructor params and set it. Update `RunDrift` to call cleanup after drift:

```go
func (s *Scheduler) RunDrift(ctx context.Context) {
	s.runCategories(ctx, s.slowCategories, s.cfg.DriftWindowDuration)
	s.runCleanup(ctx)
}

func (s *Scheduler) runCleanup(ctx context.Context) {
	if s.cleaner == nil {
		return
	}
	deleted, err := s.cleaner.DeleteOldInsights(ctx)
	if err != nil {
		s.logError("cleanup error", err)
		return
	}
	if deleted > 0 {
		s.logInfo("Old insights cleaned up", logger.Int("deleted", deleted))
	}
}
```

- [ ] **Step 8: Update bootstrap/app.go to wire cleaner**

Create cleaner and pass to scheduler:

```go
cleaner := insights.NewCleaner(esClient, cfg.Observer.InsightRetentionDays)
```

Pass to `scheduler.New(...)`.

- [ ] **Step 9: Run all ai-observer tests**

Run: `cd ai-observer && GOWORK=off go test ./... -v`
Expected: All PASS

- [ ] **Step 10: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`
Expected: No issues

- [ ] **Step 11: Commit**

```bash
git add ai-observer/internal/insights/cleanup.go ai-observer/internal/insights/cleanup_test.go \
  ai-observer/internal/bootstrap/config.go ai-observer/internal/bootstrap/config_test.go \
  ai-observer/internal/bootstrap/app.go ai-observer/internal/scheduler/scheduler.go
git commit -m "feat(ai-observer): add insight retention policy with periodic cleanup (#312)

Delete insights older than 30 days (configurable via
AI_OBSERVER_INSIGHT_RETENTION_DAYS). Runs on the slow ticker alongside
drift detection (every 6h). Prevents unbounded index growth."
```

---

## Chunk 5: Update Documentation

### Task 6: Update ai-observer CLAUDE.md and service docs

**Files:**
- Modify: `ai-observer/CLAUDE.md`

- [ ] **Step 1: Update CLAUDE.md config table**

Add new env vars to the config table:

| Variable | Default | Description |
|---|---|---|
| `AI_OBSERVER_INSIGHT_COOLDOWN_HOURS` | `6` | Dedup window — skip insights if similar one written recently |
| `AI_OBSERVER_INSIGHT_RETENTION_DAYS` | `30` | Delete insights older than this |

Update the KL threshold default from `0.15` to `0.30`.

- [ ] **Step 2: Add dedup and retention to Architecture section**

Add under `insights/`:

```
├── insights/                # ai_insights ES index writer + dedup + cleanup
```

- [ ] **Step 3: Commit**

```bash
git add ai-observer/CLAUDE.md
git commit -m "docs(ai-observer): update CLAUDE.md with dedup, retention, and threshold changes"
```

---

## Source Investigation (#311) — Research Task

This is NOT a code task. It should be done interactively in a separate session:

1. Query ES for sample documents from Battlefords News-Optimist, Western Standard, We Work Remotely
2. Check source-manager for their source configs
3. Verify crawler content extraction quality
4. Decide: fix extraction, recategorize, or remove inappropriate sources
5. Update issue #311 with findings

---

## Execution Order

1. **Task 1** (Chunk 1) — #309 truncated JSON fix — smallest, highest impact
2. **Task 2** (Chunk 2) — #308 deduplication — biggest noise reduction
3. **Task 3-4** (Chunk 3) — #310 drift threshold — config + small code change
4. **Task 5** (Chunk 4) — #312 retention — housekeeping
5. **Task 6** (Chunk 5) — docs update
6. **Source investigation** (#311) — separate session
