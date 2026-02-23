package router

import "strings"

// RecipeDomain routes recipe-classified content to recipes:* channels.
// Channels produced:
//   - articles:recipes (catch-all)
//   - recipes:category:{slug} (per category)
//   - recipes:cuisine:{slug} (per cuisine)
type RecipeDomain struct{}

// NewRecipeDomain creates a RecipeDomain.
func NewRecipeDomain() *RecipeDomain { return &RecipeDomain{} }

// Name returns the domain identifier.
func (d *RecipeDomain) Name() string { return "recipe" }

// Routes returns recipe channels for the article.
func (d *RecipeDomain) Routes(a *Article) []ChannelRoute {
	if a.Recipe == nil {
		return nil
	}

	channels := []string{"articles:recipes"}

	if a.Recipe.Category != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Recipe.Category, " ", "-"))
		channels = append(channels, "recipes:category:"+slug)
	}

	if a.Recipe.Cuisine != "" {
		slug := strings.ToLower(strings.ReplaceAll(a.Recipe.Cuisine, " ", "-"))
		channels = append(channels, "recipes:cuisine:"+slug)
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*RecipeDomain)(nil)
