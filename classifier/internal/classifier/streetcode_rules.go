// classifier/internal/classifier/streetcode_rules.go
package classifier

import (
	"regexp"
	"strings"
)

// Constants for relevance classifications.
const (
	relevanceCoreStreetCrime = "core_street_crime"
	relevancePeripheral      = "peripheral_crime"
	relevanceNotCrime        = "not_crime"
)

// Confidence score constants.
const (
	confidenceExclusion         = 0.95
	confidenceDefault           = 0.5
	confidenceHighViolent       = 0.95
	confidenceMediumViolent     = 0.90
	confidenceAssault           = 0.85
	confidenceFoundDead         = 0.80
	confidenceProperty          = 0.85
	confidenceArson             = 0.80
	confidenceDrug              = 0.90
	internationalDowngradeRatio = 0.7
)

// ruleResult holds the result of rule-based classification.
type ruleResult struct {
	relevance  string
	confidence float64
	crimeTypes []string
}

// patternWithConf pairs a pattern with its confidence score.
type patternWithConf struct {
	pattern    *regexp.Regexp
	confidence float64
}

// Exclusion patterns - if matched, article is excluded.
var excludePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^(Register|Sign up|Login|Subscribe)`),
	regexp.MustCompile(`(?i)^(Listings? By|Directory|Careers|Jobs)`),
	regexp.MustCompile(`(?i)(Part.Time|Full.Time|Hiring|Position)`),
	regexp.MustCompile(`(?i)^Local (Sports|Events|Weather)$`),
}

// Violent crime patterns.
var violentCrimePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(murder|homicide|manslaughter)`), confidenceHighViolent},
	{regexp.MustCompile(`(?i)(shooting|shootout|shot dead|gunfire)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(stab|stabbing|stabbed)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(assault|assaulted).*(charged|arrest|police)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(charged|arrest|police).*(assault|assaulted)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(sexual assault|rape|sex assault)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(found dead|human remains)`), confidenceFoundDead},
}

// Property crime patterns.
var propertyCrimePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(theft|stolen|shoplifting).*(police|arrest)`), confidenceProperty},
	{regexp.MustCompile(`(?i)(burglary|break.in)`), confidenceProperty},
	{regexp.MustCompile(`(?i)arson`), confidenceArson},
	{regexp.MustCompile(`(?i)\$[\d,]+.*(stolen|theft)`), confidenceProperty},
}

// Drug crime patterns.
var drugCrimePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(drug bust|drug raid|drug seizure)`), confidenceDrug},
	{regexp.MustCompile(`(?i)(fentanyl|cocaine|heroin).*(seiz|arrest|trafficking)`), confidenceDrug},
}

// International patterns - downgrade to peripheral.
var internationalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(Minneapolis|U\.S\.|American|Mexico|European|Israel)`),
}

var justicePattern = regexp.MustCompile(`(?i)(charged|arrest|sentenced|trial)`)

// classifyByRules applies rule-based classification.
// The body parameter is reserved for future use when body text analysis is added.
func classifyByRules(title, _ string) *ruleResult {
	// Check exclusions first
	if matchesExclusion(title) {
		return &ruleResult{relevance: relevanceNotCrime, confidence: confidenceExclusion}
	}

	result := &ruleResult{
		relevance:  relevanceNotCrime,
		confidence: confidenceDefault,
		crimeTypes: []string{},
	}

	// Check crime patterns
	result = checkViolentCrime(result, title)
	result = checkPropertyCrime(result, title)
	result = checkDrugCrime(result, title)

	// Check international (downgrade to peripheral)
	result = checkInternational(result, title)

	// Add criminal_justice if has crime types and mentions arrest/charged
	if len(result.crimeTypes) > 0 && justicePattern.MatchString(title) {
		result.crimeTypes = append(result.crimeTypes, "criminal_justice")
	}

	return result
}

func matchesExclusion(title string) bool {
	for _, p := range excludePatterns {
		if p.MatchString(title) {
			return true
		}
	}
	return false
}

func checkViolentCrime(result *ruleResult, title string) *ruleResult {
	for _, p := range violentCrimePatterns {
		if p.pattern.MatchString(title) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "violent_crime") {
				result.crimeTypes = append(result.crimeTypes, "violent_crime")
			}
		}
	}
	return result
}

func checkPropertyCrime(result *ruleResult, title string) *ruleResult {
	for _, p := range propertyCrimePatterns {
		if p.pattern.MatchString(title) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "property_crime") {
				result.crimeTypes = append(result.crimeTypes, "property_crime")
			}
		}
	}
	return result
}

func checkDrugCrime(result *ruleResult, title string) *ruleResult {
	for _, p := range drugCrimePatterns {
		if p.pattern.MatchString(title) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "drug_crime") {
				result.crimeTypes = append(result.crimeTypes, "drug_crime")
			}
		}
	}
	return result
}

func checkInternational(result *ruleResult, title string) *ruleResult {
	for _, p := range internationalPatterns {
		if p.MatchString(title) && result.relevance == relevanceCoreStreetCrime {
			result.relevance = relevancePeripheral
			result.confidence *= internationalDowngradeRatio
		}
	}
	return result
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}
	return false
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
