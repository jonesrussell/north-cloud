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
