package classifier

import (
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// recipeKeywords are phrases whose presence (case-insensitive) strongly
// indicates that the page contains a recipe. Requiring 2+ matches avoids
// false positives from pages that incidentally mention one cooking term.
var recipeKeywords = []string{
	"ingredients",
	"instructions",
	"prep time",
	"cook time",
	"servings",
	"preheat",
	"bake",
	"simmer",
	"garnish",
	"recipe",
}

// ingredientQuantityPattern matches common ingredient measurements like
// "2 cups", "1 tbsp", "250 ml", "500 g". A match counts as one keyword hit.
var ingredientQuantityPattern = regexp.MustCompile(
	`(?i)\b\d+\s?(?:cup|cups|tbsp|tablespoon|tablespoons|tsp|teaspoon|teaspoons|ml|g|grams|kg|oz|ounce|ounces)\b`,
)

// classifyFromRecipeKeywords checks title + raw_text for recipe-related
// keywords and ingredient quantity patterns. Returns ContentTypeRecipe
// with confidence 0.80 when at least 2 signals are found.
// Returns nil if no recipe signal is detected.
func (c *ContentTypeClassifier) classifyFromRecipeKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	matches := 0

	// Check keyword matches
	for _, kw := range recipeKeywords {
		if strings.Contains(combinedText, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			return c.recipeResult(raw, matches)
		}
	}

	// Check ingredient quantity pattern (counts as one match)
	if ingredientQuantityPattern.MatchString(combinedText) {
		matches++
	}

	if matches >= minKeywordMatches {
		return c.recipeResult(raw, matches)
	}

	return nil
}

// recipeResult builds the ContentTypeResult for a recipe detection.
func (c *ContentTypeClassifier) recipeResult(
	raw *domain.RawContent, matches int,
) *ContentTypeResult {
	c.logger.Debug("Recipe detected via keyword heuristic",
		infralogger.String("content_id", raw.ID),
		infralogger.Int("keyword_matches", matches),
	)
	return &ContentTypeResult{
		Type:       domain.ContentTypeRecipe,
		Confidence: keywordHeuristicConfidence,
		Method:     "keyword_heuristic",
		Reason:     "Recipe keywords detected in content",
	}
}
