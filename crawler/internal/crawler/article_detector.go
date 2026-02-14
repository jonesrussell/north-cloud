package crawler

import (
	"net/url"
	"regexp"
	"strings"

	colly "github.com/gocolly/colly/v2"
)

// Minimum number of hyphen-separated words in a slug to consider it article-like.
const minSlugWordCount = 4

// nonArticleSegments are URL path segments that indicate non-article pages.
var nonArticleSegments = map[string]bool{
	"login":    true,
	"signin":   true,
	"signup":   true,
	"register": true,
	"search":   true,
	"contact":  true,
	"about":    true,
	"privacy":  true,
	"terms":    true,
	"tag":      true,
	"category": true,
	"author":   true,
	"page":     true,
	"feed":     true,
	"rss":      true,
	"sitemap":  true,
	"admin":    true,
	"wp-admin": true,
	"account":  true,
	"cart":     true,
	"checkout": true,
}

// nonArticleExtensions are file extensions that indicate non-article resources.
var nonArticleExtensions = map[string]bool{
	".pdf":  true,
	".xml":  true,
	".json": true,
	".css":  true,
	".js":   true,
	".png":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".svg":  true,
	".ico":  true,
	".woff": true,
	".zip":  true,
	".mp3":  true,
	".mp4":  true,
}

// articlePathSegments are path segments that strongly suggest article content
// when followed by additional path content.
var articlePathSegments = map[string]bool{
	"article": true,
	"story":   true,
	"post":    true,
	"news":    true,
}

// datePathPattern matches date-based URL paths like /2026/02/14/headline or /2026/02/headline.
var datePathPattern = regexp.MustCompile(`/\d{4}/\d{2}(/\d{2})?/[^/]+`)

// isArticleURL determines whether a URL is likely an article page.
// If explicit patterns are provided, only those patterns decide.
// Otherwise, built-in heuristics are used.
func isArticleURL(pageURL string, explicitPatterns []*regexp.Regexp) bool {
	if len(explicitPatterns) > 0 {
		return matchesExplicitPatterns(pageURL, explicitPatterns)
	}

	return matchesBuiltInHeuristics(pageURL)
}

// matchesExplicitPatterns checks whether the URL matches any of the given patterns.
func matchesExplicitPatterns(pageURL string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(pageURL) {
			return true
		}
	}

	return false
}

// matchesBuiltInHeuristics applies default heuristics to detect article URLs.
func matchesBuiltInHeuristics(pageURL string) bool {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return false
	}

	path := strings.TrimRight(parsed.Path, "/")
	if path == "" {
		return false
	}

	lowerPath := strings.ToLower(path)

	if isNonArticlePath(lowerPath) {
		return false
	}

	segments := strings.Split(strings.TrimLeft(path, "/"), "/")
	if len(segments) == 1 && !hasLongSlug(segments[0]) {
		return false
	}

	return hasDatePath(path) ||
		hasArticlePathSegment(segments) ||
		hasLongSlugInPath(segments)
}

// isNonArticlePath checks if the path contains non-article segments or file extensions.
func isNonArticlePath(lowerPath string) bool {
	segments := strings.Split(strings.TrimLeft(lowerPath, "/"), "/")
	for _, seg := range segments {
		if nonArticleSegments[seg] {
			return true
		}
	}

	for ext := range nonArticleExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}

	return false
}

// hasDatePath checks if the path matches a date-based article URL pattern.
func hasDatePath(path string) bool {
	return datePathPattern.MatchString(path)
}

// hasArticlePathSegment checks if any segment is a known article indicator
// and is followed by additional content.
func hasArticlePathSegment(segments []string) bool {
	lastIndex := len(segments) - 1
	for i, seg := range segments {
		if articlePathSegments[strings.ToLower(seg)] && i < lastIndex {
			return true
		}
	}

	return false
}

// hasLongSlug checks whether a single segment is a hyphenated slug with enough words.
func hasLongSlug(segment string) bool {
	words := strings.Split(segment, "-")

	return len(words) >= minSlugWordCount
}

// hasLongSlugInPath checks if any segment in the path has a long hyphenated slug.
func hasLongSlugInPath(segments []string) bool {
	for _, seg := range segments {
		if hasLongSlug(seg) {
			return true
		}
	}

	return false
}

// hasNewsArticleJSONLD checks if the page has NewsArticle or Article JSON-LD.
func hasNewsArticleJSONLD(e *colly.HTMLElement) bool {
	found := false
	e.ForEach("script[type='application/ld+json']", func(_ int, el *colly.HTMLElement) {
		if found {
			return
		}
		if strings.Contains(el.Text, `"NewsArticle"`) || strings.Contains(el.Text, `"Article"`) {
			found = true
		}
	})
	return found
}

// isArticlePage determines whether a page is an article based on HTML metadata.
// It returns true if og:type is "article" (case insensitive) or if the page
// contains NewsArticle JSON-LD structured data.
func isArticlePage(ogType string, hasJSONLD bool) bool {
	return strings.EqualFold(ogType, "article") || hasJSONLD
}

// compileArticlePatterns compiles string patterns into regular expressions.
// Invalid patterns are silently skipped. Returns nil for empty input.
func compileArticlePatterns(patterns []string) []*regexp.Regexp {
	if len(patterns) == 0 {
		return nil
	}

	compiled := make([]*regexp.Regexp, 0, len(patterns))

	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}

		compiled = append(compiled, re)
	}

	if len(compiled) == 0 {
		return nil
	}

	return compiled
}
