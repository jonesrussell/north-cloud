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

	raw := &domain.RawContent{
		ID:        "test-1",
		Title:     "Test Article",
		RawText:   "This is a test article with enough content to be classified as an article.",
		OGType:    "article",
		WordCount: 300,
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

	rawItems := []*domain.RawContent{
		{
			ID:        "batch-1",
			Title:     "Article 1",
			OGType:    "article",
			WordCount: 300,
		},
		{
			ID:        "batch-2",
			Title:     "Video 1",
			OGType:    "video",
			WordCount: 50,
		},
		{
			ID:        "batch-3",
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
			},
			expected: true,
		},
		{
			name: "missing title",
			raw: &domain.RawContent{
				Title:           "",
				WordCount:       250,
				MetaDescription: "A description",
			},
			expected: false,
		},
		{
			name: "insufficient word count",
			raw: &domain.RawContent{
				Title:           "Short",
				WordCount:       100,
				MetaDescription: "A description",
			},
			expected: false,
		},
		{
			name: "no additional indicators",
			raw: &domain.RawContent{
				Title:     "Article",
				WordCount: 250,
				// No description, author, or date
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
