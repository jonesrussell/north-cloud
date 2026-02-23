//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecipeExtractor_SchemaOrgFullFields(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "test-1",
		Title: "Chocolate Cake Recipe",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Chocolate Cake",
  "recipeIngredient": ["2 cups flour", "1 cup sugar", "3 eggs"],
  "recipeInstructions": "Mix flour and sugar. Add eggs. Bake at 350F for 30 minutes.",
  "prepTime": "PT15M",
  "cookTime": "PT30M",
  "totalTime": "PT45M",
  "recipeYield": "8 servings",
  "recipeCategory": "Dessert",
  "recipeCuisine": "American",
  "nutrition": {"calories": "350 kcal"},
  "image": "https://example.com/cake.jpg",
  "aggregateRating": {"ratingValue": 4.5, "ratingCount": 120}
}
</script>
</head><body></body></html>`,
		RawText: "Chocolate Cake recipe content here",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "schema_org", result.ExtractionMethod)
	assert.Equal(t, "Chocolate Cake", result.Name)
	assert.Equal(t, []string{"2 cups flour", "1 cup sugar", "3 eggs"}, result.Ingredients)
	assert.Equal(t, "Mix flour and sugar. Add eggs. Bake at 350F for 30 minutes.", result.Instructions)

	require.NotNil(t, result.PrepTimeMinutes)
	assert.Equal(t, 15, *result.PrepTimeMinutes)

	require.NotNil(t, result.CookTimeMinutes)
	assert.Equal(t, 30, *result.CookTimeMinutes)

	require.NotNil(t, result.TotalTimeMinutes)
	assert.Equal(t, 45, *result.TotalTimeMinutes)

	assert.Equal(t, "8 servings", result.Servings)
	assert.Equal(t, "Dessert", result.Category)
	assert.Equal(t, "American", result.Cuisine)
	assert.Equal(t, "350 kcal", result.Calories)
	assert.Equal(t, "https://example.com/cake.jpg", result.ImageURL)

	require.NotNil(t, result.Rating)
	assert.InDelta(t, 4.5, *result.Rating, 0.001)

	require.NotNil(t, result.RatingCount)
	assert.Equal(t, 120, *result.RatingCount)
}

func TestRecipeExtractor_NotApplicable_ArticleContentType(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "test-2",
		Title:   "Breaking News Article",
		RawHTML: `<html><body><p>This is a news article.</p></body></html>`,
		RawText: "This is a news article about local events.",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"crime", "local_news"})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRecipeExtractor_HeuristicFallback(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "test-3",
		Title:   "Grandma's Famous Soup",
		RawHTML: `<html><body><p>No JSON-LD here</p></body></html>`,
		RawText: `Grandma's Famous Soup

This is the best soup you'll ever have.

Ingredients:
- 2 cups chicken broth
- 1 cup diced carrots
- 1 cup celery
- Salt and pepper to taste

Instructions:
Bring broth to a boil. Add carrots and celery. Simmer for 20 minutes. Season with salt and pepper.`,
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"recipe", "food"})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "heuristic", result.ExtractionMethod)
	require.Len(t, result.Ingredients, 4)
	assert.Equal(t, "2 cups chicken broth", result.Ingredients[0])
	assert.Equal(t, "1 cup diced carrots", result.Ingredients[1])
	assert.Equal(t, "1 cup celery", result.Ingredients[2])
	assert.Equal(t, "Salt and pepper to taste", result.Ingredients[3])
	assert.Contains(t, result.Instructions, "Bring broth to a boil")
}

func TestRecipeExtractor_HowToStepInstructions(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "test-4",
		Title: "Pasta Carbonara",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Pasta Carbonara",
  "recipeIngredient": ["200g spaghetti", "100g pancetta"],
  "recipeInstructions": [
    {"@type": "HowToStep", "text": "Cook the spaghetti in salted water."},
    {"@type": "HowToStep", "text": "Fry the pancetta until crispy."},
    {"@type": "HowToStep", "text": "Combine and serve immediately."}
  ]
}
</script>
</head><body></body></html>`,
		RawText: "Pasta Carbonara recipe",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "schema_org", result.ExtractionMethod)
	assert.Equal(t, "Pasta Carbonara", result.Name)
	assert.Contains(t, result.Instructions, "Cook the spaghetti in salted water.")
	assert.Contains(t, result.Instructions, "Fry the pancetta until crispy.")
	assert.Contains(t, result.Instructions, "Combine and serve immediately.")
}

func TestRecipeExtractor_ImageAsObject(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "test-5",
		Title: "Simple Salad",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Simple Salad",
  "recipeIngredient": ["1 head lettuce", "2 tomatoes"],
  "image": {"@type": "ImageObject", "url": "https://example.com/salad.jpg"}
}
</script>
</head><body></body></html>`,
		RawText: "Simple Salad recipe",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "schema_org", result.ExtractionMethod)
	assert.Equal(t, "https://example.com/salad.jpg", result.ImageURL)
}

func TestRecipeExtractor_TopicGating(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "test-6",
		Title:   "Cooking article without Schema.org",
		RawHTML: `<html><body><p>No structured data</p></body></html>`,
		RawText: `A cooking article with no recipe sections.
Just general discussion about food trends.`,
	}

	// Content type is article and no recipe topic - should return nil.
	result, err := extractor.Extract(ctx, raw, domain.ContentTypeArticle, []string{"food"})
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestRecipeExtractor_SchemaOrgStringArrayInstructions(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "test-7",
		Title: "Quick Omelette",
		RawHTML: `<html><head>
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Recipe",
  "name": "Quick Omelette",
  "recipeIngredient": ["3 eggs", "1 tbsp butter"],
  "recipeInstructions": ["Beat the eggs.", "Melt butter in pan.", "Pour eggs and cook."]
}
</script>
</head><body></body></html>`,
		RawText: "Quick Omelette recipe",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "schema_org", result.ExtractionMethod)
	assert.Contains(t, result.Instructions, "Beat the eggs.")
	assert.Contains(t, result.Instructions, "Melt butter in pan.")
	assert.Contains(t, result.Instructions, "Pour eggs and cook.")
}

func TestRecipeExtractor_MalformedSchemaOrgFallsToHeuristic(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "test-8",
		Title: "Bad Schema Recipe",
		RawHTML: `<html><head>
<script type="application/ld+json">
{ this is not valid JSON
</script>
</head><body></body></html>`,
		RawText: `Bad Schema Recipe

Ingredients:
- 1 cup rice
- 2 cups water

Directions:
Boil water. Add rice. Cook for 20 minutes.`,
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "heuristic", result.ExtractionMethod)
	require.Len(t, result.Ingredients, 2)
	assert.Equal(t, "1 cup rice", result.Ingredients[0])
	assert.Contains(t, result.Instructions, "Boil water")
}

func TestRecipeExtractor_HeuristicWithVariousPrefixes(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:      "test-9",
		Title:   "Numbered Ingredients Recipe",
		RawHTML: `<html><body></body></html>`,
		RawText: `A recipe with numbered items.

Ingredients:
1. 2 cups flour
2. 1 cup milk
* 3 eggs
` + "\u2022 1 tsp vanilla" + `

Method:
Combine dry ingredients. Add wet ingredients. Mix well.`,
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "heuristic", result.ExtractionMethod)
	require.Len(t, result.Ingredients, 4)
	assert.Equal(t, "2 cups flour", result.Ingredients[0])
	assert.Equal(t, "1 cup milk", result.Ingredients[1])
	assert.Equal(t, "3 eggs", result.Ingredients[2])
	assert.Equal(t, "1 tsp vanilla", result.Ingredients[3])
	assert.Contains(t, result.Instructions, "Combine dry ingredients")
}

func TestRecipeExtractor_SchemaOrg_WithoutAggregateRating(t *testing.T) {
	t.Helper()

	extractor := NewRecipeExtractor(&mockLogger{})
	ctx := context.Background()

	raw := &domain.RawContent{
		ID:    "no-rating",
		Title: "Simple Recipe",
		RawHTML: `<html><head>
<script type="application/ld+json">
{"@context":"https://schema.org","@type":"Recipe","name":"Simple Salad","recipeIngredient":["lettuce","tomato"]}
</script>
</head><body></body></html>`,
		RawText: "Simple Salad recipe",
	}

	result, err := extractor.Extract(ctx, raw, domain.ContentTypeRecipe, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Rating)
	assert.Nil(t, result.RatingCount)
	assert.Equal(t, "Simple Salad", result.Name)
}

func TestContainsTopic(t *testing.T) {
	t.Helper()

	tests := []struct {
		name     string
		topics   []string
		target   string
		wantBool bool
	}{
		{"exact match", []string{"recipe"}, "recipe", true},
		{"match among many", []string{"a", "recipe"}, "recipe", true},
		{"case sensitive no match", []string{"Recipe"}, "recipe", false},
		{"nil topics", nil, "recipe", false},
		{"empty topics", []string{}, "recipe", false},
		{"jobs match", []string{"jobs", "local"}, "jobs", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsTopic(tt.topics, tt.target)
			assert.Equal(t, tt.wantBool, got)
		})
	}
}
