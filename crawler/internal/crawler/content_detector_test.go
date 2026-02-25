package crawler_test

import (
	"regexp"
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
)

// ---------- isContentURL tests ----------

func TestIsContentURL_MatchesExplicitPattern(t *testing.T) {
	t.Parallel()

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/news/\d+`),
	}

	if !crawler.IsContentURL("https://example.com/news/12345", patterns) {
		t.Fatal("expected URL matching explicit pattern to be detected as content")
	}
}

func TestIsContentURL_NoMatchWithExplicitPatterns(t *testing.T) {
	t.Parallel()

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/news/\d+`),
	}

	if crawler.IsContentURL("https://example.com/about", patterns) {
		t.Fatal("expected URL not matching explicit pattern to be rejected")
	}
}

func TestIsContentURL_DatePatternInURL(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/2026/02/14/breaking-news-headline",
		"https://example.com/2026/02/breaking-news-headline",
	}
	for _, u := range urls {
		if !crawler.IsContentURL(u, nil) {
			t.Fatalf("expected date-based URL to be detected as content: %s", u)
		}
	}
}

func TestIsContentURL_ArticlePathSegments(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/article/some-content",
		"https://example.com/news/some-content",
		"https://example.com/story/some-content",
		"https://example.com/post/some-content",
	}
	for _, u := range urls {
		if !crawler.IsContentURL(u, nil) {
			t.Fatalf("expected content path segment URL to be detected: %s", u)
		}
	}
}

func TestIsContentURL_HomepageReturnsFalse(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com",
		"https://example.com/",
	}
	for _, u := range urls {
		if crawler.IsContentURL(u, nil) {
			t.Fatalf("expected homepage to be rejected: %s", u)
		}
	}
}

func TestIsContentURL_SingleSegmentCategoryReturnsFalse(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/sports",
		"https://example.com/politics",
		"https://example.com/entertainment",
	}
	for _, u := range urls {
		if crawler.IsContentURL(u, nil) {
			t.Fatalf("expected single-segment category page to be rejected: %s", u)
		}
	}
}

func TestIsContentURL_NonArticleURLsFiltered(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/login",
		"https://example.com/signup",
		"https://example.com/search",
		"https://example.com/contact",
		"https://example.com/about",
		"https://example.com/privacy",
		"https://example.com/terms",
		"https://example.com/tag/golang",
		"https://example.com/category/tech",
		"https://example.com/author/john",
		"https://example.com/page/2",
		"https://example.com/files/report.pdf",
		"https://example.com/data.xml",
		"https://example.com/api/data.json",
		"https://example.com/style.css",
		"https://example.com/app.js",
		"https://example.com/logo.png",
		"https://example.com/photo.jpg",
	}
	for _, u := range urls {
		if crawler.IsContentURL(u, nil) {
			t.Fatalf("expected non-content URL to be filtered out: %s", u)
		}
	}
}

func TestIsContentURL_SlugLikePathsWithHyphens(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/this-is-a-headline",
		"https://example.com/section/breaking-news-from-the-city",
	}
	for _, u := range urls {
		if !crawler.IsContentURL(u, nil) {
			t.Fatalf("expected slug-like URL with 3+ hyphens to be detected: %s", u)
		}
	}
}

// ---------- isContentPage tests ----------

func TestIsContentPage_OGTypeArticle(t *testing.T) {
	t.Parallel()

	if !crawler.IsContentPage("article", false) {
		t.Fatal("expected og:type 'article' to return true")
	}
}

func TestIsContentPage_OGTypeArticleCaseInsensitive(t *testing.T) {
	t.Parallel()

	if !crawler.IsContentPage("Article", false) {
		t.Fatal("expected og:type 'Article' (case insensitive) to return true")
	}
}

func TestIsContentPage_HasNewsArticleJSONLD(t *testing.T) {
	t.Parallel()

	if !crawler.IsContentPage("", true) {
		t.Fatal("expected hasNewsArticleJSONLD=true to return true")
	}
}

func TestIsContentPage_BothFalseReturnsFalse(t *testing.T) {
	t.Parallel()

	if crawler.IsContentPage("", false) {
		t.Fatal("expected both false to return false")
	}
	if crawler.IsContentPage("website", false) {
		t.Fatal("expected og:type 'website' without JSON-LD to return false")
	}
}

// ---------- compileContentPatterns tests ----------

func TestCompileContentPatterns_CompilesValidPatterns(t *testing.T) {
	t.Parallel()

	patterns := crawler.CompileContentPatterns([]string{
		`/news/\d+`,
		`/article/.*`,
	})

	expectedCount := 2
	if len(patterns) != expectedCount {
		t.Fatalf("expected %d compiled patterns, got %d", expectedCount, len(patterns))
	}

	if !patterns[0].MatchString("/news/123") {
		t.Fatal("expected first pattern to match /news/123")
	}
	if !patterns[1].MatchString("/article/headline") {
		t.Fatal("expected second pattern to match /article/headline")
	}
}

func TestCompileContentPatterns_SkipsInvalidPatterns(t *testing.T) {
	t.Parallel()

	patterns := crawler.CompileContentPatterns([]string{
		`/news/\d+`,
		`[invalid`,
		`/article/.*`,
	})

	expectedCount := 2
	if len(patterns) != expectedCount {
		t.Fatalf("expected %d compiled patterns (skipping invalid), got %d", expectedCount, len(patterns))
	}
}

func TestCompileContentPatterns_ReturnsNilForEmptyInput(t *testing.T) {
	t.Parallel()

	if patterns := crawler.CompileContentPatterns(nil); patterns != nil {
		t.Fatal("expected nil for nil input")
	}
	if patterns := crawler.CompileContentPatterns([]string{}); patterns != nil {
		t.Fatal("expected nil for empty slice input")
	}
}

// ---------- detectContentTypeFromURL tests ----------

func TestDetectContentTypeFromURL_PressRelease(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/press/2024/company-news",
		"https://example.com/media/releases/announcement",
		"https://example.com/newsroom/update",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentPressRelease {
			t.Fatalf("expected press_release for %s, got %s", u, ctype)
		}
	}
}

func TestDetectContentTypeFromURL_Event(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/events/concert-2024",
		"https://example.com/calendar/meetings",
		"https://example.com/upcoming/seminars",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentEvent {
			t.Fatalf("expected event for %s, got %s", u, ctype)
		}
	}
}

func TestDetectContentTypeFromURL_Advisory(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/alert/weather",
		"https://example.com/advisory/safety",
		"https://example.com/bulletin/missing-person",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentAdvisory {
			t.Fatalf("expected advisory for %s, got %s", u, ctype)
		}
	}
}

func TestDetectContentTypeFromURL_Report(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/reports/2024-annual",
		"https://example.com/report/quarterly",
		"https://example.com/docs/file.pdf",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentReport {
			t.Fatalf("expected report for %s, got %s", u, ctype)
		}
	}
}

func TestDetectContentTypeFromURL_Blotter(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/blotter/daily",
		"https://example.com/incidents/log",
		"https://example.com/arrests/summary",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentBlotter {
			t.Fatalf("expected blotter for %s, got %s", u, ctype)
		}
	}
}

func TestDetectContentTypeFromURL_CompanyAnnouncement(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/investors/update",
		"https://example.com/updates/news",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentCompanyAnnouncement {
			t.Fatalf("expected company_announcement for %s, got %s", u, ctype)
		}
	}
}

func TestDetectContentTypeFromURL_Unknown(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/about",
		"https://example.com/news/some-article",
	}
	for _, u := range urls {
		ctype := crawler.DetectContentTypeFromURL(u)
		if ctype != crawler.DetectedContentUnknown {
			t.Fatalf("expected unknown for %s, got %s", u, ctype)
		}
	}
}

// ---------- Extended content path segments tests ----------

func TestIsContentURL_NewPathSegments(t *testing.T) {
	t.Parallel()

	urls := []string{
		"https://example.com/press/release-2024",
		"https://example.com/events/upcoming-show",
		"https://example.com/alert/weather-warning",
		"https://example.com/blotter/daily-log",
		"https://example.com/investors/quarterly-results",
	}
	for _, u := range urls {
		if !crawler.IsContentURL(u, nil) {
			t.Fatalf("expected new path segment URL to be detected: %s", u)
		}
	}
}
