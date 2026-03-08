package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/indigenousmlclient"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const indigenousMaxBodyChars = 500

// IndigenousMLClassifier defines the interface for Indigenous ML classification.
type IndigenousMLClassifier interface {
	Classify(ctx context.Context, title, body string) (*indigenousmlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// IndigenousClassifier implements hybrid rule + ML Indigenous classification.
type IndigenousClassifier struct {
	mlClient IndigenousMLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewIndigenousClassifier creates a new hybrid Indigenous classifier.
func NewIndigenousClassifier(mlClient IndigenousMLClassifier, logger infralogger.Logger, enabled bool) *IndigenousClassifier {
	return &IndigenousClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid Indigenous classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *IndigenousClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.IndigenousResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyIndigenousByRules(raw.Title, raw.RawText)

	var mlResult *indigenousmlclient.ClassifyResponse
	if s.mlClient != nil {
		var err error
		mlResult, err = CallMLWithBodyLimit(
			ctx, raw.Title, raw.RawText, indigenousMaxBodyChars, s.mlClient.Classify)
		if err != nil {
			s.logger.Warn("Indigenous ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	return s.mergeResults(ruleResult, mlResult), nil
}

func (s *IndigenousClassifier) mergeResults(
	rule *indigenousRuleResult, ml *indigenousmlclient.ClassifyResponse,
) *domain.IndigenousResult {
	result := &domain.IndigenousResult{
		Relevance:       rule.relevance,
		FinalConfidence: rule.confidence,
		RuleTriggered:   rule.relevance,
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
	indigenousRuleMLDisagreeWeight = 0.7
	indigenousMLOverrideThreshold  = 0.90
	indigenousBothAgreeWeight      = 2.0
	indigenousMLOverrideWeight     = 0.8
)

//nolint:dupl // Decision matrix mirrors mining/entertainment pattern by design
func (s *IndigenousClassifier) applyDecisionLogic(
	result *domain.IndigenousResult, rule *indigenousRuleResult,
	ml *indigenousmlclient.ClassifyResponse,
) {
	switch {
	case rule.relevance == indigenousRelevanceCore && ml != nil && ml.Relevance == indigenousRelevanceCore:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / indigenousBothAgreeWeight
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == indigenousRelevanceCore && ml != nil && ml.Relevance == indigenousRelevanceNot:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = rule.confidence * indigenousRuleMLDisagreeWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == indigenousRelevanceCore:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathRulesOnly

	case ml != nil && ml.Relevance == indigenousRelevanceCore && ml.RelevanceConfidence >= indigenousMLOverrideThreshold:
		result.Relevance = indigenousRelevancePeripheral
		result.FinalConfidence = ml.RelevanceConfidence * indigenousMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == indigenousRelevancePeripheral && ml != nil && ml.Relevance == indigenousRelevanceCore:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = ml.RelevanceConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
