//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// mockLogger implements the Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) Info(msg string, fields ...infralogger.Field)        {}
func (m *mockLogger) Warn(msg string, fields ...infralogger.Field)        {}
func (m *mockLogger) Error(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) Fatal(msg string, fields ...infralogger.Field)       {}
func (m *mockLogger) With(fields ...infralogger.Field) infralogger.Logger { return m }
func (m *mockLogger) Sync() error                                         { return nil }

func TestContentTypeClassifier_Classify_Article_ViaOG(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:            "test-1",
		URL:           "https://example.com/story/article",
		Title:         "Test Article",
		RawText:       "This is a test article with enough content to be classified as an article.",
		OGType:        "article",
		WordCount:     300,
		PublishedDate: &publishedDate,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypeArticle {
		t.Errorf("expected type %s, got %s", domain.ContentTypeArticle, result.Type)
	}

	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}

	if result.Method != "og_metadata" {
		t.Errorf("expected method og_metadata, got %s", result.Method)
	}
}

func TestContentTypeClassifier_Classify_Video_ViaOG(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-video",
		Title:     "Test Video",
		RawText:   "This is a video content.",
		OGType:    "video",
		WordCount: 50,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypeVideo {
		t.Errorf("expected type %s, got %s", domain.ContentTypeVideo, result.Type)
	}

	if result.Confidence != 1.0 {
		t.Errorf("expected confidence 1.0, got %f", result.Confidence)
	}
}

func TestContentTypeClassifier_Classify_Article_ViaHeuristics(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:              "test-heuristic",
		Title:           "Breaking News Story",
		RawText:         string(make([]byte, 1000)), // 1000 bytes of content
		WordCount:       250,                        // Above minimum threshold
		MetaDescription: "This is a news article about current events",
		PublishedDate:   &publishedDate,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypeArticle {
		t.Errorf("expected type %s, got %s", domain.ContentTypeArticle, result.Type)
	}

	if result.Confidence != 0.75 {
		t.Errorf("expected confidence 0.75, got %f", result.Confidence)
	}

	if result.Method != "heuristic" {
		t.Errorf("expected method heuristic, got %s", result.Method)
	}
}

func TestContentTypeClassifier_Classify_Page_Default(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-page",
		Title:     "About Us",
		RawText:   "This is a short page with minimal content.",
		WordCount: 50, // Below article threshold
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}

	if result.Confidence != 0.6 {
		t.Errorf("expected confidence 0.6, got %f", result.Confidence)
	}

	if result.Method != "default" {
		t.Errorf("expected method default, got %s", result.Method)
	}
}

func TestContentTypeClassifier_Classify_Page_NoTitle(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-no-title",
		Title:     "", // No title
		RawText:   string(make([]byte, 1000)),
		WordCount: 250, // Enough words but no title
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to page because no title
	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}
}

func TestContentTypeClassifier_ClassifyBatch(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()
	rawItems := []*domain.RawContent{
		{
			ID:            "batch-1",
			URL:           "https://example.com/story/article",
			Title:         "Article 1",
			OGType:        "article",
			WordCount:     300,
			PublishedDate: &publishedDate,
		},
		{
			ID:        "batch-2",
			URL:       "https://example.com/video",
			Title:     "Video 1",
			OGType:    "video",
			WordCount: 50,
		},
		{
			ID:        "batch-3",
			URL:       "https://example.com/page",
			Title:     "Page 1",
			WordCount: 50,
		},
	}

	results, err := classifier.ClassifyBatch(context.Background(), rawItems)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify first result (article)
	if results[0].Type != domain.ContentTypeArticle {
		t.Errorf("batch[0]: expected type %s, got %s", domain.ContentTypeArticle, results[0].Type)
	}

	// Verify second result (video)
	if results[1].Type != domain.ContentTypeVideo {
		t.Errorf("batch[1]: expected type %s, got %s", domain.ContentTypeVideo, results[1].Type)
	}

	// Verify third result (page)
	if results[2].Type != domain.ContentTypePage {
		t.Errorf("batch[2]: expected type %s, got %s", domain.ContentTypePage, results[2].Type)
	}
}

func TestContentTypeClassifier_HasArticleCharacteristics(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()
	tests := []struct {
		name     string
		raw      *domain.RawContent
		expected bool
	}{
		{
			name: "has all characteristics",
			raw: &domain.RawContent{
				Title:           "Great Article",
				WordCount:       250,
				MetaDescription: "A description",
				PublishedDate:   &publishedDate,
			},
			expected: true,
		},
		{
			name: "missing title",
			raw: &domain.RawContent{
				Title:           "",
				WordCount:       250,
				MetaDescription: "A description",
				PublishedDate:   &publishedDate,
			},
			expected: false,
		},
		{
			name: "insufficient word count",
			raw: &domain.RawContent{
				Title:           "Short",
				WordCount:       100,
				MetaDescription: "A description",
				PublishedDate:   &publishedDate,
			},
			expected: false,
		},
		{
			name: "missing published date",
			raw: &domain.RawContent{
				Title:           "Article",
				WordCount:       250,
				MetaDescription: "A description",
				// No published date
			},
			expected: false,
		},
		{
			name: "missing description",
			raw: &domain.RawContent{
				Title:         "Article",
				WordCount:     250,
				PublishedDate: &publishedDate,
				// No description
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifier.hasArticleCharacteristics(tt.raw)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestContentTypeClassifier_Classify_LoginPage(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-login",
		URL:       "https://example.com/account/login",
		Title:     "Login Page",
		RawText:   "Please log in to continue.",
		WordCount: 250,       // Enough words
		OGType:    "article", // Site incorrectly sets OGType
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}

	if result.Method != "url_exclusion" {
		t.Errorf("expected method url_exclusion, got %s", result.Method)
	}
}

func TestContentTypeClassifier_Classify_ClassifiedsPage(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-classifieds",
		URL:       "https://example.com/classifieds/buy-sell",
		Title:     "Classifieds",
		RawText:   "Browse our classified ads.",
		WordCount: 300,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}
}

func TestContentTypeClassifier_Classify_CategoryPage(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-category",
		URL:       "https://example.com/category/news",
		Title:     "News Category",
		RawText:   "Browse news articles in this category.",
		WordCount: 250,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}
}

func TestContentTypeClassifier_Classify_Homepage(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-homepage",
		URL:       "https://example.com/",
		Title:     "Homepage",
		RawText:   "Welcome to our website.",
		WordCount: 300,
		OGType:    "article", // Site incorrectly sets OGType
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}
}

func TestContentTypeClassifier_Classify_OGTypeWithoutDate(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "test-og-no-date",
		URL:       "https://example.com/some-page",
		Title:     "Some Page",
		RawText:   "Content here.",
		OGType:    "article", // OGType says article but no published date
		WordCount: 300,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OGType is authoritative - if it says "article", we trust it regardless of other fields
	// This is the current behavior: OGType takes precedence over heuristics
	if result.Type != domain.ContentTypeArticle {
		t.Errorf("expected type %s, got %s (OGType is authoritative)", domain.ContentTypeArticle, result.Type)
	}

	// Should use og_metadata method since OGType is set
	if result.Method != "og_metadata" {
		t.Errorf("expected method og_metadata, got %s", result.Method)
	}
}

func TestContentTypeClassifier_Classify_OGTypeWebsite(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:              "test-og-website",
		URL:             "https://example.com/article",
		Title:           "Article Title",
		RawText:         "Article content here.",
		OGType:          "website", // Default OGType, should use heuristics instead
		WordCount:       250,
		MetaDescription: "Article description",
		PublishedDate:   &publishedDate,
	}

	result, err := classifier.Classify(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should classify as article via heuristics (has published date + description)
	if result.Type != domain.ContentTypeArticle {
		t.Errorf("expected type %s, got %s", domain.ContentTypeArticle, result.Type)
	}

	if result.Method != "heuristic" {
		t.Errorf("expected method heuristic, got %s", result.Method)
	}
}

// Test new matchesURLPattern helper function
func TestMatchesURLPattern(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pattern string
		want    bool
	}{
		// Test without trailing slashes
		{
			name:    "classifieds exact match",
			path:    "/classifieds",
			pattern: "/classifieds",
			want:    true,
		},
		{
			name:    "classifieds with subpath",
			path:    "/classifieds/job-listings",
			pattern: "/classifieds",
			want:    true,
		},
		{
			name:    "directory with subpath",
			path:    "/directory/some-business",
			pattern: "/directory",
			want:    true,
		},
		{
			name:    "submissions with subpath",
			path:    "/submissions/newstip",
			pattern: "/submissions",
			want:    true,
		},
		{
			name:    "news article should not match classifieds",
			path:    "/local-news/article-title",
			pattern: "/classifieds",
			want:    false,
		},
		// Test with trailing slashes in pattern
		{
			name:    "pattern with slash matches prefix",
			path:    "/classifieds/job-listings",
			pattern: "/classifieds/",
			want:    true,
		},
		{
			name:    "pattern without slash matches subpath",
			path:    "/account/settings",
			pattern: "/account",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesURLPattern(tt.path, tt.pattern)
			if got != tt.want {
				t.Errorf("matchesURLPattern(%q, %q) = %v, want %v", tt.path, tt.pattern, got, tt.want)
			}
		})
	}
}

// Test baytoday.ca specific URLs with new patterns
func TestContentTypeClassifier_BaytodayURLs(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	tests := []struct {
		name         string
		url          string
		expectedType string
	}{
		{
			name:         "classifieds homepage (no trailing slash)",
			url:          "https://www.baytoday.ca/classifieds",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "classifieds job listings",
			url:          "https://www.baytoday.ca/classifieds/job-listings",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "directory page",
			url:          "https://www.baytoday.ca/directory/health-care/wellwise",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "submissions page",
			url:          "https://www.baytoday.ca/submissions/newstip",
			expectedType: domain.ContentTypePage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:        "test-" + tt.name,
				URL:       tt.url,
				Title:     "Test Page",
				RawText:   "Some content",
				WordCount: 250,
			}

			result, err := classifier.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Type != tt.expectedType {
				t.Errorf("URL %s: expected type %s, got %s (method: %s)", tt.url, tt.expectedType, result.Type, result.Method)
			}
		})
	}
}

// Test pagination query parameter detection
func TestContentTypeClassifier_PaginationQueryParams(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	tests := []struct {
		name         string
		url          string
		expectedType string
	}{
		{
			name:         "pagination with page=5",
			url:          "https://www.sudbury.com/ontario-news?page=5",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "pagination with p=2",
			url:          "https://example.com/news?p=2",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "pagination with pagenum=3",
			url:          "https://example.com/articles?pagenum=3",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "pagination with offset=20",
			url:          "https://example.com/stories?offset=20",
			expectedType: domain.ContentTypePage,
		},
		{
			name:         "no pagination - regular article",
			url:          "https://example.com/story/article-title",
			expectedType: domain.ContentTypeArticle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishedDate := time.Now()
			raw := &domain.RawContent{
				ID:              "test-" + tt.name,
				URL:             tt.url,
				Title:           "Test Content",
				RawText:         "This is test content with enough words to potentially be an article.",
				WordCount:       250,
				MetaDescription: "Test description",
				PublishedDate:   &publishedDate,
			}

			result, err := classifier.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Type != tt.expectedType {
				t.Errorf("URL %s: expected type %s, got %s (method: %s, reason: %s)",
					tt.url, tt.expectedType, result.Type, result.Method, result.Reason)
			}
		})
	}
}

// Test listing page content detection
func TestContentTypeClassifier_ListingPageContent(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	tests := []struct {
		name         string
		rawText      string
		expectedType string
	}{
		{
			name: "listing page with multiple read more links",
			rawText: `Toronto police investigating after second incident of mezuzahs stolen
			TORONTO — Toronto police are investigating after multiple mezuzahs were stolen outside Jewish homes for the second time this month.
			Read more >
			Future uncertain for Ontario college students as federal policy brings cuts, layoffs
			TORONTO — The tightening of Canada's international student regime has had ripple effects across higher education.
			Read more >
			Toronto police probing Christmas Eve storefront collision that left one dead
			TORONTO — Toronto police have released more information about a Christmas Eve crash into a storefront.
			Read more >`,
			expectedType: domain.ContentTypePage,
		},
		{
			name: "listing page with multiple datelines",
			rawText: `TORONTO — First article summary here with some content.
			Dec 26, 2025 9:31 AM
			OTTAWA — Second article summary here with some content.
			Dec 26, 2025 4:00 AM
			TORONTO — Third article summary here with some content.
			Dec 25, 2025 11:11 AM
			ONTARIO — Fourth article summary here with some content.
			Dec 24, 2025 7:23 PM`,
			expectedType: domain.ContentTypePage,
		},
		{
			name: "listing page with multiple dates",
			rawText: `Article one summary here. Dec 26, 2025 9:31 AM
			Article two summary here. Dec 26, 2025 4:00 AM
			Article three summary here. Dec 25, 2025 11:11 AM
			Article four summary here. Dec 24, 2025 7:23 PM
			Article five summary here. Dec 24, 2025 6:07 PM
			Article six summary here. Dec 24, 2025 2:37 PM`,
			expectedType: domain.ContentTypePage,
		},
		{
			name: "regular article (not a listing page)",
			rawText: `This is a regular news article with a single topic and narrative.
			It has enough content to be classified as an article. The content flows
			coherently from one paragraph to the next, discussing a single subject
			in depth. There are no multiple article summaries or "Read more" links.
			This is the kind of content that should be classified as an article.`,
			expectedType: domain.ContentTypeArticle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishedDate := time.Now()
			raw := &domain.RawContent{
				ID:              "test-" + tt.name,
				URL:             "https://example.com/content",
				Title:           "Test Content",
				RawText:         tt.rawText,
				WordCount:       300, // Enough words to potentially be an article
				MetaDescription: "Test description",
				PublishedDate:   &publishedDate,
			}

			result, err := classifier.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Type != tt.expectedType {
				t.Errorf("expected type %s, got %s (method: %s, reason: %s)",
					tt.expectedType, result.Type, result.Method, result.Reason)
			}
		})
	}
}

// TestContentTypeClassifier_SectionURLExclusion verifies that section index pages
// are excluded but articles within those sections are NOT excluded.
func TestContentTypeClassifier_SectionURLExclusion(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()

	tests := []struct {
		name           string
		url            string
		expectedType   string
		expectedMethod string
	}{
		// Section index pages should be excluded
		{
			name:           "news index page",
			url:            "https://example.com/news",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "news index page with trailing slash",
			url:            "https://example.com/news/",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "articles index page",
			url:            "https://example.com/articles",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "blog index page",
			url:            "https://example.com/blog",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "local-news index page",
			url:            "https://example.com/local-news",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "ontario-news index page",
			url:            "https://example.com/ontario-news",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "breaking-news index page",
			url:            "https://example.com/breaking-news",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "stories index page",
			url:            "https://example.com/stories",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "posts index page",
			url:            "https://example.com/posts",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},

		// Articles WITHIN section paths should NOT be excluded by URL
		{
			name:           "article in news section",
			url:            "https://example.com/news/six-men-charged-drug-bust",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in local-news section",
			url:            "https://example.com/local-news/fire-downtown-causes-evacuations",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in ontario-news section",
			url:            "https://www.sudbury.com/ontario-news/man-arrested-after-standoff",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in breaking-news section",
			url:            "https://example.com/breaking-news/highway-closed-due-to-collision",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in articles section",
			url:            "https://example.com/articles/climate-change-report-2026",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in blog section",
			url:            "https://example.com/blog/my-first-post",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in stories section",
			url:            "https://example.com/stories/community-hero-saves-child",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},
		{
			name:           "article in posts section",
			url:            "https://example.com/posts/weekly-update",
			expectedType:   domain.ContentTypeArticle,
			expectedMethod: "heuristic",
		},

		// Always-excluded paths should still match as prefixes
		{
			name:           "account subpath still excluded",
			url:            "https://example.com/account/settings",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "classifieds subpath still excluded",
			url:            "https://example.com/classifieds/job-listings/plumber",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "directory subpath still excluded",
			url:            "https://example.com/directory/health-care/wellwise",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "login subpath still excluded",
			url:            "https://example.com/login/reset-password",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "category subpath still excluded",
			url:            "https://example.com/category/sports",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
		{
			name:           "search subpath still excluded",
			url:            "https://example.com/search/results",
			expectedType:   domain.ContentTypePage,
			expectedMethod: "url_exclusion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &domain.RawContent{
				ID:              "test-section-" + tt.name,
				URL:             tt.url,
				Title:           "Test Article Title",
				RawText:         "This is a test article with substantial content to be classified.",
				WordCount:       300,
				MetaDescription: "Test article description for classification",
				PublishedDate:   &publishedDate,
			}

			result, err := classifier.Classify(context.Background(), raw)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.Type != tt.expectedType {
				t.Errorf("URL %s: expected type %s, got %s (method: %s)",
					tt.url, tt.expectedType, result.Type, result.Method)
			}

			if result.Method != tt.expectedMethod {
				t.Errorf("URL %s: expected method %s, got %s",
					tt.url, tt.expectedMethod, result.Method)
			}
		})
	}
}
