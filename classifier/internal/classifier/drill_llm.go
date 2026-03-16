package classifier

import (
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// DrillExtractor is the interface for LLM-based drill extraction.
type DrillExtractor interface {
	Extract(body string) ([]domain.DrillResult, error)
}

// orchestrateDrillExtraction runs the two-stage extraction pipeline:
// 1. Regex first pass
// 2. LLM fallback if needed and enabled
// Returns normalized results and the extraction method used.
func orchestrateDrillExtraction(
	body string,
	drillKeywordMatched bool,
	llmFallbackEnabled bool,
	llmClient DrillExtractor,
) ([]domain.DrillResult, string) {
	// Stage 1: Regex extraction
	regexResults, confidence := extractDrillRegex(body)

	switch confidence {
	case drillConfidenceComplete:
		// Regex got complete results — no LLM needed
		return normalizeDrillResults(regexResults), "regex"

	case drillConfidencePartial:
		if !llmFallbackEnabled || llmClient == nil {
			// Return what regex found
			return normalizeDrillResults(regexResults), "regex"
		}
		// Try LLM to fill in gaps
		llmResults, err := llmClient.Extract(body)
		if err != nil || len(llmResults) == 0 {
			// LLM failed — fall back to regex results
			return normalizeDrillResults(regexResults), "regex"
		}
		// Merge: LLM results take precedence, then add unique regex results
		merged := mergeDrillResults(llmResults, regexResults)
		return normalizeDrillResults(merged), "hybrid"

	case drillConfidenceNone:
		if !drillKeywordMatched || !llmFallbackEnabled || llmClient == nil {
			return nil, ""
		}
		// Drill keywords matched but regex found nothing — try LLM
		llmResults, err := llmClient.Extract(body)
		if err != nil || len(llmResults) == 0 {
			return nil, ""
		}
		return normalizeDrillResults(llmResults), "llm"
	}

	return nil, ""
}

// mergeDrillResults merges primary results with secondary, deduplicating by hole_id.
// Primary results take precedence for matching hole IDs.
// Results with empty HoleID (e.g. sub-intervals) are always included from both sets;
// normalizeDrillResults handles final dedup using a composite hole_id+intercept+grade key.
func mergeDrillResults(primary, secondary []domain.DrillResult) []domain.DrillResult {
	seen := make(map[string]bool)
	var merged []domain.DrillResult

	for _, r := range primary {
		key := r.HoleID
		if key != "" {
			seen[key] = true
		}
		merged = append(merged, r)
	}

	for _, r := range secondary {
		if r.HoleID != "" && seen[r.HoleID] {
			continue // already have results for this hole from primary
		}
		merged = append(merged, r)
	}

	return merged
}
