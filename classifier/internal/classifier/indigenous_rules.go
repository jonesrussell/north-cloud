package classifier

import (
	"regexp"
	"strings"
)

// Indigenous relevance constants.
const (
	indigenousRelevanceCore       = "core_indigenous"
	indigenousRelevancePeripheral = "peripheral_indigenous"
	indigenousRelevanceNot        = "not_indigenous"
)

const (
	indigenousConfidenceCore       = 0.90
	indigenousConfidencePeripheral = 0.70
	indigenousConfidenceDefault    = 0.5
)

type indigenousRuleResult struct {
	relevance  string
	confidence float64
}

var indigenousCorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b`),
	regexp.MustCompile(`(?i)\b(first nations|indigenous peoples|indigenous community)\b`),
	regexp.MustCompile(`(?i)\b(métis|metis nation)\b`),
	regexp.MustCompile(`(?i)\b(inuit|inuk)\b`),
	regexp.MustCompile(`(?i)\b(residential school|treaty rights|land rights|aboriginal)\b`),
	regexp.MustCompile(`(?i)\b(seven grandfathers|midewiwin|grand council)\b`),
}

var indigenousPeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(indigenous|native american|first nation)\b`),
	regexp.MustCompile(`(?i)\b(reconciliation|truth and reconciliation)\b`),
	regexp.MustCompile(`(?i)\b(reserve|reservation)\b`),
}

const indigenousRuleMaxBodyChars = 500

func classifyIndigenousByRules(title, body string) *indigenousRuleResult {
	text := title + " " + body
	if len(body) > indigenousRuleMaxBodyChars {
		text = title + " " + body[:indigenousRuleMaxBodyChars]
	}
	lower := strings.ToLower(text)

	for _, p := range indigenousCorePatterns {
		if p.MatchString(lower) {
			return &indigenousRuleResult{relevance: indigenousRelevanceCore, confidence: indigenousConfidenceCore}
		}
	}
	for _, p := range indigenousPeripheralPatterns {
		if p.MatchString(lower) {
			return &indigenousRuleResult{relevance: indigenousRelevancePeripheral, confidence: indigenousConfidencePeripheral}
		}
	}
	return &indigenousRuleResult{relevance: indigenousRelevanceNot, confidence: indigenousConfidenceDefault}
}
