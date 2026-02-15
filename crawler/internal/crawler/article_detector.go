package crawler

import (
	"net/url"
	"regexp"
	"strings"

	colly "github.com/gocolly/colly/v2"
)

// Minimum number of hyphen-separated words in a slug to consider it article-like.
const minSlugWordCount = 4

// DetectedContentType identifies the structured content type detected from URL/HTML.
// Used by the crawler to decide extraction strategy and by the classifier for routing.
const (
	DetectedContentArticle             = "article"
	DetectedContentPressRelease        = "press_release"
	DetectedContentBlogPost            = "blog_post"
	DetectedContentEvent               = "event"
	DetectedContentAdvisory            = "advisory"
	DetectedContentReport              = "report"
	DetectedContentBlotter             = "blotter"
	DetectedContentCompanyAnnouncement = "company_announcement"
	DetectedContentUnknown             = ""
)

// JSON-LD schema types that indicate structured content we collect.
var structuredContentJSONLDTypes = []string{
	"NewsArticle", "Article", "BlogPosting", "PressRelease",
	"Event", "SpecialAnnouncement", "Report", "Obituary", "Review",
}

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

// urlContentTypePatterns map URL path substrings to detected content types.
// Order matters: first match wins. More specific patterns should come first.
var urlContentTypePatterns = []struct {
	pattern string
	ctype   string
}{
	{"/press/", DetectedContentPressRelease},
	{"/media/", DetectedContentPressRelease},
	{"/newsroom/", DetectedContentPressRelease},
	{"/events/", DetectedContentEvent},
	{"/event/", DetectedContentEvent},
	{"/calendar/", DetectedContentEvent},
	{"/upcoming/", DetectedContentEvent},
	{"/alert/", DetectedContentAdvisory},
	{"/alerts/", DetectedContentAdvisory},
	{"/advisory/", DetectedContentAdvisory},
	{"/advisories/", DetectedContentAdvisory},
	{"/bulletin/", DetectedContentAdvisory},
	{"/bulletins/", DetectedContentAdvisory},
	{"/reports/", DetectedContentReport},
	{"/report/", DetectedContentReport},
	{"/blotter/", DetectedContentBlotter},
	{"/blotters/", DetectedContentBlotter},
	{"/incidents/", DetectedContentBlotter},
	{"/arrests/", DetectedContentBlotter},
	{"/investors/", DetectedContentCompanyAnnouncement},
	{"/investor/", DetectedContentCompanyAnnouncement},
	{"/updates/", DetectedContentCompanyAnnouncement},
}

// pdfSuffix matches URLs that point to PDF documents (report type).
const pdfSuffix = ".pdf"

// articlePathSegments are path segments that strongly suggest article content
// when followed by additional path content.
var articlePathSegments = map[string]bool{
	"article":    true,
	"story":      true,
	"post":       true,
	"news":       true,
	"press":      true,
	"media":      true,
	"newsroom":   true,
	"events":     true,
	"event":      true,
	"calendar":   true,
	"upcoming":   true,
	"alert":      true,
	"alerts":     true,
	"advisory":   true,
	"advisories": true,
	"bulletin":   true,
	"bulletins":  true,
	"blotter":    true,
	"blotters":   true,
	"incidents":  true,
	"arrests":    true,
	"reports":    true,
	"report":     true,
	"investors":  true,
	"investor":   true,
	"updates":    true,
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
	return hasStructuredContentJSONLD(e)
}

// hasStructuredContentJSONLD checks if the page has any JSON-LD schema type we collect.
func hasStructuredContentJSONLD(e *colly.HTMLElement) bool {
	found := false
	e.ForEach("script[type='application/ld+json']", func(_ int, el *colly.HTMLElement) {
		if found {
			return
		}
		text := el.Text
		for _, schemaType := range structuredContentJSONLDTypes {
			if strings.Contains(text, `"`+schemaType+`"`) {
				found = true
				return
			}
		}
	})
	return found
}

// detectContentTypeFromURL returns the content type inferred from URL path patterns.
// Returns DetectedContentUnknown if no pattern matches.
func detectContentTypeFromURL(pageURL string) string {
	parsed, err := url.Parse(pageURL)
	if err != nil {
		return DetectedContentUnknown
	}
	lowerPath := strings.ToLower(parsed.Path)
	if strings.HasSuffix(lowerPath, pdfSuffix) {
		return DetectedContentReport
	}
	for _, p := range urlContentTypePatterns {
		if strings.Contains(lowerPath, p.pattern) {
			return p.ctype
		}
	}
	return DetectedContentUnknown
}

// detectContentTypeFromJSONLD returns the content type from JSON-LD @type in the page.
// Returns DetectedContentUnknown if not found or not a type we collect.
func detectContentTypeFromJSONLD(e *colly.HTMLElement) string {
	jsonldToDetected := map[string]string{
		"NewsArticle":         DetectedContentArticle,
		"Article":             DetectedContentArticle,
		"BlogPosting":         DetectedContentBlogPost,
		"PressRelease":        DetectedContentPressRelease,
		"Event":               DetectedContentEvent,
		"SpecialAnnouncement": DetectedContentAdvisory,
		"Report":              DetectedContentReport,
	}
	var detected string
	e.ForEach("script[type='application/ld+json']", func(_ int, el *colly.HTMLElement) {
		if detected != "" {
			return
		}
		text := strings.TrimSpace(el.Text)
		if text == "" {
			return
		}
		for jsonldType, ctype := range jsonldToDetected {
			if strings.Contains(text, `"@type":"`+jsonldType+`"`) ||
				strings.Contains(text, `"@type": "`+jsonldType+`"`) {
				detected = ctype
				return
			}
		}
	})
	return detected
}

// detectContentTypeFromHTML combines JSON-LD @type, og:type, and URL to detect content type.
func detectContentTypeFromHTML(e *colly.HTMLElement, pageURL string) string {
	if ctype := detectContentTypeFromJSONLD(e); ctype != DetectedContentUnknown {
		return ctype
	}
	if ctype := detectContentTypeFromURL(pageURL); ctype != DetectedContentUnknown {
		return ctype
	}
	ogType := e.ChildAttr("meta[property='og:type']", "content")
	if strings.EqualFold(ogType, "article") {
		return DetectedContentArticle
	}
	return DetectedContentUnknown
}

// IsStructuredContentPage returns true if the page is structured content we collect,
// and the detected content type. Used by the collector to gate extraction.
func IsStructuredContentPage(e *colly.HTMLElement, pageURL string, explicitPatterns []*regexp.Regexp) (found bool, contentType string) {
	ctype := detectContentTypeFromHTML(e, pageURL)
	if ctype != DetectedContentUnknown {
		return true, ctype
	}
	if hasStructuredContentJSONLD(e) {
		return true, DetectedContentArticle
	}
	ogType := e.ChildAttr("meta[property='og:type']", "content")
	if strings.EqualFold(ogType, "article") {
		return true, DetectedContentArticle
	}
	if isArticleURL(pageURL, explicitPatterns) {
		return true, DetectedContentArticle
	}
	return false, DetectedContentUnknown
}

// isArticlePage determines whether a page is an article based on HTML metadata.
// It returns true if og:type is "article" (case insensitive) or if the page
// contains NewsArticle JSON-LD structured data. Kept for backward compatibility.
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
