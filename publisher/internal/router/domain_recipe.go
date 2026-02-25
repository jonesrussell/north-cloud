package router

import "strings"

// RecipeDomain routes recipe-classified content to recipes:* channels.
// Channels produced:
//   - content:recipes (catch-all)
//   - recipes:category:{slug} (per category)
//   - recipes:cuisine:{slug} (per cuisine)
type RecipeDomain struct{}

// NewRecipeDomain creates a RecipeDomain.
func NewRecipeDomain() *RecipeDomain { return &RecipeDomain{} }

// Name returns the domain identifier.
func (d *RecipeDomain) Name() string { return "recipe" }

// Routes returns recipe channels for the content item.
func (d *RecipeDomain) Routes(item *ContentItem) []ChannelRoute {
	if item.Recipe == nil {
		return nil
	}

	channels := []string{"content:recipes"}

	if item.Recipe.Category != "" {
		slug := strings.ToLower(strings.ReplaceAll(item.Recipe.Category, " ", "-"))
		channels = append(channels, "recipes:category:"+slug)
	}

	if item.Recipe.Cuisine != "" {
		slug := strings.ToLower(strings.ReplaceAll(item.Recipe.Cuisine, " ", "-"))
		channels = append(channels, "recipes:cuisine:"+slug)
	}

	return channelRoutesFromSlice(channels)
}

// compile-time interface check
var _ RoutingDomain = (*RecipeDomain)(nil)
