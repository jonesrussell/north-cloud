package classifier

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const indigenousMaxBodyChars = 500

// indigenousMLResponse holds domain-specific fields from the indigenous ML sidecar result.
type indigenousMLResponse struct {
	Categories []string `json:"categories"`
}

// indigenousMLEnvelope holds the parsed ML response with both envelope and domain fields.
type indigenousMLEnvelope struct {
	Relevance           string
	RelevanceConfidence float64
	Categories          []string
	ProcessingTimeMs    int64
	ModelVersion        string
}

// IndigenousClassifier implements hybrid rule + ML Indigenous classification.
type IndigenousClassifier struct {
	mlClient MLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// NewIndigenousClassifier creates a new hybrid Indigenous classifier.
func NewIndigenousClassifier(
	mlClient MLClassifier, logger infralogger.Logger, enabled bool,
) *IndigenousClassifier {
	return &IndigenousClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid Indigenous classification on raw content.
// Returns (nil, nil) when classification is disabled.
func (s *IndigenousClassifier) Classify(
	ctx context.Context, raw *domain.RawContent,
) (*domain.IndigenousResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled
	}

	ruleResult := classifyIndigenousByRules(raw.Title, raw.RawText)

	mlResult := s.callIndigenousML(ctx, raw)

	result := s.mergeResults(ruleResult, mlResult)

	// Pass through indigenous_region from source metadata if present.
	if region, ok := raw.Meta["indigenous_region"].(string); ok && region != "" {
		result.Region = region
	}

	return result, nil
}

// callIndigenousML calls the ML sidecar and parses the response. Returns nil on failure or if client is nil.
func (s *IndigenousClassifier) callIndigenousML(
	ctx context.Context, raw *domain.RawContent,
) *indigenousMLEnvelope {
	if s.mlClient == nil {
		return nil
	}
	body := truncateBody(raw.RawText, indigenousMaxBodyChars)
	resp, err := s.mlClient.Classify(ctx, raw.Title, body)
	if err != nil {
		s.logger.Warn("Indigenous ML classification failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(err))
		return nil
	}
	parsed, parseErr := parseIndigenousMLResponse(resp)
	if parseErr != nil {
		s.logger.Warn("Indigenous ML response parse failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(parseErr))
		return nil
	}
	return parsed
}

// parseIndigenousMLResponse extracts indigenous-specific fields from the unified ML response.
func parseIndigenousMLResponse(
	resp *mlclient.StandardResponse,
) (*indigenousMLEnvelope, error) {
	var domainResp indigenousMLResponse
	if unmarshalErr := json.Unmarshal(resp.Result, &domainResp); unmarshalErr != nil {
		return nil, fmt.Errorf("indigenous result decode: %w", unmarshalErr)
	}

	env := &indigenousMLEnvelope{
		Categories:       domainResp.Categories,
		ProcessingTimeMs: int64(resp.ProcessingTimeMs),
		ModelVersion:     resp.Version,
	}

	if resp.Relevance != nil {
		env.Relevance = mapIndigenousRelevanceScore(*resp.Relevance)
	}
	if resp.Confidence != nil {
		env.RelevanceConfidence = *resp.Confidence
	}

	return env, nil
}

// mapIndigenousRelevanceScore maps a numeric relevance to indigenous relevance class.
func mapIndigenousRelevanceScore(score float64) string {
	const (
		coreThreshold       = 0.7
		peripheralThreshold = 0.3
	)
	switch {
	case score >= coreThreshold:
		return indigenousRelevanceCore
	case score >= peripheralThreshold:
		return indigenousRelevancePeripheral
	default:
		return indigenousRelevanceNot
	}
}

func (s *IndigenousClassifier) mergeResults(
	rule *indigenousRuleResult, ml *indigenousMLEnvelope,
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
	ml *indigenousMLEnvelope,
) {
	mlRelevance := ""
	mlConfidence := 0.0
	if ml != nil {
		mlRelevance = ml.Relevance
		mlConfidence = ml.RelevanceConfidence
	}

	switch {
	case rule.relevance == indigenousRelevanceCore && mlRelevance == indigenousRelevanceCore:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = (rule.confidence + mlConfidence) / indigenousBothAgreeWeight
		result.ReviewRequired = false
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == indigenousRelevanceCore && mlRelevance == indigenousRelevanceNot:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = rule.confidence * indigenousRuleMLDisagreeWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == indigenousRelevanceCore:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = rule.confidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathRulesOnly

	case mlRelevance == indigenousRelevanceCore && mlConfidence >= indigenousMLOverrideThreshold:
		result.Relevance = indigenousRelevancePeripheral
		result.FinalConfidence = mlConfidence * indigenousMLOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	case rule.relevance == indigenousRelevancePeripheral && mlRelevance == indigenousRelevanceCore:
		result.Relevance = indigenousRelevanceCore
		result.FinalConfidence = mlConfidence
		result.ReviewRequired = false
		result.DecisionPath = decisionPathMLUpgrade

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}
