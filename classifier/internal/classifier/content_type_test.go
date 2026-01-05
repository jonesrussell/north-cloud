//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// mockLogger implements the Logger interface for testing
type mockLogger struct{}

func (m *mockLogger) Debug(msg string, keysAndValues ...interface{}) {}
func (m *mockLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (m *mockLogger) Error(msg string, keysAndValues ...interface{}) {}

func TestContentTypeClassifier_Classify_Article_ViaOG(t *testing.T) {
	classifier := NewContentTypeClassifier(&mockLogger{})

	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:            "test-1",
		URL:           "https://example.com/news/article",
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
			URL:           "https://example.com/news/article",
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

	// Should classify as page because missing published date (falls through to default)
	if result.Type != domain.ContentTypePage {
		t.Errorf("expected type %s, got %s", domain.ContentTypePage, result.Type)
	}

	// After our fix, this falls through to default method instead of og_metadata_validation
	if result.Method != "default" {
		t.Errorf("expected method default, got %s", result.Method)
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
		name        string
		url         string
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
