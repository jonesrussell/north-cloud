package rawcontent_test

import (
	"testing"
	"time"

	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
)

// BenchmarkExtractRawContent benchmarks HTML parsing and raw content extraction
func BenchmarkExtractRawContent(b *testing.B) {
	// Realistic test HTML with article structure
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Sample News Article - Breaking News</title>
    <meta property="og:title" content="Breaking: Major Event Unfolds Downtown" />
    <meta property="og:description" content="Detailed coverage of today's significant event that occurred in the city center." />
    <meta property="og:image" content="https://example.com/images/article.jpg" />
    <meta property="og:type" content="article" />
    <meta name="description" content="Full story with details and analysis" />
    <meta name="keywords" content="news, breaking, city, event" />
</head>
<body>
    <article>
        <h1>Breaking: Major Event Unfolds Downtown</h1>
        <p class="author">By Jane Reporter</p>
        <time datetime="2026-01-03T10:00:00Z">January 3, 2026</time>
        <div class="content">
            <p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.</p>
            <p>Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.</p>
            <p>Sed ut perspiciatis unde omnis iste natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta sunt explicabo.</p>
        </div>
    </article>
</body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		b.Fatalf("Failed to create test document: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Extract title
		_ = doc.Find("title").First().Text()

		// Extract OG tags
		_ = doc.Find("meta[property='og:title']").AttrOr("content", "")
		_ = doc.Find("meta[property='og:description']").AttrOr("content", "")
		_ = doc.Find("meta[property='og:image']").AttrOr("content", "")

		// Extract text content
		_ = doc.Find("article").Text()

		// Extract published date
		_ = doc.Find("time").AttrOr("datetime", "")
	}
}

// BenchmarkParseHTML benchmarks just the HTML parsing step
func BenchmarkParseHTML(b *testing.B) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test Article</title></head>
<body>
    <article>
        <h1>Headline</h1>
        <p>Content paragraph with some text.</p>
        <p>Another paragraph with more content.</p>
    </article>
</body>
</html>`

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		_, err := goquery.NewDocumentFromReader(strings.NewReader(html))
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkExtractMetadata benchmarks metadata extraction from HTML
func BenchmarkExtractMetadata(b *testing.B) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Sample Article Title</title>
    <meta property="og:title" content="OG Title" />
    <meta property="og:description" content="OG Description" />
    <meta property="og:image" content="https://example.com/image.jpg" />
    <meta property="og:type" content="article" />
    <meta property="og:url" content="https://example.com/article" />
    <meta name="description" content="Meta description" />
    <meta name="keywords" content="keyword1, keyword2, keyword3" />
    <meta name="author" content="John Doe" />
</head>
<body></body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		b.Fatalf("Failed to create test document: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		metadata := make(map[string]string)

		// Extract all OG tags
		doc.Find("meta[property^='og:']").Each(func(_ int, s *goquery.Selection) {
			property, _ := s.Attr("property")
			content, _ := s.Attr("content")
			metadata[property] = content
		})

		// Extract standard meta tags
		doc.Find("meta[name]").Each(func(_ int, s *goquery.Selection) {
			name, _ := s.Attr("name")
			content, _ := s.Attr("content")
			metadata[name] = content
		})
	}
}

// BenchmarkTextExtraction benchmarks extracting and cleaning text content
func BenchmarkTextExtraction(b *testing.B) {
	html := `<!DOCTYPE html>
<html>
<body>
    <article>
        <h1>Article Headline Goes Here</h1>
        <div class="content">
            <p>First paragraph with some interesting content about the topic at hand. This paragraph contains several sentences to make it realistic.</p>
            <p>Second paragraph continues the story with more details and analysis. We include enough text to make the benchmark meaningful.</p>
            <p>Third paragraph concludes the article with final thoughts and observations. More sentences ensure realistic text volume.</p>
            <p>Fourth paragraph adds additional context and background information that readers might find useful.</p>
        </div>
    </article>
</body>
</html>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		b.Fatalf("Failed to create test document: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Extract and clean text
		text := doc.Find("article").Text()
		_ = strings.TrimSpace(text)
	}
}

// Helper function to create test raw content (for other benchmarks)
func createTestRawContent() *storage.RawContent {
	publishedDate := time.Now().UTC()
	return &storage.RawContent{
		ID:                   "test-article-1",
		URL:                  "https://example.com/article",
		SourceName:           "test-source",
		Title:                "Test Article Title",
		RawText:              "This is the raw text content of the article with multiple sentences. It contains enough text to be realistic for benchmarking purposes.",
		RawHTML:              "<html><body><p>Test content</p></body></html>",
		PublishedDate:        &publishedDate,
		OGTitle:              "OG Title",
		OGDescription:        "OG Description",
		OGImage:              "https://example.com/image.jpg",
		OGType:               "article",
		ClassificationStatus: "pending",
		CrawledAt:            time.Now().UTC(),
		WordCount:            150,
	}
}

// BenchmarkRawContentValidation benchmarks validation of raw content structure
func BenchmarkRawContentValidation(b *testing.B) {
	rc := createTestRawContent()

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		// Simulate validation checks
		_ = rc.URL != ""
		_ = rc.Title != ""
		_ = rc.RawText != ""
		_ = rc.RawText != ""
		_ = rc.SourceName != ""
		_ = rc.ClassificationStatus != ""
	}
}
