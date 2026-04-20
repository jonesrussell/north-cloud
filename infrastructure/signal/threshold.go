// Package signal defines the shared threshold contract that gates need-signal
// detection across the North Cloud data plane.
//
// Both signal-crawler's scoring gate and the classifier's
// content_type_need_signal heuristic delegate here. The contract is specified
// in docs/specs/lead-pipeline.md — any change must update the spec in the
// same PR.
package signal

import "strings"

const (
	// MinKeywordMatches is the number of distinct keyword substrings that
	// must appear in the combined title+body before a signal qualifies.
	MinKeywordMatches = 2

	// RequiredConfidence is the confidence reported when the threshold is
	// met. Downstream consumers treat anything below this as rejected.
	RequiredConfidence = 0.80
)

// Evaluate counts how many keyword phrases occur in lowerText (which the
// caller must lowercase) and reports whether the unified threshold is met.
// It short-circuits as soon as MinKeywordMatches is reached so long keyword
// lists do not pay for extra work.
//
// Returns (true, RequiredConfidence, n) when n >= MinKeywordMatches,
// otherwise (false, 0, n) where n is the partial count.
func Evaluate(lowerText string, keywords []string) (ok bool, confidence float64, matches int) {
	for _, kw := range keywords {
		if strings.Contains(lowerText, kw) {
			matches++
			if matches >= MinKeywordMatches {
				return true, RequiredConfidence, matches
			}
		}
	}
	return false, 0, matches
}
