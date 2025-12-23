package classifier

import (
	"context"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// SourceReputationScorer evaluates and tracks source trustworthiness
type SourceReputationScorer struct {
	logger Logger
	db     SourceReputationDB
	config SourceReputationConfig
}

// SourceReputationDB defines the interface for database operations
type SourceReputationDB interface {
	GetSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error)
	CreateSource(ctx context.Context, source *domain.SourceReputation) error
	UpdateSource(ctx context.Context, source *domain.SourceReputation) error
	GetOrCreateSource(ctx context.Context, sourceName string) (*domain.SourceReputation, error)
}

// SourceReputationConfig defines configuration for source reputation scoring
type SourceReputationConfig struct {
	DefaultScore               int     // Default score for new sources (0-100)
	UpdateOnEachClassification bool    // Whether to update reputation on each classification
	SpamThreshold              int     // Quality score below which content is considered spam
	MinArticlesForTrust        int     // Minimum articles before source is considered established
	ReputationDecayRate        float64 // Rate at which reputation decays for spam (0.0-1.0)
}

// SourceReputationResult represents the result of source reputation scoring
type SourceReputationResult struct {
	Score    int    `json:"score"`    // 0-100
	Category string `json:"category"` // "news", "blog", "government", "unknown"
	Rank     string `json:"rank"`     // "trusted", "moderate", "low", "spam"
}

// NewSourceReputationScorer creates a new source reputation scorer
func NewSourceReputationScorer(logger Logger, db SourceReputationDB) *SourceReputationScorer {
	return &SourceReputationScorer{
		logger: logger,
		db:     db,
		config: SourceReputationConfig{
			DefaultScore:               50,
			UpdateOnEachClassification: true,
			SpamThreshold:              30,
			MinArticlesForTrust:        10,
			ReputationDecayRate:        0.1,
		},
	}
}

// NewSourceReputationScorerWithConfig creates a scorer with custom config
func NewSourceReputationScorerWithConfig(logger Logger, db SourceReputationDB, config SourceReputationConfig) *SourceReputationScorer {
	return &SourceReputationScorer{
		logger: logger,
		db:     db,
		config: config,
	}
}

// Score retrieves or calculates the reputation score for a source
func (s *SourceReputationScorer) Score(ctx context.Context, sourceName string) (*SourceReputationResult, error) {
	// Get or create source record
	sourceRecord, err := s.db.GetOrCreateSource(ctx, sourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	// Calculate current reputation score
	score := s.calculateReputationScore(sourceRecord)

	// Determine category
	category := s.categorizeSource(sourceRecord)

	// Determine rank
	rank := s.determineRank(score, sourceRecord.TotalArticles)

	s.logger.Debug("Source reputation scored",
		"source_name", sourceName,
		"score", score,
		"category", category,
		"rank", rank,
		"total_articles", sourceRecord.TotalArticles,
	)

	return &SourceReputationResult{
		Score:    score,
		Category: category,
		Rank:     rank,
	}, nil
}

// UpdateAfterClassification updates source reputation after classifying an article
func (s *SourceReputationScorer) UpdateAfterClassification(
	ctx context.Context,
	sourceName string,
	qualityScore int,
	isSpam bool,
) error {
	if !s.config.UpdateOnEachClassification {
		return nil
	}

	sourceRecord, err := s.db.GetOrCreateSource(ctx, sourceName)
	if err != nil {
		return fmt.Errorf("failed to get source for update: %w", err)
	}

	// Update statistics
	sourceRecord.TotalArticles++

	// Update average quality score (rolling average)
	if sourceRecord.TotalArticles == 1 {
		sourceRecord.AverageQualityScore = float64(qualityScore)
	} else {
		sourceRecord.AverageQualityScore = (sourceRecord.AverageQualityScore*float64(sourceRecord.TotalArticles-1) +
			float64(qualityScore)) / float64(sourceRecord.TotalArticles)
	}

	// Update spam count
	if isSpam || qualityScore < s.config.SpamThreshold {
		sourceRecord.SpamCount++
	}

	// Recalculate reputation score
	sourceRecord.ReputationScore = s.calculateReputationScore(sourceRecord)

	// Save updated record
	if err := s.db.UpdateSource(ctx, sourceRecord); err != nil {
		return fmt.Errorf("failed to update source: %w", err)
	}

	s.logger.Debug("Source reputation updated",
		"source_name", sourceName,
		"new_score", sourceRecord.ReputationScore,
		"total_articles", sourceRecord.TotalArticles,
		"avg_quality", sourceRecord.AverageQualityScore,
		"spam_count", sourceRecord.SpamCount,
	)

	return nil
}

// calculateReputationScore calculates reputation based on historical data
func (s *SourceReputationScorer) calculateReputationScore(source *domain.SourceReputation) int {
	if source.TotalArticles == 0 {
		return s.config.DefaultScore
	}

	// Start with average quality score
	score := source.AverageQualityScore

	// Apply spam penalty
	if source.TotalArticles > 0 {
		spamRatio := float64(source.SpamCount) / float64(source.TotalArticles)
		score = score * (1.0 - spamRatio*s.config.ReputationDecayRate)
	}

	// Boost score for established sources with good track record
	if source.TotalArticles >= s.config.MinArticlesForTrust {
		trustSpamRatio := float64(source.SpamCount) / float64(source.TotalArticles)
		if source.AverageQualityScore >= 70 && trustSpamRatio < 0.05 {
			score = score * 1.1 // 10% boost for trusted sources
		}
	}

	// Ensure score is within 0-100 range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return int(score)
}

// categorizeSource determines the category of a source
func (s *SourceReputationScorer) categorizeSource(source *domain.SourceReputation) string {
	// If category is already set, return it
	if source.Category != "" && source.Category != domain.SourceCategoryUnknown {
		return source.Category
	}

	// TODO: Implement category detection based on source URL/name patterns
	// For now, return the existing category or default to unknown
	if source.Category != "" {
		return source.Category
	}

	return domain.SourceCategoryUnknown
}

// determineRank determines the rank based on score and article count
func (s *SourceReputationScorer) determineRank(score int, totalArticles int) string {
	// Need minimum articles to be considered established
	isEstablished := totalArticles >= s.config.MinArticlesForTrust

	switch {
	case score >= 75 && isEstablished:
		return "trusted"
	case score >= 50:
		return "moderate"
	case score >= 30:
		return "low"
	default:
		return "spam"
	}
}

// ScoreBatch scores multiple sources efficiently
func (s *SourceReputationScorer) ScoreBatch(ctx context.Context, sourceNames []string) ([]*SourceReputationResult, error) {
	results := make([]*SourceReputationResult, len(sourceNames))

	for i, name := range sourceNames {
		result, err := s.Score(ctx, name)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	return results, nil
}
