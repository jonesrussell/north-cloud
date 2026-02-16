package classifier

import (
	"regexp"
	"strings"
)

// Anishinaabe relevance constants.
const (
	anishinaabeRelevanceCore       = "core_anishinaabe"
	anishinaabeRelevancePeripheral = "peripheral_anishinaabe"
	anishinaabeRelevanceNot        = "not_anishinaabe"
)

const (
	anishinaabeConfidenceCore       = 0.90
	anishinaabeConfidencePeripheral = 0.70
	anishinaabeConfidenceDefault    = 0.5
)

type anishinaabeRuleResult struct {
	relevance  string
	confidence float64
}

var anishinaabeCorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b`),
	regexp.MustCompile(`(?i)\b(first nations|indigenous peoples|indigenous community)\b`),
	regexp.MustCompile(`(?i)\b(métis|metis nation)\b`),
	regexp.MustCompile(`(?i)\b(inuit|inuk)\b`),
	regexp.MustCompile(`(?i)\b(residential school|treaty rights|land rights|aboriginal)\b`),
	regexp.MustCompile(`(?i)\b(seven grandfathers|midewiwin|grand council)\b`),
}

var anishinaabePeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(indigenous|native american|first nation)\b`),
	regexp.MustCompile(`(?i)\b(reconciliation|truth and reconciliation)\b`),
	regexp.MustCompile(`(?i)\b(reserve|reservation)\b`),
}

const anishinaabeRuleMaxBodyChars = 500

func classifyAnishinaabeByRules(title, body string) *anishinaabeRuleResult {
	text := title + " " + body
	if len(body) > anishinaabeRuleMaxBodyChars {
		text = title + " " + body[:anishinaabeRuleMaxBodyChars]
	}
	lower := strings.ToLower(text)

	for _, p := range anishinaabeCorePatterns {
		if p.MatchString(lower) {
			return &anishinaabeRuleResult{relevance: anishinaabeRelevanceCore, confidence: anishinaabeConfidenceCore}
		}
	}
	for _, p := range anishinaabePeripheralPatterns {
		if p.MatchString(lower) {
			return &anishinaabeRuleResult{relevance: anishinaabeRelevancePeripheral, confidence: anishinaabeConfidencePeripheral}
		}
	}
	return &anishinaabeRuleResult{relevance: anishinaabeRelevanceNot, confidence: anishinaabeConfidenceDefault}
}
