package integration

import (
	"context"

	"github.com/gopost/integration/internal/config"
	"github.com/gopost/integration/internal/drupal"
)

// ElasticsearchClient defines the interface for Elasticsearch operations.
// This allows for easier testing and potential future implementations.
type ElasticsearchClient interface {
	// Search performs a search query on the specified index
	Search(ctx context.Context, index string, query map[string]any) (*SearchResult, error)
}

// DrupalClient defines the interface for Drupal operations.
type DrupalClient interface {
	// PostArticle posts an article to Drupal via JSON:API
	PostArticle(ctx context.Context, req drupal.ArticleRequest) error
}

// DedupTracker defines the interface for deduplication tracking.
type DedupTracker interface {
	// HasPosted checks if an article has already been posted
	HasPosted(ctx context.Context, articleID string) bool
	// MarkPosted marks an article as posted
	MarkPosted(ctx context.Context, articleID string) error
	// Clear removes an article from the posted cache
	Clear(ctx context.Context, articleID string) error
	// FlushAll removes all posted article keys from the cache
	FlushAll(ctx context.Context) error
}

// ArticleFinder defines the interface for finding articles.
type ArticleFinder interface {
	// FindCrimeArticles finds crime-related articles for a city
	FindCrimeArticles(ctx context.Context, cityCfg config.CityConfig) ([]Article, error)
}

// ArticleProcessor defines the interface for processing articles.
type ArticleProcessor interface {
	// ProcessArticle processes a single article (check dedup, post to Drupal, etc.)
	ProcessArticle(ctx context.Context, article Article, cityCfg config.CityConfig) error
}

// SearchResult represents the result of an Elasticsearch search
type SearchResult struct {
	Total int
	Hits  []Article
}
