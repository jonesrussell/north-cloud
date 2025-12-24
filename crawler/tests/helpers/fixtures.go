// Package helpers provides testing utilities for integration tests.
package helpers

import (
	"time"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	sourcestypes "github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
)

const (
	// DefaultTestMaxDepth is the default max depth for test sources.
	DefaultTestMaxDepth = 2
	// MinURLLength is the minimum URL length for domain extraction.
	MinURLLength = 8 // "https://" is 8 characters
)

// SourceOption is a function that modifies a source configuration.
type SourceOption func(*sourcestypes.SourceConfig)

// TestSource creates a test source configuration.
func TestSource(name, url string, opts ...SourceOption) *sourcestypes.SourceConfig {
	source := &sourcestypes.SourceConfig{
		Name:           name,
		URL:            url,
		AllowedDomains: []string{extractDomain(url)},
		StartURLs:      []string{url},
		RateLimit:      1 * time.Second,
		MaxDepth:       DefaultTestMaxDepth,
		Index:          "test_pages",
		ArticleIndex:   "test_articles",
		PageIndex:      "test_pages",
		Selectors: sourcestypes.SelectorConfig{
			Article: sourcestypes.ArticleSelectors{
				Container:     "article",
				Title:         "h1",
				Body:          ".content",
				Intro:         ".intro",
				PublishedTime: "time",
			},
			Page: sourcestypes.PageSelectors{
				Container: "main",
				Title:     "h1",
				Content:   ".content",
			},
		},
		Rules: configtypes.Rules{
			{
				Pattern:  url + "/*",
				Action:   "allow",
				Priority: 1,
			},
		},
	}

	for _, opt := range opts {
		opt(source)
	}

	return source
}

// WithMaxDepth sets the max depth for a test source.
func WithMaxDepth(depth int) SourceOption {
	return func(s *sourcestypes.SourceConfig) {
		s.MaxDepth = depth
	}
}

// WithStartURLs sets the start URLs for a test source.
func WithStartURLs(urls []string) SourceOption {
	return func(s *sourcestypes.SourceConfig) {
		s.StartURLs = urls
	}
}

// WithRateLimit sets the rate limit for a test source.
func WithRateLimit(limit time.Duration) SourceOption {
	return func(s *sourcestypes.SourceConfig) {
		s.RateLimit = limit
	}
}

// WithIndexName sets the index name for a test source.
func WithIndexName(index string) SourceOption {
	return func(s *sourcestypes.SourceConfig) {
		s.Index = index
	}
}

// TestHTMLPage returns HTML content for testing.
func TestHTMLPage(title, body string) string {
	return `<!DOCTYPE html>
<html>
<head>
	<title>` + title + `</title>
	<meta name="description" content="Test page description">
</head>
<body>
	<main>
		<h1>` + title + `</h1>
		<div class="content">` + body + `</div>
	</main>
</body>
</html>`
}

// TestArticleHTML returns HTML content for an article.
func TestArticleHTML(title, body string) string {
	return `<!DOCTYPE html>
<html>
<head>
	<title>` + title + `</title>
	<meta name="description" content="Test article description">
</head>
<body>
	<article>
		<h1>` + title + `</h1>
		<div class="intro">Article introduction</div>
		<time datetime="2024-01-01">January 1, 2024</time>
		<div class="content">` + body + `</div>
	</article>
</body>
</html>`
}

// extractDomain extracts the domain from a URL.
func extractDomain(url string) string {
	// Simple domain extraction - assumes http:// or https://
	if len(url) > MinURLLength-1 {
		start := 0
		if url[:4] == "http" {
			if url[:5] == "https" {
				start = 8
			} else {
				start = 7
			}
		}
		end := start
		for end < len(url) && url[end] != '/' && url[end] != ':' {
			end++
		}
		return url[start:end]
	}
	return "example.com"
}
