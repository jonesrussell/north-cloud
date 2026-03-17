// classifier/internal/classifier/crime.go
package classifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// Crime classification thresholds.
const (
	HomepageMinConfidence = 0.75
	RuleHighConfidence    = 0.85
	MLOverrideThreshold   = 0.90
	maxBodyChars          = 500
	bothAgreeWeight       = 2.0
	ruleMLDisagreeWeight  = 0.7
	mlOverrideWeight      = 0.8
)

// Sub-label constants for peripheral_crime articles.
const (
	SubLabelCriminalJustice = "criminal_justice"
	SubLabelCrimeContext    = "crime_context"
)

// Minimum signals required for criminal_justice classification.
const minCriminalJusticeSignals = 2

// MLClassifier defines the interface for ML classification.
type MLClassifier interface {
	Classify(ctx context.Context, title, body string) (*mlclient.StandardResponse, error)
}

// crimeMLResponse holds domain-specific fields from the crime ML sidecar result.
type crimeMLResponse struct {
	CrimeTypes      []string           `json:"crime_types"`
	CrimeTypeScores map[string]float64 `json:"crime_type_scores"`
	Location        string             `json:"location"`
}

// CrimeClassifier implements hybrid rule + ML classification.
type CrimeClassifier struct {
	mlClient MLClassifier
	logger   infralogger.Logger
	enabled  bool
}

// CrimeResult holds the hybrid classification result.
type CrimeResult struct {
	Relevance           string   `json:"street_crime_relevance"`
	SubLabel            string   `json:"sub_label,omitempty"` // "criminal_justice" or "crime_context" for peripheral_crime
	CrimeTypes          []string `json:"crime_types"`
	LocationSpecificity string   `json:"location_specificity"`
	FinalConfidence     float64  `json:"final_confidence"`
	HomepageEligible    bool     `json:"homepage_eligible"`
	CategoryPages       []string `json:"category_pages"`
	ReviewRequired      bool     `json:"review_required"`
	RuleRelevance       string   `json:"rule_relevance"`
	RuleConfidence      float64  `json:"rule_confidence"`
	MLRelevance         string   `json:"ml_relevance,omitempty"`
	MLConfidence        float64  `json:"ml_confidence,omitempty"`
	DecisionPath        string   `json:"decision_path,omitempty"`
	ProcessingTimeMs    int64    `json:"processing_time_ms,omitempty"`
}

// NewCrimeClassifier creates a new hybrid classifier.
func NewCrimeClassifier(mlClient MLClassifier, logger infralogger.Logger, enabled bool) *CrimeClassifier {
	return &CrimeClassifier{
		mlClient: mlClient,
		logger:   logger,
		enabled:  enabled,
	}
}

// Classify performs hybrid classification on raw content.
// Returns (nil, nil) when classification is disabled - this is intentional to indicate
// "no result available, don't add Crime fields".
func (s *CrimeClassifier) Classify(ctx context.Context, raw *domain.RawContent) (*CrimeResult, error) {
	if !s.enabled {
		return nil, nil //nolint:nilnil // Intentional: nil result signals disabled, not an error
	}

	// Layer 1 & 2: Rule-based classification
	ruleResult := classifyByRules(raw.Title, raw.RawText)

	// Layer 3: ML classification (if ML service available)
	mlResp := s.callCrimeML(ctx, raw)

	// Decision layer: merge results
	result := s.mergeResults(ruleResult, mlResp)

	// Determine sub-label for peripheral_crime
	s.determineSubLabel(result, raw.Title, raw.RawText)

	return result, nil
}

// callCrimeML calls the ML sidecar and parses the response. Returns nil on failure or if client is nil.
func (s *CrimeClassifier) callCrimeML(ctx context.Context, raw *domain.RawContent) *crimeMLEnvelope {
	if s.mlClient == nil {
		return nil
	}
	body := truncateBody(raw.RawText, maxBodyChars)
	resp, err := s.mlClient.Classify(ctx, raw.Title, body)
	if err != nil {
		s.logger.Warn("ML classification failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(err))
		return nil
	}
	parsed, parseErr := parseCrimeMLResponse(resp)
	if parseErr != nil {
		s.logger.Warn("ML response parse failed, using rules only",
			infralogger.String("content_id", raw.ID), infralogger.Error(parseErr))
		return nil
	}
	return parsed
}

// crimeMLEnvelope holds the parsed ML response with both envelope and domain fields.
type crimeMLEnvelope struct {
	Relevance           float64
	RelevanceConfidence float64
	CrimeTypes          []string
	Location            string
	ProcessingTimeMs    int64
}

// parseCrimeMLResponse extracts crime-specific fields from the unified ML response.
func parseCrimeMLResponse(resp *mlclient.StandardResponse) (*crimeMLEnvelope, error) {
	var domainResp crimeMLResponse
	if unmarshalErr := json.Unmarshal(resp.Result, &domainResp); unmarshalErr != nil {
		return nil, fmt.Errorf("crime result decode: %w", unmarshalErr)
	}

	env := &crimeMLEnvelope{
		CrimeTypes:       domainResp.CrimeTypes,
		Location:         domainResp.Location,
		ProcessingTimeMs: int64(resp.ProcessingTimeMs),
	}
	if resp.Relevance != nil {
		env.Relevance = *resp.Relevance
	}
	if resp.Confidence != nil {
		env.RelevanceConfidence = *resp.Confidence
	}

	return env, nil
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *CrimeClassifier) mergeResults(rule *ruleResult, ml *crimeMLEnvelope) *CrimeResult {
	result := &CrimeResult{
		RuleRelevance:  rule.relevance,
		RuleConfidence: rule.confidence,
		CrimeTypes:     rule.crimeTypes,
	}

	if ml != nil {
		result.MLRelevance = mapRelevanceScore(ml.Relevance)
		result.MLConfidence = ml.RelevanceConfidence
		result.LocationSpecificity = ml.Location
		result.ProcessingTimeMs = ml.ProcessingTimeMs
	}

	// Decision logic
	s.applyDecisionLogic(result, rule, ml)

	// Merge crime types from ML
	if ml != nil {
		for _, ct := range ml.CrimeTypes {
			if !containsString(result.CrimeTypes, ct) {
				result.CrimeTypes = append(result.CrimeTypes, ct)
			}
		}
	}

	// Map to category pages
	result.CategoryPages = mapToCategoryPages(result.CrimeTypes)

	return result
}

// mapRelevanceScore maps a numeric relevance score to a relevance class string.
// Scores >= 0.7 map to core_street_crime, >= 0.3 to peripheral_crime, else not_crime.
func mapRelevanceScore(score float64) string {
	const (
		coreThreshold       = 0.7
		peripheralThreshold = 0.3
	)
	switch {
	case score >= coreThreshold:
		return relevanceCoreStreetCrime
	case score >= peripheralThreshold:
		return relevancePeripheral
	default:
		return relevanceNotCrime
	}
}

// applyDecisionLogic applies the decision matrix for relevance classification.
func (s *CrimeClassifier) applyDecisionLogic(
	result *CrimeResult, rule *ruleResult, ml *crimeMLEnvelope,
) {
	mlRelevance := ""
	mlConfidence := 0.0
	if ml != nil {
		mlRelevance = result.MLRelevance
		mlConfidence = ml.RelevanceConfidence
	}

	switch {
	case rule.relevance == relevanceCoreStreetCrime && mlRelevance == relevanceCoreStreetCrime:
		// Both agree: high confidence
		result.Relevance = relevanceCoreStreetCrime
		result.FinalConfidence = (rule.confidence + mlConfidence) / bothAgreeWeight
		result.HomepageEligible = result.FinalConfidence >= HomepageMinConfidence
		result.DecisionPath = decisionPathBothAgree

	case rule.relevance == relevanceCoreStreetCrime && mlRelevance == relevanceNotCrime:
		// Rule says core, ML says not_crime: flag for review
		result.Relevance = relevanceCoreStreetCrime
		result.FinalConfidence = rule.confidence * ruleMLDisagreeWeight
		result.HomepageEligible = rule.confidence >= RuleHighConfidence
		result.ReviewRequired = true
		result.DecisionPath = decisionPathRuleOverride

	case rule.relevance == relevanceCoreStreetCrime:
		// Rule says core, ML unavailable or uncertain
		result.Relevance = relevanceCoreStreetCrime
		result.FinalConfidence = rule.confidence
		result.HomepageEligible = rule.confidence >= RuleHighConfidence
		result.DecisionPath = decisionPathRulesOnly

	case mlRelevance == relevanceCoreStreetCrime && mlConfidence >= MLOverrideThreshold:
		// ML says core with high confidence, rule missed it
		result.Relevance = relevancePeripheral
		result.FinalConfidence = mlConfidence * mlOverrideWeight
		result.ReviewRequired = true
		result.DecisionPath = decisionPathMLOverride

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
		result.DecisionPath = decisionPathDefault
	}
}

// mapToCategoryPages converts crime types to Crime category page slugs.
func mapToCategoryPages(crimeTypes []string) []string {
	mapping := map[string][]string{
		"violent_crime":    {"violent-crime", "crime"},
		"property_crime":   {"property-crime", "crime"},
		"drug_crime":       {"drug-crime", "crime"},
		"gang_violence":    {"gang-violence", "crime"},
		"organized_crime":  {"organized-crime", "crime"},
		"criminal_justice": {"court-news"},
		"other_crime":      {"crime"},
	}

	pages := make(map[string]bool)
	for _, ct := range crimeTypes {
		for _, page := range mapping[ct] {
			pages[page] = true
		}
	}

	result := make([]string, 0, len(pages))
	for page := range pages {
		result = append(result, page)
	}
	return result
}

// criminalJusticeVerbs are verbs indicating active legal proceedings.
var criminalJusticeVerbs = []string{
	"charged", "arrested", "arraigned", "pleads", "pleaded",
	"sentenced", "convicted", "acquitted", "appeals", "appealed",
	"investigation launched", "warrant issued", "indicted",
}

// jurisdictionIndicators are terms that suggest criminal justice context.
var jurisdictionIndicators = []string{
	"court", "judge", "prosecutor", "crown", "district attorney",
	"police", "rcmp", "opp", "fbi", "doj", "justice department",
}

// determineSubLabel sets the sub_label for peripheral_crime articles.
func (s *CrimeClassifier) determineSubLabel(result *CrimeResult, title, body string) {
	// Only peripheral_crime gets sub-labels
	if result.Relevance != relevancePeripheral {
		result.SubLabel = ""
		return
	}

	text := strings.ToLower(title + " " + body)

	// Count signals for criminal_justice
	cjScore := 0

	// Check jurisdiction indicators
	for _, indicator := range jurisdictionIndicators {
		if strings.Contains(text, indicator) {
			cjScore++
			break // Only count once
		}
	}

	// Check criminal justice verbs
	for _, verb := range criminalJusticeVerbs {
		if strings.Contains(text, verb) {
			cjScore++
			break
		}
	}

	// Decision: criminal_justice needs 2+ signals, otherwise crime_context
	if cjScore >= minCriminalJusticeSignals {
		result.SubLabel = SubLabelCriminalJustice
	} else {
		// Default for peripheral_crime (crime_context covers document releases and ambiguous cases)
		result.SubLabel = SubLabelCrimeContext
	}
}
