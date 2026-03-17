package classifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// coforgeMLResponse holds domain-specific fields from the coforge ML sidecar result.
type coforgeMLResponse struct {
	Audience           string             `json:"audience"`
	AudienceConfidence float64            `json:"audience_confidence"`
	Topics             []string           `json:"topics"`
	TopicScores        map[string]float64 `json:"topic_scores"`
	Industries         []string           `json:"industries"`
	IndustryScores     map[string]float64 `json:"industry_scores"`
}

// coforgeMLEnvelope holds the parsed ML response with both envelope and domain fields.
type coforgeMLEnvelope struct {
	Relevance           string
	RelevanceConfidence float64
	Audience            string
	AudienceConfidence  float64
	Topics              []string
	Industries          []string
	ProcessingTimeMs    int64
	ModelVersion        string
}

// CoforgeClassifier implements hybrid rule + ML coforge classification.
type CoforgeClassifier struct {
	mlClient MLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewCoforgeClassifier creates a new hybrid coforge classifier.
func NewCoforgeClassifier(
	mlClient MLClassifier, logger infralogger.Logger, enabled bool,
) *CoforgeClassifier {
	return &CoforgeClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid coforge classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *CoforgeClassifier) Classify(
	ctx context.Context, raw *domain.RawContent,
) (*domain.CoforgeResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyCoforgeByRules(raw.Title, raw.RawText)

	mlResult := s.callCoforgeML(ctx, raw)

	return s.mergeResults(ruleResult, mlResult), nil
}

// callCoforgeML calls the ML sidecar and parses the response. Returns nil on failure or if client is nil.
func (s *CoforgeClassifier) callCoforgeML(ctx context.Context, raw *domain.RawContent) *coforgeMLEnvelope {
	if s.mlClient == nil {
		return nil
	}
	body := truncateBody(raw.RawText)
	resp, err := s.mlClient.Classify(ctx, raw.Title, body)
	if err != nil {
		s.logger.Warn("Coforge ML classification failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(err))
		return nil
	}
	parsed, parseErr := parseCoforgeMLResponse(resp)
	if parseErr != nil {
		s.logger.Warn("Coforge ML response parse failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(parseErr))
		return nil
	}
	return parsed
}

// parseCoforgeMLResponse extracts coforge-specific fields from the unified ML response.
func parseCoforgeMLResponse(resp *mlclient.StandardResponse) (*coforgeMLEnvelope, error) {
	var domainResp coforgeMLResponse
	if unmarshalErr := json.Unmarshal(resp.Result, &domainResp); unmarshalErr != nil {
		return nil, fmt.Errorf("coforge result decode: %w", unmarshalErr)
	}

	env := &coforgeMLEnvelope{
		Audience:           domainResp.Audience,
		AudienceConfidence: domainResp.AudienceConfidence,
		Topics:             domainResp.Topics,
		Industries:         domainResp.Industries,
		ProcessingTimeMs:   int64(resp.ProcessingTimeMs),
		ModelVersion:       resp.Version,
	}

	if resp.Relevance != nil {
		env.Relevance = mapCoforgeRelevanceScore(*resp.Relevance)
	}
	if resp.Confidence != nil {
		env.RelevanceConfidence = *resp.Confidence
	}

	return env, nil
}

// mapCoforgeRelevanceScore maps a numeric relevance to coforge relevance class.
func mapCoforgeRelevanceScore(score float64) string {
	const (
		coreThreshold       = 0.7
		peripheralThreshold = 0.3
	)
	switch {
	case score >= coreThreshold:
		return coforgeRelevanceCore
	case score >= peripheralThreshold:
		return coforgeRelevancePeripheral
	default:
		return coforgeRelevanceNot
	}
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *CoforgeClassifier) mergeResults(
	rule *coforgeRuleResult, ml *coforgeMLEnvelope,
) *domain.CoforgeResult {
	result := &domain.CoforgeResult{
		Relevance:       rule.relevance,
		FinalConfidence: rule.confidence,
		RuleTriggered:   rule.relevance,
	}

	if ml != nil {
		result.ModelVersion = ml.ModelVersion
		result.Audience = ml.Audience
		result.AudienceConfidence = ml.AudienceConfidence
		result.Topics = append([]string{}, ml.Topics...)
		result.Industries = append([]string{}, ml.Industries...)
		result.MLConfidenceRaw = ml.RelevanceConfidence
		result.ProcessingTimeMs = ml.ProcessingTimeMs
	}

	s.applyDecisionLogic(result, rule, ml)

	return result
}

// applyDecisionLogic applies the decision matrix for coforge relevance.
// Unlike mining, coforge tracks RelevanceConfidence separately from FinalConfidence
// to support audience-aware routing decisions downstream.
func (s *CoforgeClassifier) applyDecisionLogic(
	result *domain.CoforgeResult, rule *coforgeRuleResult, ml *coforgeMLEnvelope,
) {
	mlRelevance := ""
	mlConfidence := 0.0
	if ml != nil {
		mlRelevance = ml.Relevance
		mlConfidence = ml.RelevanceConfidence
	}

	switch {
	case rule.relevance == coforgeRelevanceCore && mlRelevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = mlConfidence
		result.FinalConfidence = (rule.confidence + mlConfidence) / coforgeBothAgreeWeight
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == coforgeRelevanceCore && mlRelevance == coforgeRelevanceNot:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = rule.confidence
		result.FinalConfidence = rule.confidence * coforgeRuleMLDisagreeWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = rule.confidence
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathRulesOnly

	case mlRelevance == coforgeRelevanceCore && mlConfidence >= coforgeMLOverrideThreshold:
		result.Relevance = coforgeRelevancePeripheral
		result.RelevanceConfidence = mlConfidence
		result.FinalConfidence = mlConfidence * coforgeMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == coforgeRelevancePeripheral && mlRelevance == coforgeRelevanceCore:
		result.Relevance = coforgeRelevanceCore
		result.RelevanceConfidence = mlConfidence
		result.FinalConfidence = mlConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.RelevanceConfidence = rule.confidence
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
