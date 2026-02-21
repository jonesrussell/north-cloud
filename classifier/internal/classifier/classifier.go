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
	anishinaabe      *AnishinaabeClassifier
	location         *LocationClassifier
	logger           infralogger.Logger
	version          string
	routingTable     map[string][]string // route key -> sidecar names (e.g. "article:event" -> ["location"])
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
	AnishinaabeClassifier   *AnishinaabeClassifier   // Optional: hybrid anishinaabe classifier
	RoutingTable            map[string][]string      // Optional: content-type routing (see ResolveSidecars)
}

// NewClassifier creates a new classifier with all strategies
func NewClassifier(
	logger infralogger.Logger,
	rules []domain.ClassificationRule,
	sourceRepDB SourceReputationDB,
	config Config,
) *Classifier {
	routingTable := make(map[string][]string)
	for k, v := range config.RoutingTable {
		routingTable[k] = append([]string(nil), v...)
	}
	// Warn at startup if routing table references a disabled (nil) sidecar classifier.
	sidecarEnabled := map[string]bool{
		"crime":         config.CrimeClassifier != nil,
		"mining":        config.MiningClassifier != nil,
		"coforge":       config.CoforgeClassifier != nil,
		"entertainment": config.EntertainmentClassifier != nil,
		"anishinaabe":   config.AnishinaabeClassifier != nil,
		"location":      true, // always constructed below
	}
	for routeKey, names := range routingTable {
		for _, name := range names {
			if enabled, known := sidecarEnabled[name]; known && !enabled {
				logger.Warn("Routing table references disabled sidecar classifier",
					infralogger.String("routing_key", routeKey),
					infralogger.String("sidecar_name", name),
				)
			}
		}
	}
	return &Classifier{
		contentType:      NewContentTypeClassifier(logger),
		quality:          NewQualityScorerWithConfig(logger, config.QualityConfig),
		topic:            NewTopicClassifier(logger, rules),
		sourceReputation: NewSourceReputationScorerWithConfig(logger, sourceRepDB, config.SourceReputationConfig),
		crime:            config.CrimeClassifier,
		mining:           config.MiningClassifier,
		coforge:          config.CoforgeClassifier,
		entertainment:    config.EntertainmentClassifier,
		anishinaabe:      config.AnishinaabeClassifier,
		location:         NewLocationClassifier(logger),
		logger:           logger,
		version:          config.Version,
		routingTable:     routingTable,
	}
}

// ResolveSidecars returns the list of sidecar names to run for the given content type and subtype.
// Lookup order: for article, try article:<subtype> then article; otherwise try contentType.
// Logs clearly and returns nil when no routing entry exists.
func (c *Classifier) ResolveSidecars(contentType, subtype string) []string {
	var key string
	if contentType == domain.ContentTypeArticle && subtype != "" {
		key = "article:" + subtype
		if names, ok := c.routingTable[key]; ok {
			return names
		}
		c.logger.Debug("No routing entry for article subtype; falling back to article key",
			infralogger.String("content_subtype", subtype),
			infralogger.String("fallback_key", "article"),
		)
		key = "article"
	} else {
		key = contentType
	}
	if names, ok := c.routingTable[key]; ok {
		return names
	}
	c.logger.Debug("No routing entry for content type; skipping optional classifiers",
		infralogger.String("content_type", contentType),
		infralogger.String("content_subtype", subtype),
		infralogger.String("routing_key", key),
	)
	return nil
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

	// 5-9. Optional classifiers — gate by content type and subtype (pages never reach publisher)
	crimeResult, miningResult, coforgeResult, entertainmentResult, anishinaabeResult, locationResult := c.classifyOptionalForPublishable(
		ctx, raw, contentTypeResult.Type, contentTypeResult.Subtype)

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
		ContentSubtype:       contentTypeResult.Subtype,
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
		Anishinaabe:          anishinaabeResult,
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

// classifyOptionalForPublishable runs optional classifiers according to the declarative routing table.
// ResolveSidecars(contentType, contentSubtype) determines which sidecars to run; runOptionalClassifiers runs only those.
//
//nolint:gocritic // 6 return values match optional classifier pattern; refactor would require wider changes
func (c *Classifier) classifyOptionalForPublishable(
	ctx context.Context, raw *domain.RawContent, contentType, contentSubtype string,
) (*domain.CrimeResult, *domain.MiningResult, *domain.CoforgeResult, *domain.EntertainmentResult, *domain.AnishinaabeResult, *domain.LocationResult) {
	sidecars := c.ResolveSidecars(contentType, contentSubtype)
	return c.runOptionalClassifiers(ctx, raw, contentType, sidecars)
}

// runOptionalClassifiers runs only the sidecars listed in sidecars (e.g. from ResolveSidecars).
// Each listed sidecar is run only if the corresponding classifier is non-nil (enabled in registry).
//
//nolint:gocritic // 6 return values match optional classifier pattern
func (c *Classifier) runOptionalClassifiers(
	ctx context.Context, raw *domain.RawContent, contentType string, sidecars []string,
) (*domain.CrimeResult, *domain.MiningResult, *domain.CoforgeResult, *domain.EntertainmentResult, *domain.AnishinaabeResult, *domain.LocationResult) {
	knownSidecarNames := map[string]bool{
		"crime": true, "mining": true, "coforge": true,
		"entertainment": true, "anishinaabe": true, "location": true,
	}
	allowed := make(map[string]bool)
	for _, name := range sidecars {
		allowed[name] = true
		if !knownSidecarNames[name] {
			c.logger.Warn("Routing table contains unknown sidecar name; it will be ignored",
				infralogger.String("sidecar_name", name),
				infralogger.String("content_type", contentType),
				infralogger.String("content_id", raw.ID),
			)
		}
	}
	return c.runCrimeOptional(ctx, raw, contentType, allowed["crime"]),
		c.runMiningOptional(ctx, raw, contentType, allowed["mining"]),
		c.runCoforgeOptional(ctx, raw, contentType, allowed["coforge"]),
		c.runEntertainmentOptional(ctx, raw, contentType, allowed["entertainment"]),
		c.runAnishinaabeOptional(ctx, raw, contentType, allowed["anishinaabe"]),
		c.runLocationOptional(ctx, raw, allowed["location"])
}

func (c *Classifier) runCrimeOptional(
	ctx context.Context, raw *domain.RawContent, contentType string, run bool,
) *domain.CrimeResult {
	if !run || c.crime == nil {
		return nil
	}
	start := time.Now()
	scResult, scErr := c.crime.Classify(ctx, raw)
	latencyMs := time.Since(start).Milliseconds()
	if scErr != nil {
		c.logSidecarError("crime-ml", raw, contentType, scErr, latencyMs)
		return nil
	}
	if scResult == nil {
		c.logSidecarNilResult("crime-ml", raw.ID, latencyMs)
		return nil
	}
	crimeResult := convertCrimeResult(scResult)
	c.logSidecarSuccess("crime-ml", raw, contentType,
		crimeResult.Relevance, crimeResult.FinalConfidence,
		crimeResult.MLConfidenceRaw, crimeResult.RuleTriggered,
		crimeResult.DecisionPath, latencyMs, crimeResult.ProcessingTimeMs, "")
	return crimeResult
}

func (c *Classifier) runMiningOptional(
	ctx context.Context, raw *domain.RawContent, contentType string, run bool,
) *domain.MiningResult {
	if !run || c.mining == nil {
		return nil
	}
	start := time.Now()
	minResult, minErr := c.mining.Classify(ctx, raw)
	latencyMs := time.Since(start).Milliseconds()
	if minErr != nil {
		c.logSidecarError("mining-ml", raw, contentType, minErr, latencyMs)
		return nil
	}
	if minResult == nil {
		c.logSidecarNilResult("mining-ml", raw.ID, latencyMs)
		return nil
	}
	c.logSidecarSuccess("mining-ml", raw, contentType,
		minResult.Relevance, minResult.FinalConfidence,
		minResult.MLConfidenceRaw, minResult.RuleTriggered,
		minResult.DecisionPath, latencyMs, minResult.ProcessingTimeMs, minResult.ModelVersion)
	return minResult
}

func (c *Classifier) runCoforgeOptional(
	ctx context.Context, raw *domain.RawContent, contentType string, run bool,
) *domain.CoforgeResult {
	if !run || c.coforge == nil {
		return nil
	}
	start := time.Now()
	cfResult, cfErr := c.coforge.Classify(ctx, raw)
	latencyMs := time.Since(start).Milliseconds()
	if cfErr != nil {
		c.logSidecarError("coforge-ml", raw, contentType, cfErr, latencyMs)
		return nil
	}
	if cfResult == nil {
		c.logSidecarNilResult("coforge-ml", raw.ID, latencyMs)
		return nil
	}
	c.logSidecarSuccess("coforge-ml", raw, contentType,
		cfResult.Relevance, cfResult.FinalConfidence,
		cfResult.MLConfidenceRaw, cfResult.RuleTriggered,
		cfResult.DecisionPath, latencyMs, cfResult.ProcessingTimeMs, cfResult.ModelVersion)
	return cfResult
}

func (c *Classifier) runEntertainmentOptional(
	ctx context.Context, raw *domain.RawContent, contentType string, run bool,
) *domain.EntertainmentResult {
	if !run || c.entertainment == nil {
		return nil
	}
	start := time.Now()
	entResult, entErr := c.entertainment.Classify(ctx, raw)
	latencyMs := time.Since(start).Milliseconds()
	if entErr != nil {
		c.logSidecarError("entertainment-ml", raw, contentType, entErr, latencyMs)
		return nil
	}
	if entResult == nil {
		c.logSidecarNilResult("entertainment-ml", raw.ID, latencyMs)
		return nil
	}
	c.logSidecarSuccess("entertainment-ml", raw, contentType,
		entResult.Relevance, entResult.FinalConfidence,
		entResult.MLConfidenceRaw, entResult.RuleTriggered,
		entResult.DecisionPath, latencyMs, entResult.ProcessingTimeMs, entResult.ModelVersion)
	return entResult
}

func (c *Classifier) runAnishinaabeOptional(
	ctx context.Context, raw *domain.RawContent, contentType string, run bool,
) *domain.AnishinaabeResult {
	if !run || c.anishinaabe == nil {
		return nil
	}
	start := time.Now()
	aResult, aErr := c.anishinaabe.Classify(ctx, raw)
	latencyMs := time.Since(start).Milliseconds()
	if aErr != nil {
		c.logSidecarError("anishinaabe-ml", raw, contentType, aErr, latencyMs)
		return nil
	}
	if aResult == nil {
		c.logSidecarNilResult("anishinaabe-ml", raw.ID, latencyMs)
		return nil
	}
	c.logSidecarSuccess("anishinaabe-ml", raw, contentType,
		aResult.Relevance, aResult.FinalConfidence,
		aResult.MLConfidenceRaw, aResult.RuleTriggered,
		aResult.DecisionPath, latencyMs, aResult.ProcessingTimeMs, aResult.ModelVersion)
	return aResult
}

func (c *Classifier) runLocationOptional(
	ctx context.Context, raw *domain.RawContent, run bool,
) *domain.LocationResult {
	if !run || c.location == nil {
		return nil
	}
	start := time.Now()
	locResult, locErr := c.location.Classify(ctx, raw)
	latencyMs := time.Since(start).Milliseconds()
	if locErr != nil {
		c.logSidecarError("location", raw, "", locErr, latencyMs)
		return nil
	}
	if locResult == nil {
		c.logSidecarNilResult("location", raw.ID, latencyMs)
		return nil
	}
	c.logSidecarSuccess("location", raw, "",
		"", 0, 0, "", "", latencyMs, 0, "")
	return locResult
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
		Anishinaabe:          result.Anishinaabe,
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
		DecisionPath:        sc.DecisionPath,
		MLConfidenceRaw:     sc.MLConfidence,
		RuleTriggered:       sc.RuleRelevance,
		ProcessingTimeMs:    sc.ProcessingTimeMs,
	}
}
