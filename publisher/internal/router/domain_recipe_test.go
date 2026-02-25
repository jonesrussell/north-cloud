package router_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/publisher/internal/router"
)

func TestRecipeDomain_NilRecipe(t *testing.T) {
	t.Helper()
	d := router.NewRecipeDomain()
	routes := d.Routes(&router.ContentItem{})
	if routes != nil {
		t.Error("expected nil routes for article without recipe data")
	}
}

func TestRecipeDomain_WithCategoryAndCuisine(t *testing.T) {
	t.Helper()
	d := router.NewRecipeDomain()
	article := &router.ContentItem{
		Recipe: &router.RecipeData{
			ExtractionMethod: "schema_org",
			Category:         "Dessert",
			Cuisine:          "Italian",
		},
	}
	routes := d.Routes(article)
	if len(routes) == 0 {
		t.Fatal("expected routes")
	}

	channels := make(map[string]bool)
	for _, r := range routes {
		channels[r.Channel] = true
	}

	if !channels["content:recipes"] {
		t.Error("expected articles:recipes channel")
	}
	if !channels["recipes:category:dessert"] {
		t.Error("expected recipes:category:dessert channel")
	}
	if !channels["recipes:cuisine:italian"] {
		t.Error("expected recipes:cuisine:italian channel")
	}
}

func TestRecipeDomain_Name(t *testing.T) {
	t.Helper()
	d := router.NewRecipeDomain()
	if d.Name() != "recipe" {
		t.Errorf("expected name 'recipe', got %q", d.Name())
	}
}

func TestRecipeDomain_OnlyCategory(t *testing.T) {
	t.Helper()
	d := router.NewRecipeDomain()
	article := &router.ContentItem{
		Recipe: &router.RecipeData{
			Category: "Soup",
		},
	}
	routes := d.Routes(article)
	channels := make(map[string]bool)
	for _, r := range routes {
		channels[r.Channel] = true
	}
	if !channels["content:recipes"] {
		t.Error("expected articles:recipes")
	}
	if !channels["recipes:category:soup"] {
		t.Error("expected recipes:category:soup")
	}

	const expectedRouteCount = 2
	if len(routes) != expectedRouteCount {
		t.Errorf("expected %d routes, got %d", expectedRouteCount, len(routes))
	}
}
