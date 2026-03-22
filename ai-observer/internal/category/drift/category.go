// Package drift implements the drift detection category for the AI observer.
package drift

import (
	"context"
	"errors"
	"fmt"
	"time"

	es "github.com/elastic/go-elasticsearch/v8"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/category"
	driftpkg "github.com/jonesrussell/north-cloud/ai-observer/internal/drift"
	"github.com/jonesrussell/north-cloud/ai-observer/internal/provider"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const (
	categoryName    = "drift"
	modelTier       = "haiku"
	maxEventsPerRun = 0 // drift doesn't use event-based sampling
)

// Config holds drift category configuration.
type Config struct {
	KLThreshold        float64
	PSIThreshold       float64
	MatrixThreshold    float64
	BaselineWindowDays int
}

// Category implements the category.Category interface for drift detection.
type Category struct {
	collector    *driftpkg.Collector
	store        *driftpkg.Store
	thresholds   driftpkg.Thresholds
	baselineDays int
	log          infralogger.Logger
}

// New creates a new drift category.
func New(esClient *es.Client, cfg Config, log infralogger.Logger) *Category {
	return &Category{
		collector: driftpkg.NewCollector(esClient),
		store:     driftpkg.NewStore(esClient),
		thresholds: driftpkg.Thresholds{
			KLDivergence:    cfg.KLThreshold,
			PSI:             cfg.PSIThreshold,
			MatrixDeviation: cfg.MatrixThreshold,
		},
		baselineDays: cfg.BaselineWindowDays,
		log:          log,
	}
}

// Name returns the category name.
func (c *Category) Name() string { return categoryName }

// MaxEventsPerRun returns zero; drift uses distribution sampling, not event sampling.
func (c *Category) MaxEventsPerRun() int { return maxEventsPerRun }

// ModelTier returns the LLM tier used for drift analysis.
func (c *Category) ModelTier() string { return modelTier }

// Sample collects the current window distribution. Returns a synthetic event carrying
// drift signals in Metadata -- the drift category doesn't use traditional event sampling.
func (c *Category) Sample(ctx context.Context, window time.Duration) ([]category.Event, error) {
	current, err := c.collector.CollectCurrentWindow(ctx, window)
	if err != nil {
		return nil, fmt.Errorf("collect current window: %w", err)
	}
	if current == nil {
		return nil, nil
	}

	baseline, baselineErr := c.store.LoadLatestBaseline(ctx)
	if baselineErr != nil {
		if !errors.Is(baselineErr, driftpkg.ErrNoBaseline) {
			return nil, fmt.Errorf("load baseline: %w", baselineErr)
		}
		// No baseline yet -- compute and store one, skip evaluation.
		return c.initBaseline(ctx)
	}

	// Refresh stale baselines to avoid false-positive drift alerts from natural shifts.
	if c.isBaselineStale(baseline) {
		if c.log != nil {
			c.log.Info("Baseline stale, refreshing",
				infralogger.String("computed_at", baseline.ComputedAt),
				infralogger.Int("window_days", c.baselineDays),
			)
		}
		return c.initBaseline(ctx)
	}

	signals := driftpkg.Evaluate(baseline, current, c.thresholds)

	breached := countBreaches(signals)
	if c.log != nil {
		c.log.Info("Drift evaluation complete",
			infralogger.Int("signal_count", len(signals)),
			infralogger.Int("breach_count", breached),
		)
	}

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

// isBaselineStale returns true if the baseline is older than the configured window.
func (c *Category) isBaselineStale(baseline *driftpkg.Baseline) bool {
	computed, err := time.Parse(time.RFC3339, baseline.ComputedAt)
	if err != nil {
		return true // unparseable timestamp — treat as stale
	}
	age := time.Since(computed)
	return age > time.Duration(c.baselineDays)*24*time.Hour
}

func (c *Category) initBaseline(ctx context.Context) ([]category.Event, error) {
	newBaseline, baselineErr := c.collector.CollectBaselineWindow(ctx, c.baselineDays)
	if baselineErr != nil {
		return nil, fmt.Errorf("compute initial baseline: %w", baselineErr)
	}
	if newBaseline == nil {
		if c.log != nil {
			c.log.Warn("Baseline collection returned nil — no classified docs in window",
				infralogger.Int("window_days", c.baselineDays),
			)
		}
		return nil, nil
	}
	if storeErr := c.store.StoreBaseline(ctx, newBaseline); storeErr != nil {
		return nil, fmt.Errorf("store initial baseline: %w", storeErr)
	}
	if c.log != nil {
		c.log.Info("Baseline stored",
			infralogger.Int("sample_count", newBaseline.SampleCount),
			infralogger.Int("window_days", newBaseline.WindowDays),
		)
	}
	return nil, nil
}

// Analyze processes drift signals. If thresholds are breached, invokes the LLM
// for contextual explanation. Always returns at least one insight with metric values.
func (c *Category) Analyze(
	ctx context.Context,
	events []category.Event,
	p provider.LLMProvider,
) ([]category.Insight, error) {
	if len(events) == 0 {
		return nil, nil
	}

	signals, ok := events[0].Metadata["signals"].([]driftpkg.DriftSignal)
	if !ok {
		return nil, errors.New("unexpected signals type in metadata")
	}

	severity := driftpkg.SeverityFromSignals(signals)

	insight := category.Insight{
		Category: categoryName,
		Severity: severity,
		Summary:  buildSummary(signals),
		Details:  buildDetails(signals),
	}

	if severity == "low" {
		return []category.Insight{insight}, nil
	}

	// Breached -- invoke LLM for contextual analysis.
	if p != nil {
		return []category.Insight{enrichWithLLM(ctx, p, signals, insight)}, nil
	}

	return []category.Insight{insight}, nil
}

func enrichWithLLM(
	ctx context.Context,
	p provider.LLMProvider,
	signals []driftpkg.DriftSignal,
	insight category.Insight,
) category.Insight {
	llmInsight, llmErr := analyzeDrift(ctx, p, signals)
	if llmErr != nil {
		insight.SuggestedActions = []string{"LLM analysis failed: " + llmErr.Error()}
		return insight
	}
	insight.SuggestedActions = llmInsight.SuggestedActions
	insight.TokensUsed = llmInsight.TokensUsed
	insight.Model = llmInsight.Model
	if llmInsight.Summary != "" {
		insight.Summary = llmInsight.Summary
	}
	for k, v := range llmInsight.Details {
		insight.Details[k] = v
	}
	return insight
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
		return "No drift detected -- all metrics within thresholds"
	}
	if breached == 1 {
		return fmt.Sprintf("%s %.3f (threshold %.2f) in %s",
			first.Metric, first.Value, first.Threshold, first.Scope)
	}
	return fmt.Sprintf("%d metrics breached -- %s %.3f in %s (and %d more)",
		breached, first.Metric, first.Value, first.Scope, breached-1)
}

func buildDetails(signals []driftpkg.DriftSignal) map[string]any {
	details := map[string]any{
		"signal_count": len(signals),
		"breach_count": countBreaches(signals),
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
