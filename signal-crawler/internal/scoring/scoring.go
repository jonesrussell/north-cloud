package scoring

import (
	"strings"

	"github.com/jonesrussell/north-cloud/infrastructure/signal"
)

const (
	// ScoreDirectAsk is for posts explicitly looking for technical help.
	ScoreDirectAsk = 90
	// ScoreStrongSignal is for posts indicating active technical need.
	ScoreStrongSignal = 70
	// ScoreWeakSignal is for posts hinting at future technical need.
	ScoreWeakSignal = 40
)

type keyword struct {
	phrase string
	score  int
}

var keywords = []keyword{
	// Direct ask — score 90
	{phrase: "looking for cto", score: ScoreDirectAsk},
	{phrase: "looking for a cto", score: ScoreDirectAsk},
	{phrase: "need developer", score: ScoreDirectAsk},
	{phrase: "need a developer", score: ScoreDirectAsk},
	{phrase: "need an engineer", score: ScoreDirectAsk},
	{phrase: "hiring first engineer", score: ScoreDirectAsk},
	{phrase: "hiring our first", score: ScoreDirectAsk},
	{phrase: "technical co-founder", score: ScoreDirectAsk},
	// Job board — direct ask
	{phrase: "hiring platform engineer", score: ScoreDirectAsk},
	{phrase: "need cloud architect", score: ScoreDirectAsk},
	{phrase: "looking for devops", score: ScoreDirectAsk},

	// Strong signal — score 70
	{phrase: "rebuild mvp", score: ScoreStrongSignal},
	{phrase: "rewriting our stack", score: ScoreStrongSignal},
	{phrase: "migrating to cloud", score: ScoreStrongSignal},
	{phrase: "scaling infrastructure", score: ScoreStrongSignal},
	{phrase: "rewrite from scratch", score: ScoreStrongSignal},
	{phrase: "modernize our", score: ScoreStrongSignal},
	{phrase: "platform migration", score: ScoreStrongSignal},
	{phrase: "moving to microservices", score: ScoreStrongSignal},
	// Job board — strong signal
	{phrase: "monolith to microservices", score: ScoreStrongSignal},
	{phrase: "cloud migration", score: ScoreStrongSignal},
	{phrase: "infrastructure overhaul", score: ScoreStrongSignal},
	{phrase: "platform modernization", score: ScoreStrongSignal},

	// Weak signal — score 40
	{phrase: "considering rewrite", score: ScoreWeakSignal},
	{phrase: "evaluating platforms", score: ScoreWeakSignal},
	{phrase: "tech debt", score: ScoreWeakSignal},
	{phrase: "technical debt", score: ScoreWeakSignal},
	{phrase: "legacy system", score: ScoreWeakSignal},
	{phrase: "need to modernize", score: ScoreWeakSignal},
	// Job board — weak signal
	{phrase: "scaling challenges", score: ScoreWeakSignal},
	{phrase: "growing engineering team", score: ScoreWeakSignal},
	{phrase: "modernizing stack", score: ScoreWeakSignal},
}

// Score returns the highest matching keyword score and the matched phrase.
// Returns (0, "") if no keyword matches.
func Score(text string) (score int, phrase string) {
	lower := strings.ToLower(text)
	best := 0
	matched := ""
	for _, kw := range keywords {
		if strings.Contains(lower, kw.phrase) && kw.score > best {
			best = kw.score
			matched = kw.phrase
		}
	}
	return best, matched
}

// Phrases returns a copy of the keyword phrase list used by Score and Passes.
func Phrases() []string {
	phrases := make([]string, 0, len(keywords))
	for _, kw := range keywords {
		phrases = append(phrases, kw.phrase)
	}
	return phrases
}

// MatchCount returns the number of distinct configured keyword phrases found
// in text. It is intended for diagnostics where exact recall counts matter.
func MatchCount(text string) int {
	return len(MatchedPhrases(text))
}

// MatchedPhrases returns the configured keyword phrases found in text.
func MatchedPhrases(text string) []string {
	lower := strings.ToLower(text)
	matches := make([]string, 0)
	for _, kw := range keywords {
		if strings.Contains(lower, kw.phrase) {
			matches = append(matches, kw.phrase)
		}
	}
	return matches
}

// PassesAt reports whether text meets a diagnostic keyword threshold. Runtime
// adapter gating should continue to call Passes so it stays tied to the shared
// infrastructure/signal contract.
func PassesAt(text string, minKeywordMatches int) (ok bool, confidence float64, matches int) {
	if minKeywordMatches <= 0 {
		minKeywordMatches = 1
	}
	matches = MatchCount(text)
	if matches >= minKeywordMatches {
		return true, signal.RequiredConfidence, matches
	}
	return false, 0, matches
}

// Passes reports whether text meets the unified threshold contract defined in
// infrastructure/signal (≥MinKeywordMatches distinct keyword hits, confidence
// ≥RequiredConfidence). The shared helper keeps this service in lock-step
// with the classifier's need_signal heuristic — see docs/specs/lead-pipeline.md.
func Passes(text string) (ok bool, confidence float64, matches int) {
	return signal.Evaluate(strings.ToLower(text), Phrases())
}
