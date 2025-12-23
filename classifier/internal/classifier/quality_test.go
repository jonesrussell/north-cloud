package classifier

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

func TestQualityScorer_Score_HighQuality(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	publishedDate := time.Now()
	raw := &domain.RawContent{
		ID:              "high-quality",
		Title:           "Comprehensive News Article",
		RawText:         string(make([]byte, 2000)), // Substantial content
		WordCount:       1200,                       // Excellent word count
		MetaDescription: "A detailed description of the article",
		MetaKeywords:    "news, breaking, important",
		OGTitle:         "Comprehensive News Article",
		OGDescription:   "A detailed description",
		OGImage:         "https://example.com/image.jpg",
		OGURL:           "https://example.com/article",
		OGType:          "article",
		CanonicalURL:    "https://example.com/article",
		PublishedDate:   &publishedDate,
	}

	result, err := scorer.Score(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// High quality content should score well (>75)
	if result.TotalScore < 75 {
		t.Errorf("expected high score (>75), got %d", result.TotalScore)
	}

	// Verify factors exist
	if result.Factors == nil {
		t.Fatal("expected factors map, got nil")
	}

	// Verify word count factor
	wordCountFactor, ok := result.Factors["word_count"].(map[string]interface{})
	if !ok {
		t.Fatal("expected word_count factor")
	}
	if wordCountFactor["score"].(int) != 25 {
		t.Errorf("expected word_count score 25 (max), got %d", wordCountFactor["score"].(int))
	}
}

func TestQualityScorer_Score_LowQuality(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	raw := &domain.RawContent{
		ID:        "low-quality",
		Title:     "Short",
		RawText:   "Very short content.",
		WordCount: 50, // Very low word count
		// No metadata, no OG tags, nothing
	}

	result, err := scorer.Score(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Low quality content should score poorly (<40)
	if result.TotalScore >= 40 {
		t.Errorf("expected low score (<40), got %d", result.TotalScore)
	}
}

func TestQualityScorer_Score_MediumQuality(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	raw := &domain.RawContent{
		ID:              "medium-quality",
		Title:           "Decent Article",
		RawText:         string(make([]byte, 800)),
		WordCount:       400,                       // Medium word count
		MetaDescription: "A brief description",
		OGImage:         "https://example.com/image.jpg",
	}

	result, err := scorer.Score(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Medium quality should score in middle range (40-75)
	if result.TotalScore < 40 || result.TotalScore > 75 {
		t.Errorf("expected medium score (40-75), got %d", result.TotalScore)
	}
}

func TestQualityScorer_CalculateWordCountScore(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	tests := []struct {
		name      string
		wordCount int
		expected  int
	}{
		{"very short", 50, 0},
		{"minimum threshold", 100, 10},
		{"short", 250, 10},
		{"medium", 400, 15},
		{"good", 700, 20},
		{"excellent", 1200, 25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.calculateWordCountScore(tt.wordCount)
			if score != tt.expected {
				t.Errorf("wordCount=%d: expected %d, got %d", tt.wordCount, tt.expected, score)
			}
		})
	}
}

func TestQualityScorer_CalculateMetadataScore(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	publishedDate := time.Now()

	tests := []struct {
		name     string
		raw      *domain.RawContent
		expected int
	}{
		{
			name: "no metadata",
			raw: &domain.RawContent{
				ID: "test",
			},
			expected: 0,
		},
		{
			name: "only title",
			raw: &domain.RawContent{
				ID:    "test",
				Title: "Test",
			},
			expected: 5,
		},
		{
			name: "title and description",
			raw: &domain.RawContent{
				ID:              "test",
				Title:           "Test",
				MetaDescription: "Description",
			},
			expected: 10,
		},
		{
			name: "full metadata",
			raw: &domain.RawContent{
				ID:              "test",
				Title:           "Test",
				MetaDescription: "Description",
				PublishedDate:   &publishedDate,
				OGTitle:         "OG Title",
				MetaKeywords:    "keywords",
			},
			expected: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.calculateMetadataScore(tt.raw)
			score := result["score"].(int)
			if score != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, score)
			}
		})
	}
}

func TestQualityScorer_CalculateRichnessScore(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	tests := []struct {
		name     string
		raw      *domain.RawContent
		expected int
	}{
		{
			name: "no richness",
			raw: &domain.RawContent{
				ID: "test",
			},
			expected: 0,
		},
		{
			name: "has image",
			raw: &domain.RawContent{
				ID:      "test",
				OGImage: "https://example.com/image.jpg",
			},
			expected: 10,
		},
		{
			name: "has keywords",
			raw: &domain.RawContent{
				ID:           "test",
				MetaKeywords: "keyword1, keyword2",
			},
			expected: 5,
		},
		{
			name: "full richness",
			raw: &domain.RawContent{
				ID:           "test",
				OGImage:      "https://example.com/image.jpg",
				MetaKeywords: "keywords",
				CanonicalURL: "https://example.com/canonical",
				OGType:       "article",
				OGURL:        "https://example.com/og",
			},
			expected: 25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scorer.calculateRichnessScore(tt.raw)
			score := result["score"].(int)
			if score != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, score)
			}
		})
	}
}

func TestQualityScorer_ScoreBatch(t *testing.T) {
	scorer := NewQualityScorer(&mockLogger{})

	rawItems := []*domain.RawContent{
		{
			ID:        "batch-1",
			Title:     "High Quality",
			WordCount: 1000,
			OGImage:   "image.jpg",
		},
		{
			ID:        "batch-2",
			Title:     "Low Quality",
			WordCount: 50,
		},
	}

	results, err := scorer.ScoreBatch(context.Background(), rawItems)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// First should score higher than second
	if results[0].TotalScore <= results[1].TotalScore {
		t.Errorf("expected first item to score higher: %d vs %d",
			results[0].TotalScore, results[1].TotalScore)
	}
}

func TestQualityScorer_CustomConfig(t *testing.T) {
	config := QualityConfig{
		WordCountWeight:   0.5, // Emphasize word count more
		MetadataWeight:    0.2,
		RichnessWeight:    0.2,
		ReadabilityWeight: 0.1,
		MinWordCount:      200,
		OptimalWordCount:  2000,
	}

	scorer := NewQualityScorerWithConfig(&mockLogger{}, config)

	raw := &domain.RawContent{
		ID:        "custom-config",
		Title:     "Test",
		WordCount: 1500, // Good word count should be weighted heavily
	}

	result, err := scorer.Score(context.Background(), raw)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With heavy word count weighting, should still get decent score
	if result.TotalScore < 30 {
		t.Errorf("expected decent score with custom config, got %d", result.TotalScore)
	}
}
