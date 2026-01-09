package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// Quality scoring constants
	maxQualityScore         = 100
	maxComponentScore       = 25
	wordCountThreshold300   = 300
	wordCountThreshold500   = 500
	wordCountThreshold200   = 200
	wordCountThreshold100   = 100
	wordCountScore10        = 10
	wordCountScore15        = 15
	wordCountScore20        = 20
	readabilityScore200     = 20 // 80% of max (20/25)
	readabilityScore100     = 15 // 60% of max
	readabilityScoreDefault = 10 // 40% of max
	qualityFactorCount      = 4  // Number of quality factors: word_count, metadata_completeness, content_richness, readability
	// Default quality config constants
	defaultQualityWeight025     = 0.25
	defaultMinWordCount100      = 100
	defaultOptimalWordCount1000 = 1000
)

// QualityScorer evaluates content quality on a 0-100 scale
type QualityScorer struct {
	logger infralogger.Logger
	config QualityConfig
}

// QualityConfig defines weights for different quality factors
type QualityConfig struct {
	WordCountWeight   float64 // Default: 0.25
	MetadataWeight    float64 // Default: 0.25
	RichnessWeight    float64 // Default: 0.25
	ReadabilityWeight float64 // Default: 0.25
	MinWordCount      int     // Minimum word count threshold
	OptimalWordCount  int     // Optimal word count for max score
}

// QualityResult represents the quality scoring result
type QualityResult struct {
	TotalScore int            `json:"total_score"` // 0-100
	Factors    map[string]any `json:"factors"`     // Breakdown of scores
}

// NewQualityScorer creates a new quality scorer with default config
func NewQualityScorer(logger infralogger.Logger) *QualityScorer {
	return &QualityScorer{
		logger: logger,
		config: QualityConfig{
			WordCountWeight:   defaultQualityWeight025,
			MetadataWeight:    defaultQualityWeight025,
			RichnessWeight:    defaultQualityWeight025,
			ReadabilityWeight: defaultQualityWeight025,
			MinWordCount:      defaultMinWordCount100,
			OptimalWordCount:  defaultOptimalWordCount1000,
		},
	}
}

// NewQualityScorerWithConfig creates a new quality scorer with custom config
func NewQualityScorerWithConfig(logger infralogger.Logger, config QualityConfig) *QualityScorer {
	return &QualityScorer{
		logger: logger,
		config: config,
	}
}

// Score calculates the quality score for the given content
func (q *QualityScorer) Score(ctx context.Context, raw *domain.RawContent) (*QualityResult, error) {
	factors := make(map[string]any, qualityFactorCount)

	// 1. Word count scoring (0-25 points)
	wordCountScore := q.calculateWordCountScore(raw.WordCount)
	factors["word_count"] = map[string]any{
		"value": raw.WordCount,
		"score": wordCountScore,
		"max":   maxComponentScore,
	}

	// 2. Metadata completeness (0-25 points)
	metadataScore := q.calculateMetadataScore(raw)
	factors["metadata_completeness"] = metadataScore

	// 3. Content richness (0-25 points)
	richnessScore := q.calculateRichnessScore(raw)
	factors["content_richness"] = richnessScore

	// 4. Readability (0-25 points)
	// For now, use a default mid-range score
	// Future: Implement Flesch-Kincaid or similar readability scoring
	readabilityScore := q.calculateReadabilityScore(raw)
	factors["readability"] = map[string]any{
		"score":  readabilityScore,
		"max":    maxComponentScore,
		"method": "default",
	}

	// Calculate total score (each component is 0-25, sum to 0-100)
	metadataScoreInt, ok := metadataScore["score"].(int)
	if !ok {
		metadataScoreInt = 0
	}
	richnessScoreInt, ok := richnessScore["score"].(int)
	if !ok {
		richnessScoreInt = 0
	}
	totalScore := wordCountScore +
		metadataScoreInt +
		richnessScoreInt +
		readabilityScore

	// Ensure score is within 0-100 range
	if totalScore < 0 {
		totalScore = 0
	}
	if totalScore > maxQualityScore {
		totalScore = maxQualityScore
	}

	q.logger.Debug("Quality score calculated",
		infralogger.String("content_id", raw.ID),
		infralogger.Int("total_score", totalScore),
		infralogger.Int("word_count", raw.WordCount),
	)

	return &QualityResult{
		TotalScore: totalScore,
		Factors:    factors,
	}, nil
}

// calculateWordCountScore scores based on word count (0-25 points)
func (q *QualityScorer) calculateWordCountScore(wordCount int) int {
	// Scoring tiers:
	// < 100 words: 0 points
	// 100-300: 10 points
	// 300-500: 15 points
	// 500-1000: 20 points
	// 1000+: 25 points

	if wordCount < q.config.MinWordCount {
		return 0
	}
	if wordCount < wordCountThreshold300 {
		return wordCountScore10
	}
	if wordCount < wordCountThreshold500 {
		return wordCountScore15
	}
	if wordCount < q.config.OptimalWordCount {
		return wordCountScore20
	}
	return maxComponentScore
}

// calculateMetadataScore scores based on metadata completeness (0-25 points)
func (q *QualityScorer) calculateMetadataScore(raw *domain.RawContent) map[string]any {
	score := 0
	details := make(map[string]bool)

	// Title present (5 points)
	if raw.Title != "" {
		score += 5
		details["has_title"] = true
	}

	// Description/Intro present (5 points)
	if raw.MetaDescription != "" || raw.OGDescription != "" {
		score += 5
		details["has_description"] = true
	}

	// Published date present (5 points)
	if raw.PublishedDate != nil {
		score += 5
		details["has_published_date"] = true
	}

	// OG metadata present (5 points)
	if raw.OGTitle != "" || raw.OGImage != "" {
		score += 5
		details["has_og_metadata"] = true
	}

	// Keywords present (5 points)
	if raw.MetaKeywords != "" {
		score += 5
		details["has_keywords"] = true
	}

	return map[string]any{
		"score":   score,
		"max":     maxComponentScore,
		"details": details,
	}
}

// calculateRichnessScore scores based on content richness (0-25 points)
func (q *QualityScorer) calculateRichnessScore(raw *domain.RawContent) map[string]any {
	score := 0
	details := make(map[string]bool)

	// Has image (10 points)
	if raw.OGImage != "" {
		score += 10
		details["has_image"] = true
	}

	// Has keywords (5 points)
	if raw.MetaKeywords != "" {
		score += 5
		details["has_keywords"] = true
	}

	// Has canonical URL (5 points)
	if raw.CanonicalURL != "" {
		score += 5
		details["has_canonical_url"] = true
	}

	// Has structured OG metadata (5 points)
	if raw.OGType != "" && raw.OGURL != "" {
		score += 5
		details["has_structured_og"] = true
	}

	return map[string]any{
		"score":   score,
		"max":     maxComponentScore,
		"details": details,
	}
}

// calculateReadabilityScore scores based on readability (0-25 points)
// For now, returns a default mid-range score
// Future: Implement Flesch-Kincaid reading ease or similar metrics
func (q *QualityScorer) calculateReadabilityScore(raw *domain.RawContent) int {
	// Default mid-range score until we implement actual readability analysis
	// Future implementation would analyze:
	// - Average sentence length
	// - Average word length
	// - Syllables per word
	// - Flesch-Kincaid reading ease score

	// For now, give a decent score if we have substantial content
	if raw.WordCount >= wordCountThreshold200 {
		return readabilityScore200 // 80% of max (20/25)
	}
	if raw.WordCount >= wordCountThreshold100 {
		return readabilityScore100 // 60% of max
	}
	return readabilityScoreDefault // 40% of max
}

// ScoreBatch scores multiple content items efficiently
func (q *QualityScorer) ScoreBatch(ctx context.Context, rawItems []*domain.RawContent) ([]*QualityResult, error) {
	results := make([]*QualityResult, len(rawItems))

	for i, raw := range rawItems {
		result, err := q.Score(ctx, raw)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	return results, nil
}
