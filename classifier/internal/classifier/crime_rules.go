// classifier/internal/classifier/crime_rules.go
package classifier

import (
	"regexp"
	"strings"
)

// crimeRuleBodyPrefixLen is the number of body characters used for rule matching.
// Crime signals in the body (e.g. "arrested after armed robbery") can upgrade
// relevance when the title is vague.
const crimeRuleBodyPrefixLen = 500

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
	// Opinion/editorial content
	regexp.MustCompile(`(?i)^(opinion|editorial|commentary|letters?|column|op-ed)\s*:`),
	regexp.MustCompile(`(?i)\b(i think|in my view|in our view|we believe|my view)\b`),
	// Lifestyle/non-crime content
	regexp.MustCompile(`(?i)\b(renovation|contractor|tournament|recipe|travel guide|lifeline)\b`),
	regexp.MustCompile(`(?i)\bbest\s+.+\s+in\s+the\s+.+\s+area\b`),
}

// authorityIndicators are words that signal real crime reporting (police, courts, arrests).
// Used to prevent fiction, metaphors, and opinion from being classified as core_street_crime.
const authorityIndicators = `police|rcmp|opp|sq|court|judge|investigation|suspect|accused|` +
	`officer|constable|detective|prosecution|charged|arrest|sentenced|convicted|` +
	`custody|detained|apprehended|wanted|manhunt`

// Violent crime patterns — murder/shooting/stabbing require authority indicators.
var violentCrimePatterns = []patternWithConf{
	// Murder/homicide: action + authority (either order)
	{regexp.MustCompile(`(?i)(murder|homicide|manslaughter).*(` + authorityIndicators + `)`), confidenceHighViolent},
	{regexp.MustCompile(`(?i)(` + authorityIndicators + `).*(murder|homicide|manslaughter)`), confidenceHighViolent},
	// Shooting: action + authority (either order)
	{regexp.MustCompile(`(?i)(shooting|shootout|shot dead|gunfire).*(` + authorityIndicators + `)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(` + authorityIndicators + `).*(shooting|shootout|shot dead|gunfire)`), confidenceMediumViolent},
	// Stabbing: action + authority (either order)
	{regexp.MustCompile(`(?i)(stab|stabbing|stabbed).*(` + authorityIndicators + `)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(` + authorityIndicators + `).*(stab|stabbing|stabbed)`), confidenceMediumViolent},
	// Assault already requires context (unchanged)
	{regexp.MustCompile(`(?i)(assault|assaulted).*(charged|arrest|police)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(charged|arrest|police).*(assault|assaulted)`), confidenceAssault},
	// Sexual assault and found dead are inherently crime-related (unchanged)
	{regexp.MustCompile(`(?i)(sexual assault|rape|sex assault)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(found dead|human remains)`), confidenceFoundDead},
	// Robbery/armed robbery: action + authority (either order)
	{regexp.MustCompile(`(?i)(robbery|robbed|armed robbery).*(` + authorityIndicators + `)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(` + authorityIndicators + `).*(robbery|robbed|armed robbery)`), confidenceAssault},
	// Carjacking: action + authority (either order)
	{regexp.MustCompile(`(?i)(carjack\w*).*(` + authorityIndicators + `)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(` + authorityIndicators + `).*(carjack\w*)`), confidenceMediumViolent},
	// Kidnapping/abduction: action + authority (either order)
	{regexp.MustCompile(`(?i)(kidnap\w*|abduct\w*).*(` + authorityIndicators + `)`), confidenceMediumViolent},
	{regexp.MustCompile(`(?i)(` + authorityIndicators + `).*(kidnap\w*|abduct\w*)`), confidenceMediumViolent},
	// Hostage situations are inherently crime-related
	{regexp.MustCompile(`(?i)(hostage)`), confidenceMediumViolent},
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

// Court outcome patterns — sentencing/verdict with authority context.
var courtOutcomePatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(sentenced|convicts?\b|convicted|found guilty|pleaded guilty|prison term)` +
		`.*(court|judge|jury|prison|jail|penitentiary|charges)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(court|judge|jury)` +
		`.*(sentenced|convicts?\b|convicted|found guilty|pleaded guilty|prison term)`), confidenceAssault},
}

// Accusation-style patterns — "faces/facing/charged with ... charges" plus crime-type terms.
// Catches headlines like "resident faces drug, weapon, assault charges" that lack "charged" (verb).
var accusationChargesPatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(faces?|facing|charged with).*(assault|drug|weapon|theft|robbery).*charges`), confidenceAssault},
	{regexp.MustCompile(`(?i)(assault|drug|weapon|theft|robbery).*charges.*(faces?|facing|charged with)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(faces?|facing|charged with).*charges.*(assault|drug|weapon|theft|robbery)`), confidenceAssault},
}

// Weapon + authority/charges — weapon(s) in charge or arrest context.
var weaponAuthorityPatterns = []patternWithConf{
	{regexp.MustCompile(`(?i)(weapons?).*(charges|arrest|charged|police)`), confidenceAssault},
	{regexp.MustCompile(`(?i)(charges|arrest|charged|police).*(weapons?)`), confidenceAssault},
}

// International patterns - downgrade to peripheral.
var internationalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(Minneapolis|U\.S\.|American|Mexico|European|Israel)`),
}

var justicePattern = regexp.MustCompile(
	`(?i)(charged|arrest|sentenced|trial|convicts?\b|convicted|found guilty|pleaded guilty|prison term)`,
)

// truncateBody returns up to the first n runes of body for rule matching.
func truncateBody(body string, n int) string {
	if n <= 0 || body == "" {
		return ""
	}
	runes := []rune(body)
	if len(runes) <= n {
		return body
	}
	return string(runes[:n])
}

// classifyByRules applies rule-based classification.
// Positive crime checks use title + body prefix so crime terms in the body
// can trigger rule-based upgrades. Exclusion and international checks use
// title only to avoid false negatives from stray body text.
func classifyByRules(title, body string) *ruleResult {
	// Exclusion: title-only (e.g. Register, Login, Listings, job ads)
	if matchesExclusion(title) {
		return &ruleResult{relevance: relevanceNotCrime, confidence: confidenceExclusion}
	}

	text := title + " " + truncateBody(body, crimeRuleBodyPrefixLen)

	result := &ruleResult{
		relevance:  relevanceNotCrime,
		confidence: confidenceDefault,
		crimeTypes: []string{},
	}

	// Positive crime checks: title + body prefix
	result = checkViolentCrime(result, text)
	result = checkPropertyCrime(result, text)
	result = checkDrugCrime(result, text)
	result = checkCourtOutcomes(result, text)
	result = checkAccusationCharges(result, text)
	result = checkWeaponAuthority(result, text)

	// International: title-only (downgrade to peripheral)
	result = checkInternational(result, title)

	// Add criminal_justice if has crime types and mentions arrest/charged (in title or body prefix)
	if len(result.crimeTypes) > 0 && justicePattern.MatchString(text) {
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

func checkViolentCrime(result *ruleResult, text string) *ruleResult {
	for _, p := range violentCrimePatterns {
		if p.pattern.MatchString(text) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "violent_crime") {
				result.crimeTypes = append(result.crimeTypes, "violent_crime")
			}
		}
	}
	return result
}

func checkPropertyCrime(result *ruleResult, text string) *ruleResult {
	for _, p := range propertyCrimePatterns {
		if p.pattern.MatchString(text) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "property_crime") {
				result.crimeTypes = append(result.crimeTypes, "property_crime")
			}
		}
	}
	return result
}

func checkDrugCrime(result *ruleResult, text string) *ruleResult {
	for _, p := range drugCrimePatterns {
		if p.pattern.MatchString(text) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "drug_crime") {
				result.crimeTypes = append(result.crimeTypes, "drug_crime")
			}
		}
	}
	return result
}

func checkCourtOutcomes(result *ruleResult, text string) *ruleResult {
	for _, p := range courtOutcomePatterns {
		if p.pattern.MatchString(text) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "criminal_justice") {
				result.crimeTypes = append(result.crimeTypes, "criminal_justice")
			}
		}
	}
	return result
}

func checkAccusationCharges(result *ruleResult, text string) *ruleResult {
	for _, p := range accusationChargesPatterns {
		if !p.pattern.MatchString(text) {
			continue
		}
		result.relevance = relevanceCoreStreetCrime
		result.confidence = maxFloat(result.confidence, p.confidence)
		addAccusationCrimeTypes(result, strings.ToLower(text))
		break
	}
	return result
}

func addAccusationCrimeTypes(result *ruleResult, lower string) {
	if (strings.Contains(lower, "assault") || strings.Contains(lower, "weapon") || strings.Contains(lower, "robbery")) &&
		!containsString(result.crimeTypes, "violent_crime") {
		result.crimeTypes = append(result.crimeTypes, "violent_crime")
	}
	if strings.Contains(lower, "drug") && !containsString(result.crimeTypes, "drug_crime") {
		result.crimeTypes = append(result.crimeTypes, "drug_crime")
	}
	if strings.Contains(lower, "theft") && !containsString(result.crimeTypes, "property_crime") {
		result.crimeTypes = append(result.crimeTypes, "property_crime")
	}
}

func checkWeaponAuthority(result *ruleResult, text string) *ruleResult {
	for _, p := range weaponAuthorityPatterns {
		if p.pattern.MatchString(text) {
			result.relevance = relevanceCoreStreetCrime
			result.confidence = maxFloat(result.confidence, p.confidence)
			if !containsString(result.crimeTypes, "violent_crime") {
				result.crimeTypes = append(result.crimeTypes, "violent_crime")
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
