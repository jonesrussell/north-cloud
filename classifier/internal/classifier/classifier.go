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
	crime            *CrimeClassifier
	mining           *MiningClassifier
	coforge          *CoforgeClassifier
	entertainment    *EntertainmentClassifier
	location         *LocationClassifier
	logger           infralogger.Logger
	version          string
}

// Config holds configuration for the classifier
type Config struct {
	Version                 string
	MinQualityScore         int
	UpdateSourceRep         bool
	QualityConfig           QualityConfig
	SourceReputationConfig  SourceReputationConfig
	CrimeClassifier         *CrimeClassifier         // Optional: hybrid street crime classifier
	MiningClassifier        *MiningClassifier        // Optional: hybrid mining classifier
	CoforgeClassifier       *CoforgeClassifier       // Optional: hybrid coforge classifier
	EntertainmentClassifier *EntertainmentClassifier // Optional: hybrid entertainment classifier
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
		crime:            config.CrimeClassifier,
		mining:           config.MiningClassifier,
		coforge:          config.CoforgeClassifier,
		entertainment:    config.EntertainmentClassifier,
		location:         NewLocationClassifier(logger),
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

	// 5-8. Optional classifiers (crime, mining, coforge, entertainment, location)
	crimeResult, miningResult, coforgeResult, entertainmentResult, locationResult := c.runOptionalClassifiers(ctx, raw)

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
		Crime:                crimeResult,
		Mining:               miningResult,
		Coforge:              coforgeResult,
		Entertainment:        entertainmentResult,
		Location:             locationResult,
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

// runOptionalClassifiers runs crime, mining, coforge, entertainment, and location classifiers if enabled.
func (c *Classifier) runOptionalClassifiers(
	ctx context.Context, raw *domain.RawContent,
) (*domain.CrimeResult, *domain.MiningResult, *domain.CoforgeResult, *domain.EntertainmentResult, *domain.LocationResult) {
	var crimeResult *domain.CrimeResult
	if c.crime != nil {
		scResult, scErr := c.crime.Classify(ctx, raw)
		if scErr != nil {
			c.logger.Warn("Crime classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(scErr))
		} else if scResult != nil {
			crimeResult = convertCrimeResult(scResult)
		}
	}

	var miningResult *domain.MiningResult
	if c.mining != nil {
		minResult, minErr := c.mining.Classify(ctx, raw)
		if minErr != nil {
			c.logger.Warn("Mining classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(minErr))
		} else if minResult != nil {
			miningResult = minResult
		}
	}

	var coforgeResult *domain.CoforgeResult
	if c.coforge != nil {
		cfResult, cfErr := c.coforge.Classify(ctx, raw)
		if cfErr != nil {
			c.logger.Warn("Coforge classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(cfErr))
		} else if cfResult != nil {
			coforgeResult = cfResult
		}
	}

	var entertainmentResult *domain.EntertainmentResult
	if c.entertainment != nil {
		entResult, entErr := c.entertainment.Classify(ctx, raw)
		if entErr != nil {
			c.logger.Warn("Entertainment classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(entErr))
		} else if entResult != nil {
			entertainmentResult = entResult
		}
	}

	var locationResult *domain.LocationResult
	if c.location != nil {
		locResult, locErr := c.location.Classify(ctx, raw)
		if locErr != nil {
			c.logger.Warn("Location classification failed",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(locErr))
		} else if locResult != nil {
			locationResult = locResult
		}
	}

	return crimeResult, miningResult, coforgeResult, entertainmentResult, locationResult
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
		Crime:                result.Crime,
		Mining:               result.Mining,
		Coforge:              result.Coforge,
		Entertainment:        result.Entertainment,
		Location:             result.Location,
		// Publisher compatibility aliases
		Body:   raw.RawText, // Alias for RawText
		Source: raw.URL,     // Alias for URL
	}
}

// convertCrimeResult converts classifier.CrimeResult to domain.CrimeResult
func convertCrimeResult(sc *CrimeResult) *domain.CrimeResult {
	return &domain.CrimeResult{
		Relevance:           sc.Relevance,
		SubLabel:            sc.SubLabel,
		CrimeTypes:          sc.CrimeTypes,
		LocationSpecificity: sc.LocationSpecificity,
		FinalConfidence:     sc.FinalConfidence,
		HomepageEligible:    sc.HomepageEligible,
		CategoryPages:       sc.CategoryPages,
		ReviewRequired:      sc.ReviewRequired,
	}
}
