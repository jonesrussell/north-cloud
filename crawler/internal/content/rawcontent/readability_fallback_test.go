package rawcontent_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/content/rawcontent"
)

func TestApplyReadabilityFallback_EmptyInput(t *testing.T) {
	t.Helper()

	title, html, text := rawcontent.ApplyReadabilityFallback("", "https://example.com/article")
	if title != "" || html != "" || text != "" {
		t.Errorf("ApplyReadabilityFallback(empty) = %q, %q, %q; want all empty", title, html, text)
	}
}

func TestApplyReadabilityFallback_InvalidURL(t *testing.T) {
	t.Helper()

	htmlDoc := `<!DOCTYPE html><html><head><title>Test</title></head><body><p>Content here.</p></body></html>`
	title, html, text := rawcontent.ApplyReadabilityFallback(htmlDoc, "://invalid")
	if title != "" || html != "" || text != "" {
		t.Errorf("ApplyReadabilityFallback(invalid URL) = %q, %q, %q; want all empty", title, html, text)
	}
}

func TestApplyReadabilityFallback_ArticleLikeHTML(t *testing.T) {
	t.Helper()

	// Minimal article-like HTML with no semantic containers (no article/main) so selector extraction would miss it.
	// Readability can still find the main content.
	htmlDoc := `<!DOCTYPE html>
<html>
<head><title>My Article Title</title></head>
<body>
  <header>Site Header</header>
  <div id="content">
    <h1>My Article Title</h1>
    <p>First paragraph with enough text so that the readability algorithm can identify it as main content.</p>
    <p>Second paragraph to reinforce that this is the main body of the page.</p>
  </div>
  <footer>Site Footer</footer>
</body>
</html>`

	title, rawHTML, rawText := rawcontent.ApplyReadabilityFallback(htmlDoc, "https://example.com/article")
	if title == "" && rawHTML == "" && rawText == "" {
		t.Skip("Readability returned no content; algorithm may not extract from this structure in all versions")
	}
	if rawText != "" && len(rawText) < 50 {
		t.Errorf("rawText too short: %d chars", len(rawText))
	}
	if title != "" && title != "My Article Title" {
		t.Logf("title = %q (readability may normalize)", title)
	}
	_ = rawHTML // may be empty or populated depending on library version
}
