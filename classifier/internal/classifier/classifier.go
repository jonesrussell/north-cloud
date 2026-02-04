package classifier

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// Classification constants
	spamThresholdScore     = 30
	confidenceDivisor      = 3.0
	qualityScoreNormalizer = 100.0
	lowConfidenceThreshold = 0.3
)

// Classifier orchestrates all classification strategies
type Classifier struct {
	contentType      *ContentTypeClassifier
	quality          *QualityScorer
	topic            *TopicClassifier
	sourceReputation *SourceReputationScorer
	streetcode       *StreetCodeClassifier
	logger           infralogger.Logger
	version          string
}

// Config holds configuration for the classifier
type Config struct {
	Version                string
	MinQualityScore        int
	UpdateSourceRep        bool
	QualityConfig          QualityConfig
	SourceReputationConfig SourceReputationConfig
	StreetCodeClassifier   *StreetCodeClassifier // Optional: hybrid street crime classifier
}

// NewClassifier creates a new classifier with all strategies
func NewClassifier(
	logger infralogger.Logger,
	rules []domain.ClassificationRule,
	sourceRepDB SourceReputationDB,
	config Config,
) *Classifier {
	return &Classifier{
		contentType:      NewContentTypeClassifier(logger),
		quality:          NewQualityScorerWithConfig(logger, config.QualityConfig),
		topic:            NewTopicClassifier(logger, rules),
		sourceReputation: NewSourceReputationScorerWithConfig(logger, sourceRepDB, config.SourceReputationConfig),
		streetcode:       config.StreetCodeClassifier,
		logger:           logger,
		version:          config.Version,
	}
}

// Classify performs full classification on raw content
func (c *Classifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.ClassificationResult, error) {
	startTime := time.Now()

	c.logger.Debug("Starting classification",
		infralogger.String("content_id", raw.ID),
		infralogger.String("source_name", raw.SourceName),
		infralogger.Int("word_count", raw.WordCount),
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

	// 5. StreetCode Classification (if enabled)
	var streetcodeResult *domain.StreetCodeResult
	if c.streetcode != nil {
		scResult, scErr := c.streetcode.Classify(ctx, raw)
		if scErr != nil {
			c.logger.Warn("StreetCode classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(scErr))
		} else if scResult != nil {
			streetcodeResult = convertStreetCodeResult(scResult)
		}
	}

	// Update source reputation if enabled
	isSpam := qualityResult.TotalScore < spamThresholdScore // Spam threshold
	if err = c.sourceReputation.UpdateAfterClassification(ctx, raw.SourceName, qualityResult.TotalScore, isSpam); err != nil {
		c.logger.Warn("Failed to update source reputation",
			infralogger.String("source_name", raw.SourceName),
			infralogger.Error(err),
		)
		// Don't fail the whole classification if reputation update fails
	}

	// Calculate overall confidence (average of all confidences)
	overallConfidence := (contentTypeResult.Confidence +
		float64(qualityResult.TotalScore)/qualityScoreNormalizer +
		c.calculateTopicConfidence(topicResult)) / confidenceDivisor

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
		SourceReputation:     sourceRepResult.Score,
		SourceCategory:       sourceRepResult.Category,
		ClassifierVersion:    c.version,
		ClassificationMethod: domain.MethodRuleBased,
		ModelVersion:         "",
		Confidence:           overallConfidence,
		ProcessingTimeMs:     time.Since(startTime).Milliseconds(),
		ClassifiedAt:         time.Now(),
		StreetCode:           streetcodeResult,
	}

	c.logger.Info("Classification complete",
		infralogger.String("content_id", raw.ID),
		infralogger.String("content_type", result.ContentType),
		infralogger.Int("quality_score", result.QualityScore),
		infralogger.Any("topics", result.Topics),
		infralogger.Int64("processing_time_ms", result.ProcessingTimeMs),
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
				infralogger.Int("index", i),
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err),
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
	// Convert []ClassificationRule to []*ClassificationRule
	rulePointers := make([]*domain.ClassificationRule, len(rules))
	for i := range rules {
		rulePointers[i] = &rules[i]
	}
	c.topic.UpdateRules(rulePointers)
}

// GetRules returns the current classification rules
func (c *Classifier) GetRules() []domain.ClassificationRule {
	return c.topic.GetRules()
}

// calculateTopicConfidence calculates overall topic confidence
// If no topics matched, confidence is low
// If topics matched, use the highest topic score
func (c *Classifier) calculateTopicConfidence(result *TopicResult) float64 {
	if len(result.TopicScores) == 0 {
		return lowConfidenceThreshold // Low confidence when no topics match
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
		SourceReputation:     result.SourceReputation,
		SourceCategory:       result.SourceCategory,
		ClassifierVersion:    result.ClassifierVersion,
		ClassificationMethod: result.ClassificationMethod,
		ModelVersion:         result.ModelVersion,
		Confidence:           result.Confidence,
		StreetCode:           result.StreetCode,
		// Publisher compatibility aliases
		Body:   raw.RawText, // Alias for RawText
		Source: raw.URL,     // Alias for URL
	}
}

// convertStreetCodeResult converts classifier.StreetCodeResult to domain.StreetCodeResult
func convertStreetCodeResult(sc *StreetCodeResult) *domain.StreetCodeResult {
	return &domain.StreetCodeResult{
		Relevance:           sc.Relevance,
		CrimeTypes:          sc.CrimeTypes,
		LocationSpecificity: sc.LocationSpecificity,
		FinalConfidence:     sc.FinalConfidence,
		HomepageEligible:    sc.HomepageEligible,
		CategoryPages:       sc.CategoryPages,
		ReviewRequired:      sc.ReviewRequired,
	}
}
