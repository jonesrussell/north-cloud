package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/entertainmentmlclient"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const entertainmentMaxBodyChars = 500

// EntertainmentMLClassifier defines the interface for Entertainment ML classification.
type EntertainmentMLClassifier interface {
	Classify(ctx context.Context, title, body string) (*entertainmentmlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// EntertainmentClassifier implements hybrid rule + ML entertainment classification.
type EntertainmentClassifier struct {
	mlClient EntertainmentMLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewEntertainmentClassifier creates a new hybrid entertainment classifier.
func NewEntertainmentClassifier(mlClient EntertainmentMLClassifier, logger infralogger.Logger, enabled bool) *EntertainmentClassifier {
	return &EntertainmentClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid entertainment classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *EntertainmentClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.EntertainmentResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyEntertainmentByRules(raw.Title, raw.RawText)

	var mlResult *entertainmentmlclient.ClassifyResponse
	if s.mlClient != nil {
		var err error
		mlResult, err = CallMLWithBodyLimit(
			ctx, raw.Title, raw.RawText, entertainmentMaxBodyChars, s.mlClient.Classify)
		if err != nil {
			s.logger.Warn("Entertainment ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	return s.mergeResults(ruleResult, mlResult), nil
}

func (s *EntertainmentClassifier) mergeResults(
	rule *entertainmentRuleResult, ml *entertainmentmlclient.ClassifyResponse,
) *domain.EntertainmentResult {
	result := &domain.EntertainmentResult{
		Relevance:        rule.relevance,
		FinalConfidence:  rule.confidence,
		HomepageEligible: false,
		RuleTriggered:    rule.relevance,
	}

	if ml != nil {
		result.ModelVersion = ml.ModelVersion
		result.Categories = append([]string{}, ml.Categories...)
		result.MLConfidenceRaw = ml.RelevanceConfidence
		result.ProcessingTimeMs = ml.ProcessingTimeMs
	}

	s.applyDecisionLogic(result, rule, ml)
	return result
}

const (
	entertainmentHomepageMinConfidence = 0.75
	entertainmentRuleHighConfidence    = 0.85
)

func (s *EntertainmentClassifier) applyDecisionLogic(
	result *domain.EntertainmentResult, rule *entertainmentRuleResult,
	ml *entertainmentmlclient.ClassifyResponse,
) {
	switch {
	case rule.relevance == entertainmentRelevanceCore && ml != nil && ml.Relevance == entertainmentRelevanceCore:
		result.Relevance = entertainmentRelevanceCore
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / entertainmentBothAgreeWeight
		result.HomepageEligible = result.FinalConfidence >= entertainmentHomepageMinConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == entertainmentRelevanceCore && ml != nil && ml.Relevance == entertainmentRelevanceNot:
		result.Relevance = entertainmentRelevanceCore
		result.FinalConfidence = rule.confidence * entertainmentRuleMLDisagreeWeight
		result.HomepageEligible = rule.confidence >= entertainmentRuleHighConfidence
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == entertainmentRelevanceCore:
		result.Relevance = entertainmentRelevanceCore
		result.FinalConfidence = rule.confidence
		result.HomepageEligible = rule.confidence >= entertainmentRuleHighConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathRulesOnly

	case ml != nil && ml.Relevance == entertainmentRelevanceCore && ml.RelevanceConfidence >= entertainmentMLOverrideThreshold:
		result.Relevance = entertainmentRelevancePeripheral
		result.FinalConfidence = ml.RelevanceConfidence * entertainmentMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == entertainmentRelevancePeripheral && ml != nil && ml.Relevance == entertainmentRelevanceCore:
		result.Relevance = entertainmentRelevanceCore
		result.FinalConfidence = ml.RelevanceConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
