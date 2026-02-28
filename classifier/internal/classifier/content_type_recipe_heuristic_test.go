//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyFromRecipeKeywords_TwoKeywords(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "recipe-2kw",
		Title:   "Grandma's Famous Pasta Recipe",
		RawText: "Preheat oven to 350F. Combine the ingredients in a bowl.",
	}

	result := c.classifyFromRecipeKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeRecipe, result.Type)
	assert.InDelta(t, keywordHeuristicConfidence, result.Confidence, 0.001)
	assert.Equal(t, "keyword_heuristic", result.Method)
}

func TestClassifyFromRecipeKeywords_SingleKeyword(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "recipe-1kw",
		Title:   "Best cooking tips",
		RawText: "Always preheat your oven before cooking.",
	}

	result := c.classifyFromRecipeKeywords(raw)
	assert.Nil(t, result, "single keyword match should not classify as recipe")
}

func TestClassifyFromRecipeKeywords_IngredientQuantityPlusKeyword(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "recipe-qty",
		Title:   "Simple Soup",
		RawText: "Add 2 cups of broth and simmer for 20 minutes.",
	}

	result := c.classifyFromRecipeKeywords(raw)
	require.NotNil(t, result, "ingredient quantity + keyword should classify as recipe")
	assert.Equal(t, domain.ContentTypeRecipe, result.Type)
}

func TestClassifyFromRecipeKeywords_IngredientQuantityAlone(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "recipe-qty-only",
		Title:   "Chemistry Lab Report",
		RawText: "We measured 500 ml of solution into the beaker.",
	}

	result := c.classifyFromRecipeKeywords(raw)
	assert.Nil(t, result, "ingredient quantity alone should not classify as recipe")
}

func TestClassifyFromRecipeKeywords_NoSignals(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "recipe-none",
		Title:   "City Council Meeting Minutes",
		RawText: "The council discussed zoning changes for the downtown area.",
	}

	result := c.classifyFromRecipeKeywords(raw)
	assert.Nil(t, result)
}

func TestClassifyFromRecipeKeywords_CaseInsensitive(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	raw := &domain.RawContent{
		ID:      "recipe-case",
		Title:   "RECIPE for Success",
		RawText: "INGREDIENTS: flour, sugar, eggs. PREHEAT the oven.",
	}

	result := c.classifyFromRecipeKeywords(raw)
	require.NotNil(t, result)
	assert.Equal(t, domain.ContentTypeRecipe, result.Type)
}

// Cross-strategy conflict: recipe page that also satisfies article heuristics.
func TestRecipeWinsOverArticleHeuristic(t *testing.T) {
	t.Helper()

	c := NewContentTypeClassifier(&mockLogger{})
	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:    "recipe-vs-article",
		URL:   "https://example.com/food/pasta-recipe",
		Title: "Classic Italian Pasta Recipe",
		RawText: "This recipe has been in my family for generations. " +
			"Preheat the oven to 375F. Combine the ingredients. " +
			string(make([]byte, 500)),
		WordCount:       350,
		MetaDescription: "A classic Italian pasta recipe passed down through generations",
		PublishedDate:   &publishedDate,
		OGType:          "article",
	}

	result, err := c.Classify(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, domain.ContentTypeRecipe, result.Type, "recipe heuristic should win over article")
	assert.Equal(t, "keyword_heuristic", result.Method)
}
