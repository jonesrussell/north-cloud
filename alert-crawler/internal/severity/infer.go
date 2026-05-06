package severity

import (
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// FloorSeverity is the minimum severity returned by Infer when no substance
// matches are found in the table.
const FloorSeverity = domain.SeverityMedium

// Infer derives a Severity for hazard by looking up every substance name
// (from HarmReduction.Substances and HarmReduction.Composition[].Name)
// in table and returning the highest-ranked match.
//
// Rules:
//   - If hazard.HarmReduction is nil, FloorSeverity is returned immediately.
//   - If no substance matches any table entry, FloorSeverity is returned.
//   - If one or more substances match, the highest-ranked severity wins.
func Infer(hazard domain.Hazard, table Table) domain.Severity {
	if hazard.HarmReduction == nil {
		return FloorSeverity
	}

	best := FloorSeverity
	bestRank := severityRank(best)

	best, bestRank = checkSubstances(hazard.HarmReduction.Substances, table, best, bestRank)
	best, _ = checkComposition(hazard.HarmReduction.Composition, table, best, bestRank)

	return best
}

// checkSubstances iterates over plain substance name strings, updating best/bestRank
// when a higher-ranked match is found.
func checkSubstances(substances []string, table Table, best domain.Severity, bestRank int) (outBest domain.Severity, outRank int) {
	outBest = best
	outRank = bestRank

	for _, name := range substances {
		if s, ok := table.Lookup(name); ok {
			if r := severityRank(s); r > outRank {
				outBest = s
				outRank = r
			}
		}
	}

	return outBest, outRank
}

// checkComposition iterates over Substance structs, updating best/bestRank
// when a higher-ranked match is found via the Name field.
func checkComposition(composition []domain.Substance, table Table, best domain.Severity, bestRank int) (outBest domain.Severity, outRank int) {
	outBest = best
	outRank = bestRank

	for _, sub := range composition {
		if s, ok := table.Lookup(sub.Name); ok {
			if r := severityRank(s); r > outRank {
				outBest = s
				outRank = r
			}
		}
	}

	return outBest, outRank
}
