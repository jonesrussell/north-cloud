# AI Observer Source Suppression Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add result-time source suppression and minimum sample size filtering to the AI Observer analyzer, eliminating non-actionable insights while preserving population stat integrity.

**Architecture:** Two new config fields (`SuppressedSources`, `MinDomainSamples`) flow from `bootstrap/config.go` through the `Category` struct to a new `filterDomainStats()` function in `analyzer.go`. Filtering happens after `aggregateStats()` and before LLM prompt construction. Sampler and writer are untouched.

**Tech Stack:** Go 1.26, stdlib testing (matching existing test patterns)

**Spec:** `docs/superpowers/specs/2026-03-20-ai-observer-source-suppression-design.md`

---

### Task 1: Add config fields

**Files:**
- Modify: `ai-observer/internal/bootstrap/config.go:32-54` (CategoriesConfig struct + LoadConfig)

- [ ] **Step 1: Write the failing test**

Create `ai-observer/internal/bootstrap/config_test.go`:

```go
package bootstrap

import (
	"testing"
)

func TestLoadConfig_SuppressedSources(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")
	t.Setenv("AI_OBSERVER_SUPPRESSED_SOURCES", "source_a,source_b,source_c")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if len(cfg.Observer.Categories.SuppressedSources) != 3 {
		t.Fatalf("expected 3 suppressed sources, got %d", len(cfg.Observer.Categories.SuppressedSources))
	}
	for _, s := range []string{"source_a", "source_b", "source_c"} {
		if !cfg.Observer.Categories.SuppressedSources[s] {
			t.Errorf("expected %q in suppressed sources", s)
		}
	}
}

func TestLoadConfig_SuppressedSourcesEmpty(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Observer.Categories.SuppressedSources == nil {
		t.Fatal("expected non-nil map, got nil")
	}
	if len(cfg.Observer.Categories.SuppressedSources) != 0 {
		t.Errorf("expected 0 suppressed sources, got %d", len(cfg.Observer.Categories.SuppressedSources))
	}
}

func TestLoadConfig_MinDomainSamples(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")
	t.Setenv("AI_OBSERVER_MIN_DOMAIN_SAMPLES", "10")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Observer.Categories.MinDomainSamples != 10 {
		t.Errorf("expected 10, got %d", cfg.Observer.Categories.MinDomainSamples)
	}
}

func TestLoadConfig_MinDomainSamplesDefault(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.Observer.Categories.MinDomainSamples != defaultMinDomainSamples {
		t.Errorf("expected default %d, got %d", defaultMinDomainSamples, cfg.Observer.Categories.MinDomainSamples)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd ai-observer && GOWORK=off go test ./internal/bootstrap/ -run TestLoadConfig_Suppressed -v`
Expected: FAIL — `SuppressedSources` and `MinDomainSamples` fields don't exist

- [ ] **Step 3: Implement config changes**

In `config.go`, add to `CategoriesConfig` struct (after line 53):

```go
SuppressedSources map[string]bool
MinDomainSamples  int
```

Add constant (after line 74):

```go
defaultMinDomainSamples = 5
```

Add to `LoadConfig()` function, before the return statement (after line 136, inside the `Categories` block):

```go
SuppressedSources: parseSuppressedSources(os.Getenv("AI_OBSERVER_SUPPRESSED_SOURCES")),
MinDomainSamples:  minDomainSamples,
```

Add `minDomainSamples` loading before `driftCfg` (after line 113):

```go
minDomainSamples, err := envInt("AI_OBSERVER_MIN_DOMAIN_SAMPLES", defaultMinDomainSamples)
if err != nil {
    return Config{}, err
}
```

Add helper function at the bottom of `config.go`:

```go
func parseSuppressedSources(raw string) map[string]bool {
    result := make(map[string]bool)
    if raw == "" {
        return result
    }
    for _, s := range strings.Split(raw, ",") {
        s = strings.TrimSpace(s)
        if s != "" {
            result[s] = true
        }
    }
    return result
}
```

Add `"strings"` to the import block.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ai-observer && GOWORK=off go test ./internal/bootstrap/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add ai-observer/internal/bootstrap/config.go ai-observer/internal/bootstrap/config_test.go
git commit -m "feat(ai-observer): add suppressed sources and min domain samples config"
```

---

### Task 2: Add filterDomainStats function with tests

**Files:**
- Modify: `ai-observer/internal/category/classifier/analyzer.go`
- Modify: `ai-observer/internal/category/classifier/analyzer_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `analyzer_test.go`:

```go
func TestFilterDomainStats_SuppressesSources(t *testing.T) {
	stats := []domainStats{
		{Domain: "good_source", Label: "article", Count: 10, BorderlineCount: 2, AvgConfidence: 0.7},
		{Domain: "noisy_source", Label: "article", Count: 10, BorderlineCount: 8, AvgConfidence: 0.5},
		{Domain: "good_source", Label: "event", Count: 6, BorderlineCount: 1, AvgConfidence: 0.75},
	}
	suppressed := map[string]bool{"noisy_source": true}

	fr := filterDomainStats(stats, suppressed, 1)

	if len(fr.stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(fr.stats))
	}
	if fr.suppressedCount != 1 {
		t.Errorf("expected 1 suppressed, got %d", fr.suppressedCount)
	}
	for _, s := range fr.stats {
		if s.Domain == "noisy_source" {
			t.Error("noisy_source should have been filtered")
		}
	}
}

func TestFilterDomainStats_MinSampleSize(t *testing.T) {
	stats := []domainStats{
		{Domain: "big_source", Label: "article", Count: 20, BorderlineCount: 4, AvgConfidence: 0.7},
		{Domain: "tiny_source", Label: "article", Count: 2, BorderlineCount: 2, AvgConfidence: 0.5},
		{Domain: "medium_source", Label: "article", Count: 5, BorderlineCount: 1, AvgConfidence: 0.68},
	}

	fr := filterDomainStats(stats, nil, 5)

	if len(fr.stats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(fr.stats))
	}
	if fr.belowMinCount != 1 {
		t.Errorf("expected 1 below min, got %d", fr.belowMinCount)
	}
	for _, s := range fr.stats {
		if s.Domain == "tiny_source" {
			t.Error("tiny_source should have been filtered (count < 5)")
		}
	}
}

func TestFilterDomainStats_BothFilters(t *testing.T) {
	stats := []domainStats{
		{Domain: "keep_me", Label: "article", Count: 10, BorderlineCount: 2, AvgConfidence: 0.7},
		{Domain: "suppressed", Label: "article", Count: 10, BorderlineCount: 8, AvgConfidence: 0.5},
		{Domain: "too_small", Label: "article", Count: 3, BorderlineCount: 3, AvgConfidence: 0.4},
	}
	suppressed := map[string]bool{"suppressed": true}

	fr := filterDomainStats(stats, suppressed, 5)

	if len(fr.stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(fr.stats))
	}
	if fr.stats[0].Domain != "keep_me" {
		t.Errorf("expected keep_me, got %q", fr.stats[0].Domain)
	}
	if fr.suppressedCount != 1 {
		t.Errorf("expected 1 suppressed, got %d", fr.suppressedCount)
	}
	if fr.belowMinCount != 1 {
		t.Errorf("expected 1 below min, got %d", fr.belowMinCount)
	}
}

func TestFilterDomainStats_AllFilteredReturnsEmpty(t *testing.T) {
	stats := []domainStats{
		{Domain: "suppressed", Label: "article", Count: 10, BorderlineCount: 8, AvgConfidence: 0.5},
	}
	suppressed := map[string]bool{"suppressed": true}

	fr := filterDomainStats(stats, suppressed, 1)

	if len(fr.stats) != 0 {
		t.Fatalf("expected 0 stats, got %d", len(fr.stats))
	}
}

func TestFilterDomainStats_NilSuppressed(t *testing.T) {
	stats := []domainStats{
		{Domain: "source_a", Label: "article", Count: 10, BorderlineCount: 2, AvgConfidence: 0.7},
	}

	fr := filterDomainStats(stats, nil, 1)

	if len(fr.stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(fr.stats))
	}
	if fr.suppressedCount != 0 {
		t.Errorf("expected 0 suppressed, got %d", fr.suppressedCount)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/category/classifier/ -run TestFilterDomainStats -v`
Expected: FAIL — `filterDomainStats` and `filterResult` undefined

- [ ] **Step 3: Implement filterDomainStats**

Add to `analyzer.go` (after `aggregateStats`, before `buildPrompt`):

```go
// filterResult holds the filtered domain stats and counts of what was removed.
type filterResult struct {
	stats           []domainStats
	suppressedCount int
	belowMinCount   int
}

// filterDomainStats removes suppressed sources and low-sample pairs from the
// domain stats sent to the LLM. domainStats.Domain corresponds to source_name
// in Elasticsearch (not URL domain).
func filterDomainStats(stats []domainStats, suppressed map[string]bool, minSamples int) filterResult {
	var result filterResult
	result.stats = make([]domainStats, 0, len(stats))
	for _, s := range stats {
		if suppressed[s.Domain] {
			result.suppressedCount++
			continue
		}
		if s.Count < minSamples {
			result.belowMinCount++
			continue
		}
		result.stats = append(result.stats, s)
	}
	return result
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd ai-observer && GOWORK=off go test ./internal/category/classifier/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add ai-observer/internal/category/classifier/analyzer.go ai-observer/internal/category/classifier/analyzer_test.go
git commit -m "feat(ai-observer): add filterDomainStats function with tests"
```

---

### Task 3: Wire filtering into analyze() and Category

**Files:**
- Modify: `ai-observer/internal/category/classifier/category.go:14-29` (struct + New + Analyze)
- Modify: `ai-observer/internal/category/classifier/analyzer.go:52-58` (analyze function)
- Modify: `ai-observer/internal/bootstrap/app.go:110-114` (buildCategories call)
- Modify: `ai-observer/internal/category/classifier/category_test.go` (update New() calls)

- [ ] **Step 1: Update Category struct and New()**

In `category.go`, add fields to the struct:

```go
type Category struct {
	esClient          *es.Client
	maxEvents         int
	modelTier         string
	suppressedSources map[string]bool
	minDomainSamples  int
	lastPopulation    PopulationStats
}
```

Update `New()`:

```go
func New(esClient *es.Client, maxEvents int, modelTier string, suppressedSources map[string]bool, minDomainSamples int) *Category {
	return &Category{
		esClient:          esClient,
		maxEvents:         maxEvents,
		modelTier:         modelTier,
		suppressedSources: suppressedSources,
		minDomainSamples:  minDomainSamples,
	}
}
```

Update `Analyze()` to pass suppression config:

```go
func (c *Category) Analyze(ctx context.Context, events []category.Event, p provider.LLMProvider) ([]category.Insight, error) {
	return analyze(ctx, events, c.lastPopulation, p, c.modelTier, c.suppressedSources, c.minDomainSamples)
}
```

- [ ] **Step 2: Update analyze() function signature**

In `analyzer.go`, update `analyze()` to accept and use suppression config:

```go
func analyze(ctx context.Context, events []category.Event, pop PopulationStats, p provider.LLMProvider, model string, suppressedSources map[string]bool, minDomainSamples int) ([]category.Insight, error) {
	if len(events) == 0 {
		return nil, nil
	}

	stats := aggregateStats(events)
	fr := filterDomainStats(stats, suppressedSources, minDomainSamples)

	if len(fr.stats) == 0 {
		return nil, nil
	}

	userPrompt := buildPrompt(fr.stats, pop)

	resp, err := p.Generate(ctx, provider.GenerateRequest{
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxResponseTokens,
		JSONSchema:   insightJSONSchema,
	})
	if err != nil {
		return nil, fmt.Errorf("generate: %w", err)
	}

	return parseInsights(resp.Content, resp.InputTokens+resp.OutputTokens, model)
}
```

- [ ] **Step 3: Update buildCategories in app.go**

In `app.go`, update the `classifiercategory.New()` call:

```go
fast = append(fast, classifiercategory.New(
	esClient,
	cfg.Observer.Categories.ClassifierMaxEvents,
	cfg.Observer.Categories.ClassifierModel,
	cfg.Observer.Categories.SuppressedSources,
	cfg.Observer.Categories.MinDomainSamples,
))
```

- [ ] **Step 4: Update existing tests**

In `category_test.go`, update `New()` calls to include new params:

```go
func TestClassifierCategory_Name(t *testing.T) {
	t.Helper()
	c := classifiercategory.New(nil, 200, "claude-haiku-4-5-20251001", nil, 5)
	if c.Name() != "classifier" {
		t.Errorf("expected name 'classifier', got %q", c.Name())
	}
}

func TestClassifierCategory_MaxEventsPerRun(t *testing.T) {
	t.Helper()
	const maxEvents = 150
	c := classifiercategory.New(nil, maxEvents, "claude-haiku-4-5-20251001", nil, 5)
	if c.MaxEventsPerRun() != maxEvents {
		t.Errorf("expected %d, got %d", maxEvents, c.MaxEventsPerRun())
	}
}
```

- [ ] **Step 5: Run all tests**

Run: `cd ai-observer && GOWORK=off go test ./... -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add ai-observer/internal/category/classifier/category.go ai-observer/internal/category/classifier/analyzer.go ai-observer/internal/bootstrap/app.go ai-observer/internal/category/classifier/category_test.go
git commit -m "feat(ai-observer): wire source suppression into analyze pipeline"
```

---

### Task 4: Update docs

**Files:**
- Modify: `ai-observer/CLAUDE.md` (config table)
- Modify: `ai-observer/config.yml.example` (document new vars)

- [ ] **Step 1: Update CLAUDE.md config table**

Add two rows to the config table in `ai-observer/CLAUDE.md` (after `AI_OBSERVER_CATEGORY_CLASSIFIER_ENABLED`):

```markdown
| `AI_OBSERVER_SUPPRESSED_SOURCES` | `""` | Comma-separated source names excluded from insight generation |
| `AI_OBSERVER_MIN_DOMAIN_SAMPLES` | `5` | Min docs per domain+label pair for LLM analysis |
```

- [ ] **Step 2: Update config.yml.example**

Add to `ai-observer/config.yml.example`:

```yaml
# Source suppression — exclude known low-value sources from insight generation.
# Population stats still include these sources (result-time exclusion).
# AI_OBSERVER_SUPPRESSED_SOURCES=battlefords_news_optimist,battlefordsnow_com

# Minimum sample size — domain+label pairs with fewer docs are excluded from LLM analysis.
# AI_OBSERVER_MIN_DOMAIN_SAMPLES=5
```

- [ ] **Step 3: Commit**

```bash
git add ai-observer/CLAUDE.md ai-observer/config.yml.example
git commit -m "docs(ai-observer): document source suppression config"
```

---

### Task 5: Add compose env vars and verify

**Files:**
- Modify: `docker-compose.base.yml:662-675` (ai-observer environment block)

- [ ] **Step 1: Add env vars to base compose**

In `docker-compose.base.yml`, add to the ai-observer service environment block (after line 667, the `ANTHROPIC_API_KEY` line):

```yaml
      - AI_OBSERVER_SUPPRESSED_SOURCES=${AI_OBSERVER_SUPPRESSED_SOURCES:-}
      - AI_OBSERVER_MIN_DOMAIN_SAMPLES=${AI_OBSERVER_MIN_DOMAIN_SAMPLES:-5}
```

Production values (`AI_OBSERVER_SUPPRESSED_SOURCES=battlefords_news_optimist`) should be set in the production `.env` file on the server.

- [ ] **Step 2: Run full test suite**

Run: `cd ai-observer && GOWORK=off go test ./... -v -count=1`
Expected: ALL PASS

- [ ] **Step 3: Run lint**

Run: `cd ai-observer && GOWORK=off go vet ./...`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add docker-compose.base.yml
git commit -m "chore(ai-observer): add source suppression env vars to compose"
```
