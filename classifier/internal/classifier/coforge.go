package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/coforgemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const coforgeMaxBodyChars = 500

// CoforgeMLClassifier defines the interface for Coforge ML classification.
type CoforgeMLClassifier interface {
	Classify(ctx context.Context, title, body string) (*coforgemlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// CoforgeClassifier implements hybrid rule + ML coforge classification.
type CoforgeClassifier struct {
	mlClient CoforgeMLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewCoforgeClassifier creates a new hybrid coforge classifier.
func NewCoforgeClassifier(mlClient CoforgeMLClassifier, logger infralogger.Logger, enabled bool) *CoforgeClassifier {
	return &CoforgeClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid coforge classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *CoforgeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.CoforgeResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyCoforgeByRules(raw.Title, raw.RawText)

	var mlResult *coforgemlclient.ClassifyResponse
	if s.mlClient != nil {
		var err error
		mlResult, err = CallMLWithBodyLimit(ctx, raw.Title, raw.RawText, coforgeMaxBodyChars, s.mlClient.Classify)
		if err != nil {
			s.logger.Warn("Coforge ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	return s.mergeResults(ruleResult, mlResult), nil
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *CoforgeClassifier) mergeResults(rule *coforgeRuleResult, ml *coforgemlclient.ClassifyResponse) *domain.CoforgeResult {
	result := &domain.CoforgeResult{
		Relevance:       rule.relevance,
		FinalConfidence: rule.confidence,
	}

	if ml != nil {
		result.ModelVersion = ml.ModelVersion
		result.Audience = ml.Audience
		result.AudienceConfidence = ml.AudienceConfidence
		result.Topics = append([]string{}, ml.Topics...)
		result.Industries = append([]string{}, ml.Industries...)
	}

	s.applyDecisionLogic(result, rule, ml)

	return result
}

// applyDecisionLogic applies the decision matrix for coforge relevance.
// Unlike mining, coforge tracks RelevanceConfidence separately from FinalConfidence
// to support audience-aware routing decisions downstream.
func (s *CoforgeClassifier) applyDecisionLogic(result *domain.CoforgeResult, rule *coforgeRuleResult, ml *coforgemlclient.ClassifyResponse) {
	switch {
	case rule.relevance == coforgeRelevanceCore && ml != nil && ml.Relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = ml.RelevanceConfidence
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / coforgeBothAgreeWeight
		result.ReviewRequired = false

	case rule.relevance == coforgeRelevanceCore && ml != nil && ml.Relevance == coforgeRelevanceNot:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = rule.confidence
		result.FinalConfidence = rule.confidence * coforgeRuleMLDisagreeWeight
		result.ReviewRequired = true

	case rule.relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = rule.confidence
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false

	case ml != nil && ml.Relevance == coforgeRelevanceCore && ml.RelevanceConfidence >= coforgeMLOverrideThreshold:
		result.Relevance = coforgeRelevancePeripheral
		result.RelevanceConfidence = ml.RelevanceConfidence
		result.FinalConfidence = ml.RelevanceConfidence * coforgeMLOverrideWeight
		result.ReviewRequired = true

	case rule.relevance == coforgeRelevancePeripheral && ml != nil && ml.Relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = ml.RelevanceConfidence
		result.FinalConfidence = ml.RelevanceConfidence
		result.ReviewRequired = false

	default:
		result.Relevance = rule.relevance
		result.RelevanceConfidence = rule.confidence
		result.FinalConfidence = rule.confidence
	}
}
