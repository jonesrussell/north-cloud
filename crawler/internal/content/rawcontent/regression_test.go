//go:build integration

// Package rawcontent_test contains fixture-based regression tests for extraction quality.
// Run with: go test -tags=integration ./internal/content/rawcontent/...
package rawcontent_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
)

// ExtractionFixture defines the expected outcomes for one HTML snapshot.
type ExtractionFixture struct {
	// Name is a human-readable identifier for the fixture.
	Name string
	// Template is the expected CMS template name detected for this HTML.
	// Empty string means no template is expected to match.
	ExpectedTemplate string
	// ExpectedPageType is the page type that should be assigned after extraction
	// (article, listing, stub, other).
	ExpectedPageType string
	// MinWordCount is the minimum number of words expected in the extracted body text.
	MinWordCount int
	// TitleRequired asserts that a non-empty title must be extracted.
	TitleRequired bool
	// HTML is the raw HTML content of the fixture (inline snapshot).
	HTML string
}

// fixtureCount is the number of fixtures defined in this test suite.
const fixtureCount = 8

// extractionFixtures covers each CMS template in templateRegistry plus a generic case.
var extractionFixtures = [fixtureCount]ExtractionFixture{
	{
		Name:             "wordpress",
		ExpectedTemplate: "wordpress",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     200,
		TitleRequired:    true,
		HTML:             wordpressHTML,
	},
	{
		Name:             "drupal",
		ExpectedTemplate: "drupal",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     200,
		TitleRequired:    true,
		HTML:             drupalHTML,
	},
	{
		Name:             "html5-article (generic_og_article)",
		ExpectedTemplate: "generic_og_article",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     150,
		TitleRequired:    true,
		HTML:             html5ArticleHTML,
	},
	{
		Name:             "postmedia",
		ExpectedTemplate: "postmedia",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     200,
		TitleRequired:    true,
		HTML:             postmediaHTML,
	},
	{
		Name:             "torstar",
		ExpectedTemplate: "torstar",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     200,
		TitleRequired:    true,
		HTML:             torstarHTML,
	},
	{
		Name:             "village-media",
		ExpectedTemplate: "village_media",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     200,
		TitleRequired:    true,
		HTML:             villageMediaHTML,
	},
	{
		Name:             "black-press",
		ExpectedTemplate: "black_press",
		ExpectedPageType: rawcontent.PageTypeArticle,
		MinWordCount:     200,
		TitleRequired:    true,
		HTML:             blackPressHTML,
	},
	{
		Name:             "listing-page (no template)",
		ExpectedTemplate: "",
		ExpectedPageType: rawcontent.PageTypeListing,
		MinWordCount:     0,
		TitleRequired:    false,
		HTML:             listingHTML,
	},
}

func TestExtractionFixtures(t *testing.T) {
	t.Helper()

	for _, fix := range extractionFixtures {
		fix := fix
		t.Run(fix.Name, func(t *testing.T) {
			t.Helper()
			runFixture(t, fix)
		})
	}
}

// runFixture executes one fixture assertion: template detection, page type, and extraction quality.
func runFixture(t *testing.T, fix ExtractionFixture) {
	t.Helper()

	assertTemplateDetection(t, fix)
	assertExtraction(t, fix)
}

// assertTemplateDetection verifies that template detection returns the expected template name.
func assertTemplateDetection(t *testing.T, fix ExtractionFixture) {
	t.Helper()

	tmpl, ok := rawcontent.DetectTemplateByHTML(fix.HTML)
	if fix.ExpectedTemplate == "" {
		if ok {
			t.Errorf("fixture %q: expected no template, got %q", fix.Name, tmpl.Name)
		}
		return
	}

	if !ok {
		t.Errorf("fixture %q: expected template %q, got no match", fix.Name, fix.ExpectedTemplate)
		return
	}
	if tmpl.Name != fix.ExpectedTemplate {
		t.Errorf("fixture %q: expected template %q, got %q", fix.Name, fix.ExpectedTemplate, tmpl.Name)
	}
}

// assertExtraction verifies that ExtractRawContent produces output satisfying fixture expectations.
func assertExtraction(t *testing.T, fix ExtractionFixture) {
	t.Helper()

	e := newHTMLElement(t, fix.HTML)

	// Resolve selectors from detected template (if any).
	selectors := resolveSelectors(fix.HTML)

	data := rawcontent.ExtractRawContent(e, "https://example.com/article", selectors.Title, selectors.Body, selectors.Container, selectors.Exclude)

	if fix.TitleRequired && strings.TrimSpace(data.Title) == "" {
		t.Errorf("fixture %q: expected non-empty title, got empty", fix.Name)
	}

	wordCount := countWords(data.RawText)
	if wordCount < fix.MinWordCount {
		t.Errorf("fixture %q: expected >= %d words, got %d", fix.Name, fix.MinWordCount, wordCount)
	}

	assertPageType(t, fix, data, wordCount)
}

// assertPageType verifies the page type classification.
func assertPageType(t *testing.T, fix ExtractionFixture, data *rawcontent.RawContentData, wordCount int) {
	t.Helper()

	if fix.ExpectedPageType == "" {
		return
	}

	linkCount := strings.Count(data.RawHTML, "<a ")
	articleTagCount, hasDateTime, hasSignInText := rawcontent.ExtractHTMLSignals(data.RawHTML)

	signals := rawcontent.MakeSignals(
		data.Title,
		wordCount,
		linkCount,
		data.OGType,
		"",
		"",
		articleTagCount,
		hasDateTime,
		hasSignInText,
	)
	got := rawcontent.ClassifyPageType(signals)

	if got != fix.ExpectedPageType {
		t.Errorf("fixture %q: expected page type %q, got %q (words=%d, links=%d)",
			fix.Name, fix.ExpectedPageType, got, wordCount, linkCount)
	}
}

// resolveSelectors returns SourceSelectors from the first matching template for the HTML,
// or zero-value selectors if no template matches.
func resolveSelectors(html string) rawcontent.SourceSelectors {
	tmpl, ok := rawcontent.DetectTemplateByHTML(html)
	if !ok {
		return rawcontent.SourceSelectors{}
	}
	return tmpl.Selectors
}

// countWords returns the number of whitespace-separated words in s.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// articleBody generates a body of approximately n words for use in HTML fixtures.
func articleBody(n int) string {
	const sentence = "The quick brown fox jumps over the lazy dog. "
	words := strings.Fields(sentence)
	wordLen := len(words)
	var sb strings.Builder
	for i := 0; i < n; i++ {
		sb.WriteString(words[i%wordLen])
		sb.WriteByte(' ')
		if (i+1)%10 == 0 {
			sb.WriteString("</p><p>")
		}
	}
	return sb.String()
}

// wordpressHTML is an HTML snapshot representative of a WordPress article page.
var wordpressHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta name="generator" content="WordPress 6.4">
<meta property="og:type" content="article">
<title>Test WordPress Article</title>
</head>
<body>
<article>
<h1 class="entry-title">Test WordPress Article</h1>
<div class="entry-content">
<p>%s</p>
</div>
</article>
</body>
</html>`, articleBody(250))

// drupalHTML is an HTML snapshot representative of a Drupal article page.
var drupalHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta name="generator" content="Drupal 10 (https://www.drupal.org)">
<meta property="og:type" content="article">
<title>Test Drupal Article</title>
</head>
<body>
<article>
<h1 class="page-title">Test Drupal Article</h1>
<div class="field--name-body">
<p>%s</p>
</div>
</article>
</body>
</html>`, articleBody(250))

// html5ArticleHTML is an HTML snapshot for a generic HTML5 page with og:type=article and <article>.
var html5ArticleHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta property="og:type" content="article">
<meta property="og:title" content="Test HTML5 Article">
<title>Test HTML5 Article</title>
</head>
<body>
<article>
<h1>Test HTML5 Article</h1>
<div class="entry-content">
<p>%s</p>
</div>
</article>
</body>
</html>`, articleBody(200))

// postmediaHTML is an HTML snapshot representative of a Postmedia article page.
var postmediaHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta property="og:type" content="article">
<title>Test Postmedia Article</title>
</head>
<body>
<article class="article-content">
<h1 class="article-title">Test Postmedia Article</h1>
<div class="article-content__content-group">
<p>%s</p>
</div>
</article>
</body>
</html>`, articleBody(250))

// torstarHTML is an HTML snapshot representative of a Toronto Star article page.
var torstarHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta property="og:type" content="article">
<title>Test Torstar Article</title>
</head>
<body>
<article>
<h1>Test Torstar Article</h1>
<div class="c-article-body__content">
<p>%s</p>
</div>
</article>
</body>
</html>`, articleBody(250))

// villageMediaHTML is an HTML snapshot representative of a Village Media article page.
var villageMediaHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta property="og:type" content="article">
<title>Test Village Media Article</title>
</head>
<body>
<div class="article-detail">
<h1 class="article-detail__title">Test Village Media Article</h1>
<div class="article-detail__body">
<p>%s</p>
</div>
</div>
</body>
</html>`, articleBody(250))

// blackPressHTML is an HTML snapshot representative of a Black Press article page.
var blackPressHTML = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta property="og:type" content="article">
<title>Test Black Press Article</title>
</head>
<body>
<article>
<h1>Test Black Press Article</h1>
<div class="article-body-text">
<p>%s</p>
</div>
</article>
</body>
</html>`, articleBody(250))

// listingHTML is an HTML snapshot of a typical news listing/index page with many links.
var listingHTML = func() string {
	const linkCount = 30
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html>
<html>
<head><title>News Index</title></head>
<body>
<main>
<h1>Latest News</h1>
<ul>
`)
	for i := range linkCount {
		fmt.Fprintf(&sb, `<li><a href="/story/%d">Story headline number %d</a></li>`+"\n", i, i)
	}
	sb.WriteString(`</ul>
</main>
</body>
</html>`)
	return sb.String()
}()
