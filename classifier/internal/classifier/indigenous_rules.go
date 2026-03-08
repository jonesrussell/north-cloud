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

// indigenousCorePatterns are strong multilingual signals for indigenous content.
var indigenousCorePatterns = []*regexp.Regexp{
	// English (Canada / North America)
	regexp.MustCompile(`(?i)\b(anishinaabe|anishinaabemowin|ojibwe|ojibwa|chippewa)\b`),
	regexp.MustCompile(`(?i)\b(first nations|indigenous peoples|indigenous community)\b`),
	regexp.MustCompile(`(?i)\b(m[eé]tis|metis nation)\b`),
	regexp.MustCompile(`(?i)\b(inuit|inuk)\b`),
	regexp.MustCompile(`(?i)\b(residential school|treaty rights|land rights|aboriginal)\b`),
	regexp.MustCompile(`(?i)\b(seven grandfathers|midewiwin|grand council)\b`),
	// English (Oceania)
	regexp.MustCompile(`(?i)\b(m[aā]ori|iwi|hap[uū]|wh[aā]nau)\b`),
	regexp.MustCompile(`(?i)\b(aboriginal australian|torres strait islander)\b`),
	// English (US / Hawaii)
	regexp.MustCompile(`(?i)\b(native hawaiian|tribal sovereignty|tribal nation)\b`),
	// English (Nordic)
	regexp.MustCompile(`(?i)\b(sami people|sámi|saami)\b`),
	// Spanish
	regexp.MustCompile(`(?i)\b(pueblos ind[ií]genas|comunidad ind[ií]gena)\b`),
	regexp.MustCompile(`(?i)\b(territorio ancestral|derechos ind[ií]genas)\b`),
	// French
	regexp.MustCompile(`(?i)\b(peuples autochtones|premi[eè]res nations)\b`),
	regexp.MustCompile(`(?i)\b(droits autochtones|communaut[eé] autochtone)\b`),
	// Portuguese
	regexp.MustCompile(`(?i)\b(povos ind[ií]genas|terra ind[ií]gena|demarca[cç][aã]o)\b`),
	// Nordic (Sami)
	regexp.MustCompile(`(?i)\b(samefolket|urfolk|samisk|s[aá]pmi)\b`),
	regexp.MustCompile(`(?i)\b(alkuper[aä]iskansa|ursprungsfolk)\b`),
	// Te Reo Māori
	regexp.MustCompile(`(?i)\b(tangata whenua|te tiriti|mana whenua)\b`),
	// Japanese (Ainu)
	regexp.MustCompile(`(アイヌ|先住民族|アイヌ民族)`),
}

// indigenousPeripheralPatterns are weaker multilingual signals.
var indigenousPeripheralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(indigenous|native american|first nation)\b`),
	regexp.MustCompile(`(?i)\b(reconciliation|truth and reconciliation)\b`),
	regexp.MustCompile(`(?i)\b(reserve|reservation)\b`),
	regexp.MustCompile(`(?i)\b(autochtone?)\b`),
	regexp.MustCompile(`(?i)\b(ind[ií]gena)\b`),
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
