package classifier

import (
	"regexp"
	"strings"
)

// Coforge relevance constants.
const (
	coforgeRelevanceCore       = "core_coforge"
	coforgeRelevancePeripheral = "peripheral"
	coforgeRelevanceNot        = "not_relevant"
)

// Coforge rule confidence constants.
const (
	coforgeConfidenceCore       = 0.90
	coforgeConfidencePeripheral = 0.70
	coforgeConfidenceDefault    = 0.5
	coforgeRuleMLDisagreeWeight = 0.7
	coforgeMLOverrideThreshold  = 0.90
	coforgeBothAgreeWeight      = 2.0
	coforgeMLOverrideWeight     = 0.8
)

// coforgeRuleResult holds the result of rule-based coforge classification.
type coforgeRuleResult struct {
	relevance  string
	confidence float64
}

// Core coforge patterns - strong dev+entrepreneur intersection signal.
var coforgeCorePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(startup|company)\s+(open[- ]source|release|launch)\s+(sdk|api|tool|framework)`),
	regexp.MustCompile(`(?i)(series\s+[a-c]|seed\s+round|raised?\s+\$[\d.]+[mb])\s+.*(developer|dev\s+tool|sdk|api|platform)`),
	regexp.MustCompile(`(?i)(developer|dev)\s+(tool|platform|sdk|api)\s+.*(funding|launch|acqui)`),
	regexp.MustCompile(`(?i)(open[- ]source)\s+.*(business|revenue|funding|monetiz)`),
}

// Peripheral coforge patterns - single-domain signal.
var coforgePeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(series\s+[abc]|seed\s+round|ipo|funding\s+round)\b`),
	regexp.MustCompile(`(?i)\b(framework|sdk|api)\s+(release|launch|update)\b`),
	regexp.MustCompile(`(?i)\b(open[- ]source|github|npm|crates\.io)\b`),
	regexp.MustCompile(`(?i)\b(acqui\w+|merger|partner\w+)\b`),
	regexp.MustCompile(`(?i)\b(saas|devtools|developer\s+experience)\b`),
}

const coforgeRuleMaxBodyChars = 500

// classifyCoforgeByRules applies rule-based coforge classification.
func classifyCoforgeByRules(title, body string) *coforgeRuleResult {
	text := strings.ToLower(title + " " + body)
	if len(body) > coforgeRuleMaxBodyChars {
		text = strings.ToLower(title + " " + body[:coforgeRuleMaxBodyChars])
	}

	for _, p := range coforgeCorePatterns {
		if p.MatchString(text) {
			return &coforgeRuleResult{relevance: coforgeRelevanceCore, confidence: coforgeConfidenceCore}
		}
	}

	for _, p := range coforgePeripheralPatterns {
		if p.MatchString(text) {
			return &coforgeRuleResult{relevance: coforgeRelevancePeripheral, confidence: coforgeConfidencePeripheral}
		}
	}

	return &coforgeRuleResult{relevance: coforgeRelevanceNot, confidence: coforgeConfidenceDefault}
}
