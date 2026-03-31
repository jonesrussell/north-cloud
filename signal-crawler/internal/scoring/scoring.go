package scoring

import "strings"

type keyword struct {
	phrase string
	score  int
}

var keywords = []keyword{
	// Direct ask — score 90
	{phrase: "looking for cto", score: 90},
	{phrase: "looking for a cto", score: 90},
	{phrase: "need developer", score: 90},
	{phrase: "need a developer", score: 90},
	{phrase: "need an engineer", score: 90},
	{phrase: "hiring first engineer", score: 90},
	{phrase: "hiring our first", score: 90},
	{phrase: "technical co-founder", score: 90},

	// Strong signal — score 70
	{phrase: "rebuild mvp", score: 70},
	{phrase: "rewriting our stack", score: 70},
	{phrase: "migrating to cloud", score: 70},
	{phrase: "scaling infrastructure", score: 70},
	{phrase: "rewrite from scratch", score: 70},
	{phrase: "modernize our", score: 70},
	{phrase: "platform migration", score: 70},
	{phrase: "moving to microservices", score: 70},

	// Weak signal — score 40
	{phrase: "considering rewrite", score: 40},
	{phrase: "evaluating platforms", score: 40},
	{phrase: "tech debt", score: 40},
	{phrase: "technical debt", score: 40},
	{phrase: "legacy system", score: 40},
	{phrase: "need to modernize", score: 40},
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
