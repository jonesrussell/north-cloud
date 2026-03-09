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
