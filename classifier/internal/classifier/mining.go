package classifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// miningMLResponse holds domain-specific fields from the mining ML sidecar result.
type miningMLResponse struct {
	MiningStage           string             `json:"mining_stage"`
	MiningStageConfidence float64            `json:"mining_stage_confidence"`
	Commodities           []string           `json:"commodities"`
	CommodityScores       map[string]float64 `json:"commodity_scores"`
	Location              string             `json:"location"`
}

// miningMLEnvelope holds the parsed ML response with both envelope and domain fields.
type miningMLEnvelope struct {
	Relevance           string
	RelevanceConfidence float64
	MiningStage         string
	Commodities         []string
	Location            string
	ProcessingTimeMs    int64
	ModelVersion        string
}

// MiningClassifier implements hybrid rule + ML mining classification.
type MiningClassifier struct {
	mlClient MLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewMiningClassifier creates a new hybrid mining classifier.
func NewMiningClassifier(
	mlClient MLClassifier, logger infralogger.Logger, enabled bool,
) *MiningClassifier {
	return &MiningClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid mining classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *MiningClassifier) Classify(
	ctx context.Context, raw *domain.RawContent,
) (*domain.MiningResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyMiningByRules(raw.Title, raw.RawText)

	mlResult := s.callMiningML(ctx, raw)
	sourceTextUsed := "title"
	if s.mlClient != nil && raw.RawText != "" {
		sourceTextUsed = "title+body_500"
	}

	result := s.mergeResults(ruleResult, mlResult)
	result.SourceTextUsed = sourceTextUsed
	return result, nil
}

// callMiningML calls the ML sidecar and parses the response. Returns nil on failure or if client is nil.
func (s *MiningClassifier) callMiningML(ctx context.Context, raw *domain.RawContent) *miningMLEnvelope {
	if s.mlClient == nil {
		return nil
	}
	body := truncateBody(raw.RawText)
	resp, err := s.mlClient.Classify(ctx, raw.Title, body)
	if err != nil {
		s.logger.Warn("Mining ML classification failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(err))
		return nil
	}
	parsed, parseErr := parseMiningMLResponse(resp)
	if parseErr != nil {
		s.logger.Warn("Mining ML response parse failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(parseErr))
		return nil
	}
	return parsed
}

// parseMiningMLResponse extracts mining-specific fields from the unified ML response.
func parseMiningMLResponse(resp *mlclient.StandardResponse) (*miningMLEnvelope, error) {
	var domainResp miningMLResponse
	if unmarshalErr := json.Unmarshal(resp.Result, &domainResp); unmarshalErr != nil {
		return nil, fmt.Errorf("mining result decode: %w", unmarshalErr)
	}

	env := &miningMLEnvelope{
		MiningStage:      domainResp.MiningStage,
		Commodities:      domainResp.Commodities,
		Location:         domainResp.Location,
		ProcessingTimeMs: int64(resp.ProcessingTimeMs),
		ModelVersion:     resp.Version,
	}

	// Map envelope relevance/confidence to relevance class
	if resp.Relevance != nil {
		env.Relevance = mapMiningRelevanceScore(*resp.Relevance)
	}
	if resp.Confidence != nil {
		env.RelevanceConfidence = *resp.Confidence
	}

	return env, nil
}

// mapMiningRelevanceScore maps a numeric relevance to mining relevance class.
func mapMiningRelevanceScore(score float64) string {
	const (
		coreThreshold       = 0.7
		peripheralThreshold = 0.3
	)
	switch {
	case score >= coreThreshold:
		return miningRelevanceCore
	case score >= peripheralThreshold:
		return miningRelevancePeripheral
	default:
		return miningRelevanceNot
	}
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *MiningClassifier) mergeResults(
	rule *miningRuleResult, ml *miningMLEnvelope,
) *domain.MiningResult {
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
//nolint:dupl // Decision matrix mirrors indigenous/entertainment pattern by design
func (s *MiningClassifier) applyDecisionLogic(
	result *domain.MiningResult, rule *miningRuleResult, ml *miningMLEnvelope,
) {
	mlRelevance := ""
	mlConfidence := 0.0
	if ml != nil {
		mlRelevance = ml.Relevance
		mlConfidence = ml.RelevanceConfidence
	}

	switch {
	case rule.relevance == miningRelevanceCore && mlRelevance == miningRelevanceCore:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = (rule.confidence + mlConfidence) / miningBothAgreeWeight
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == miningRelevanceCore && mlRelevance == miningRelevanceNot:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = rule.confidence * miningRuleMLDisagreeWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == miningRelevanceCore:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathRulesOnly

	case mlRelevance == miningRelevanceCore && mlConfidence >= miningMLOverrideThreshold:
		result.Relevance = miningRelevancePeripheral
		result.FinalConfidence = mlConfidence * miningMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == miningRelevancePeripheral && mlRelevance == miningRelevanceCore:
		result.Relevance = miningRelevanceCore
		result.FinalConfidence = mlConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
