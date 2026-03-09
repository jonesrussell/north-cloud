# Drift-Aware Governor Design

**Date:** 2026-03-08
**Status:** Approved
**Service:** ai-observer (extension)

---

## Overview

Upgrade the AI Observer from advisory-only LLM analysis to a drift-aware governor that computes statistical drift metrics, detects threshold breaches, and auto-generates draft PRs with proposed rule patches. The governor never self-merges — human review is always required.

## Key Decisions

| Decision | Choice |
|----------|--------|
| Autonomy | Advisory + auto-generate draft PRs (human merges) |
| Baseline storage | Elasticsearch (`drift_baselines` index) |
| LLM role | Statistical first, LLM only on threshold breach for context |
| Patch scope | Topic keyword rules only |
| Interval | 6 hours (separate from 30-min observer ticker) |
| GitHub integration | GitHub Actions workflow (observer writes report, GHA creates issues/PRs) |
| Architecture | Extend existing ai-observer service (Approach A) |

---

## 1. Drift Metrics Engine

Three statistical metrics computed every 6 hours from `*_classified_content` docs in the current window.

### A. KL Divergence (Category Distributions)

- Builds probability distribution of `topics` field values from current 6h window
- Compares against baseline distribution stored in `drift_baselines`
- Computed per-region (using `source_region`) and globally
- **Threshold: > 0.15**
- Catches: category collapse, overfitting, sudden shifts in source mix

### B. PSI (Population Stability Index) on Confidence Scores

- Bins confidence scores into 10 equal-width buckets (0.0–0.1, 0.1–0.2, ...)
- Compares current window's bin distribution against baseline
- Computed per-domain classifier (crime, mining, coforge, entertainment, indigenous) and globally
- **Threshold: > 0.25**
- Catches: classifier degradation, language drift, content structure changes

### C. Region/Category Cross-Matrix Stability

- Builds matrix: rows = regions, columns = categories
- Each cell = proportion of docs in that region assigned that category
- Compares against baseline using cell-wise percentage deviation
- Only cells with baseline count >= 5 are evaluated (avoids noise from sparse cells)
- **Threshold: > 20% deviation**
- Catches: region-specific failures, language-specific regressions, template changes

### Implementation

New package: `ai-observer/internal/drift/`

```go
type DriftSignal struct {
    Metric    string         // "kl_divergence", "psi", "cross_matrix"
    Scope     string         // "global", "region:north_america", "domain:crime"
    Value     float64        // computed metric value
    Threshold float64        // configured threshold
    Breached  bool
    Details   map[string]any // breakdown data for LLM context
}
```

Files:
- `metrics.go` — KL divergence, PSI, cross-matrix functions (pure math, no ES dependency)
- `collector.go` — ES queries to build current-window distributions
- `evaluator.go` — compares current vs baseline, returns `[]DriftSignal`

---

## 2. Rolling Baseline Sampler

The baseline is the ground truth distribution that drift is measured against.

### Sampling Strategy

A daily job (gated within the 6h ticker, runs once per 24h) builds the baseline:

1. Query the last 7 days of `*_classified_content` (rolling window)
2. Sample: 50 docs per region, 10 per category, stratified
3. Compute distributions:
   - Category distribution (topic → probability) per region and global
   - Confidence score histogram (10 bins) per domain classifier and global
   - Region/category cross-matrix (cell proportions)
4. Write a single baseline document to `drift_baselines`

### ES Index: `drift_baselines`

```json
{
  "computed_at": "2026-03-08T06:00:00Z",
  "window_days": 7,
  "sample_count": 850,
  "category_distribution": {
    "global": {"technology": 0.23, "politics": 0.18},
    "north_america": {"technology": 0.25}
  },
  "confidence_histograms": {
    "global": [0.02, 0.03, 0.05, 0.08, 0.12, 0.15, 0.20, 0.18, 0.10, 0.07],
    "crime": [0.01, 0.02]
  },
  "cross_matrix": {
    "north_america": {"technology": 0.25, "politics": 0.18},
    "oceania": {"indigenous": 0.35}
  }
}
```

### Retention

Keep the last 30 baselines (30 days). Drift evaluator always compares against the most recent. Historical baselines enable trend analysis in Grafana.

### Implementation

File: `ai-observer/internal/drift/baseline.go`
- `BaselineSampler` struct with ES client
- `ComputeBaseline(ctx) (*Baseline, error)`
- `StoreBaseline(ctx, *Baseline) error`
- `LoadLatestBaseline(ctx) (*Baseline, error)`

---

## 3. Alert & Remediation Pipeline

### Step 1: Write Drift Alert Insight

On threshold breach, write to existing `ai_insights` ES index:

```go
Insight{
    Category: "drift",
    Severity: severityFromSignals(signals),
    Summary:  "KL divergence 0.23 (threshold 0.15) in region north_america",
    Details: map[string]any{
        "signals":  signals,
        "baseline": baselineID,
        "window":   "6h",
    },
}
```

### Step 2: LLM Contextual Analysis (on breach only)

Invokes LLM with:
- Breached signals and values
- 20 sample docs from drifting region/category
- Baseline distribution for comparison

LLM returns:
- Human-readable explanation of likely cause
- Suggested keyword rule changes (additions/removals)
- Confidence in recommendation (high/medium/low)

Output stored as `drift_report` insight in `ai_insights`.

### Step 3: Drift Report for GitHub Actions

The `drift_report` insight includes structured data for GHA consumption:

```json
{
  "category": "drift",
  "severity": "high",
  "details": {
    "action_type": "rule_patch",
    "suggested_rules": [
      {"operation": "add", "topic": "indigenous", "keyword": "First Nations consultation", "region": "north_america"},
      {"operation": "remove", "topic": "mining", "keyword": "resource", "reason": "false positive rate 34%"}
    ],
    "issue_title": "Classifier Drift: Indigenous category collapse in North America",
    "issue_body": "KL divergence 0.23 (threshold 0.15)...",
    "pr_description": "Auto-generated rule patch for indigenous topic drift..."
  }
}
```

### Step 4: GitHub Actions Workflow

Scheduled workflow (every 6h, offset from observer):

1. Query `ai_insights` for unprocessed `drift_report` insights (severity >= medium)
2. For each report:
   - Create GitHub issue with drift details, metrics, recommended actions
   - If `action_type == "rule_patch"`: generate migration SQL, open draft PR
   - Tag with `classifier-drift` label and relevant milestone
3. Mark insight as processed (update `processed_at` field)

### Draft PR Content

- Migration file: `classifier/internal/database/migrations/NNNN_drift_rule_patch.up.sql`
- SQL `INSERT`/`DELETE` statements for `classification_rules` table
- PR description with drift metrics, LLM explanation, distribution comparison

### Severity Mapping

| Condition | Severity | Action |
|-----------|----------|--------|
| 1 metric breached, value < 2x threshold | medium | GitHub issue only |
| 1 metric breached, value >= 2x threshold | high | GitHub issue + draft PR |
| 2+ metrics breached | high | GitHub issue + draft PR |
| Cross-matrix anomaly in single region | medium | GitHub issue only |

---

## 4. Integration with Existing Observer

### Dual-Ticker Scheduler

Two goroutines in the scheduler:
- **Fast ticker (30 min):** Existing `classifier` category (LLM-based sampling)
- **Slow ticker (6 hours):** New `drift` category (statistical metrics + baseline refresh)

The slow ticker gates daily baseline refresh (runs once per 24h based on `last_baseline_at`).

### Configuration

New env vars:
```
AI_OBSERVER_DRIFT_ENABLED=false
AI_OBSERVER_DRIFT_INTERVAL_SECONDS=21600
AI_OBSERVER_DRIFT_KL_THRESHOLD=0.15
AI_OBSERVER_DRIFT_PSI_THRESHOLD=0.25
AI_OBSERVER_DRIFT_MATRIX_THRESHOLD=0.20
AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS=7
AI_OBSERVER_DRIFT_BASELINE_RETENTION=30
```

### ES Indices

One new index created at startup: `drift_baselines`. Drift reports use existing `ai_insights` index with `category: "drift"`.

### Token Budget

LLM calls only on threshold breach. ~2,000–5,000 tokens per breach (20 sample docs + metric context). At most 4 LLM calls/day from drift. Well within existing 25,000 token/interval budget.

### Grafana Dashboard

Extend AI Insights dashboard with "Drift Governor" row:
- KL divergence time series (per region)
- PSI time series (per domain classifier)
- Cross-matrix heatmap
- Threshold lines overlaid
- Drift alert count panel

### File Structure

```
ai-observer/
├── internal/
│   ├── drift/
│   │   ├── metrics.go          # KL, PSI, cross-matrix math
│   │   ├── metrics_test.go
│   │   ├── collector.go        # ES queries for current window
│   │   ├── collector_test.go
│   │   ├── evaluator.go        # compare current vs baseline
│   │   ├── evaluator_test.go
│   │   ├── baseline.go         # sampling and storage
│   │   └── baseline_test.go
│   ├── categories/
│   │   ├── classifier.go       # existing
│   │   └── drift.go            # new category handler
│   └── scheduler/
│       └── scheduler.go        # add slow ticker
├── ...
.github/
└── workflows/
    └── drift-remediation.yml   # GHA workflow
```

---

## Rollout Phases

| Phase | Description | Gate |
|-------|-------------|------|
| 0 | Implement metrics engine + baseline sampler. Compute and store metrics. No alerts. | Tests pass, metrics appear in Grafana |
| 1 | Enable threshold evaluation. Write drift insights to `ai_insights`. No LLM, no GHA. | Metrics stable for 1 week in dev |
| 2 | Enable LLM analysis on breach. Write drift reports. No GHA. | LLM explanations are accurate |
| 3 | Enable GHA workflow. Create issues on breach. No draft PRs yet. | Issues are actionable |
| 4 | Enable draft PR generation in GHA workflow. | PRs contain valid migration SQL |

---

## Non-Goals

- Auto-merging PRs (human review always required)
- Patching confidence thresholds or ML sidecar weights (rules only)
- Real-time alerting (6h interval by design)
- Replacing the existing LLM-based classifier category (it continues on 30-min ticker)
