// classifier/internal/classifier/location.go
package classifier

import (
	"context"
	"regexp"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/data"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// Zone weights for location scoring.
const (
	HeadlineWeight = 3.0
	LedeWeight     = 2.5
	BodyWeight     = 1.0
)

// Specificity bonuses.
const (
	CityBonus     = 3
	ProvinceBonus = 2
	CountryBonus  = 1
)

// DominanceThreshold is the minimum margin winner must have over second place.
const DominanceThreshold = 0.30

// Confidence values.
const (
	AmbiguousConfidence = 0.5  // When no clear winner
	HighConfidence      = 0.95 // When single location dominates
	BaseConfidence      = 0.6  // Minimum confidence for winners
	ConfidenceRange     = 0.35 // Range from base to high
)

// Entity type constants.
const (
	EntityTypeCity     = "city"
	EntityTypeProvince = "province"
	EntityTypeCountry  = "country"
)

// LocationEntity represents a detected location mention.
type LocationEntity struct {
	Raw        string
	Normalized string
	EntityType string
	Province   string // For cities, the province they're in
}

// LocationClassifier detects article locations from content.
type LocationClassifier struct {
	log infralogger.Logger
}

// NewLocationClassifier creates a new location classifier.
func NewLocationClassifier(log infralogger.Logger) *LocationClassifier {
	return &LocationClassifier{log: log}
}

// provincePatterns maps regex patterns to province codes.
// Only full province names are matched to avoid false positives.
// Abbreviations like "ON", "BC" are ambiguous and excluded.
var provincePatterns = map[string]string{
	`\bontario\b`:                   "ON",
	`\bquebec\b`:                    "QC",
	`\bbritish columbia\b`:          "BC",
	`\balberta\b`:                   "AB",
	`\bmanitoba\b`:                  "MB",
	`\bsaskatchewan\b`:              "SK",
	`\bnova scotia\b`:               "NS",
	`\bnew brunswick\b`:             "NB",
	`\bnewfoundland\b`:              "NL",
	`\bnewfoundland and labrador\b`: "NL",
	`\bprince edward island\b`:      "PE",
	`\bnorthwest territories\b`:     "NT",
	`\byukon\b`:                     "YT",
	`\bnunavut\b`:                   "NU",
}

// provincePatternRegexes holds compiled regexes for province patterns.
var provincePatternRegexes = func() map[*regexp.Regexp]string {
	result := make(map[*regexp.Regexp]string, len(provincePatterns))
	for pattern, code := range provincePatterns {
		result[regexp.MustCompile("(?i)"+pattern)] = code
	}
	return result
}()

// countryPatterns maps regex patterns to country names.
// These patterns use word boundaries to avoid false matches.
var countryPatterns = map[string]string{
	`\bcanada\b`:        "canada",
	`\bcanadian\b`:      "canada",
	`\bunited states\b`: "united_states",
	`\bu\.s\.\b`:        "united_states",
	`\bus\b`:            "united_states",
	`\bu\.s\.a\.\b`:     "united_states",
	`\busa\b`:           "united_states",
	`\bamerican\b`:      "united_states",
	`\bamerica\b`:       "united_states",
}

// countryPatternRegexes holds compiled regexes for country patterns.
var countryPatternRegexes = func() map[*regexp.Regexp]string {
	result := make(map[*regexp.Regexp]string, len(countryPatterns))
	for pattern, country := range countryPatterns {
		result[regexp.MustCompile("(?i)"+pattern)] = country
	}
	return result
}()

// wordPattern matches potential location names (capitalized words).
// We extract individual capitalized words and check them against our city list.
var wordPattern = regexp.MustCompile(`\b([A-Z][a-z]+)\b`)

// ExtractEntities finds location mentions in text.
func (lc *LocationClassifier) ExtractEntities(text string) []LocationEntity {
	entities := make([]LocationEntity, 0)

	// Extract Canadian cities (validated against our list)
	matches := wordPattern.FindAllString(text, -1)

	seen := make(map[string]bool)
	for _, match := range matches {
		normalized := strings.ToLower(strings.TrimSpace(match))
		if seen[normalized] {
			continue
		}

		if data.IsValidCanadianCity(normalized) {
			seen[normalized] = true
			canonicalSlug := data.NormalizeCityName(normalized)
			province, _ := data.GetProvinceForCity(normalized)
			entities = append(entities, LocationEntity{
				Raw:        match,
				Normalized: canonicalSlug,
				EntityType: EntityTypeCity,
				Province:   province,
			})
		}
	}

	// Extract provinces using word-boundary regex patterns
	for re, code := range provincePatternRegexes {
		if re.MatchString(text) {
			if !seen["province:"+code] {
				seen["province:"+code] = true
				entities = append(entities, LocationEntity{
					Raw:        re.String(),
					Normalized: code,
					EntityType: EntityTypeProvince,
				})
			}
		}
	}

	// Extract countries using word-boundary regex patterns
	for re, country := range countryPatternRegexes {
		if re.MatchString(text) {
			if !seen["country:"+country] {
				seen["country:"+country] = true
				entities = append(entities, LocationEntity{
					Raw:        re.String(),
					Normalized: country,
					EntityType: EntityTypeCountry,
				})
			}
		}
	}

	return entities
}

// locationScore holds accumulated scores for a location.
type locationScore struct {
	entity LocationEntity
	score  float64
}

// scoreLocations determines the dominant location from headline, lede, and body.
func (lc *LocationClassifier) scoreLocations(headline, lede, body string) *domain.LocationResult {
	scores := make(map[string]*locationScore)

	// Score headline entities (3.0x weight)
	lc.scoreZone(headline, HeadlineWeight, scores)

	// Score lede entities (2.5x weight)
	lc.scoreZone(lede, LedeWeight, scores)

	// Score body entities (1.0x weight)
	lc.scoreZone(body, BodyWeight, scores)

	// Find dominant location
	return lc.determineDominant(scores)
}

// scoreZone extracts entities from a text zone and adds weighted scores.
func (lc *LocationClassifier) scoreZone(text string, weight float64, scores map[string]*locationScore) {
	entities := lc.ExtractEntities(text)

	for _, e := range entities {
		key := e.EntityType + ":" + e.Normalized
		bonus := lc.getSpecificityBonus(e.EntityType)

		if existing, ok := scores[key]; ok {
			existing.score += weight * float64(bonus)
		} else {
			scores[key] = &locationScore{
				entity: e,
				score:  weight * float64(bonus),
			}
		}
	}
}

// getSpecificityBonus returns the bonus multiplier for an entity type.
func (lc *LocationClassifier) getSpecificityBonus(entityType string) int {
	switch entityType {
	case EntityTypeCity:
		return CityBonus
	case EntityTypeProvince:
		return ProvinceBonus
	case EntityTypeCountry:
		return CountryBonus
	default:
		return 0
	}
}

// determineDominant finds the winning location using dominance rule.
func (lc *LocationClassifier) determineDominant(scores map[string]*locationScore) *domain.LocationResult {
	if len(scores) == 0 {
		return &domain.LocationResult{
			Country:     "unknown",
			Specificity: domain.SpecificityUnknown,
			Confidence:  0,
		}
	}

	// Find top two scores
	var first, second *locationScore
	for _, s := range scores {
		if first == nil || s.score > first.score {
			second = first
			first = s
		} else if second == nil || s.score > second.score {
			second = s
		}
	}

	// Apply dominance rule: winner must beat second by 30%
	if second != nil {
		margin := (first.score - second.score) / first.score
		if margin < DominanceThreshold {
			return &domain.LocationResult{
				Country:     "unknown",
				Specificity: domain.SpecificityUnknown,
				Confidence:  AmbiguousConfidence,
			}
		}
	}

	// Build result from winner
	result := &domain.LocationResult{
		Confidence: lc.calculateConfidence(first.score, second),
	}

	switch first.entity.EntityType {
	case EntityTypeCity:
		result.City = first.entity.Normalized
		result.Province = first.entity.Province
		result.Country = "canada" // All validated cities are Canadian
		result.Specificity = domain.SpecificityCity
	case EntityTypeProvince:
		result.Province = first.entity.Normalized
		result.Country = "canada"
		result.Specificity = domain.SpecificityProvince
	case EntityTypeCountry:
		result.Country = first.entity.Normalized
		result.Specificity = domain.SpecificityCountry
	}

	return result
}

// calculateConfidence computes confidence based on score margin.
func (lc *LocationClassifier) calculateConfidence(winnerScore float64, second *locationScore) float64 {
	if second == nil {
		return HighConfidence // No competition = high confidence
	}
	margin := (winnerScore - second.score) / winnerScore
	// Scale margin (0.3 to 1.0) to confidence (0.6 to 0.95)
	marginRange := 1 - DominanceThreshold
	return BaseConfidence + (margin-DominanceThreshold)/marginRange*ConfidenceRange
}

// Classify determines the location of an article from its content.
// It never considers the publisher's location - only content-derived entities.
func (lc *LocationClassifier) Classify(_ context.Context, raw *domain.RawContent) (*domain.LocationResult, error) {
	headline := raw.Title
	lede := lc.extractLede(raw.RawText)
	body := raw.RawText

	result := lc.scoreLocations(headline, lede, body)
	result.Specificity = result.GetSpecificity()

	return result, nil
}

// ledeMaxChars is the maximum number of characters for the lede.
const ledeMaxChars = 500

// extractLede gets the first paragraph from the raw text.
func (lc *LocationClassifier) extractLede(text string) string {
	// Split by double newlines (paragraph breaks)
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) > 0 {
		return strings.TrimSpace(paragraphs[0])
	}

	// Fallback: first 500 characters
	if len(text) > ledeMaxChars {
		return text[:ledeMaxChars]
	}
	return text
}
