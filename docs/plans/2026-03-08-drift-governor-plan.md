# Drift-Aware Governor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add statistical drift detection (KL divergence, PSI, cross-matrix stability) to the AI Observer with rolling baselines, threshold-triggered LLM analysis, and GitHub Actions remediation workflow.

**Architecture:** Extend the existing `ai-observer` service with a new `drift` category running on a separate 6-hour ticker. Pure-math metrics package with no ES dependency. Baselines stored in a dedicated ES index. LLM invoked only on threshold breach. GitHub Actions workflow consumes drift reports and creates issues/draft PRs.

**Tech Stack:** Go 1.26+, Elasticsearch 8, existing `ai-observer` infra (scheduler, insights writer, LLM provider, Grafana)

**Design Doc:** `docs/plans/2026-03-08-drift-governor-design.md`

---

### Task 1: Drift Metrics — Pure Math Functions

**Files:**
- Create: `ai-observer/internal/drift/metrics.go`
- Test: `ai-observer/internal/drift/metrics_test.go`

This task implements three statistical functions with zero external dependencies.

**Step 1: Write failing tests for KL divergence**

```go
// ai-observer/internal/drift/metrics_test.go
package drift_test

import (
	"math"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

const floatTolerance = 1e-9

func TestKLDivergence_IdenticalDistributions(t *testing.T) {
	t.Helper()
	baseline := map[string]float64{"a": 0.5, "b": 0.3, "c": 0.2}
	current := map[string]float64{"a": 0.5, "b": 0.3, "c": 0.2}

	got := drift.KLDivergence(baseline, current)
	if math.Abs(got) > floatTolerance {
		t.Errorf("expected 0 for identical distributions, got %f", got)
	}
}

func TestKLDivergence_DriftedDistribution(t *testing.T) {
	t.Helper()
	baseline := map[string]float64{"a": 0.5, "b": 0.3, "c": 0.2}
	current := map[string]float64{"a": 0.8, "b": 0.1, "c": 0.1}

	got := drift.KLDivergence(baseline, current)
	if got <= 0 {
		t.Errorf("expected positive KL divergence for drifted distribution, got %f", got)
	}
}

func TestKLDivergence_NewCategoryInCurrent(t *testing.T) {
	t.Helper()
	baseline := map[string]float64{"a": 0.5, "b": 0.5}
	current := map[string]float64{"a": 0.4, "b": 0.3, "c": 0.3}

	got := drift.KLDivergence(baseline, current)
	// Should handle new categories gracefully (smoothing)
	if math.IsNaN(got) || math.IsInf(got, 0) {
		t.Errorf("expected finite value, got %f", got)
	}
}

func TestKLDivergence_EmptyDistributions(t *testing.T) {
	t.Helper()
	got := drift.KLDivergence(nil, nil)
	if got != 0 {
		t.Errorf("expected 0 for empty distributions, got %f", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestKLDivergence`
Expected: FAIL — package does not exist

**Step 3: Implement KL divergence**

```go
// ai-observer/internal/drift/metrics.go
package drift

import "math"

// smoothingEpsilon prevents log(0) in KL divergence and PSI calculations.
const smoothingEpsilon = 1e-10

// KLDivergence computes the Kullback-Leibler divergence from baseline to current.
// KL(P||Q) = Σ P(x) * log(P(x) / Q(x))
// Uses additive smoothing to handle zero probabilities.
// Returns 0 for empty distributions.
func KLDivergence(baseline, current map[string]float64) float64 {
	if len(baseline) == 0 && len(current) == 0 {
		return 0
	}

	// Collect all keys from both distributions.
	keys := allKeys(baseline, current)

	var kl float64
	for _, k := range keys {
		p := baseline[k] + smoothingEpsilon
		q := current[k] + smoothingEpsilon
		kl += p * math.Log(p/q)
	}

	return kl
}

// allKeys returns the union of keys from two maps, sorted for determinism.
func allKeys(a, b map[string]float64) []string {
	seen := make(map[string]struct{}, len(a)+len(b))
	for k := range a {
		seen[k] = struct{}{}
	}
	for k := range b {
		seen[k] = struct{}{}
	}

	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}

	return keys
}
```

**Step 4: Run tests to verify they pass**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestKLDivergence`
Expected: PASS

**Step 5: Write failing tests for PSI**

Add to `metrics_test.go`:

```go
func TestPSI_IdenticalHistograms(t *testing.T) {
	t.Helper()
	baseline := []float64{0.1, 0.2, 0.3, 0.2, 0.1, 0.05, 0.03, 0.01, 0.005, 0.005}
	current := []float64{0.1, 0.2, 0.3, 0.2, 0.1, 0.05, 0.03, 0.01, 0.005, 0.005}

	got := drift.PSI(baseline, current)
	if math.Abs(got) > floatTolerance {
		t.Errorf("expected 0 for identical histograms, got %f", got)
	}
}

func TestPSI_DriftedHistogram(t *testing.T) {
	t.Helper()
	baseline := []float64{0.1, 0.2, 0.3, 0.2, 0.1, 0.05, 0.03, 0.01, 0.005, 0.005}
	current := []float64{0.3, 0.3, 0.2, 0.1, 0.05, 0.03, 0.01, 0.005, 0.003, 0.002}

	got := drift.PSI(baseline, current)
	if got <= 0 {
		t.Errorf("expected positive PSI for drifted histogram, got %f", got)
	}
}

func TestPSI_MismatchedLengths(t *testing.T) {
	t.Helper()
	got := drift.PSI([]float64{0.5, 0.5}, []float64{0.3, 0.3, 0.4})
	if got != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", got)
	}
}

func TestPSI_EmptyHistograms(t *testing.T) {
	t.Helper()
	got := drift.PSI(nil, nil)
	if got != 0 {
		t.Errorf("expected 0 for empty histograms, got %f", got)
	}
}
```

**Step 6: Run tests to verify PSI tests fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestPSI`
Expected: FAIL — `drift.PSI` undefined

**Step 7: Implement PSI**

Add to `metrics.go`:

```go
// PSI computes the Population Stability Index between two histograms.
// PSI = Σ (actual_i - expected_i) * ln(actual_i / expected_i)
// Returns 0 for empty or mismatched-length histograms.
func PSI(baseline, current []float64) float64 {
	if len(baseline) == 0 || len(baseline) != len(current) {
		return 0
	}

	var psi float64
	for i := range baseline {
		b := baseline[i] + smoothingEpsilon
		c := current[i] + smoothingEpsilon
		psi += (c - b) * math.Log(c/b)
	}

	return psi
}
```

**Step 8: Run tests to verify PSI tests pass**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestPSI`
Expected: PASS

**Step 9: Write failing tests for cross-matrix deviation**

Add to `metrics_test.go`:

```go
func TestCrossMatrixDeviation_Stable(t *testing.T) {
	t.Helper()
	baseline := map[string]map[string]float64{
		"north_america": {"technology": 0.25, "politics": 0.18},
		"oceania":       {"indigenous": 0.35, "politics": 0.10},
	}
	current := map[string]map[string]float64{
		"north_america": {"technology": 0.26, "politics": 0.17},
		"oceania":       {"indigenous": 0.34, "politics": 0.11},
	}
	baselineCounts := map[string]map[string]int{
		"north_america": {"technology": 50, "politics": 36},
		"oceania":       {"indigenous": 35, "politics": 10},
	}

	deviations := drift.CrossMatrixDeviation(baseline, current, baselineCounts)
	for _, d := range deviations {
		if d.Deviation > 0.20 {
			t.Errorf("expected no deviations above 20%%, got %s/%s at %.2f", d.Region, d.Category, d.Deviation)
		}
	}
}

func TestCrossMatrixDeviation_Drifted(t *testing.T) {
	t.Helper()
	baseline := map[string]map[string]float64{
		"north_america": {"technology": 0.50},
	}
	current := map[string]map[string]float64{
		"north_america": {"technology": 0.10},
	}
	baselineCounts := map[string]map[string]int{
		"north_america": {"technology": 50},
	}

	deviations := drift.CrossMatrixDeviation(baseline, current, baselineCounts)
	if len(deviations) == 0 {
		t.Fatal("expected at least one deviation")
	}
	if deviations[0].Deviation <= 0.20 {
		t.Errorf("expected deviation > 20%%, got %.2f", deviations[0].Deviation)
	}
}

func TestCrossMatrixDeviation_SparseCellsSkipped(t *testing.T) {
	t.Helper()
	baseline := map[string]map[string]float64{
		"north_america": {"technology": 0.50},
	}
	current := map[string]map[string]float64{
		"north_america": {"technology": 0.10},
	}
	// Count below minCellCount threshold — should be skipped
	baselineCounts := map[string]map[string]int{
		"north_america": {"technology": 3},
	}

	deviations := drift.CrossMatrixDeviation(baseline, current, baselineCounts)
	if len(deviations) != 0 {
		t.Errorf("expected sparse cells to be skipped, got %d deviations", len(deviations))
	}
}
```

**Step 10: Run tests to verify cross-matrix tests fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestCrossMatrix`
Expected: FAIL — `drift.CrossMatrixDeviation` undefined

**Step 11: Implement cross-matrix deviation**

Add to `metrics.go`:

```go
// minCellCount is the minimum baseline count for a cell to be evaluated.
const minCellCount = 5

// CellDeviation represents a single region/category cell that has drifted.
type CellDeviation struct {
	Region    string
	Category  string
	Baseline  float64
	Current   float64
	Deviation float64 // absolute percentage deviation: |current - baseline| / baseline
}

// CrossMatrixDeviation computes cell-wise percentage deviation between baseline
// and current region/category matrices. Only evaluates cells with baseline count >= minCellCount.
func CrossMatrixDeviation(
	baseline, current map[string]map[string]float64,
	baselineCounts map[string]map[string]int,
) []CellDeviation {
	if len(baseline) == 0 {
		return nil
	}

	var deviations []CellDeviation
	for region, categories := range baseline {
		for cat, baseVal := range categories {
			// Skip sparse cells.
			if baselineCounts[region][cat] < minCellCount {
				continue
			}
			curVal := current[region][cat]
			if baseVal == 0 {
				continue
			}
			deviation := math.Abs(curVal-baseVal) / baseVal
			deviations = append(deviations, CellDeviation{
				Region:    region,
				Category:  cat,
				Baseline:  baseVal,
				Current:   curVal,
				Deviation: deviation,
			})
		}
	}

	return deviations
}
```

**Step 12: Run all metrics tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v`
Expected: ALL PASS

**Step 13: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/drift/`
Expected: No errors

**Step 14: Commit**

```bash
git add ai-observer/internal/drift/metrics.go ai-observer/internal/drift/metrics_test.go
git commit -m "feat(drift): add KL divergence, PSI, and cross-matrix metrics"
```

---

### Task 2: DriftSignal Type and Evaluator

**Files:**
- Create: `ai-observer/internal/drift/signal.go`
- Create: `ai-observer/internal/drift/evaluator.go`
- Test: `ai-observer/internal/drift/evaluator_test.go`

The evaluator compares current distributions against a baseline and returns `[]DriftSignal`.

**Step 1: Write the DriftSignal type**

```go
// ai-observer/internal/drift/signal.go
package drift

// DriftSignal represents the result of evaluating a single drift metric.
type DriftSignal struct {
	// Metric is the type of drift metric: "kl_divergence", "psi", "cross_matrix".
	Metric string
	// Scope identifies what was measured: "global", "region:north_america", "domain:crime".
	Scope string
	// Value is the computed metric value.
	Value float64
	// Threshold is the configured threshold for this metric.
	Threshold float64
	// Breached is true if Value exceeds Threshold.
	Breached bool
	// Details holds metric-specific breakdown data.
	Details map[string]any
}
```

**Step 2: Write the Baseline type**

Add to `signal.go`:

```go
// Baseline holds precomputed distributions for drift comparison.
type Baseline struct {
	// ComputedAt is when this baseline was generated.
	ComputedAt string `json:"computed_at"`
	// WindowDays is the number of days in the rolling window.
	WindowDays int `json:"window_days"`
	// SampleCount is the total number of docs sampled.
	SampleCount int `json:"sample_count"`
	// CategoryDistribution maps scope (e.g. "global", "north_america") to topic→probability.
	CategoryDistribution map[string]map[string]float64 `json:"category_distribution"`
	// ConfidenceHistograms maps scope (e.g. "global", "crime") to 10-bin histogram.
	ConfidenceHistograms map[string][]float64 `json:"confidence_histograms"`
	// CrossMatrix maps region to category→proportion.
	CrossMatrix map[string]map[string]float64 `json:"cross_matrix"`
	// CrossMatrixCounts maps region to category→doc count (for sparse cell filtering).
	CrossMatrixCounts map[string]map[string]int `json:"cross_matrix_counts"`
}

// Thresholds holds the configured drift thresholds.
type Thresholds struct {
	KLDivergence float64
	PSI          float64
	MatrixDeviation float64
}
```

**Step 3: Write failing evaluator tests**

```go
// ai-observer/internal/drift/evaluator_test.go
package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestEvaluate_NoBreaches(t *testing.T) {
	t.Helper()
	baseline := &drift.Baseline{
		CategoryDistribution: map[string]map[string]float64{
			"global": {"tech": 0.5, "politics": 0.3, "sports": 0.2},
		},
		ConfidenceHistograms: map[string][]float64{
			"global": {0.02, 0.03, 0.05, 0.08, 0.12, 0.15, 0.20, 0.18, 0.10, 0.07},
		},
		CrossMatrix: map[string]map[string]float64{
			"north_america": {"tech": 0.50},
		},
		CrossMatrixCounts: map[string]map[string]int{
			"north_america": {"tech": 50},
		},
	}
	current := &drift.Baseline{
		CategoryDistribution: map[string]map[string]float64{
			"global": {"tech": 0.5, "politics": 0.3, "sports": 0.2},
		},
		ConfidenceHistograms: map[string][]float64{
			"global": {0.02, 0.03, 0.05, 0.08, 0.12, 0.15, 0.20, 0.18, 0.10, 0.07},
		},
		CrossMatrix: map[string]map[string]float64{
			"north_america": {"tech": 0.50},
		},
		CrossMatrixCounts: map[string]map[string]int{
			"north_america": {"tech": 50},
		},
	}
	thresholds := drift.Thresholds{
		KLDivergence:    0.15,
		PSI:             0.25,
		MatrixDeviation: 0.20,
	}

	signals := drift.Evaluate(baseline, current, thresholds)
	for _, s := range signals {
		if s.Breached {
			t.Errorf("expected no breaches, but %s/%s breached (value=%.4f)", s.Metric, s.Scope, s.Value)
		}
	}
}

func TestEvaluate_KLBreach(t *testing.T) {
	t.Helper()
	baseline := &drift.Baseline{
		CategoryDistribution: map[string]map[string]float64{
			"global": {"tech": 0.5, "politics": 0.3, "sports": 0.2},
		},
		ConfidenceHistograms: map[string][]float64{},
		CrossMatrix:          map[string]map[string]float64{},
		CrossMatrixCounts:    map[string]map[string]int{},
	}
	current := &drift.Baseline{
		CategoryDistribution: map[string]map[string]float64{
			"global": {"tech": 0.9, "politics": 0.05, "sports": 0.05},
		},
		ConfidenceHistograms: map[string][]float64{},
		CrossMatrix:          map[string]map[string]float64{},
		CrossMatrixCounts:    map[string]map[string]int{},
	}
	thresholds := drift.Thresholds{
		KLDivergence:    0.15,
		PSI:             0.25,
		MatrixDeviation: 0.20,
	}

	signals := drift.Evaluate(baseline, current, thresholds)
	var found bool
	for _, s := range signals {
		if s.Metric == "kl_divergence" && s.Scope == "global" && s.Breached {
			found = true
		}
	}
	if !found {
		t.Error("expected KL divergence breach for global scope")
	}
}

func TestEvaluate_NilBaseline(t *testing.T) {
	t.Helper()
	signals := drift.Evaluate(nil, nil, drift.Thresholds{})
	if len(signals) != 0 {
		t.Errorf("expected no signals for nil baseline, got %d", len(signals))
	}
}
```

**Step 4: Run tests to verify they fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestEvaluate`
Expected: FAIL — `drift.Evaluate` undefined

**Step 5: Implement evaluator**

```go
// ai-observer/internal/drift/evaluator.go
package drift

// Evaluate compares current distributions against a baseline and returns all drift signals.
// Each metric/scope combination produces one signal, regardless of whether it breached.
func Evaluate(baseline, current *Baseline, thresholds Thresholds) []DriftSignal {
	if baseline == nil || current == nil {
		return nil
	}

	var signals []DriftSignal
	signals = append(signals, evaluateKL(baseline, current, thresholds.KLDivergence)...)
	signals = append(signals, evaluatePSI(baseline, current, thresholds.PSI)...)
	signals = append(signals, evaluateMatrix(baseline, current, thresholds.MatrixDeviation)...)

	return signals
}

func evaluateKL(baseline, current *Baseline, threshold float64) []DriftSignal {
	// Collect all scopes from both baseline and current.
	scopes := make(map[string]struct{})
	for scope := range baseline.CategoryDistribution {
		scopes[scope] = struct{}{}
	}
	for scope := range current.CategoryDistribution {
		scopes[scope] = struct{}{}
	}

	signals := make([]DriftSignal, 0, len(scopes))
	for scope := range scopes {
		value := KLDivergence(baseline.CategoryDistribution[scope], current.CategoryDistribution[scope])
		signals = append(signals, DriftSignal{
			Metric:    "kl_divergence",
			Scope:     scope,
			Value:     value,
			Threshold: threshold,
			Breached:  value > threshold,
		})
	}

	return signals
}

func evaluatePSI(baseline, current *Baseline, threshold float64) []DriftSignal {
	scopes := make(map[string]struct{})
	for scope := range baseline.ConfidenceHistograms {
		scopes[scope] = struct{}{}
	}
	for scope := range current.ConfidenceHistograms {
		scopes[scope] = struct{}{}
	}

	signals := make([]DriftSignal, 0, len(scopes))
	for scope := range scopes {
		value := PSI(baseline.ConfidenceHistograms[scope], current.ConfidenceHistograms[scope])
		signals = append(signals, DriftSignal{
			Metric:    "psi",
			Scope:     scope,
			Value:     value,
			Threshold: threshold,
			Breached:  value > threshold,
		})
	}

	return signals
}

func evaluateMatrix(baseline, current *Baseline, threshold float64) []DriftSignal {
	deviations := CrossMatrixDeviation(baseline.CrossMatrix, current.CrossMatrix, baseline.CrossMatrixCounts)

	signals := make([]DriftSignal, 0, len(deviations))
	for _, d := range deviations {
		signals = append(signals, DriftSignal{
			Metric:    "cross_matrix",
			Scope:     d.Region + ":" + d.Category,
			Value:     d.Deviation,
			Threshold: threshold,
			Breached:  d.Deviation > threshold,
			Details: map[string]any{
				"region":   d.Region,
				"category": d.Category,
				"baseline": d.Baseline,
				"current":  d.Current,
			},
		})
	}

	return signals
}
```

**Step 6: Run all tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v`
Expected: ALL PASS

**Step 7: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/drift/`
Expected: No errors

**Step 8: Commit**

```bash
git add ai-observer/internal/drift/signal.go ai-observer/internal/drift/evaluator.go ai-observer/internal/drift/evaluator_test.go
git commit -m "feat(drift): add DriftSignal type and evaluator"
```

---

### Task 3: Baseline Sampler — ES Queries and Distribution Builder

**Files:**
- Create: `ai-observer/internal/drift/baseline.go`
- Test: `ai-observer/internal/drift/baseline_test.go`

The baseline sampler queries `*_classified_content` over a 7-day window and computes distributions.

**Step 1: Write failing tests for distribution builders**

```go
// ai-observer/internal/drift/baseline_test.go
package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestBuildCategoryDistribution(t *testing.T) {
	t.Helper()
	docs := []drift.ClassifiedDoc{
		{Topics: []string{"tech", "sports"}, SourceRegion: "north_america"},
		{Topics: []string{"tech"}, SourceRegion: "north_america"},
		{Topics: []string{"politics"}, SourceRegion: "oceania"},
	}

	dist := drift.BuildCategoryDistribution(docs)

	// Global: tech=2/4, sports=1/4, politics=1/4
	if got := dist["global"]["tech"]; got < 0.49 || got > 0.51 {
		t.Errorf("expected global tech ~0.50, got %f", got)
	}
	// north_america: tech=2/3, sports=1/3
	if got := dist["north_america"]["tech"]; got < 0.66 || got > 0.67 {
		t.Errorf("expected north_america tech ~0.67, got %f", got)
	}
}

func TestBuildConfidenceHistograms(t *testing.T) {
	t.Helper()
	docs := []drift.ClassifiedDoc{
		{Confidence: 0.15, CrimeConfidence: 0.8},
		{Confidence: 0.85, CrimeConfidence: 0.2},
		{Confidence: 0.15},
	}

	histograms := drift.BuildConfidenceHistograms(docs)

	global := histograms["global"]
	if len(global) != drift.HistogramBins {
		t.Fatalf("expected %d bins, got %d", drift.HistogramBins, len(global))
	}
	// Bin 1 (0.1-0.2) should have 2/3 of docs
	if global[1] < 0.66 || global[1] > 0.67 {
		t.Errorf("expected bin[1] ~0.67, got %f", global[1])
	}
}

func TestBuildCrossMatrix(t *testing.T) {
	t.Helper()
	docs := []drift.ClassifiedDoc{
		{Topics: []string{"tech"}, SourceRegion: "north_america"},
		{Topics: []string{"tech"}, SourceRegion: "north_america"},
		{Topics: []string{"politics"}, SourceRegion: "north_america"},
		{Topics: []string{"indigenous"}, SourceRegion: "oceania"},
	}

	matrix, counts := drift.BuildCrossMatrix(docs)

	if got := matrix["north_america"]["tech"]; got < 0.66 || got > 0.67 {
		t.Errorf("expected north_america/tech ~0.67, got %f", got)
	}
	if got := counts["north_america"]["tech"]; got != 2 {
		t.Errorf("expected north_america/tech count 2, got %d", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run "TestBuild"`
Expected: FAIL — `drift.ClassifiedDoc` undefined

**Step 3: Implement distribution builders**

```go
// ai-observer/internal/drift/baseline.go
package drift

import (
	"time"
)

// HistogramBins is the number of bins for confidence score histograms.
const HistogramBins = 10

// ClassifiedDoc is a projection of an ES classified_content document for drift analysis.
type ClassifiedDoc struct {
	SourceRegion    string             `json:"source_region"`
	Topics          []string           `json:"topics"`
	Confidence      float64            `json:"confidence"`
	CrimeConfidence float64            `json:"crime.final_confidence"`
	MiningConfidence float64           `json:"mining.final_confidence"`
	ClassifiedAt    time.Time          `json:"classified_at"`
}

// BuildCategoryDistribution computes topic probability distributions per region and globally.
func BuildCategoryDistribution(docs []ClassifiedDoc) map[string]map[string]float64 {
	// Count topic occurrences per scope.
	counts := make(map[string]map[string]int)  // scope -> topic -> count
	totals := make(map[string]int)             // scope -> total topic mentions

	for _, doc := range docs {
		for _, topic := range doc.Topics {
			addCount(counts, "global", topic)
			totals["global"]++
			if doc.SourceRegion != "" {
				addCount(counts, doc.SourceRegion, topic)
				totals[doc.SourceRegion]++
			}
		}
	}

	// Convert counts to probabilities.
	dist := make(map[string]map[string]float64, len(counts))
	for scope, topicCounts := range counts {
		dist[scope] = make(map[string]float64, len(topicCounts))
		total := totals[scope]
		if total == 0 {
			continue
		}
		for topic, count := range topicCounts {
			dist[scope][topic] = float64(count) / float64(total)
		}
	}

	return dist
}

// BuildConfidenceHistograms builds 10-bin confidence histograms per domain classifier and globally.
func BuildConfidenceHistograms(docs []ClassifiedDoc) map[string][]float64 {
	bins := make(map[string][]int)    // scope -> bin counts
	totals := make(map[string]int)

	for _, doc := range docs {
		addToBin(bins, "global", doc.Confidence)
		totals["global"]++

		if doc.CrimeConfidence > 0 {
			addToBin(bins, "crime", doc.CrimeConfidence)
			totals["crime"]++
		}
		if doc.MiningConfidence > 0 {
			addToBin(bins, "mining", doc.MiningConfidence)
			totals["mining"]++
		}
	}

	// Convert to proportions.
	histograms := make(map[string][]float64, len(bins))
	for scope, binCounts := range bins {
		total := totals[scope]
		hist := make([]float64, HistogramBins)
		if total > 0 {
			for i, count := range binCounts {
				hist[i] = float64(count) / float64(total)
			}
		}
		histograms[scope] = hist
	}

	return histograms
}

// BuildCrossMatrix builds a region/category proportion matrix with raw counts.
func BuildCrossMatrix(docs []ClassifiedDoc) (matrix map[string]map[string]float64, counts map[string]map[string]int) {
	rawCounts := make(map[string]map[string]int)
	regionTotals := make(map[string]int)

	for _, doc := range docs {
		if doc.SourceRegion == "" {
			continue
		}
		for _, topic := range doc.Topics {
			addCount(rawCounts, doc.SourceRegion, topic)
			regionTotals[doc.SourceRegion]++
		}
	}

	matrix = make(map[string]map[string]float64, len(rawCounts))
	for region, topicCounts := range rawCounts {
		matrix[region] = make(map[string]float64, len(topicCounts))
		total := regionTotals[region]
		if total == 0 {
			continue
		}
		for topic, count := range topicCounts {
			matrix[region][topic] = float64(count) / float64(total)
		}
	}

	return matrix, rawCounts
}

func addCount(m map[string]map[string]int, scope, key string) {
	if m[scope] == nil {
		m[scope] = make(map[string]int)
	}
	m[scope][key]++
}

func addToBin(bins map[string][]int, scope string, value float64) {
	if bins[scope] == nil {
		bins[scope] = make([]int, HistogramBins)
	}
	bin := int(value * float64(HistogramBins))
	if bin >= HistogramBins {
		bin = HistogramBins - 1
	}
	if bin < 0 {
		bin = 0
	}
	bins[scope][bin]++
}
```

**Step 4: Run tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run "TestBuild"`
Expected: PASS

**Step 5: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/drift/`
Expected: No errors

**Step 6: Commit**

```bash
git add ai-observer/internal/drift/baseline.go ai-observer/internal/drift/baseline_test.go
git commit -m "feat(drift): add baseline distribution builders"
```

---

### Task 4: Baseline ES Storage (Index Mapping + Read/Write)

**Files:**
- Create: `ai-observer/internal/drift/mapping.go`
- Create: `ai-observer/internal/drift/store.go`
- Test: `ai-observer/internal/drift/store_test.go`

Follows the same pattern as `insights/mapping.go` and `insights/writer.go`.

**Step 1: Write the ES mapping**

```go
// ai-observer/internal/drift/mapping.go
package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	es "github.com/elastic/go-elasticsearch/v8"
)

const baselinesIndex = "drift_baselines"

var baselineMapping = map[string]any{
	"mappings": map[string]any{
		"properties": map[string]any{
			"computed_at":            map[string]any{"type": "date"},
			"window_days":            map[string]any{"type": "integer"},
			"sample_count":           map[string]any{"type": "integer"},
			"category_distribution":  map[string]any{"type": "flattened"},
			"confidence_histograms":  map[string]any{"type": "flattened"},
			"cross_matrix":           map[string]any{"type": "flattened"},
			"cross_matrix_counts":    map[string]any{"type": "flattened"},
		},
	},
}

// EnsureBaselineMapping creates the drift_baselines index if it does not exist.
func EnsureBaselineMapping(ctx context.Context, esClient *es.Client) error {
	mappingBytes, err := json.Marshal(baselineMapping)
	if err != nil {
		return fmt.Errorf("marshal baseline mapping: %w", err)
	}

	res, err := esClient.Indices.Create(
		baselinesIndex,
		esClient.Indices.Create.WithContext(ctx),
		esClient.Indices.Create.WithBody(bytes.NewReader(mappingBytes)),
	)
	if err != nil {
		return fmt.Errorf("create baseline index: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() && !strings.Contains(res.String(), "resource_already_exists_exception") {
		return fmt.Errorf("create baseline index error: %s", res.String())
	}

	return nil
}
```

**Step 2: Write the store (read/write baseline)**

```go
// ai-observer/internal/drift/store.go
package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	es "github.com/elastic/go-elasticsearch/v8"
)

// Store handles reading and writing baselines to Elasticsearch.
type Store struct {
	esClient *es.Client
}

// NewStore creates a new drift Store.
func NewStore(esClient *es.Client) *Store {
	return &Store{esClient: esClient}
}

// StoreBaseline writes a baseline to the drift_baselines index.
func (s *Store) StoreBaseline(ctx context.Context, baseline *Baseline) error {
	docBytes, err := json.Marshal(baseline)
	if err != nil {
		return fmt.Errorf("marshal baseline: %w", err)
	}

	res, err := s.esClient.Index(
		baselinesIndex,
		bytes.NewReader(docBytes),
		s.esClient.Index.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("index baseline: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return fmt.Errorf("index baseline error: %s", res.String())
	}

	return nil
}

// LoadLatestBaseline reads the most recent baseline from the drift_baselines index.
// Returns nil, nil if no baselines exist.
func (s *Store) LoadLatestBaseline(ctx context.Context) (*Baseline, error) {
	query := map[string]any{
		"query": map[string]any{"match_all": map[string]any{}},
		"size":  1,
		"sort":  []map[string]any{{"computed_at": map[string]any{"order": "desc"}}},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal query: %w", err)
	}

	res, err := s.esClient.Search(
		s.esClient.Search.WithContext(ctx),
		s.esClient.Search.WithIndex(baselinesIndex),
		s.esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("search baselines: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("search baselines error: %s", res.String())
	}

	return decodeBaselineHit(res.Body)
}

func decodeBaselineHit(body io.Reader) (*Baseline, error) {
	var result struct {
		Hits struct {
			Hits []struct {
				Source Baseline `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode baseline response: %w", err)
	}

	if len(result.Hits.Hits) == 0 {
		return nil, nil
	}

	baseline := result.Hits.Hits[0].Source
	return &baseline, nil
}
```

**Step 3: Write a unit test for BuildBaselineDocument serialization**

```go
// ai-observer/internal/drift/store_test.go
package drift_test

import (
	"encoding/json"
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestBaseline_JSONRoundTrip(t *testing.T) {
	t.Helper()
	baseline := &drift.Baseline{
		ComputedAt:  "2026-03-08T06:00:00Z",
		WindowDays:  7,
		SampleCount: 100,
		CategoryDistribution: map[string]map[string]float64{
			"global": {"tech": 0.5, "politics": 0.3},
		},
		ConfidenceHistograms: map[string][]float64{
			"global": {0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		},
		CrossMatrix: map[string]map[string]float64{
			"north_america": {"tech": 0.5},
		},
		CrossMatrixCounts: map[string]map[string]int{
			"north_america": {"tech": 50},
		},
	}

	data, err := json.Marshal(baseline)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got drift.Baseline
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.SampleCount != 100 {
		t.Errorf("expected SampleCount 100, got %d", got.SampleCount)
	}
	if got.CategoryDistribution["global"]["tech"] != 0.5 {
		t.Errorf("expected global/tech 0.5, got %f", got.CategoryDistribution["global"]["tech"])
	}
}
```

**Step 4: Run tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v`
Expected: ALL PASS

**Step 5: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/drift/`
Expected: No errors

**Step 6: Commit**

```bash
git add ai-observer/internal/drift/mapping.go ai-observer/internal/drift/store.go ai-observer/internal/drift/store_test.go
git commit -m "feat(drift): add baseline ES mapping and store"
```

---

### Task 5: Drift Collector — ES Query for Current Window

**Files:**
- Create: `ai-observer/internal/drift/collector.go`
- Test: `ai-observer/internal/drift/collector_test.go`

The collector queries `*_classified_content` for the current 6h window and builds distributions.

**Step 1: Write the collector**

```go
// ai-observer/internal/drift/collector.go
package drift

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
)

const (
	classifiedIndexPattern = "*_classified_content"
	// maxCollectorDocs is the maximum number of docs to fetch for distribution building.
	maxCollectorDocs = 5000
)

// Collector queries ES for classified content and builds current-window distributions.
type Collector struct {
	esClient *es.Client
}

// NewCollector creates a new drift Collector.
func NewCollector(esClient *es.Client) *Collector {
	return &Collector{esClient: esClient}
}

// CollectCurrentWindow queries docs classified within the given window and builds distributions.
// Returns a Baseline struct representing the current window (not a stored baseline).
func (c *Collector) CollectCurrentWindow(ctx context.Context, window time.Duration) (*Baseline, error) {
	docs, err := c.queryDocs(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("query docs: %w", err)
	}

	if len(docs) == 0 {
		return nil, nil
	}

	catDist := BuildCategoryDistribution(docs)
	confHist := BuildConfidenceHistograms(docs)
	matrix, matrixCounts := BuildCrossMatrix(docs)

	return &Baseline{
		ComputedAt:           time.Now().UTC().Format(time.RFC3339),
		SampleCount:          len(docs),
		CategoryDistribution: catDist,
		ConfidenceHistograms: confHist,
		CrossMatrix:          matrix,
		CrossMatrixCounts:    matrixCounts,
	}, nil
}

// CollectBaselineWindow queries docs over a multi-day window for baseline computation.
func (c *Collector) CollectBaselineWindow(ctx context.Context, days int) (*Baseline, error) {
	window := time.Duration(days) * 24 * time.Hour
	baseline, err := c.CollectCurrentWindow(ctx, window)
	if err != nil {
		return nil, err
	}
	if baseline != nil {
		baseline.WindowDays = days
	}
	return baseline, nil
}

func (c *Collector) queryDocs(ctx context.Context, window time.Duration) ([]ClassifiedDoc, error) {
	since := time.Now().UTC().Add(-window)

	query := map[string]any{
		"query": map[string]any{
			"range": map[string]any{
				"classified_at": map[string]any{
					"gte": since.Format(time.RFC3339),
				},
			},
		},
		"size": maxCollectorDocs,
		"_source": []string{
			"source_region", "topics", "confidence",
			"crime.final_confidence", "mining.final_confidence",
			"classified_at",
		},
	}

	queryBytes, err := json.Marshal(query)
	if err != nil {
		return nil, fmt.Errorf("marshal collector query: %w", err)
	}

	res, err := c.esClient.Search(
		c.esClient.Search.WithContext(ctx),
		c.esClient.Search.WithIndex(classifiedIndexPattern),
		c.esClient.Search.WithBody(bytes.NewReader(queryBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("es search: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("es search error: %s", res.String())
	}

	return decodeClassifiedHits(res.Body)
}

func decodeClassifiedHits(body io.Reader) ([]ClassifiedDoc, error) {
	var result struct {
		Hits struct {
			Hits []struct {
				Source ClassifiedDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode classified hits: %w", err)
	}

	docs := make([]ClassifiedDoc, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		docs = append(docs, hit.Source)
	}

	return docs, nil
}
```

**Step 2: Write a unit test for query structure**

```go
// ai-observer/internal/drift/collector_test.go
package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestCollector_New(t *testing.T) {
	t.Helper()
	// Verify collector can be created with nil client (for unit testing).
	c := drift.NewCollector(nil)
	if c == nil {
		t.Error("expected non-nil collector")
	}
}
```

Note: Full integration tests for the collector require a running ES instance. The collector's logic delegates to `BuildCategoryDistribution`, `BuildConfidenceHistograms`, and `BuildCrossMatrix` which are already unit-tested in Task 3.

**Step 3: Run tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v`
Expected: ALL PASS

**Step 4: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/drift/`
Expected: No errors

**Step 5: Commit**

```bash
git add ai-observer/internal/drift/collector.go ai-observer/internal/drift/collector_test.go
git commit -m "feat(drift): add collector for current-window distributions"
```

---

### Task 6: Severity Calculation

**Files:**
- Create: `ai-observer/internal/drift/severity.go`
- Test: `ai-observer/internal/drift/severity_test.go`

Maps drift signals to severity levels per the design doc's severity table.

**Step 1: Write failing tests**

```go
// ai-observer/internal/drift/severity_test.go
package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
)

func TestSeverityFromSignals_NoBreach(t *testing.T) {
	t.Helper()
	signals := []drift.DriftSignal{
		{Metric: "kl_divergence", Value: 0.10, Threshold: 0.15, Breached: false},
	}
	if got := drift.SeverityFromSignals(signals); got != "low" {
		t.Errorf("expected low, got %s", got)
	}
}

func TestSeverityFromSignals_SingleBreachUnderDouble(t *testing.T) {
	t.Helper()
	signals := []drift.DriftSignal{
		{Metric: "kl_divergence", Value: 0.20, Threshold: 0.15, Breached: true},
	}
	if got := drift.SeverityFromSignals(signals); got != "medium" {
		t.Errorf("expected medium, got %s", got)
	}
}

func TestSeverityFromSignals_SingleBreachOverDouble(t *testing.T) {
	t.Helper()
	signals := []drift.DriftSignal{
		{Metric: "kl_divergence", Value: 0.35, Threshold: 0.15, Breached: true},
	}
	if got := drift.SeverityFromSignals(signals); got != "high" {
		t.Errorf("expected high, got %s", got)
	}
}

func TestSeverityFromSignals_MultipleBreaches(t *testing.T) {
	t.Helper()
	signals := []drift.DriftSignal{
		{Metric: "kl_divergence", Value: 0.20, Threshold: 0.15, Breached: true},
		{Metric: "psi", Value: 0.30, Threshold: 0.25, Breached: true},
	}
	if got := drift.SeverityFromSignals(signals); got != "high" {
		t.Errorf("expected high, got %s", got)
	}
}

func TestSeverityFromSignals_Empty(t *testing.T) {
	t.Helper()
	if got := drift.SeverityFromSignals(nil); got != "low" {
		t.Errorf("expected low, got %s", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestSeverity`
Expected: FAIL

**Step 3: Implement severity calculation**

```go
// ai-observer/internal/drift/severity.go
package drift

const (
	// doubleFactor is the multiplier for determining high severity on single breaches.
	doubleFactor = 2.0
)

// SeverityFromSignals computes the overall severity based on drift signals.
//
// Rules:
//   - No breaches → "low"
//   - 1 breach, value < 2x threshold → "medium"
//   - 1 breach, value >= 2x threshold → "high"
//   - 2+ breaches → "high"
func SeverityFromSignals(signals []DriftSignal) string {
	var breachCount int
	var hasDoubleBreach bool

	for _, s := range signals {
		if !s.Breached {
			continue
		}
		breachCount++
		if s.Value >= s.Threshold*doubleFactor {
			hasDoubleBreach = true
		}
	}

	switch {
	case breachCount == 0:
		return "low"
	case breachCount >= 2 || hasDoubleBreach:
		return "high"
	default:
		return "medium"
	}
}
```

**Step 4: Run tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ -v -run TestSeverity`
Expected: ALL PASS

**Step 5: Lint and commit**

```bash
cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/drift/
git add ai-observer/internal/drift/severity.go ai-observer/internal/drift/severity_test.go
git commit -m "feat(drift): add severity calculation from drift signals"
```

---

### Task 7: Drift Category — Orchestrates the Full Pipeline

**Files:**
- Create: `ai-observer/internal/category/drift/category.go`
- Test: `ai-observer/internal/category/drift/category_test.go`

This implements the `category.Category` interface for drift detection. It orchestrates: collect current window → load baseline → evaluate → severity → build insights. LLM is called only when thresholds are breached.

**Step 1: Write failing interface test**

```go
// ai-observer/internal/category/drift/category_test.go
package drift_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	driftcat "github.com/jonesrussell/north-cloud/ai-observer/internal/category/drift"
)

func TestDriftCategory_ImplementsInterface(t *testing.T) {
	t.Helper()
	var _ category.Category = (*driftcat.Category)(nil)
}

func TestDriftCategory_Name(t *testing.T) {
	t.Helper()
	c := driftcat.New(nil, driftcat.Config{})
	if got := c.Name(); got != "drift" {
		t.Errorf("expected name 'drift', got %q", got)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd ai-observer && GOWORK=off go test ./internal/category/drift/ -v`
Expected: FAIL

**Step 3: Implement the drift category**

```go
// ai-observer/internal/category/drift/category.go
package drift

import (
	"context"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	driftpkg "github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const (
	categoryName       = "drift"
	modelTier          = "haiku"
	maxEventsPerRun    = 0 // drift doesn't use event-based sampling
	driftAnalysisModel = "claude-haiku-4-5-20251001"
	// sampleDocsForLLM is how many docs to include in the LLM prompt on breach.
	sampleDocsForLLM   = 20
	// maxResponseTokens is the LLM response token limit for drift analysis.
	maxResponseTokens  = 1500
)

// Config holds drift category configuration.
type Config struct {
	KLThreshold      float64
	PSIThreshold     float64
	MatrixThreshold  float64
	BaselineWindowDays int
}

// Category implements the category.Category interface for drift detection.
type Category struct {
	collector  *driftpkg.Collector
	store      *driftpkg.Store
	thresholds driftpkg.Thresholds
	baselineDays int
}

// New creates a new drift category.
func New(esClient *es.Client, cfg Config) *Category {
	return &Category{
		collector: driftpkg.NewCollector(esClient),
		store:     driftpkg.NewStore(esClient),
		thresholds: driftpkg.Thresholds{
			KLDivergence:    cfg.KLThreshold,
			PSI:             cfg.PSIThreshold,
			MatrixDeviation: cfg.MatrixThreshold,
		},
		baselineDays: cfg.BaselineWindowDays,
	}
}

func (c *Category) Name() string          { return categoryName }
func (c *Category) MaxEventsPerRun() int   { return maxEventsPerRun }
func (c *Category) ModelTier() string      { return modelTier }

// Sample collects the current window distribution. The "events" returned are a
// synthetic single event carrying the current distributions in Metadata —
// the drift category doesn't use traditional event sampling.
func (c *Category) Sample(ctx context.Context, window time.Duration) ([]category.Event, error) {
	current, err := c.collector.CollectCurrentWindow(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("collect current window: %w", err)
	}
	if current == nil {
		return nil, nil
	}

	baseline, err := c.store.LoadLatestBaseline(ctx)
	if err != nil {
		return nil, fmt.Errorf("load baseline: %w", err)
	}
	if baseline == nil {
		// No baseline yet — compute and store one, skip evaluation.
		newBaseline, baselineErr := c.collector.CollectBaselineWindow(ctx, c.baselineDays)
		if baselineErr != nil {
			return nil, fmt.Errorf("compute initial baseline: %w", baselineErr)
		}
		if newBaseline != nil {
			if storeErr := c.store.StoreBaseline(ctx, newBaseline); storeErr != nil {
				return nil, fmt.Errorf("store initial baseline: %w", storeErr)
			}
		}
		return nil, nil
	}

	signals := driftpkg.Evaluate(baseline, current, c.thresholds)

	// Pack signals into a synthetic event for Analyze to process.
	return []category.Event{
		{
			Source:    "drift_evaluator",
			Label:     "drift_signals",
			Timestamp: time.Now().UTC(),
			Metadata: map[string]any{
				"signals":  signals,
				"baseline": baseline,
				"current":  current,
			},
		},
	}, nil
}

// Analyze processes drift signals. If any thresholds are breached, invokes the LLM
// for contextual explanation. Always returns at least one insight with metric values.
func (c *Category) Analyze(ctx context.Context, events []category.Event, p provider.LLMProvider) ([]category.Insight, error) {
	if len(events) == 0 {
		return nil, nil
	}

	signals, ok := events[0].Metadata["signals"].([]driftpkg.DriftSignal)
	if !ok {
		return nil, fmt.Errorf("unexpected signals type in metadata")
	}

	severity := driftpkg.SeverityFromSignals(signals)

	// Build base insight with metric values.
	insight := category.Insight{
		Category: categoryName,
		Severity: severity,
		Summary:  buildSummary(signals),
		Details:  buildDetails(signals),
	}

	// If no breaches, record metrics only (no LLM call).
	if severity == "low" {
		return []category.Insight{insight}, nil
	}

	// Breached — invoke LLM for contextual analysis.
	if p != nil {
		llmInsight, err := analyzeDrift(ctx, p, signals)
		if err != nil {
			// LLM failure is non-fatal; return metric-only insight.
			insight.SuggestedActions = []string{"LLM analysis failed: " + err.Error()}
			return []category.Insight{insight}, nil
		}
		insight.SuggestedActions = llmInsight.SuggestedActions
		insight.TokensUsed = llmInsight.TokensUsed
		insight.Model = llmInsight.Model
		if llmInsight.Summary != "" {
			insight.Summary = llmInsight.Summary
		}
		// Merge LLM details into insight.
		for k, v := range llmInsight.Details {
			insight.Details[k] = v
		}
	}

	return []category.Insight{insight}, nil
}

func buildSummary(signals []driftpkg.DriftSignal) string {
	var breached int
	var first driftpkg.DriftSignal
	for _, s := range signals {
		if s.Breached {
			breached++
			if breached == 1 {
				first = s
			}
		}
	}

	if breached == 0 {
		return "No drift detected — all metrics within thresholds"
	}
	if breached == 1 {
		return fmt.Sprintf("%s %.3f (threshold %.2f) in %s", first.Metric, first.Value, first.Threshold, first.Scope)
	}
	return fmt.Sprintf("%d metrics breached — %s %.3f in %s (and %d more)",
		breached, first.Metric, first.Value, first.Scope, breached-1)
}

func buildDetails(signals []driftpkg.DriftSignal) map[string]any {
	details := map[string]any{
		"signal_count":  len(signals),
		"breach_count":  countBreaches(signals),
	}

	breachedSignals := make([]map[string]any, 0)
	for _, s := range signals {
		if s.Breached {
			breachedSignals = append(breachedSignals, map[string]any{
				"metric":    s.Metric,
				"scope":     s.Scope,
				"value":     s.Value,
				"threshold": s.Threshold,
			})
		}
	}
	if len(breachedSignals) > 0 {
		details["breached_signals"] = breachedSignals
	}

	return details
}

func countBreaches(signals []driftpkg.DriftSignal) int {
	var count int
	for _, s := range signals {
		if s.Breached {
			count++
		}
	}
	return count
}
```

**Step 4: Implement the LLM analysis helper**

Create a separate file to keep `category.go` under the funlen limit:

```go
// ai-observer/internal/category/drift/analyzer.go
package drift

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	driftpkg "github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
)

const driftSystemPrompt = `You are an AI system observer analyzing statistical drift in a news content classifier.
You receive drift metrics (KL divergence, PSI, cross-matrix deviations) that have breached thresholds.
Your job is to:
1. Explain the likely cause of the drift in plain language
2. Suggest specific keyword rule changes (additions or removals) to address the drift
3. Rate your confidence in each suggestion (high/medium/low)

Respond ONLY with valid JSON matching this schema:
{
  "summary": "one sentence explanation",
  "suggested_actions": ["action 1", "action 2"],
  "suggested_rules": [
    {"operation": "add|remove", "topic": "topic_name", "keyword": "keyword", "confidence": "high|medium|low"}
  ]
}`

func analyzeDrift(ctx context.Context, p provider.LLMProvider, signals []driftpkg.DriftSignal) (*category.Insight, error) {
	breachedSignals := make([]driftpkg.DriftSignal, 0)
	for _, s := range signals {
		if s.Breached {
			breachedSignals = append(breachedSignals, s)
		}
	}

	promptData, err := json.Marshal(breachedSignals)
	if err != nil {
		return nil, fmt.Errorf("marshal signals for prompt: %w", err)
	}

	userPrompt := fmt.Sprintf("The following drift metrics have breached their thresholds:\n\n%s\n\nAnalyze the drift and suggest rule changes.", string(promptData))

	resp, err := p.Generate(ctx, provider.GenerateRequest{
		SystemPrompt: driftSystemPrompt,
		UserPrompt:   userPrompt,
		MaxTokens:    maxResponseTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("llm generate: %w", err)
	}

	return parseDriftResponse(resp)
}

type driftLLMResponse struct {
	Summary          string         `json:"summary"`
	SuggestedActions []string       `json:"suggested_actions"`
	SuggestedRules   []any          `json:"suggested_rules"`
}

func parseDriftResponse(resp provider.GenerateResponse) (*category.Insight, error) {
	content := stripFences(resp.Content)

	var parsed driftLLMResponse
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse drift LLM response: %w", err)
	}

	details := map[string]any{}
	if len(parsed.SuggestedRules) > 0 {
		details["suggested_rules"] = parsed.SuggestedRules
		details["action_type"] = "rule_patch"
	}

	totalTokens := resp.InputTokens + resp.OutputTokens

	return &category.Insight{
		Category:         categoryName,
		Summary:          parsed.Summary,
		Details:          details,
		SuggestedActions: parsed.SuggestedActions,
		TokensUsed:       totalTokens,
		Model:            driftAnalysisModel,
	}, nil
}

// stripFences removes markdown code fences from LLM output.
func stripFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
```

**Step 5: Run tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/category/drift/ -v`
Expected: PASS

**Step 6: Run all drift tests**

Run: `cd ai-observer && GOWORK=off go test ./internal/drift/ ./internal/category/drift/ -v`
Expected: ALL PASS

**Step 7: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./internal/category/drift/ ./internal/drift/`
Expected: No errors

**Step 8: Commit**

```bash
git add ai-observer/internal/category/drift/
git commit -m "feat(drift): add drift category implementing Category interface"
```

---

### Task 8: Config Extension and Bootstrap Integration

**Files:**
- Modify: `ai-observer/internal/bootstrap/config.go`
- Modify: `ai-observer/internal/bootstrap/config_test.go`
- Modify: `ai-observer/internal/bootstrap/app.go`

Wire the drift category into the bootstrap and scheduler.

**Step 1: Add drift config fields**

Add to `CategoriesConfig` in `config.go`:

```go
type CategoriesConfig struct {
	ClassifierEnabled   bool
	ClassifierMaxEvents int
	ClassifierModel     string
	// Drift governor config
	DriftEnabled        bool
	DriftIntervalSeconds int
	DriftKLThreshold    float64
	DriftPSIThreshold   float64
	DriftMatrixThreshold float64
	DriftBaselineWindowDays int
	DriftBaselineRetention  int
}
```

Add constants:

```go
const (
	defaultDriftIntervalSeconds    = 21600  // 6 hours
	defaultDriftKLThreshold        = 0.15
	defaultDriftPSIThreshold       = 0.25
	defaultDriftMatrixThreshold    = 0.20
	defaultDriftBaselineWindowDays = 7
	defaultDriftBaselineRetention  = 30
)
```

Add env loading in `LoadConfig()`:

```go
driftInterval, err := envInt("AI_OBSERVER_DRIFT_INTERVAL_SECONDS", defaultDriftIntervalSeconds)
if err != nil {
    return Config{}, err
}

driftKL, err := envFloat("AI_OBSERVER_DRIFT_KL_THRESHOLD", defaultDriftKLThreshold)
if err != nil {
    return Config{}, err
}

driftPSI, err := envFloat("AI_OBSERVER_DRIFT_PSI_THRESHOLD", defaultDriftPSIThreshold)
if err != nil {
    return Config{}, err
}

driftMatrix, err := envFloat("AI_OBSERVER_DRIFT_MATRIX_THRESHOLD", defaultDriftMatrixThreshold)
if err != nil {
    return Config{}, err
}

driftBaselineDays, err := envInt("AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS", defaultDriftBaselineWindowDays)
if err != nil {
    return Config{}, err
}

driftRetention, err := envInt("AI_OBSERVER_DRIFT_BASELINE_RETENTION", defaultDriftBaselineRetention)
if err != nil {
    return Config{}, err
}
```

Add `envFloat` helper:

```go
func envFloat(key string, def float64) (float64, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return n, nil
}
```

Wire into config struct:

```go
Categories: CategoriesConfig{
    // ... existing fields ...
    DriftEnabled:           os.Getenv("AI_OBSERVER_DRIFT_ENABLED") == "true",
    DriftIntervalSeconds:   driftInterval,
    DriftKLThreshold:       driftKL,
    DriftPSIThreshold:      driftPSI,
    DriftMatrixThreshold:   driftMatrix,
    DriftBaselineWindowDays: driftBaselineDays,
    DriftBaselineRetention: driftRetention,
},
```

**Step 2: Update config test**

Add to `config_test.go`:

```go
func TestLoadConfig_DriftDefaults(t *testing.T) {
	t.Helper()
	t.Setenv("AI_OBSERVER_ENABLED", "false")
	cfg, err := bootstrap.LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Observer.Categories.DriftEnabled {
		t.Error("expected drift disabled by default")
	}
	if cfg.Observer.Categories.DriftKLThreshold != 0.15 {
		t.Errorf("expected KL threshold 0.15, got %f", cfg.Observer.Categories.DriftKLThreshold)
	}
}
```

**Step 3: Wire drift category in app.go**

Update `buildCategories`:

```go
import (
	// ... existing imports ...
	driftcategory "github.com/jonesrussell/north-cloud/ai-observer/internal/category/drift"
)

func buildCategories(cfg Config, esClient *es.Client) []category.Category {
	const maxCategories = 2 // v0.2: classifier + drift
	cats := make([]category.Category, 0, maxCategories)
	if cfg.Observer.Categories.ClassifierEnabled {
		cats = append(cats, classifiercategory.New(
			esClient,
			cfg.Observer.Categories.ClassifierMaxEvents,
			cfg.Observer.Categories.ClassifierModel,
		))
	}
	if cfg.Observer.Categories.DriftEnabled {
		cats = append(cats, driftcategory.New(esClient, driftcategory.Config{
			KLThreshold:        cfg.Observer.Categories.DriftKLThreshold,
			PSIThreshold:       cfg.Observer.Categories.DriftPSIThreshold,
			MatrixThreshold:    cfg.Observer.Categories.DriftMatrixThreshold,
			BaselineWindowDays: cfg.Observer.Categories.DriftBaselineWindowDays,
		}))
	}
	return cats
}
```

Add baseline index mapping to `Start()`, after the `ai_insights` mapping:

```go
if err = driftpkg.EnsureBaselineMapping(ctx, esClient); err != nil {
    return fmt.Errorf("drift_baselines mapping: %w", err)
}
log.Info("drift_baselines index mapping ready")
```

**Step 4: Run tests**

Run: `cd ai-observer && GOWORK=off go test ./... -v`
Expected: ALL PASS

**Step 5: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`
Expected: No errors

**Step 6: Commit**

```bash
git add ai-observer/internal/bootstrap/config.go ai-observer/internal/bootstrap/config_test.go ai-observer/internal/bootstrap/app.go
git commit -m "feat(drift): wire drift category into bootstrap and config"
```

---

### Task 9: Dual-Ticker Scheduler

**Files:**
- Modify: `ai-observer/internal/scheduler/scheduler.go`
- Modify: `ai-observer/internal/scheduler/scheduler_test.go`

Add a second ticker for drift categories that runs on a different interval.

**Step 1: Write failing test for dual-ticker**

Add to `scheduler_test.go`:

```go
func TestScheduler_DualTicker_Config(t *testing.T) {
	t.Helper()
	cfg := scheduler.Config{
		IntervalSeconds:      30,
		MaxTokensPerInterval: 1000,
		WindowDuration:       time.Hour,
		DryRun:               false,
		DriftIntervalSeconds: 21600,
		DriftWindowDuration:  6 * time.Hour,
	}
	s := scheduler.New(nil, nil, nil, nil, cfg)
	// Should not panic with nil categories
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	s.RunOnce(ctx)
}
```

**Step 2: Run test to verify it fails**

Run: `cd ai-observer && GOWORK=off go test ./internal/scheduler/ -v -run TestScheduler_DualTicker`
Expected: FAIL — `DriftIntervalSeconds` field doesn't exist

**Step 3: Update scheduler for dual-ticker**

Update `Config`:

```go
type Config struct {
	IntervalSeconds      int
	MaxTokensPerInterval int
	WindowDuration       time.Duration
	DryRun               bool
	// Drift-specific config
	DriftIntervalSeconds int
	DriftWindowDuration  time.Duration
}
```

Update `Scheduler` to accept two category slices:

```go
type Scheduler struct {
	fastCategories  []category.Category // 30-min ticker
	slowCategories  []category.Category // 6h drift ticker
	writer          *insights.Writer
	provider        provider.LLMProvider
	cfg             Config
	log             logger.Logger
}

func New(
	fastCategories []category.Category,
	slowCategories []category.Category,
	writer *insights.Writer,
	p provider.LLMProvider,
	cfg Config,
) *Scheduler {
	return &Scheduler{
		fastCategories: fastCategories,
		slowCategories: slowCategories,
		writer:         writer,
		provider:       p,
		cfg:            cfg,
	}
}
```

Update `Run` for dual tickers:

```go
func (s *Scheduler) Run(ctx context.Context) {
	fastInterval := time.Duration(s.cfg.IntervalSeconds) * time.Second
	fastTicker := time.NewTicker(fastInterval)
	defer fastTicker.Stop()

	s.logInfo("Scheduler started",
		logger.Int("fast_interval_seconds", s.cfg.IntervalSeconds),
		logger.Int("slow_interval_seconds", s.cfg.DriftIntervalSeconds),
	)
	s.RunOnce(ctx) // run fast categories immediately

	if s.cfg.DriftIntervalSeconds <= 0 || len(s.slowCategories) == 0 {
		// No drift ticker needed — just run the fast loop.
		for {
			select {
			case <-ctx.Done():
				s.logInfo("Scheduler stopping")
				return
			case <-fastTicker.C:
				s.RunOnce(ctx)
			}
		}
	}

	slowInterval := time.Duration(s.cfg.DriftIntervalSeconds) * time.Second
	slowTicker := time.NewTicker(slowInterval)
	defer slowTicker.Stop()

	s.RunDrift(ctx) // run drift immediately on start

	for {
		select {
		case <-ctx.Done():
			s.logInfo("Scheduler stopping")
			return
		case <-fastTicker.C:
			s.RunOnce(ctx)
		case <-slowTicker.C:
			s.RunDrift(ctx)
		}
	}
}
```

Add `RunDrift`:

```go
// RunDrift executes one drift polling cycle.
func (s *Scheduler) RunDrift(ctx context.Context) {
	s.runCategories(ctx, s.slowCategories, s.cfg.DriftWindowDuration)
}

// RunOnce executes one fast polling cycle.
func (s *Scheduler) RunOnce(ctx context.Context) {
	s.runCategories(ctx, s.fastCategories, s.cfg.WindowDuration)
}

func (s *Scheduler) runCategories(ctx context.Context, cats []category.Category, window time.Duration) {
	if len(cats) == 0 {
		return
	}

	budget := NewBudget(s.cfg.MaxTokensPerInterval)
	results := make(chan categoryResult, len(cats))

	var wg sync.WaitGroup
	for _, cat := range cats {
		wg.Add(1)
		go func(c category.Category) {
			defer wg.Done()
			ins, err := s.runCategory(ctx, c, budget, window)
			results <- categoryResult{insights: ins, err: err}
		}(cat)
	}

	wg.Wait()
	close(results)

	allInsights := s.collectInsights(results)
	s.writeInsights(ctx, allInsights)
}
```

Update `runCategory` to accept window:

```go
func (s *Scheduler) runCategory(ctx context.Context, cat category.Category, budget *Budget, window time.Duration) ([]category.Insight, error) {
	if s.cfg.DryRun {
		s.logInfo("Dry run: skipping category", logger.String("category", cat.Name()))
		return nil, nil
	}

	catCtx, cancel := context.WithTimeout(ctx, categoryTimeout)
	defer cancel()

	events, err := cat.Sample(catCtx, window)
	if err != nil {
		return nil, err
	}

	if len(events) == 0 {
		return nil, nil
	}

	estimatedTokens := len(events) * tokensPerEvent
	if !budget.Deduct(estimatedTokens) {
		s.logInfo("budget_exceeded",
			logger.String("category", cat.Name()),
			logger.Int("estimated_tokens", estimatedTokens),
		)
		return nil, nil
	}

	return cat.Analyze(catCtx, events, s.provider)
}
```

**Step 4: Update `app.go` to pass separate category slices**

In `Start()`, split categories:

```go
fastCats, slowCats := buildCategories(cfg, esClient)

sched := scheduler.New(fastCats, slowCats, writer, p, scheduler.Config{
    IntervalSeconds:      cfg.Observer.IntervalSeconds,
    MaxTokensPerInterval: cfg.Observer.MaxTokensPerInterval,
    WindowDuration:       time.Hour,
    DryRun:               cfg.Observer.DryRun,
    DriftIntervalSeconds: cfg.Observer.Categories.DriftIntervalSeconds,
    DriftWindowDuration:  6 * time.Hour,
}).WithLogger(log)
```

Update `buildCategories` to return two slices:

```go
func buildCategories(cfg Config, esClient *es.Client) (fast []category.Category, slow []category.Category) {
	if cfg.Observer.Categories.ClassifierEnabled {
		fast = append(fast, classifiercategory.New(
			esClient,
			cfg.Observer.Categories.ClassifierMaxEvents,
			cfg.Observer.Categories.ClassifierModel,
		))
	}
	if cfg.Observer.Categories.DriftEnabled {
		slow = append(slow, driftcategory.New(esClient, driftcategory.Config{
			KLThreshold:        cfg.Observer.Categories.DriftKLThreshold,
			PSIThreshold:       cfg.Observer.Categories.DriftPSIThreshold,
			MatrixThreshold:    cfg.Observer.Categories.DriftMatrixThreshold,
			BaselineWindowDays: cfg.Observer.Categories.DriftBaselineWindowDays,
		}))
	}
	return fast, slow
}
```

**Step 5: Update existing scheduler tests to use new constructor**

Update all `scheduler.New(cats, ...)` calls to `scheduler.New(cats, nil, ...)`.

**Step 6: Run all tests**

Run: `cd ai-observer && GOWORK=off go test ./... -v`
Expected: ALL PASS

**Step 7: Lint**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`
Expected: No errors

**Step 8: Commit**

```bash
git add ai-observer/internal/scheduler/ ai-observer/internal/bootstrap/
git commit -m "feat(drift): add dual-ticker scheduler for fast and slow categories"
```

---

### Task 10: Docker Compose and Environment Config

**Files:**
- Modify: `docker-compose.base.yml` (add drift env vars to ai-observer)
- Modify: `docker-compose.dev.yml` (if ai-observer has dev overrides)

**Step 1: Add drift env vars to ai-observer service**

In the ai-observer service definition, add:

```yaml
- AI_OBSERVER_DRIFT_ENABLED=${AI_OBSERVER_DRIFT_ENABLED:-false}
- AI_OBSERVER_DRIFT_INTERVAL_SECONDS=${AI_OBSERVER_DRIFT_INTERVAL_SECONDS:-21600}
- AI_OBSERVER_DRIFT_KL_THRESHOLD=${AI_OBSERVER_DRIFT_KL_THRESHOLD:-0.15}
- AI_OBSERVER_DRIFT_PSI_THRESHOLD=${AI_OBSERVER_DRIFT_PSI_THRESHOLD:-0.25}
- AI_OBSERVER_DRIFT_MATRIX_THRESHOLD=${AI_OBSERVER_DRIFT_MATRIX_THRESHOLD:-0.20}
- AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS=${AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS:-7}
- AI_OBSERVER_DRIFT_BASELINE_RETENTION=${AI_OBSERVER_DRIFT_BASELINE_RETENTION:-30}
```

**Step 2: Verify compose parses**

Run: `docker compose -f docker-compose.base.yml -f docker-compose.dev.yml config --services`
Expected: Lists services including ai-observer, no parse errors

**Step 3: Commit**

```bash
git add docker-compose.base.yml
git commit -m "feat(drift): add drift governor env vars to docker compose"
```

---

### Task 11: GitHub Actions Drift Remediation Workflow

**Files:**
- Create: `.github/workflows/drift-remediation.yml`

This workflow runs every 6 hours (offset from the observer), queries ES for unprocessed drift reports, and creates GitHub issues + draft PRs.

**Step 1: Write the workflow**

```yaml
# .github/workflows/drift-remediation.yml
name: Drift Remediation

on:
  schedule:
    # Run every 6 hours, offset 30 min from the observer's 6h tick
    - cron: '30 0,6,12,18 * * *'
  workflow_dispatch: # Allow manual triggering

permissions:
  contents: write
  issues: write
  pull-requests: write

jobs:
  check-drift-reports:
    runs-on: ubuntu-latest
    outputs:
      has_reports: ${{ steps.query.outputs.has_reports }}
      reports: ${{ steps.query.outputs.reports }}
    steps:
      - name: Query ES for unprocessed drift reports
        id: query
        env:
          ES_URL: ${{ secrets.PROD_ES_URL }}
          ES_USERNAME: ${{ secrets.PROD_ES_USERNAME }}
          ES_PASSWORD: ${{ secrets.PROD_ES_PASSWORD }}
        run: |
          RESPONSE=$(curl -s -u "$ES_USERNAME:$ES_PASSWORD" \
            "$ES_URL/ai_insights/_search" \
            -H 'Content-Type: application/json' \
            -d '{
              "query": {
                "bool": {
                  "must": [
                    {"term": {"category": "drift"}},
                    {"terms": {"severity": ["medium", "high"]}}
                  ],
                  "must_not": [
                    {"exists": {"field": "details.processed_at"}}
                  ]
                }
              },
              "size": 10,
              "sort": [{"created_at": {"order": "desc"}}]
            }')

          COUNT=$(echo "$RESPONSE" | jq '.hits.total.value // 0')
          echo "has_reports=$([ "$COUNT" -gt 0 ] && echo true || echo false)" >> "$GITHUB_OUTPUT"
          echo "reports=$(echo "$RESPONSE" | jq -c '.hits.hits')" >> "$GITHUB_OUTPUT"

  create-issues:
    needs: check-drift-reports
    if: needs.check-drift-reports.outputs.has_reports == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Create issues and draft PRs
        env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          REPORTS: ${{ needs.check-drift-reports.outputs.reports }}
          ES_URL: ${{ secrets.PROD_ES_URL }}
          ES_USERNAME: ${{ secrets.PROD_ES_USERNAME }}
          ES_PASSWORD: ${{ secrets.PROD_ES_PASSWORD }}
        run: |
          echo "$REPORTS" | jq -c '.[]' | while read -r hit; do
            SOURCE=$(echo "$hit" | jq -r '._source')
            DOC_ID=$(echo "$hit" | jq -r '._id')
            SEVERITY=$(echo "$SOURCE" | jq -r '.severity')
            SUMMARY=$(echo "$SOURCE" | jq -r '.summary')
            ISSUE_TITLE=$(echo "$SOURCE" | jq -r '.details.issue_title // .summary')
            ACTIONS=$(echo "$SOURCE" | jq -r '.suggested_actions[]? // empty' | sed 's/^/- /')
            ACTION_TYPE=$(echo "$SOURCE" | jq -r '.details.action_type // "none"')

            # Create issue
            ISSUE_BODY="## Classifier Drift Detected

          **Severity:** $SEVERITY
          **Summary:** $SUMMARY

          ### Suggested Actions
          $ACTIONS

          ---
          *Auto-generated by drift governor*"

            gh issue create \
              --title "$ISSUE_TITLE" \
              --body "$ISSUE_BODY" \
              --label "classifier-drift"

            # Mark as processed in ES
            curl -s -u "$ES_USERNAME:$ES_PASSWORD" \
              "$ES_URL/ai_insights/_update/$DOC_ID" \
              -H 'Content-Type: application/json' \
              -d "{\"doc\": {\"details\": {\"processed_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}}}"
          done
```

Note: Draft PR generation for rule patches will be added in a later phase once the drift governor is validated in production. The workflow currently creates issues only.

**Step 2: Verify workflow syntax**

Run: `cd /home/fsd42/dev/north-cloud && gh workflow list` (to confirm gh CLI works)

**Step 3: Commit**

```bash
git add .github/workflows/drift-remediation.yml
git commit -m "feat(drift): add GitHub Actions drift remediation workflow"
```

---

### Task 12: Update CLAUDE.md and Observer Docs

**Files:**
- Modify: `ai-observer/CLAUDE.md`
- Modify: `CLAUDE.md` (root)

**Step 1: Update ai-observer CLAUDE.md**

Add to the Config table:

```
| `AI_OBSERVER_DRIFT_ENABLED` | `false` | Enable drift governor |
| `AI_OBSERVER_DRIFT_INTERVAL_SECONDS` | `21600` | Drift check interval (6h) |
| `AI_OBSERVER_DRIFT_KL_THRESHOLD` | `0.15` | KL divergence alert threshold |
| `AI_OBSERVER_DRIFT_PSI_THRESHOLD` | `0.25` | PSI alert threshold |
| `AI_OBSERVER_DRIFT_MATRIX_THRESHOLD` | `0.20` | Cross-matrix deviation threshold |
| `AI_OBSERVER_DRIFT_BASELINE_WINDOW_DAYS` | `7` | Rolling baseline window |
| `AI_OBSERVER_DRIFT_BASELINE_RETENTION` | `30` | Baselines to retain |
```

Add to Architecture section:

```
    ├── drift/               # Statistical drift metrics, baseline sampler, evaluator
```

Add to Key Design Decisions:

```
- **Dual-ticker**: Fast (30 min) for LLM-based classifier analysis, slow (6h) for statistical drift detection
- **Statistical first**: KL, PSI, cross-matrix computed without LLM. LLM only invoked on breach for context.
- **Advisory + draft PRs**: Governor proposes changes via GitHub Actions, never auto-merges
```

**Step 2: Update root CLAUDE.md**

Add to Content Pipeline Layers after Publisher routing:

```
**Drift Governor** (within ai-observer, 6h ticker):
- Computes KL divergence, PSI, cross-matrix stability against rolling 7-day baseline
- On threshold breach → LLM analysis → GitHub issue + draft PR with rule patches
- Config: `AI_OBSERVER_DRIFT_ENABLED`, thresholds configurable per-metric
```

**Step 3: Commit**

```bash
git add ai-observer/CLAUDE.md CLAUDE.md
git commit -m "docs: update CLAUDE.md with drift governor documentation"
```

---

### Task 13: Final Integration Test

Run the full test and lint suite to verify everything works together.

**Step 1: Run all ai-observer tests**

Run: `cd ai-observer && GOWORK=off go test ./... -v -count=1`
Expected: ALL PASS

**Step 2: Run linter**

Run: `cd ai-observer && GOWORK=off golangci-lint run --config ../.golangci.yml ./...`
Expected: No errors

**Step 3: Run full repo lint (changed services)**

Run: `task lint:force`
Expected: No errors

**Step 4: Verify build**

Run: `cd ai-observer && GOWORK=off go build ./...`
Expected: Build succeeds

**Step 5: Smoke test (dry-run)**

Run: `cd ai-observer && AI_OBSERVER_ENABLED=true AI_OBSERVER_DRY_RUN=true AI_OBSERVER_DRIFT_ENABLED=true ANTHROPIC_API_KEY=dummy GOWORK=off go run .`
Expected: Logs startup, drift category registered, dry-run mode active. Ctrl+C to stop.
