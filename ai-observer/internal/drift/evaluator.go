package drift

// Evaluate compares current distributions against a baseline and returns all drift signals.
// Each metric/scope combination produces one signal, regardless of whether it breached.
func Evaluate(baseline, current *Baseline, thresholds Thresholds) []DriftSignal {
	if baseline == nil || current == nil {
		return nil
	}

	klSignals := evaluateKL(baseline, current, thresholds.KLDivergence)
	psiSignals := evaluatePSI(baseline, current, thresholds.PSI)
	matrixSignals := evaluateMatrix(baseline, current, thresholds.MatrixDeviation)

	signals := make([]DriftSignal, 0, len(klSignals)+len(psiSignals)+len(matrixSignals))
	signals = append(signals, klSignals...)
	signals = append(signals, psiSignals...)
	signals = append(signals, matrixSignals...)

	return signals
}

func evaluateKL(baseline, current *Baseline, threshold float64) []DriftSignal {
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
