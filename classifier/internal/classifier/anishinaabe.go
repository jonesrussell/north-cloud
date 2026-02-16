package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/anishinaabemlclient"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const anishinaabeMaxBodyChars = 500

// AnishinaabeMLClassifier defines the interface for Anishinaabe ML classification.
type AnishinaabeMLClassifier interface {
	Classify(ctx context.Context, title, body string) (*anishinaabemlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// AnishinaabeClassifier implements hybrid rule + ML Anishinaabe classification.
type AnishinaabeClassifier struct {
	mlClient AnishinaabeMLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewAnishinaabeClassifier creates a new hybrid Anishinaabe classifier.
func NewAnishinaabeClassifier(mlClient AnishinaabeMLClassifier, logger infralogger.Logger, enabled bool) *AnishinaabeClassifier {
	return &AnishinaabeClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid Anishinaabe classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *AnishinaabeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.AnishinaabeResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyAnishinaabeByRules(raw.Title, raw.RawText)

	var mlResult *anishinaabemlclient.ClassifyResponse
	if s.mlClient != nil {
		var err error
		mlResult, err = CallMLWithBodyLimit(
			ctx, raw.Title, raw.RawText, anishinaabeMaxBodyChars, s.mlClient.Classify)
		if err != nil {
			s.logger.Warn("Anishinaabe ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	return s.mergeResults(ruleResult, mlResult), nil
}

func (s *AnishinaabeClassifier) mergeResults(
	rule *anishinaabeRuleResult, ml *anishinaabemlclient.ClassifyResponse,
) *domain.AnishinaabeResult {
	result := &domain.AnishinaabeResult{
		Relevance:       rule.relevance,
		FinalConfidence: rule.confidence,
	}

	if ml != nil {
		result.ModelVersion = ml.ModelVersion
		result.Categories = append([]string{}, ml.Categories...)
	}

	s.applyDecisionLogic(result, rule, ml)
	return result
}

const (
	anishinaabeRuleMLDisagreeWeight = 0.7
	anishinaabeMLOverrideThreshold  = 0.90
	anishinaabeBothAgreeWeight      = 2.0
	anishinaabeMLOverrideWeight     = 0.8
)

func (s *AnishinaabeClassifier) applyDecisionLogic(
	result *domain.AnishinaabeResult, rule *anishinaabeRuleResult,
	ml *anishinaabemlclient.ClassifyResponse,
) {
	switch {
	case rule.relevance == anishinaabeRelevanceCore && ml != nil && ml.Relevance == anishinaabeRelevanceCore:
		result.Relevance = anishinaabeRelevanceCore
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / anishinaabeBothAgreeWeight
		result.ReviewRequired = false

	case rule.relevance == anishinaabeRelevanceCore && ml != nil && ml.Relevance == anishinaabeRelevanceNot:
		result.Relevance = anishinaabeRelevanceCore
		result.FinalConfidence = rule.confidence * anishinaabeRuleMLDisagreeWeight
		result.ReviewRequired = true

	case rule.relevance == anishinaabeRelevanceCore:
		result.Relevance = anishinaabeRelevanceCore
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false

	case ml != nil && ml.Relevance == anishinaabeRelevanceCore && ml.RelevanceConfidence >= anishinaabeMLOverrideThreshold:
		result.Relevance = anishinaabeRelevancePeripheral
		result.FinalConfidence = ml.RelevanceConfidence * anishinaabeMLOverrideWeight
		result.ReviewRequired = true

	case rule.relevance == anishinaabeRelevancePeripheral && ml != nil && ml.Relevance == anishinaabeRelevanceCore:
		result.Relevance = anishinaabeRelevanceCore
		result.FinalConfidence = ml.RelevanceConfidence
		result.ReviewRequired = false

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
	}
}
