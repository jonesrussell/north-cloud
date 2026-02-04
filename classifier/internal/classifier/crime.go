// classifier/internal/classifier/crime.go
package classifier

import (
	"context"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	"github.com/jonesrussell/north-cloud/classifier/internal/mlclient"
	infralogger "github.com/north-cloud/infrastructure/logger"
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

// MLClassifier defines the interface for ML classification.
type MLClassifier interface {
	Classify(ctx context.Context, title, body string) (*mlclient.ClassifyResponse, error)
	Health(ctx context.Context) error
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
	var mlResult *mlclient.ClassifyResponse
	if s.mlClient != nil {
		body := raw.RawText
		if len(body) > maxBodyChars {
			body = body[:maxBodyChars]
		}
		var err error
		mlResult, err = s.mlClient.Classify(ctx, raw.Title, body)
		if err != nil {
			s.logger.Warn("ML classification failed, using rules only",
				infralogger.String("content_id", raw.ID),
				infralogger.Error(err))
		}
	}

	// Decision layer: merge results
	return s.mergeResults(ruleResult, mlResult), nil
}

// mergeResults combines rule and ML results using the decision matrix.
func (s *CrimeClassifier) mergeResults(rule *ruleResult, ml *mlclient.ClassifyResponse) *CrimeResult {
	result := &CrimeResult{
		RuleRelevance:  rule.relevance,
		RuleConfidence: rule.confidence,
		CrimeTypes:     rule.crimeTypes,
	}

	if ml != nil {
		result.MLRelevance = ml.Relevance
		result.MLConfidence = ml.RelevanceConfidence
		result.LocationSpecificity = ml.Location
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

// applyDecisionLogic applies the decision matrix for relevance classification.
func (s *CrimeClassifier) applyDecisionLogic(result *CrimeResult, rule *ruleResult, ml *mlclient.ClassifyResponse) {
	switch {
	case rule.relevance == relevanceCoreStreetCrime && ml != nil && ml.Relevance == relevanceCoreStreetCrime:
		// Both agree: high confidence
		result.Relevance = relevanceCoreStreetCrime
		result.FinalConfidence = (rule.confidence + ml.RelevanceConfidence) / bothAgreeWeight
		result.HomepageEligible = result.FinalConfidence >= HomepageMinConfidence

	case rule.relevance == relevanceCoreStreetCrime && ml != nil && ml.Relevance == relevanceNotCrime:
		// Rule says core, ML says not_crime: flag for review
		result.Relevance = relevanceCoreStreetCrime
		result.FinalConfidence = rule.confidence * ruleMLDisagreeWeight
		result.HomepageEligible = rule.confidence >= RuleHighConfidence
		result.ReviewRequired = true

	case rule.relevance == relevanceCoreStreetCrime:
		// Rule says core, ML unavailable or uncertain
		result.Relevance = relevanceCoreStreetCrime
		result.FinalConfidence = rule.confidence
		result.HomepageEligible = rule.confidence >= RuleHighConfidence

	case ml != nil && ml.Relevance == relevanceCoreStreetCrime && ml.RelevanceConfidence >= MLOverrideThreshold:
		// ML says core with high confidence, rule missed it
		result.Relevance = relevancePeripheral
		result.FinalConfidence = ml.RelevanceConfidence * mlOverrideWeight
		result.ReviewRequired = true

	default:
		result.Relevance = rule.relevance
		result.FinalConfidence = rule.confidence
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
