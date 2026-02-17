package fetcher_test

import (
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
)

const (
	testSourceID = "test-source-id"
	testPageURL  = "https://example.com/article/1"
)

// fullArticleHTML is a complete article page with title, meta description, author, and article body.
const fullArticleHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Breaking News: Test Article</title>
  <meta name="description" content="A test article description.">
  <meta name="author" content="Jane Doe">
</head>
<body>
  <nav>Navigation links</nav>
  <article>
    <h1>Breaking News: Test Article</h1>
    <p>This is the article body text for testing purposes.</p>
  </article>
  <footer>Footer content</footer>
</body>
</html>`

// ogTitleHTML has no <title> tag but has an og:title meta tag.
const ogTitleHTML = `<!DOCTYPE html>
<html>
<head>
  <meta property="og:title" content="OG Title Fallback">
  <meta property="og:description" content="OG description fallback.">
</head>
<body>
  <p>Some body content here.</p>
</body>
</html>`

// articlePreferredHTML has content in both <article> and outside it in <body>.
const articlePreferredHTML = `<!DOCTYPE html>
<html>
<head><title>Page Title</title></head>
<body>
  <div>Body-level sidebar content that should be ignored.</div>
  <article>
    <p>Article-specific content that should be extracted.</p>
  </article>
  <div>More body-level content to ignore.</div>
</body>
</html>`

// scriptStyleHTML has script and style elements embedded in the body.
const scriptStyleHTML = `<!DOCTYPE html>
<html>
<head><title>Script Test</title></head>
<body>
  <p>Visible text content.</p>
  <script>var x = "should not appear";</script>
  <style>.hidden { display: none; }</style>
  <p>More visible text.</p>
</body>
</html>`

// minimalHTML is a minimal valid HTML document with an empty body.
const minimalHTML = `<!DOCTYPE html>
<html>
<head></head>
<body></body>
</html>`

func newExtractor(t *testing.T) *fetcher.ContentExtractor {
	t.Helper()

	return fetcher.NewContentExtractor()
}

func TestExtract_FullArticle(t *testing.T) {
	t.Parallel()

	ext := newExtractor(t)

	content, err := ext.Extract(testSourceID, testPageURL, []byte(fullArticleHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Title", "Breaking News: Test Article", content.Title)
	assertEqual(t, "Description", "A test article description.", content.Description)
	assertEqual(t, "Author", "Jane Doe", content.Author)
	assertEqual(t, "URL", testPageURL, content.URL)
	assertEqual(t, "SourceID", testSourceID, content.SourceID)

	assertBodyContains(t, content.Body, "article body text for testing")
	assertBodyNotContains(t, content.Body, "Navigation links")
	assertBodyNotContains(t, content.Body, "Footer content")
	assertNonEmpty(t, "ContentHash", content.ContentHash)
}

func TestExtract_TitleFallbackToOGTitle(t *testing.T) {
	t.Parallel()

	ext := newExtractor(t)

	content, err := ext.Extract(testSourceID, testPageURL, []byte(ogTitleHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Title", "OG Title Fallback", content.Title)
	assertEqual(t, "Description", "OG description fallback.", content.Description)
}

func TestExtract_ArticleElementPreferred(t *testing.T) {
	t.Parallel()

	ext := newExtractor(t)

	content, err := ext.Extract(testSourceID, testPageURL, []byte(articlePreferredHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertBodyContains(t, content.Body, "Article-specific content")
	assertBodyNotContains(t, content.Body, "sidebar content that should be ignored")
	assertBodyNotContains(t, content.Body, "More body-level content to ignore")
}

func TestExtract_ScriptsAndStylesStripped(t *testing.T) {
	t.Parallel()

	ext := newExtractor(t)

	content, err := ext.Extract(testSourceID, testPageURL, []byte(scriptStyleHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertBodyContains(t, content.Body, "Visible text content.")
	assertBodyContains(t, content.Body, "More visible text.")
	assertBodyNotContains(t, content.Body, "should not appear")
	assertBodyNotContains(t, content.Body, "display: none")
}

func TestExtract_ContentHashComputed(t *testing.T) {
	t.Parallel()

	ext := newExtractor(t)

	content1, err := ext.Extract(testSourceID, testPageURL, []byte(fullArticleHTML))
	if err != nil {
		t.Fatalf("unexpected error extracting content1: %v", err)
	}

	content2, err := ext.Extract(testSourceID, testPageURL, []byte(scriptStyleHTML))
	if err != nil {
		t.Fatalf("unexpected error extracting content2: %v", err)
	}

	assertNonEmpty(t, "ContentHash1", content1.ContentHash)
	assertNonEmpty(t, "ContentHash2", content2.ContentHash)

	if content1.ContentHash == content2.ContentHash {
		t.Error("expected different hashes for different body text")
	}
}

func TestExtract_EmptyBody(t *testing.T) {
	t.Parallel()

	ext := newExtractor(t)

	content, err := ext.Extract(testSourceID, testPageURL, []byte(minimalHTML))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertEqual(t, "Title", "", content.Title)
	assertEqual(t, "Body", "", content.Body)
	assertNonEmpty(t, "ContentHash", content.ContentHash)
}

// --- test helpers ---

func assertEqual(t *testing.T, field, expected, actual string) {
	t.Helper()

	if actual != expected {
		t.Errorf("%s: expected %q, got %q", field, expected, actual)
	}
}

func assertBodyContains(t *testing.T, body, needle string) {
	t.Helper()

	if !strings.Contains(body, needle) {
		t.Errorf("Body: expected to contain %q, got %q", needle, body)
	}
}

func assertBodyNotContains(t *testing.T, body, needle string) {
	t.Helper()

	if strings.Contains(body, needle) {
		t.Errorf("Body: expected NOT to contain %q, but it did", needle)
	}
}

func assertNonEmpty(t *testing.T, field, value string) {
	t.Helper()

	if value == "" {
		t.Errorf("%s: expected non-empty value", field)
	}
}
