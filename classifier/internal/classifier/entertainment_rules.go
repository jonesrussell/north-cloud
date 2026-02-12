package classifier

import (
	"regexp"
	"strings"
)

// Entertainment relevance constants.
const (
	entertainmentRelevanceCore       = "core_entertainment"
	entertainmentRelevancePeripheral = "peripheral_entertainment"
	entertainmentRelevanceNot        = "not_entertainment"
)

const (
	entertainmentConfidenceCore       = 0.90
	entertainmentConfidencePeripheral = 0.70
	entertainmentConfidenceDefault    = 0.5
	entertainmentRuleMLDisagreeWeight = 0.7
	entertainmentMLOverrideThreshold  = 0.90
	entertainmentBothAgreeWeight      = 2.0
	entertainmentMLOverrideWeight     = 0.8
)

type entertainmentRuleResult struct {
	relevance  string
	confidence float64
}

var entertainmentCorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(film|movie|cinema|box office)\b`),
	regexp.MustCompile(`(?i)\b(tv show|series|premiere|finale|episode)\b`),
	regexp.MustCompile(`(?i)\b(album|single|tour|concert|grammy|billboard)\b`),
	regexp.MustCompile(`(?i)\b(video game|gaming|esports|release date)\b`),
	regexp.MustCompile(`(?i)\b(review|rating|oscar|emmy|golden globe)\b`),
	regexp.MustCompile(`(?i)\b(celebrity|starring|cast|trailer)\b`),
	// War-film specific patterns to boost entertainment relevance for war movies.
	regexp.MustCompile(`(?i)\b(war film|war movie|combat film|military drama)\b`),
	regexp.MustCompile(`(?i)\b(world war i+ film|wwi+ film|vietnam war (?:film|movie))\b`),
}

var entertainmentPeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(entertainment|arts|culture)\b`),
	regexp.MustCompile(`(?i)\b(music|film|television)\b`),
	regexp.MustCompile(`(?i)\b(streaming|netflix|spotify)\b`),
}

const entertainmentRuleMaxBodyChars = 500

func classifyEntertainmentByRules(title, body string) *entertainmentRuleResult {
	text := title + " " + body
	if len(body) > entertainmentRuleMaxBodyChars {
		text = title + " " + body[:entertainmentRuleMaxBodyChars]
	}
	lower := strings.ToLower(text)

	for _, p := range entertainmentCorePatterns {
		if p.MatchString(lower) {
			return &entertainmentRuleResult{relevance: entertainmentRelevanceCore, confidence: entertainmentConfidenceCore}
		}
	}
	for _, p := range entertainmentPeripheralPatterns {
		if p.MatchString(lower) {
			return &entertainmentRuleResult{relevance: entertainmentRelevancePeripheral, confidence: entertainmentConfidencePeripheral}
		}
	}
	return &entertainmentRuleResult{relevance: entertainmentRelevanceNot, confidence: entertainmentConfidenceDefault}
}
