package classifier

import (
	"context"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/classifier/jsonld"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Extraction method constants.
const (
	extractionMethodSchemaOrg = "schema_org"
	extractionMethodHeuristic = "heuristic"
	recipeTopicName           = "recipe"
	schemaOrgTypeRecipe       = "Recipe"
	instructionSeparator      = " "
)

// Heuristic section header labels (case-insensitive matching).
var ingredientHeaders = []string{"ingredients:"}

var instructionHeaders = []string{
	"instructions:",
	"directions:",
	"method:",
	"steps:",
}

// bulletPrefixes are stripped from ingredient list items during heuristic parsing.
var bulletPrefixes = []string{"- ", "* ", "\u2022 "} // dash, asterisk, bullet

// RecipeExtractor extracts structured recipe data from raw content using
// Schema.org JSON-LD (tier 1) or heuristic text parsing (tier 2).
type RecipeExtractor struct {
	logger infralogger.Logger
}

// NewRecipeExtractor creates a new RecipeExtractor.
func NewRecipeExtractor(logger infralogger.Logger) *RecipeExtractor {
	return &RecipeExtractor{logger: logger}
}

// Extract attempts to extract structured recipe fields from raw content.
// Returns (nil, nil) when content is not a recipe.
func (e *RecipeExtractor) Extract(
	ctx context.Context, raw *domain.RawContent, contentType string, topics []string,
) (*domain.RecipeResult, error) {
	_ = ctx // reserved for future async/tracing use

	isRecipeType := contentType == domain.ContentTypeRecipe
	hasRecipeTopic := containsTopic(topics, recipeTopicName)

	if !isRecipeType && !hasRecipeTopic {
		return nil, nil //nolint:nilnil // Intentional: nil result signals content is not a recipe
	}

	// Tier 1: Schema.org JSON-LD extraction.
	if result := e.extractSchemaOrg(raw.RawHTML); result != nil {
		e.logger.Debug("recipe extracted via Schema.org",
			infralogger.String("content_id", raw.ID),
			infralogger.String("name", result.Name),
		)
		return result, nil
	}

	// Tier 2: Heuristic extraction (only when topic matched).
	if hasRecipeTopic || isRecipeType {
		if result := e.extractHeuristic(raw.RawText); result != nil {
			e.logger.Debug("recipe extracted via heuristic",
				infralogger.String("content_id", raw.ID),
			)
			return result, nil
		}
	}

	return nil, nil //nolint:nilnil // Intentional: nil result signals no recipe data found
}

// extractSchemaOrg tries to extract a Recipe from JSON-LD blocks in HTML.
// Returns nil if no valid Recipe block is found.
func (e *RecipeExtractor) extractSchemaOrg(html string) *domain.RecipeResult {
	blocks := jsonld.Extract(html, nil)
	recipe := jsonld.FindByType(blocks, schemaOrgTypeRecipe)

	if recipe == nil {
		return nil
	}

	result := &domain.RecipeResult{
		ExtractionMethod: extractionMethodSchemaOrg,
		Name:             jsonld.StringVal(recipe, "name"),
		Ingredients:      jsonld.StringSliceVal(recipe, "recipeIngredient"),
		Instructions:     extractInstructions(recipe),
		PrepTimeMinutes:  jsonld.ParseISO8601Duration(jsonld.StringVal(recipe, "prepTime")),
		CookTimeMinutes:  jsonld.ParseISO8601Duration(jsonld.StringVal(recipe, "cookTime")),
		TotalTimeMinutes: jsonld.ParseISO8601Duration(jsonld.StringVal(recipe, "totalTime")),
		Servings:         jsonld.StringVal(recipe, "recipeYield"),
		Category:         jsonld.StringVal(recipe, "recipeCategory"),
		Cuisine:          jsonld.StringVal(recipe, "recipeCuisine"),
		Calories:         jsonld.NestedStringVal(recipe, "nutrition", "calories"),
		ImageURL:         extractImageURL(recipe),
	}

	e.extractRating(recipe, result)

	return result
}

// extractRating populates rating fields from an aggregateRating nested map.
func (e *RecipeExtractor) extractRating(recipe map[string]any, result *domain.RecipeResult) {
	ratingMap, ok := recipe["aggregateRating"].(map[string]any)
	if !ok {
		return
	}

	result.Rating = jsonld.FloatVal(ratingMap, "ratingValue")
	result.RatingCount = jsonld.IntVal(ratingMap, "ratingCount")
}

// extractInstructions handles three Schema.org instruction formats:
//  1. Plain string
//  2. Array of strings
//  3. Array of HowToStep objects with "text" fields
func extractInstructions(recipe map[string]any) string {
	raw, exists := recipe["recipeInstructions"]
	if !exists {
		return ""
	}

	// Format 1: plain string.
	if s, ok := raw.(string); ok {
		return s
	}

	// Format 2 & 3: array (could be strings or HowToStep objects).
	arr, ok := raw.([]any)
	if !ok {
		return ""
	}

	steps := make([]string, 0, len(arr))

	for _, elem := range arr {
		switch v := elem.(type) {
		case string:
			steps = append(steps, v)
		case map[string]any:
			if text := jsonld.StringVal(v, "text"); text != "" {
				steps = append(steps, text)
			}
		}
	}

	return strings.Join(steps, instructionSeparator)
}

// extractImageURL handles image as either a string or an object with a "url" field.
func extractImageURL(recipe map[string]any) string {
	raw, exists := recipe["image"]
	if !exists {
		return ""
	}

	// String format.
	if s, ok := raw.(string); ok {
		return s
	}

	// Object format: {"@type": "ImageObject", "url": "..."}
	if obj, ok := raw.(map[string]any); ok {
		return jsonld.StringVal(obj, "url")
	}

	return ""
}

// extractHeuristic performs text-based recipe extraction by looking for
// section headers such as "Ingredients:" and "Instructions:".
// Returns nil if no recognizable recipe sections are found.
func (e *RecipeExtractor) extractHeuristic(rawText string) *domain.RecipeResult {
	lowerText := strings.ToLower(rawText)

	ingredients := extractIngredientSection(rawText, lowerText)
	instructions := extractInstructionSection(rawText, lowerText)

	if len(ingredients) == 0 && instructions == "" {
		return nil
	}

	return &domain.RecipeResult{
		ExtractionMethod: extractionMethodHeuristic,
		Ingredients:      ingredients,
		Instructions:     instructions,
	}
}

// extractIngredientSection finds and parses the ingredients section from raw text.
func extractIngredientSection(rawText, lowerText string) []string {
	headerIdx := findSectionHeader(lowerText, ingredientHeaders)
	if headerIdx < 0 {
		return nil
	}

	// Find the header line end to start from the next line.
	lineStart := strings.Index(rawText[headerIdx:], "\n")
	if lineStart < 0 {
		return nil
	}

	sectionStart := headerIdx + lineStart + 1
	sectionEnd := findNextSectionEnd(rawText, sectionStart)
	section := rawText[sectionStart:sectionEnd]

	return parseIngredientLines(section)
}

// extractInstructionSection finds and returns the instructions section text.
func extractInstructionSection(rawText, lowerText string) string {
	headerIdx := findSectionHeader(lowerText, instructionHeaders)
	if headerIdx < 0 {
		return ""
	}

	lineStart := strings.Index(rawText[headerIdx:], "\n")
	if lineStart < 0 {
		return ""
	}

	sectionStart := headerIdx + lineStart + 1
	sectionEnd := findNextSectionEnd(rawText, sectionStart)
	section := strings.TrimSpace(rawText[sectionStart:sectionEnd])

	return section
}

// findSectionHeader returns the index of the first matching section header
// in the lowercase text. Returns -1 if no header is found.
func findSectionHeader(lowerText string, headers []string) int {
	for _, header := range headers {
		idx := strings.Index(lowerText, header)
		if idx >= 0 {
			return idx
		}
	}

	return -1
}

// findNextSectionEnd returns the index of the next blank line or end of text,
// signaling the end of the current section.
func findNextSectionEnd(rawText string, start int) int {
	remaining := rawText[start:]
	blankLine := strings.Index(remaining, "\n\n")

	if blankLine >= 0 {
		return start + blankLine
	}

	return len(rawText)
}

// parseIngredientLines splits a section into individual ingredient items,
// stripping common bullet and numbered list prefixes.
func parseIngredientLines(section string) []string {
	lines := strings.Split(section, "\n")
	ingredients := make([]string, 0, len(lines))

	for _, line := range lines {
		cleaned := cleanIngredientLine(line)
		if cleaned != "" {
			ingredients = append(ingredients, cleaned)
		}
	}

	return ingredients
}

// cleanIngredientLine strips bullet prefixes, numbered prefixes, and whitespace.
func cleanIngredientLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	// Strip bullet prefixes (-, *, bullet).
	for _, prefix := range bulletPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			trimmed = strings.TrimSpace(trimmed[len(prefix):])
			return trimmed
		}
	}

	// Strip numbered prefixes like "1. ", "2. ".
	trimmed = stripNumberedPrefix(trimmed)

	return trimmed
}

// stripNumberedPrefix removes a leading "N. " pattern from a string.
func stripNumberedPrefix(s string) string {
	dotIdx := strings.Index(s, ". ")
	if dotIdx < 0 {
		return s
	}

	prefix := s[:dotIdx]
	for _, c := range prefix {
		if c < '0' || c > '9' {
			return s
		}
	}

	return strings.TrimSpace(s[dotIdx+2:])
}

// containsTopic checks whether a topic name exists in the topics slice.
func containsTopic(topics []string, target string) bool {
	for _, t := range topics {
		if t == target {
			return true
		}
	}

	return false
}
