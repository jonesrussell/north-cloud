package classifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const entertainmentMaxBodyChars = 500

// entertainmentMLResponse holds domain-specific fields from the entertainment ML sidecar result.
type entertainmentMLResponse struct {
	Categories []string `json:"categories"`
}

// entertainmentMLEnvelope holds the parsed ML response with both envelope and domain fields.
type entertainmentMLEnvelope struct {
	Relevance           string
	RelevanceConfidence float64
	Categories          []string
	ProcessingTimeMs    int64
	ModelVersion        string
}

// EntertainmentClassifier implements hybrid rule + ML entertainment classification.
type EntertainmentClassifier struct {
	mlClient MLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewEntertainmentClassifier creates a new hybrid entertainment classifier.
func NewEntertainmentClassifier(
	mlClient MLClassifier, logger infralogger.Logger, enabled bool,
) *EntertainmentClassifier {
	return &EntertainmentClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid entertainment classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *EntertainmentClassifier) Classify(
	ctx context.Context, raw *domain.RawContent,
) (*domain.EntertainmentResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyEntertainmentByRules(raw.Title, raw.RawText)

	mlResult := s.callEntertainmentML(ctx, raw)

	return s.mergeResults(ruleResult, mlResult), nil
}

// callEntertainmentML calls the ML sidecar and parses the response. Returns nil on failure or if client is nil.
func (s *EntertainmentClassifier) callEntertainmentML(
	ctx context.Context, raw *domain.RawContent,
) *entertainmentMLEnvelope {
	if s.mlClient == nil {
		return nil
	}
	body := truncateBody(raw.RawText, entertainmentMaxBodyChars)
	resp, err := s.mlClient.Classify(ctx, raw.Title, body)
	if err != nil {
		s.logger.Warn("Entertainment ML classification failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(err))
		return nil
	}
	parsed, parseErr := parseEntertainmentMLResponse(resp)
	if parseErr != nil {
		s.logger.Warn("Entertainment ML response parse failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(parseErr))
		return nil
	}
	return parsed
}

// parseEntertainmentMLResponse extracts entertainment-specific fields from the unified ML response.
func parseEntertainmentMLResponse(
	resp *mlclient.StandardResponse,
) (*entertainmentMLEnvelope, error) {
	var domainResp entertainmentMLResponse
	if unmarshalErr := json.Unmarshal(resp.Result, &domainResp); unmarshalErr != nil {
		return nil, fmt.Errorf("entertainment result decode: %w", unmarshalErr)
	}

	env := &entertainmentMLEnvelope{
		Categories:       domainResp.Categories,
		ProcessingTimeMs: int64(resp.ProcessingTimeMs),
		ModelVersion:     resp.Version,
	}

	if resp.Relevance != nil {
		env.Relevance = mapEntertainmentRelevanceScore(*resp.Relevance)
	}
	if resp.Confidence != nil {
		env.RelevanceConfidence = *resp.Confidence
	}

	return env, nil
}

// mapEntertainmentRelevanceScore maps a numeric relevance to entertainment relevance class.
func mapEntertainmentRelevanceScore(score float64) string {
	const (
		coreThreshold       = 0.7
		peripheralThreshold = 0.3
	)
	switch {
	case score >= coreThreshold:
		return entertainmentRelevanceCore
	case score >= peripheralThreshold:
		return entertainmentRelevancePeripheral
	default:
		return entertainmentRelevanceNot
	}
}

func (s *EntertainmentClassifier) mergeResults(
	rule *entertainmentRuleResult, ml *entertainmentMLEnvelope,
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
	ml *entertainmentMLEnvelope,
) {
	mlRelevance := ""
	mlConfidence := 0.0
	if ml != nil {
		mlRelevance = ml.Relevance
		mlConfidence = ml.RelevanceConfidence
	}

	switch {
	case rule.relevance == entertainmentRelevanceCore && mlRelevance == entertainmentRelevanceCore:
		result.Relevance = entertainmentRelevanceCore
		result.FinalConfidence = (rule.confidence + mlConfidence) / entertainmentBothAgreeWeight
		result.HomepageEligible = result.FinalConfidence >= entertainmentHomepageMinConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == entertainmentRelevanceCore && mlRelevance == entertainmentRelevanceNot:
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

	case mlRelevance == entertainmentRelevanceCore && mlConfidence >= entertainmentMLOverrideThreshold:
		result.Relevance = entertainmentRelevancePeripheral
		result.FinalConfidence = mlConfidence * entertainmentMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == entertainmentRelevancePeripheral && mlRelevance == entertainmentRelevanceCore:
		result.Relevance = entertainmentRelevanceCore
		result.FinalConfidence = mlConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
