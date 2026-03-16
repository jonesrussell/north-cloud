package classifier

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
)

// Drill extraction confidence levels.
const (
	drillConfidenceComplete = "complete" // >= 1 full result (hole_id + intercept + grade)
	drillConfidencePartial  = "partial"  // found hole IDs but no grades, or vice versa
	drillConfidenceNone     = "none"     // no drill data found
)

// Regex patterns for drill result extraction.
var (
	// Hole ID patterns: DDH-24-001, RC-001, DH-001, BH-001, HOLE-A1, HQ-001, NQ-001, PQ-001
	reHoleID = regexp.MustCompile(`(?i)\b((?:DDH|RC|DH|BH|HOLE|HQ|NQ|PQ)[-\s]?\d{2,4}[-\s]?\d{1,4}[A-Z]?)\b`)

	// Intercept @ grade: "12.5m @ 3.2 g/t Au" or "12.5 metres grading 3.2 g/t gold"
	reInterceptGrade = regexp.MustCompile(`(?i)(\d+\.?\d*)\s*(?:m|metres?|meters?)\s*(?:@|grading|of|averaging)\s*(\d+\.?\d*)\s*(g/t|gpt|%|ppm|oz/t)\s*(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8|CuEq|AuEq|gold|silver|copper|nickel|zinc|lithium|uranium)?`)

	// From-to interval: "from 45.0m to 57.5m"
	reFromTo = regexp.MustCompile(`(?i)(?:from\s+)(\d+\.?\d*)\s*m?\s*(?:to|-)\s*(\d+\.?\d*)\s*m`)

	// Including sub-interval: "including 3.0m @ 8.1 g/t"
	reIncluding = regexp.MustCompile(`(?i)(?:including|incl\.?)\s+(\d+\.?\d*)\s*(?:m|metres?|meters?)\s*(?:@|of|grading)\s*(\d+\.?\d*)\s*(g/t|gpt|%|ppm|oz/t)\s*(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8|CuEq|AuEq|gold|silver|copper|nickel|zinc|lithium|uranium)?`)

	// Commodity symbol pattern (standalone)
	reCommoditySymbol = regexp.MustCompile(`(?i)\b(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8|CuEq|AuEq)\b`)
)

// extractDrillRegex runs regex-based drill result extraction on the full article body.
// Returns extracted results and a confidence level indicating completeness.
func extractDrillRegex(body string) ([]domain.DrillResult, string) {
	var results []domain.DrillResult

	// Find all hole IDs in the text
	holeMatches := reHoleID.FindAllStringSubmatch(body, -1)
	holeIDs := make([]string, 0, len(holeMatches))
	for _, m := range holeMatches {
		holeIDs = append(holeIDs, strings.ToUpper(strings.ReplaceAll(m[1], " ", "-")))
	}

	// Find all intercept @ grade matches
	interceptMatches := reInterceptGrade.FindAllStringSubmatchIndex(body, -1)
	for _, idx := range interceptMatches {
		full := body[idx[0]:idx[1]]
		sub := reInterceptGrade.FindStringSubmatch(full)
		if len(sub) < 4 {
			continue
		}

		intercept, _ := strconv.ParseFloat(sub[1], 64)
		grade, _ := strconv.ParseFloat(sub[2], 64)
		unit := normalizeUnitRaw(sub[3])
		commodity := ""
		if len(sub) >= 5 && sub[4] != "" {
			commodity = sub[4]
		}

		// Try to associate with nearest preceding hole ID
		holeID := findNearestHoleID(body, idx[0], holeIDs)

		results = append(results, domain.DrillResult{
			HoleID:     holeID,
			Commodity:  commodity,
			InterceptM: intercept,
			Grade:      grade,
			Unit:       unit,
		})
	}

	// Find from-to intervals with grades
	fromToMatches := reFromTo.FindAllStringSubmatchIndex(body, -1)
	for _, idx := range fromToMatches {
		full := body[idx[0]:idx[1]]
		sub := reFromTo.FindStringSubmatch(full)
		if len(sub) < 3 {
			continue
		}

		from, _ := strconv.ParseFloat(sub[1], 64)
		to, _ := strconv.ParseFloat(sub[2], 64)
		intercept := math.Abs(to - from)

		// Look for grade after the from-to pattern
		after := ""
		endIdx := idx[1]
		if endIdx+200 < len(body) {
			after = body[endIdx : endIdx+200]
		} else if endIdx < len(body) {
			after = body[endIdx:]
		}

		gradeMatch := reInterceptGrade.FindStringSubmatch(after)
		if gradeMatch == nil {
			// Try a simpler grade pattern after from-to
			simpleGrade := regexp.MustCompile(`(?i)(?:grading|@|of|averaging)\s*(\d+\.?\d*)\s*(g/t|gpt|%|ppm|oz/t)\s*(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8)?`)
			gradeMatch = simpleGrade.FindStringSubmatch(after)
		}

		if gradeMatch != nil && len(gradeMatch) >= 3 {
			grade, _ := strconv.ParseFloat(gradeMatch[1], 64)
			unit := normalizeUnitRaw(gradeMatch[2])
			commodity := ""
			if len(gradeMatch) >= 4 {
				commodity = gradeMatch[3]
			}
			holeID := findNearestHoleID(body, idx[0], holeIDs)

			// Don't add duplicate if already captured by intercept@grade pattern
			if !isDuplicateResult(results, holeID, intercept, grade) {
				results = append(results, domain.DrillResult{
					HoleID:     holeID,
					Commodity:  commodity,
					InterceptM: math.Round(intercept*100) / 100,
					Grade:      grade,
					Unit:       unit,
				})
			}
		}
	}

	// Find "including" sub-intervals
	inclMatches := reIncluding.FindAllStringSubmatch(body, -1)
	for _, sub := range inclMatches {
		if len(sub) < 4 {
			continue
		}
		intercept, _ := strconv.ParseFloat(sub[1], 64)
		grade, _ := strconv.ParseFloat(sub[2], 64)
		unit := normalizeUnitRaw(sub[3])
		commodity := ""
		if len(sub) >= 5 {
			commodity = sub[4]
		}

		if !isDuplicateResult(results, "", intercept, grade) {
			results = append(results, domain.DrillResult{
				HoleID:     "", // sub-intervals often don't restate the hole ID
				Commodity:  commodity,
				InterceptM: intercept,
				Grade:      grade,
				Unit:       unit,
			})
		}
	}

	// Determine confidence
	hasHoleIDs := len(holeIDs) > 0
	hasCompleteResults := false
	for _, r := range results {
		if r.HoleID != "" && r.InterceptM > 0 && r.Grade > 0 {
			hasCompleteResults = true
			break
		}
	}

	switch {
	case hasCompleteResults:
		return results, drillConfidenceComplete
	case hasHoleIDs || len(results) > 0:
		return results, drillConfidencePartial
	default:
		return nil, drillConfidenceNone
	}
}

// findNearestHoleID finds the closest preceding hole ID to a given position in the text.
func findNearestHoleID(body string, pos int, holeIDs []string) string {
	if len(holeIDs) == 0 {
		return ""
	}

	// Find all hole ID positions
	allPositions := reHoleID.FindAllStringIndex(body, -1)
	bestID := ""
	bestDist := len(body)

	for i, p := range allPositions {
		dist := pos - p[0]
		if dist >= 0 && dist < bestDist {
			bestDist = dist
			if i < len(holeIDs) {
				bestID = holeIDs[i]
			}
		}
	}

	return bestID
}

// normalizeUnitRaw does basic unit normalization for the regex extractor.
// Full normalization happens in the normalizer.
func normalizeUnitRaw(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "gpt", "g per tonne", "grams per tonne":
		return "g/t"
	case "g/t", "%", "ppm", "oz/t":
		return strings.ToLower(strings.TrimSpace(unit))
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

// isDuplicateResult checks if a result with the same intercept and grade already exists.
func isDuplicateResult(results []domain.DrillResult, holeID string, intercept, grade float64) bool {
	for _, r := range results {
		if r.InterceptM == intercept && r.Grade == grade {
			if holeID == "" || r.HoleID == holeID {
				return true
			}
		}
	}
	return false
}

// Suppress unused variable warning for reCommoditySymbol.
var _ = reCommoditySymbol
