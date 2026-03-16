package classifier

import (
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// commodityMap maps symbols and names to normalized slugs.
var commodityMap = map[string]string{
	"au":    "gold",
	"ag":    "silver",
	"cu":    "copper",
	"ni":    "nickel",
	"zn":    "zinc",
	"li":    "lithium",
	"pb":    "lead",
	"u3o8":  "uranium",
	"cueq":  "copper",
	"aueq":  "gold",
	"co":    "cobalt",
	"sn":    "tin",
	"pt":    "platinum",
	"pd":    "palladium",
	"ree":   "rare-earths",
	"fe":    "iron-ore",
	// Full names
	"gold":        "gold",
	"silver":      "silver",
	"copper":      "copper",
	"nickel":      "nickel",
	"zinc":        "zinc",
	"lithium":     "lithium",
	"lead":        "lead",
	"uranium":     "uranium",
	"cobalt":      "cobalt",
	"tin":         "tin",
	"platinum":    "platinum",
	"palladium":   "palladium",
	"rare-earths": "rare-earths",
	"iron-ore":    "iron-ore",
}

// unitMap maps unit variants to normalized forms.
var unitMap = map[string]string{
	"g/t":               "g/t",
	"gpt":               "g/t",
	"g per tonne":       "g/t",
	"grams per tonne":   "g/t",
	"grams per ton":     "g/t",
	"%":                 "%",
	"percent":           "%",
	"ppm":               "ppm",
	"parts per million": "ppm",
	"oz/t":              "oz/t",
	"ounces per ton":    "oz/t",
}

// normalizeCommodity maps a commodity symbol or name to its normalized slug.
func normalizeCommodity(raw string) string {
	if raw == "" {
		return ""
	}
	key := strings.ToLower(strings.TrimSpace(raw))
	if slug, ok := commodityMap[key]; ok {
		return slug
	}
	return key
}

// normalizeUnit maps a unit string to its normalized form.
func normalizeUnit(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if norm, ok := unitMap[key]; ok {
		return norm
	}
	return key
}

// normalizeHoleID uppercases and standardizes hole ID separators.
func normalizeHoleID(raw string) string {
	id := strings.ToUpper(strings.TrimSpace(raw))
	// Replace spaces with hyphens for consistency
	id = strings.ReplaceAll(id, " ", "-")
	return id
}

// normalizeDrillResults normalizes, deduplicates, and validates a slice of drill results.
func normalizeDrillResults(results []domain.DrillResult) []domain.DrillResult {
	seen := make(map[string]bool)
	var out []domain.DrillResult

	for _, r := range results {
		// Normalize fields
		r.HoleID = normalizeHoleID(r.HoleID)
		r.Commodity = normalizeCommodity(r.Commodity)
		r.Unit = normalizeUnit(r.Unit)

		// Validate: must have hole_id OR (intercept AND grade)
		if r.HoleID == "" && r.InterceptM == 0 && r.Grade == 0 {
			continue
		}

		// Deduplicate by hole_id + intercept + grade
		key := strings.Join([]string{
			r.HoleID,
			strings.TrimRight(strings.TrimRight(
				strconv.FormatFloat(r.InterceptM, 'f', 2, 64), "0"), "."),
			strings.TrimRight(strings.TrimRight(
				strconv.FormatFloat(r.Grade, 'f', 4, 64), "0"), "."),
		}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true

		out = append(out, r)
	}

	return out
}
