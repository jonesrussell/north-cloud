package classifier

import (
	"regexp"
	"strings"
)

// Mining relevance constants.
const (
	miningRelevanceCore       = "core_mining"
	miningRelevancePeripheral = "peripheral_mining"
	miningRelevanceNot        = "not_mining"
)

// Mining rule confidence constants.
const (
	miningConfidenceCore       = 0.90
	miningConfidencePeripheral = 0.70
	miningConfidenceDefault    = 0.5
	miningRuleMLDisagreeWeight = 0.7
	miningMLOverrideThreshold  = 0.90
	miningBothAgreeWeight      = 2.0
	miningMLOverrideWeight     = 0.8
)

// miningRuleResult holds the result of rule-based mining classification.
type miningRuleResult struct {
	relevance  string
	confidence float64
}

// Core mining patterns - strong mining industry signal.
var miningCorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(gold|silver|copper|zinc|nickel|lithium|uranium)\s+(mining|exploration|drill|assay)`),
	regexp.MustCompile(`(?i)(mining|exploration)\s+(gold|silver|copper|zinc|nickel|lithium|uranium)`),
	regexp.MustCompile(`(?i)(drill\s+results?|assay\s+results?|intercept\s+\d)`),
	regexp.MustCompile(`(?i)(orebody|ore\s+body|deposit\s+(discovery|estimate))`),
	regexp.MustCompile(`(?i)(open-pit|underground)\s+(mine|mining)`),
}

// Peripheral mining patterns - weaker or tangential mining signal.
var miningPeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bmining\b`),
	regexp.MustCompile(`(?i)\bmineral\b`),
	regexp.MustCompile(`(?i)\bexploration\b`),
	regexp.MustCompile(`(?i)\bdrilling\b`),
	regexp.MustCompile(`(?i)\b(resource|reserve)s?\s+(estimate|report)`),
	regexp.MustCompile(`(?i)\b(smelter|refinery|concentrate)\b`),
}

const miningRuleMaxBodyChars = 500

// classifyMiningByRules applies rule-based mining classification.
func classifyMiningByRules(title, body string) *miningRuleResult {
	text := strings.ToLower(title + " " + body)
	if len(body) > miningRuleMaxBodyChars {
		text = strings.ToLower(title + " " + body[:miningRuleMaxBodyChars])
	}

	for _, p := range miningCorePatterns {
		if p.MatchString(text) {
			return &miningRuleResult{relevance: miningRelevanceCore, confidence: miningConfidenceCore}
		}
	}

	for _, p := range miningPeripheralPatterns {
		if p.MatchString(text) {
			return &miningRuleResult{relevance: miningRelevancePeripheral, confidence: miningConfidencePeripheral}
		}
	}

	return &miningRuleResult{relevance: miningRelevanceNot, confidence: miningConfidenceDefault}
}
