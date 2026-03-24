# Classifier Quality Gate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a configurable quality gate to the classifier pipeline that rejects low-confidence non-article content before indexing, closing #564, #565, #566.

**Architecture:** The gate sits in the processor layer (L3) between batch classification and ES bulk indexing. It filters `[]*domain.ClassifiedContent` based on quality_score and content_type. A new `QualityGateConfig` in the config struct controls the feature flag and threshold. A `LowQuality` bool field on `ClassifiedContent` flags articles that pass below threshold.

**Tech Stack:** Go 1.26+, Elasticsearch, existing infrastructure/logger

---

### Task 1: Add LowQuality Field to Domain Model

**Files:**
- Modify: `classifier/internal/domain/classification.go:165-215`
- Test: `classifier/internal/domain/classification_test.go` (create if needed)

- [ ] **Step 1: Add LowQuality field to ClassifiedContent struct**

In `classifier/internal/domain/classification.go`, add after the `Confidence` field (line 182):

```go
	// Quality gate flag — true when article indexed despite low quality_score
	LowQuality bool `json:"low_quality,omitempty"`
```

- [ ] **Step 2: Run linter to verify**

Run: `cd classifier && golangci-lint run ./internal/domain/...`
Expected: PASS (no new violations)

- [ ] **Step 3: Commit**

```bash
git add classifier/internal/domain/classification.go
git commit -m "feat(classifier): add LowQuality field to ClassifiedContent domain model

Closes #566 (partial) — adds the low_quality flag that the quality gate
will set on articles that pass below the threshold."
```

---

### Task 2: Add LowQuality to ES Mapping

**Files:**
- Modify: `classifier/internal/elasticsearch/mappings/classified_content.go:21-93` (struct) and `239-259` (createClassificationProperties)

- [ ] **Step 1: Add LowQuality field to ClassifiedContentProperties struct**

In the `ClassifiedContentProperties` struct, add after `Confidence Field`:

```go
	// Quality gate flag
	LowQuality Field `json:"low_quality"`
```

- [ ] **Step 2: Add LowQuality to createClassificationProperties()**

In `createClassificationProperties()`, add after the `Confidence` line:

```go
		LowQuality:           Field{Type: "boolean"},
```

- [ ] **Step 3: Add LowQuality to mergeProperties()**

In `mergeProperties()`, add after the `Confidence` line in the return struct:

```go
		LowQuality: classified.LowQuality,
```

- [ ] **Step 4: Run existing mapping tests**

Run: `cd classifier && go test ./internal/elasticsearch/mappings/... -v`
Expected: PASS

- [ ] **Step 5: Run linter**

Run: `cd classifier && golangci-lint run ./internal/elasticsearch/mappings/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add classifier/internal/elasticsearch/mappings/classified_content.go
git commit -m "feat(classifier): add low_quality boolean to ES classified_content mapping"
```

---

### Task 3: Add QualityGate Config

**Files:**
- Modify: `classifier/internal/config/config.go:48-56` (Config struct), `131-156` (ClassificationConfig), `354-413` (setClassificationDefaults)

- [ ] **Step 1: Add QualityGateConfig struct**

Add after `DrillExtractionConfig` (around line 211):

```go
// QualityGateConfig holds quality gate settings.
type QualityGateConfig struct {
	Enabled   bool `env:"CLASSIFIER_QUALITY_GATE_ENABLED"   yaml:"enabled"`
	Threshold int  `env:"CLASSIFIER_QUALITY_GATE_THRESHOLD" yaml:"threshold"`
}
```

- [ ] **Step 2: Add QualityGate field to ClassificationConfig**

In `ClassificationConfig` struct, add after `DrillExtraction`:

```go
	QualityGate QualityGateConfig `yaml:"quality_gate"`
```

- [ ] **Step 3: Add default constant and defaults function**

Add to the constants block at the top:

```go
	defaultQualityGateThreshold = 40
```

Add at the end of `setClassificationDefaults()`, before the routing block:

```go
	// QualityGate defaults: disabled by default for safe rollout
	if c.QualityGate.Threshold == 0 {
		c.QualityGate.Threshold = defaultQualityGateThreshold
	}
```

- [ ] **Step 4: Run linter**

Run: `cd classifier && golangci-lint run ./internal/config/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add classifier/internal/config/config.go
git commit -m "feat(classifier): add QualityGateConfig with CLASSIFIER_QUALITY_GATE_ENABLED/THRESHOLD env vars"
```

---

### Task 4: Implement Quality Gate Filter

**Files:**
- Create: `classifier/internal/processor/quality_gate.go`
- Create: `classifier/internal/processor/quality_gate_test.go`

- [ ] **Step 1: Write the failing test**

Create `classifier/internal/processor/quality_gate_test.go`:

```go
//nolint:testpackage // Testing internal processor requires same package access
package processor

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestApplyQualityGate(t *testing.T) {
	logger := newMockLoggerWithCalls()

	tests := []struct {
		name           string
		cfg            config.QualityGateConfig
		input          []*domain.ClassifiedContent
		wantCount      int
		wantLowQuality []bool // LowQuality flag for each output item
	}{
		{
			name: "gate disabled passes all through unchanged",
			cfg:  config.QualityGateConfig{Enabled: false, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 10, ContentType: "page"},
				{QualityScore: 50, ContentType: "article"},
			},
			wantCount:      2,
			wantLowQuality: []bool{false, false},
		},
		{
			name: "high quality passes through",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: "article"},
			},
			wantCount:      1,
			wantLowQuality: []bool{false},
		},
		{
			name: "low quality article flagged but passes",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 30, ContentType: "article"},
			},
			wantCount:      1,
			wantLowQuality: []bool{true},
		},
		{
			name: "low quality page rejected",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 30, ContentType: "page"},
			},
			wantCount: 0,
		},
		{
			name: "low quality event rejected",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 35, ContentType: "event"},
			},
			wantCount: 0,
		},
		{
			name: "threshold boundary — exactly at threshold passes",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 40, ContentType: "page"},
			},
			wantCount:      1,
			wantLowQuality: []bool{false},
		},
		{
			name: "mixed batch — filters correctly",
			cfg:  config.QualityGateConfig{Enabled: true, Threshold: 40},
			input: []*domain.ClassifiedContent{
				{QualityScore: 70, ContentType: "article"},  // pass
				{QualityScore: 30, ContentType: "page"},     // reject
				{QualityScore: 35, ContentType: "article"},  // flag
				{QualityScore: 20, ContentType: "event"},    // reject
				{QualityScore: 50, ContentType: "article"},  // pass
			},
			wantCount:      3,
			wantLowQuality: []bool{false, true, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyQualityGate(tt.cfg, tt.input, logger)

			if len(result) != tt.wantCount {
				t.Errorf("applyQualityGate() returned %d items, want %d", len(result), tt.wantCount)
				return
			}

			for i, wantLQ := range tt.wantLowQuality {
				if i >= len(result) {
					break
				}
				if result[i].LowQuality != wantLQ {
					t.Errorf("result[%d].LowQuality = %v, want %v (quality_score=%d, content_type=%s)",
						i, result[i].LowQuality, wantLQ, result[i].QualityScore, result[i].ContentType)
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd classifier && go test ./internal/processor/ -run TestApplyQualityGate -v`
Expected: FAIL — `undefined: applyQualityGate`

- [ ] **Step 3: Write the implementation**

Create `classifier/internal/processor/quality_gate.go`:

```go
package processor

import (
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// applyQualityGate filters classified content based on quality_score and content_type.
// - quality_score >= threshold: pass through
// - quality_score < threshold AND content_type=article: pass with LowQuality=true
// - quality_score < threshold AND content_type!=article: reject
func applyQualityGate(
	cfg config.QualityGateConfig,
	contents []*domain.ClassifiedContent,
	logger infralogger.Logger,
) []*domain.ClassifiedContent {
	if !cfg.Enabled {
		return contents
	}

	passed := make([]*domain.ClassifiedContent, 0, len(contents))

	for _, content := range contents {
		if content.QualityScore >= cfg.Threshold {
			passed = append(passed, content)
			continue
		}

		// Below threshold — articles get flagged, non-articles get rejected
		if content.ContentType == domain.ContentTypeArticle {
			content.LowQuality = true
			passed = append(passed, content)

			logger.Info("Quality gate: flagged low-quality article",
				infralogger.String("url", content.URL),
				infralogger.String("source", content.SourceName),
				infralogger.Int("quality_score", content.QualityScore),
				infralogger.String("content_type", content.ContentType),
				infralogger.Int("threshold", cfg.Threshold),
				infralogger.String("reason", "below_threshold"),
			)

			continue
		}

		logger.Info("Quality gate: rejected non-article content",
			infralogger.String("url", content.URL),
			infralogger.String("source", content.SourceName),
			infralogger.String("content_type", content.ContentType),
			infralogger.Int("quality_score", content.QualityScore),
			infralogger.Int("threshold", cfg.Threshold),
			infralogger.String("reason", "non_article_below_threshold"),
		)
	}

	return passed
}

// QualityGateStats holds counters for quality gate decisions.
type QualityGateStats struct {
	Passed   int `json:"quality_gate_passed"`
	Flagged  int `json:"quality_gate_flagged"`
	Rejected int `json:"quality_gate_rejected"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd classifier && go test ./internal/processor/ -run TestApplyQualityGate -v`
Expected: PASS (all 7 test cases)

- [ ] **Step 5: Run linter**

Run: `cd classifier && golangci-lint run ./internal/processor/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add classifier/internal/processor/quality_gate.go classifier/internal/processor/quality_gate_test.go
git commit -m "feat(classifier): implement quality gate filter with TDD

Rejects non-article content below quality threshold, flags low-quality
articles with low_quality=true. Gate is controlled by
CLASSIFIER_QUALITY_GATE_ENABLED and CLASSIFIER_QUALITY_GATE_THRESHOLD."
```

---

### Task 5: Wire Quality Gate into Poller

**Files:**
- Modify: `classifier/internal/processor/poller.go:52-63` (Poller struct), `72-97` (NewPoller), `191-246` (indexResults)

- [ ] **Step 1: Add qualityGateCfg to Poller struct and PollerConfig**

In `PollerConfig` struct, add:

```go
	QualityGate config.QualityGateConfig
```

In `Poller` struct, add:

```go
	qualityGateCfg config.QualityGateConfig
```

In `NewPoller()`, add assignment:

```go
	qualityGateCfg: config.QualityGate,
```

- [ ] **Step 2: Call applyQualityGate in indexResults()**

In `indexResults()`, after the loop that separates successful and failed results (after line 209 `classifiedContents = append(classifiedContents, result.ClassifiedContent)`), and before the `if len(classifiedContents) == 0` check, add:

```go
	// Apply quality gate — filter/flag before indexing
	classifiedContents = applyQualityGate(p.qualityGateCfg, classifiedContents, p.logger)
```

- [ ] **Step 3: Add config import**

Add to the import block:

```go
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
```

- [ ] **Step 4: Run all processor tests**

Run: `cd classifier && go test ./internal/processor/... -v`
Expected: PASS

- [ ] **Step 5: Run linter**

Run: `cd classifier && golangci-lint run ./internal/processor/...`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add classifier/internal/processor/poller.go
git commit -m "feat(classifier): wire quality gate into poller indexResults pipeline

Gate runs between batch classification and BulkIndexClassifiedContent,
filtering the classifiedContents slice before it reaches ES."
```

---

### Task 6: Wire Config Through cmd/processor

**Files:**
- Modify: `classifier/cmd/processor/processor.go:377-380` (first PollerConfig site) and `:461-464` (second PollerConfig site)

There are **two** `PollerConfig` construction sites in `classifier/cmd/processor/processor.go`. Both must be updated.

- [ ] **Step 1: Add QualityGate to first PollerConfig construction (line ~377)**

Find the first `PollerConfig{` block and add:

```go
	QualityGate: cfg.Classification.QualityGate,
```

- [ ] **Step 2: Add QualityGate to second PollerConfig construction (line ~461)**

Find the second `PollerConfig{` block and add the same field.

- [ ] **Step 3: Run full classifier tests**

Run: `cd classifier && go test ./... -count=1`
Expected: PASS (existing integration tests use zero-value QualityGate which means gate disabled — no behavior change)

- [ ] **Step 4: Run linter**

Run: `cd classifier && golangci-lint run ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add classifier/cmd/processor/processor.go
git commit -m "feat(classifier): wire QualityGateConfig through cmd/processor to poller

Both PollerConfig construction sites updated to pass through the
quality gate config from the classifier config."
```

---

### Task 7: Add Docker Compose Env Vars

**Files:**
- Modify: `docker-compose.dev.yml` (classifier service environment block)

- [ ] **Step 1: Add quality gate env vars**

In the classifier service environment section (after `CONCURRENT_WORKERS`), add:

```yaml
      CLASSIFIER_QUALITY_GATE_ENABLED: "${CLASSIFIER_QUALITY_GATE_ENABLED:-false}"
      CLASSIFIER_QUALITY_GATE_THRESHOLD: "${CLASSIFIER_QUALITY_GATE_THRESHOLD:-40}"
```

- [ ] **Step 2: Commit**

```bash
git add docker-compose.dev.yml
git commit -m "chore: add CLASSIFIER_QUALITY_GATE env vars to docker-compose.dev.yml

Disabled by default (false). Set CLASSIFIER_QUALITY_GATE_ENABLED=true
to activate."
```

---

### Task 8: Update Basque Tribune Source Config (#564)

**Files:** None (API call only)

- [ ] **Step 1: Check current Basque Tribune source config**

Use the MCP tool or curl to check the current source config:
```
GET /api/v1/sources/4a462f0f-50ce-45bb-b10b-688e562ca2de
```

Verify whether `allowed_domains` is already set.

- [ ] **Step 2: Update source with explicit allowed_domains**

If `allowed_domains` is empty or missing, update via API:
```
PUT /api/v1/sources/4a462f0f-50ce-45bb-b10b-688e562ca2de
{
  "allowed_domains": ["naiz.eus", "www.naiz.eus"]
}
```

Note: This is a production data change — verify API endpoint and payload format before executing.

- [ ] **Step 3: Document the change**

This step closes #564. The source update prevents the crawler from following outbound links to external domains.

---

### Task 9: Update Classifier CLAUDE.md

**Files:**
- Modify: `classifier/CLAUDE.md`

- [ ] **Step 1: Add quality gate section**

Add after the "Quality Score Details" section in CLAUDE.md:

```markdown
### Quality Gate

Configurable gate that filters documents before indexing to `*_classified_content`:

| Condition | Action |
|-----------|--------|
| `quality_score >= threshold` | Index normally |
| `quality_score < threshold` AND `content_type=article` | Index with `low_quality=true` |
| `quality_score < threshold` AND `content_type!=article` | Reject (logged, not indexed) |

**Config:**
- `CLASSIFIER_QUALITY_GATE_ENABLED` (bool, default `false`) — feature flag
- `CLASSIFIER_QUALITY_GATE_THRESHOLD` (int, default `40`) — minimum quality_score

**Observability:** Rejected and flagged documents are logged at `info` level with source, content_type, quality_score, and URL.
```

- [ ] **Step 2: Add gotcha about quality gate**

Add to "Common Gotchas" section:

```markdown
10. **Quality gate is off by default**: Set `CLASSIFIER_QUALITY_GATE_ENABLED=true` to activate. When enabled, non-article content (pages, events) with `quality_score < 40` will be silently dropped from indexing. Articles below threshold are indexed with `low_quality=true` flag.
```

- [ ] **Step 3: Commit**

```bash
git add classifier/CLAUDE.md
git commit -m "docs(classifier): add quality gate documentation to CLAUDE.md"
```

---

### Task 10: Run Full CI and Verify

- [ ] **Step 1: Run full CI**

Run: `task ci:changed`
Expected: PASS (lint + test for classifier)

- [ ] **Step 2: Run spec drift check**

Run: `task drift:check`
Expected: PASS (or update classification spec if stale)

- [ ] **Step 3: Final commit if any spec updates needed**

If drift:check flags stale specs, update `docs/specs/classification.md` with quality gate info.
