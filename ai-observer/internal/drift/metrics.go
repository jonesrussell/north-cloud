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

	keys := allKeys(baseline, current)

	var kl float64
	for _, k := range keys {
		p := baseline[k] + smoothingEpsilon
		q := current[k] + smoothingEpsilon
		kl += p * math.Log(p/q)
	}

	return kl
}

// allKeys returns the union of keys from two maps.
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
