//nolint:testpackage // Testing internal classifier requires same package access
package classifier

import (
	"context"
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/testhelpers"
)

func TestSourceReputationScorer_Score_NewSource(t *testing.T) {
	db := testhelpers.NewMockSourceReputationDB()
	scorer := NewSourceReputationScorer(&mockLogger{}, db)

	result, err := scorer.Score(context.Background(), "example.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// New source should get default score (50)
	if result.Score != 50 {
		t.Errorf("expected default score 50, got %d", result.Score)
	}

	if result.Category != domain.SourceCategoryUnknown {
		t.Errorf("expected category unknown, got %s", result.Category)
	}

	if result.Rank != "moderate" {
		t.Errorf("expected rank moderate, got %s", result.Rank)
	}
}

func TestSourceReputationScorer_Score_EstablishedSource(t *testing.T) {
	db := testhelpers.NewMockSourceReputationDB()
	scorer := NewSourceReputationScorer(&mockLogger{}, db)

	// Pre-populate with an established source
	db.SetSource(&domain.SourceReputation{
		SourceName:          "trusted-news.com",
		Category:            domain.SourceCategoryNews,
		ReputationScore:     85,
		TotalArticles:       100,
		AverageQualityScore: 85.0,
		SpamCount:           2, // Very low spam ratio
	})

	result, err := scorer.Score(context.Background(), "trusted-news.com")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be high score with trusted rank
	if result.Score < 75 {
		t.Errorf("expected high score (>75), got %d", result.Score)
	}

	if result.Rank != "trusted" {
		t.Errorf("expected rank trusted, got %s", result.Rank)
	}

	if result.Category != domain.SourceCategoryNews {
		t.Errorf("expected category news, got %s", result.Category)
	}
}

func TestSourceReputationScorer_UpdateAfterClassification_HighQuality(t *testing.T) {
	db := testhelpers.NewMockSourceReputationDB()
	scorer := NewSourceReputationScorer(&mockLogger{}, db)

	sourceName := "example.com"

	// Classify a high-quality article
	err := scorer.UpdateAfterClassification(context.Background(), sourceName, 90, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check updated source
	source, _ := db.GetSource(context.Background(), sourceName)

	if source.TotalArticles != 1 {
		t.Errorf("expected total_articles 1, got %d", source.TotalArticles)
	}

	if source.AverageQualityScore != 90.0 {
		t.Errorf("expected avg quality 90.0, got %f", source.AverageQualityScore)
	}

	if source.SpamCount != 0 {
		t.Errorf("expected spam_count 0, got %d", source.SpamCount)
	}

	// Reputation should be boosted
	if source.ReputationScore < 80 {
		t.Errorf("expected high reputation (>80), got %d", source.ReputationScore)
	}
}

func TestSourceReputationScorer_UpdateAfterClassification_LowQuality(t *testing.T) {
	db := testhelpers.NewMockSourceReputationDB()
	scorer := NewSourceReputationScorer(&mockLogger{}, db)

	sourceName := "spam-site.com"

	// Classify a low-quality (spam) article
	err := scorer.UpdateAfterClassification(context.Background(), sourceName, 20, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check updated source
	source, _ := db.GetSource(context.Background(), sourceName)

	if source.TotalArticles != 1 {
		t.Errorf("expected total_articles 1, got %d", source.TotalArticles)
	}

	if source.SpamCount != 1 {
		t.Errorf("expected spam_count 1, got %d", source.SpamCount)
	}

	// Reputation should be low
	if source.ReputationScore >= 30 {
		t.Errorf("expected low reputation (<30), got %d", source.ReputationScore)
	}
}

func TestSourceReputationScorer_UpdateAfterClassification_Multiple(t *testing.T) {
	db := testhelpers.NewMockSourceReputationDB()
	scorer := NewSourceReputationScorer(&mockLogger{}, db)

	sourceName := "mixed-quality.com"

	// Classify multiple articles with varying quality
	articles := []struct {
		quality int
		isSpam  bool
	}{
		{80, false},
		{75, false},
		{90, false},
		{25, true}, // One spam article
		{85, false},
	}

	for _, article := range articles {
		err := scorer.UpdateAfterClassification(context.Background(), sourceName, article.quality, article.isSpam)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// Check final source state
	source, _ := db.GetSource(context.Background(), sourceName)

	if source.TotalArticles != 5 {
		t.Errorf("expected total_articles 5, got %d", source.TotalArticles)
	}

	if source.SpamCount != 1 {
		t.Errorf("expected spam_count 1, got %d", source.SpamCount)
	}

	// Average should be around (80+75+90+25+85)/5 = 71
	expectedAvg := (80.0 + 75.0 + 90.0 + 25.0 + 85.0) / 5.0
	if source.AverageQualityScore < expectedAvg-1 || source.AverageQualityScore > expectedAvg+1 {
		t.Errorf("expected avg quality ~%.1f, got %.1f", expectedAvg, source.AverageQualityScore)
	}

	// With 1 spam out of 5 articles (20% spam ratio), reputation should be decent but not excellent
	if source.ReputationScore < 50 || source.ReputationScore > 75 {
		t.Errorf("expected moderate reputation (50-75), got %d", source.ReputationScore)
	}
}

func TestSourceReputationScorer_DetermineRank(t *testing.T) {
	scorer := NewSourceReputationScorer(&mockLogger{}, testhelpers.NewMockSourceReputationDB())

	tests := []struct {
		name          string
		score         int
		totalArticles int
		expectedRank  string
	}{
		{"new high score", 85, 5, "moderate"},         // Not enough articles for trusted
		{"established high score", 85, 20, "trusted"}, // Enough articles for trusted
		{"moderate score", 60, 15, "moderate"},
		{"low score", 40, 10, "low"},
		{"spam score", 20, 5, "spam"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rank := scorer.determineRank(tt.score, tt.totalArticles)
			if rank != tt.expectedRank {
				t.Errorf("expected rank %s, got %s", tt.expectedRank, rank)
			}
		})
	}
}

func TestSourceReputationScorer_CalculateReputationScore(t *testing.T) {
	scorer := NewSourceReputationScorer(&mockLogger{}, testhelpers.NewMockSourceReputationDB())

	tests := []struct {
		name     string
		source   *domain.SourceReputation
		minScore int
		maxScore int
	}{
		{
			name: "no articles (default score)",
			source: &domain.SourceReputation{
				TotalArticles:       0,
				AverageQualityScore: 0,
				SpamCount:           0,
			},
			minScore: 50,
			maxScore: 50,
		},
		{
			name: "high quality, no spam",
			source: &domain.SourceReputation{
				TotalArticles:       100,
				AverageQualityScore: 90.0,
				SpamCount:           0,
			},
			minScore: 85,
			maxScore: 100,
		},
		{
			name: "low quality with spam",
			source: &domain.SourceReputation{
				TotalArticles:       100,
				AverageQualityScore: 40.0,
				SpamCount:           30, // 30% spam
			},
			minScore: 0,
			maxScore: 40,
		},
		{
			name: "good quality, some spam",
			source: &domain.SourceReputation{
				TotalArticles:       50,
				AverageQualityScore: 70.0,
				SpamCount:           5, // 10% spam
			},
			minScore: 60,
			maxScore: 75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.calculateReputationScore(tt.source)
			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("expected score between %d and %d, got %d", tt.minScore, tt.maxScore, score)
			}
		})
	}
}

func TestSourceReputationScorer_ScoreBatch(t *testing.T) {
	db := testhelpers.NewMockSourceReputationDB()
	scorer := NewSourceReputationScorer(&mockLogger{}, db)

	// Pre-populate some sources
	db.SetSource(&domain.SourceReputation{
		SourceName:          "source1.com",
		ReputationScore:     80,
		TotalArticles:       50,
		AverageQualityScore: 80.0,
		SpamCount:           0, // No spam - qualifies for trust boost
	})
	db.SetSource(&domain.SourceReputation{
		SourceName:          "source2.com",
		ReputationScore:     40,
		TotalArticles:       20,
		AverageQualityScore: 40.0,
	})

	sourceNames := []string{"source1.com", "source2.com", "source3.com"}
	results, err := scorer.ScoreBatch(context.Background(), sourceNames)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify first source (high score with trust boost: 80 * 1.1 = 88)
	if results[0].Score != 88 {
		t.Errorf("expected source1 score 88 (with trust boost), got %d", results[0].Score)
	}

	// Verify second source (low score)
	if results[1].Score != 40 {
		t.Errorf("expected source2 score 40, got %d", results[1].Score)
	}

	// Verify third source (new, default score)
	if results[2].Score != 50 {
		t.Errorf("expected source3 default score 50, got %d", results[2].Score)
	}
}
