package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/miningmlclient"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const miningMaxBodyChars = 500

// MiningMLClassifier defines the interface for Mining ML classification.
type MiningMLClassifier interface {
	Classify(ctx context.Context, title, body string) (*miningmlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
}

// MiningClassifier implements hybrid rule + ML mining classification.
type MiningClassifier struct {
	mlClient MiningMLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewMiningClassifier creates a new hybrid mining classifier.
func NewMiningClassifier(mlClient MiningMLClassifier, logger infralogger.Logger, enabled bool) *MiningClassifier {
	return &MiningClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid mining classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *MiningClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*domain.MiningResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyMiningByRules(raw.Title, raw.RawText)

	var mlResult *miningmlclient.ClassifyResponse
	sourceTextUsed := "title"
	if s.mlClient != nil {
		var err error
		mlResult, err = CallMLWithBodyLimit(ctx, raw.Title, raw.RawText, miningMaxBodyChars, s.mlClient.Classify)
		if err != nil {
			s.logger.Warn("Mining ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
		if raw.RawText != "" {
			sourceTextUsed = "title+body_500"
		}
	}

	result := s.mergeResults(ruleResult, mlResult)
	result.SourceTextUsed = sourceTextUsed
	return result, nil
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *MiningClassifier) mergeResults(rule *miningRuleResult, ml *miningmlclient.ClassifyResponse) *domain.MiningResult {
	result := &domain.MiningResult{
		Relevance:       rule.relevance,
		FinalConfidence: rule.confidence,
		RuleTriggered:   rule.relevance,
	}

	if ml != nil {
		result.ModelVersion = ml.ModelVersion
		result.MiningStage = ml.MiningStage
		result.Commodities = append([]string{}, ml.Commodities...)
		result.Location = ml.Location
		result.MLConfidenceRaw = ml.RelevanceConfidence
		result.ProcessingTimeMs = ml.ProcessingTimeMs
	}

	s.applyDecisionLogic(result, rule, ml)

	return result
}

// applyDecisionLogic applies the decision matrix for mining relevance.
//
//nolint:dupl // Decision matrix mirrors anishinaabe/entertainment pattern by design
func (s *MiningClassifier) applyDecisionLogic(result *domain.MiningResult, rule *miningRuleResult, ml *miningmlclient.ClassifyResponse) {
	switch {
	case rule.relevance == miningRelevanceCore && ml != nil && ml.Relevance == miningRelevanceCore:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / miningBothAgreeWeight
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == miningRelevanceCore && ml != nil && ml.Relevance == miningRelevanceNot:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = rule.confidence * miningRuleMLDisagreeWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == miningRelevanceCore:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathRulesOnly

	case ml != nil && ml.Relevance == miningRelevanceCore && ml.RelevanceConfidence >= miningMLOverrideThreshold:
		result.Relevance = miningRelevancePeripheral
		result.FinalConfidence = ml.RelevanceConfidence * miningMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == miningRelevancePeripheral && ml != nil && ml.Relevance == miningRelevanceCore:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = ml.RelevanceConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
