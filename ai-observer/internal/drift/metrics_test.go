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
	baselineCounts := map[string]map[string]int{
		"north_america": {"technology": 3},
	}

	deviations := drift.CrossMatrixDeviation(baseline, current, baselineCounts)
	if len(deviations) != 0 {
		t.Errorf("expected sparse cells to be skipped, got %d deviations", len(deviations))
	}
}
