package classifier

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// Classifier orchestrates all classification strategies
type Classifier struct {
	contentType      *ContentTypeClassifier
	quality          *QualityScorer
	topic            *TopicClassifier
	sourceReputation *SourceReputationScorer
	logger           Logger
	version          string
}

// Config holds configuration for the classifier
type Config struct {
	Version                string
	MinQualityScore        int
	UpdateSourceRep        bool
	QualityConfig          QualityConfig
	SourceReputationConfig SourceReputationConfig
}

// NewClassifier creates a new classifier with all strategies
func NewClassifier(
	logger Logger,
	rules []domain.ClassificationRule,
	sourceRepDB SourceReputationDB,
	config Config,
) *Classifier {
	return &Classifier{
		contentType:      NewContentTypeClassifier(logger),
		quality:          NewQualityScorerWithConfig(logger, config.QualityConfig),
		topic:            NewTopicClassifier(logger, rules),
		sourceReputation: NewSourceReputationScorerWithConfig(logger, sourceRepDB, config.SourceReputationConfig),
		logger:           logger,
		version:          config.Version,
	}
}

// Classify performs full classification on raw content
func (c *Classifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.ClassificationResult, error) {
	startTime := time.Now()

	c.logger.Debug("Starting classification",
		"content_id", raw.ID,
		"source_name", raw.SourceName,
		"word_count", raw.WordCount,
	)

	// 1. Content Type Classification
	contentTypeResult, err := c.contentType.Classify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("content type classification failed: %w", err)
	}

	// 2. Quality Scoring
	qualityResult, err := c.quality.Score(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("quality scoring failed: %w", err)
	}

	// 3. Topic Classification
	topicResult, err := c.topic.Classify(ctx, raw)
	if err != nil {
		return nil, fmt.Errorf("topic classification failed: %w", err)
	}

	// 4. Source Reputation
	sourceRepResult, err := c.sourceReputation.Score(ctx, raw.SourceName)
	if err != nil {
		return nil, fmt.Errorf("source reputation scoring failed: %w", err)
	}

	// Update source reputation if enabled
	isSpam := qualityResult.TotalScore < 30 // Spam threshold
	if err := c.sourceReputation.UpdateAfterClassification(ctx, raw.SourceName, qualityResult.TotalScore, isSpam); err != nil {
		c.logger.Warn("Failed to update source reputation",
			"source_name", raw.SourceName,
			"error", err,
		)
		// Don't fail the whole classification if reputation update fails
	}

	// Calculate overall confidence (average of all confidences)
	overallConfidence := (contentTypeResult.Confidence +
		float64(qualityResult.TotalScore)/100.0 +
		c.calculateTopicConfidence(topicResult)) / 3.0

	// Build classification result
	result := &domain.ClassificationResult{
		ContentID:            raw.ID,
		ContentType:          contentTypeResult.Type,
		ContentSubtype:       "", // TODO: Implement subtype detection
		TypeConfidence:       contentTypeResult.Confidence,
		TypeMethod:           contentTypeResult.Method,
		QualityScore:         qualityResult.TotalScore,
		QualityFactors:       qualityResult.Factors,
		Topics:               topicResult.Topics,
		TopicScores:          topicResult.TopicScores,
		IsCrimeRelated:       topicResult.IsCrimeRelated,
		SourceReputation:     sourceRepResult.Score,
		SourceCategory:       sourceRepResult.Category,
		ClassifierVersion:    c.version,
		ClassificationMethod: domain.MethodRuleBased,
		ModelVersion:         "",
		Confidence:           overallConfidence,
		ProcessingTimeMs:     time.Since(startTime).Milliseconds(),
		ClassifiedAt:         time.Now(),
	}

	c.logger.Info("Classification complete",
		"content_id", raw.ID,
		"content_type", result.ContentType,
		"quality_score", result.QualityScore,
		"is_crime_related", result.IsCrimeRelated,
		"topics", result.Topics,
		"processing_time_ms", result.ProcessingTimeMs,
	)

	return result, nil
}

// ClassifyBatch classifies multiple raw content items efficiently
func (c *Classifier) ClassifyBatch(ctx context.Context, rawItems []*domain.RawContent) ([]*domain.ClassificationResult, error) {
	results := make([]*domain.ClassificationResult, len(rawItems))

	for i, raw := range rawItems {
		result, err := c.Classify(ctx, raw)
		if err != nil {
			c.logger.Error("Batch classification failed for item",
				"index", i,
				"content_id", raw.ID,
				"error", err,
			)
			// Continue with next item instead of failing entire batch
			continue
		}
		results[i] = result
	}

	return results, nil
}

// UpdateRules updates the topic classification rules
func (c *Classifier) UpdateRules(rules []domain.ClassificationRule) {
	c.topic.UpdateRules(rules)
}

// GetRules returns the current classification rules
func (c *Classifier) GetRules() []domain.ClassificationRule {
	return c.topic.GetRules()
}

// calculateTopicConfidence calculates overall topic confidence
// If no topics matched, confidence is low (0.3)
// If topics matched, use the highest topic score
func (c *Classifier) calculateTopicConfidence(result *TopicResult) float64 {
	if len(result.TopicScores) == 0 {
		return 0.3 // Low confidence when no topics match
	}

	// Find highest topic score
	var maxScore float64
	for _, score := range result.TopicScores {
		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

// BuildClassifiedContent converts RawContent + ClassificationResult into ClassifiedContent
func (c *Classifier) BuildClassifiedContent(raw *domain.RawContent, result *domain.ClassificationResult) *domain.ClassifiedContent {
	return &domain.ClassifiedContent{
		RawContent:           *raw,
		ContentType:          result.ContentType,
		ContentSubtype:       result.ContentSubtype,
		QualityScore:         result.QualityScore,
		QualityFactors:       result.QualityFactors,
		Topics:               result.Topics,
		TopicScores:          result.TopicScores,
		IsCrimeRelated:       result.IsCrimeRelated,
		SourceReputation:     result.SourceReputation,
		SourceCategory:       result.SourceCategory,
		ClassifierVersion:    result.ClassifierVersion,
		ClassificationMethod: result.ClassificationMethod,
		ModelVersion:         result.ModelVersion,
		Confidence:           result.Confidence,
		// Publisher compatibility aliases
		Body:   raw.RawText, // Alias for RawText
		Source: raw.URL,     // Alias for URL
	}
}
