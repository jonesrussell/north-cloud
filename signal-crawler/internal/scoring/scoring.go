package scoring

import "strings"

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

	// Strong signal — score 70
	{phrase: "rebuild mvp", score: ScoreStrongSignal},
	{phrase: "rewriting our stack", score: ScoreStrongSignal},
	{phrase: "migrating to cloud", score: ScoreStrongSignal},
	{phrase: "scaling infrastructure", score: ScoreStrongSignal},
	{phrase: "rewrite from scratch", score: ScoreStrongSignal},
	{phrase: "modernize our", score: ScoreStrongSignal},
	{phrase: "platform migration", score: ScoreStrongSignal},
	{phrase: "moving to microservices", score: ScoreStrongSignal},

	// Weak signal — score 40
	{phrase: "considering rewrite", score: ScoreWeakSignal},
	{phrase: "evaluating platforms", score: ScoreWeakSignal},
	{phrase: "tech debt", score: ScoreWeakSignal},
	{phrase: "technical debt", score: ScoreWeakSignal},
	{phrase: "legacy system", score: ScoreWeakSignal},
	{phrase: "need to modernize", score: ScoreWeakSignal},
}

// Score returns the highest matching keyword score and the matched phrase.
// Returns (0, "") if no keyword matches.
func Score(text string) (int, string) {
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
