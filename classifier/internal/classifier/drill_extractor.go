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

// Named constants for magic numbers in regex field counts and rounding.
const (
	unitGramsPerTonne       = "g/t"
	minInterceptGradeFields = 4
	minHoleIDFields         = 3
	minGradeFields          = 3
	minGradeFieldsCommodity = 4
	minIncludingFields      = 4
	minIncludingCommodity   = 5
	roundingFactor          = 100
)

// Regex patterns for drill result extraction.
var (
	// Hole ID patterns: DDH-24-001, RC-001, DH-001, BH-001, HOLE-A1, HQ-001, NQ-001, PQ-001
	reHoleID = regexp.MustCompile(`(?i)\b((?:DDH|RC|DH|BH|HOLE|HQ|NQ|PQ)[-\s]?\d{2,4}[-\s]?\d{1,4}[A-Z]?)\b`)

	// Intercept @ grade: "12.5m @ 3.2 g/t Au" or "12.5 metres grading 3.2 g/t gold"
	reInterceptGrade = regexp.MustCompile(
		`(?i)(\d+\.?\d*)\s*(?:m|metres?|meters?)` +
			`\s*(?:@|grading|of|averaging)\s*(\d+\.?\d*)` +
			`\s*(g/t|gpt|%|ppm|oz/t)` +
			`\s*(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8|CuEq|AuEq` +
			`|gold|silver|copper|nickel|zinc|lithium|uranium)?`,
	)

	// From-to interval: "from 45.0m to 57.5m"
	reFromTo = regexp.MustCompile(`(?i)(?:from\s+)(\d+\.?\d*)\s*m?\s*(?:to|-)\s*(\d+\.?\d*)\s*m`)

	// Including sub-interval: "including 3.0m @ 8.1 g/t"
	reIncluding = regexp.MustCompile(
		`(?i)(?:including|incl\.?)\s+(\d+\.?\d*)` +
			`\s*(?:m|metres?|meters?)` +
			`\s*(?:@|of|grading)\s*(\d+\.?\d*)` +
			`\s*(g/t|gpt|%|ppm|oz/t)` +
			`\s*(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8|CuEq|AuEq` +
			`|gold|silver|copper|nickel|zinc|lithium|uranium)?`,
	)

	// Simple grade pattern for from-to fallback
	reSimpleGrade = regexp.MustCompile(`(?i)(?:grading|@|of|averaging)\s*(\d+\.?\d*)\s*(g/t|gpt|%|ppm|oz/t)\s*(Au|Ag|Cu|Ni|Zn|Li|Pb|U3O8)?`)
)

// extractDrillRegex runs regex-based drill result extraction on the full article body.
// Returns extracted results and a confidence level indicating completeness.
func extractDrillRegex(body string) (results []domain.DrillResult, confidence string) {
	// Find all hole IDs in the text
	holeIDs := extractHoleIDs(body)

	// Collect results from three extraction strategies
	interceptResults := extractInterceptGrades(body, holeIDs)
	results = make([]domain.DrillResult, 0, len(interceptResults))
	results = append(results, interceptResults...)
	results = append(results, extractFromToIntervals(body, holeIDs, results)...)
	results = append(results, extractIncludingIntervals(body, results)...)

	return determineDrillConfidence(results, holeIDs)
}

// extractHoleIDs finds all drill hole IDs in the text.
func extractHoleIDs(body string) []string {
	holeMatches := reHoleID.FindAllStringSubmatch(body, -1)
	holeIDs := make([]string, 0, len(holeMatches))
	for _, m := range holeMatches {
		holeIDs = append(holeIDs, strings.ToUpper(strings.ReplaceAll(m[1], " ", "-")))
	}
	return holeIDs
}

// extractInterceptGrades extracts "intercept @ grade" pattern matches.
func extractInterceptGrades(body string, holeIDs []string) []domain.DrillResult {
	var results []domain.DrillResult
	interceptMatches := reInterceptGrade.FindAllStringSubmatchIndex(body, -1)
	for _, idx := range interceptMatches {
		full := body[idx[0]:idx[1]]
		sub := reInterceptGrade.FindStringSubmatch(full)
		if len(sub) < minInterceptGradeFields {
			continue
		}

		intercept, _ := strconv.ParseFloat(sub[1], 64)
		grade, _ := strconv.ParseFloat(sub[2], 64)
		unit := normalizeUnitRaw(sub[3])
		commodity := ""
		if len(sub) >= minInterceptGradeFields+1 && sub[4] != "" {
			commodity = sub[4]
		}

		holeID := findNearestHoleID(body, idx[0], holeIDs)
		results = append(results, domain.DrillResult{
			HoleID:     holeID,
			Commodity:  commodity,
			InterceptM: intercept,
			Grade:      grade,
			Unit:       unit,
		})
	}
	return results
}

// extractFromToIntervals extracts "from X to Y" interval patterns with grades.
func extractFromToIntervals(
	body string, holeIDs []string, existing []domain.DrillResult,
) []domain.DrillResult {
	var results []domain.DrillResult
	fromToMatches := reFromTo.FindAllStringSubmatchIndex(body, -1)
	for _, idx := range fromToMatches {
		result, ok := parseFromToMatch(body, idx, holeIDs, existing)
		if ok {
			results = append(results, result)
		}
	}
	return results
}

// parseFromToMatch parses a single from-to interval match and looks for a grade.
func parseFromToMatch(
	body string, idx []int, holeIDs []string, existing []domain.DrillResult,
) (domain.DrillResult, bool) {
	full := body[idx[0]:idx[1]]
	sub := reFromTo.FindStringSubmatch(full)
	if len(sub) < minHoleIDFields {
		return domain.DrillResult{}, false
	}

	from, _ := strconv.ParseFloat(sub[1], 64)
	to, _ := strconv.ParseFloat(sub[2], 64)
	intercept := math.Abs(to - from)

	after := textAfterIndex(body, idx[1])
	gradeMatch := findGradeMatch(after)

	if len(gradeMatch) < minGradeFields {
		return domain.DrillResult{}, false
	}

	grade, _ := strconv.ParseFloat(gradeMatch[1], 64)
	unit := normalizeUnitRaw(gradeMatch[2])
	commodity := ""
	if len(gradeMatch) >= minGradeFieldsCommodity {
		commodity = gradeMatch[3]
	}
	holeID := findNearestHoleID(body, idx[0], holeIDs)

	if isDuplicateResult(existing, holeID, intercept, grade) {
		return domain.DrillResult{}, false
	}

	return domain.DrillResult{
		HoleID:     holeID,
		Commodity:  commodity,
		InterceptM: math.Round(intercept*roundingFactor) / roundingFactor,
		Grade:      grade,
		Unit:       unit,
	}, true
}

// textAfterIndex returns up to 200 characters of text after the given index.
func textAfterIndex(body string, endIdx int) string {
	const lookaheadChars = 200
	if endIdx+lookaheadChars < len(body) {
		return body[endIdx : endIdx+lookaheadChars]
	}
	if endIdx < len(body) {
		return body[endIdx:]
	}
	return ""
}

// findGradeMatch attempts to find a grade pattern in the given text.
func findGradeMatch(text string) []string {
	gradeMatch := reInterceptGrade.FindStringSubmatch(text)
	if len(gradeMatch) > 0 {
		return gradeMatch
	}
	// Try a simpler grade pattern
	return reSimpleGrade.FindStringSubmatch(text)
}

// extractIncludingIntervals extracts "including X m @ Y g/t" sub-intervals.
func extractIncludingIntervals(
	body string, existing []domain.DrillResult,
) []domain.DrillResult {
	var results []domain.DrillResult
	inclMatches := reIncluding.FindAllStringSubmatch(body, -1)
	for _, sub := range inclMatches {
		if len(sub) < minIncludingFields {
			continue
		}
		intercept, _ := strconv.ParseFloat(sub[1], 64)
		grade, _ := strconv.ParseFloat(sub[2], 64)
		unit := normalizeUnitRaw(sub[3])
		commodity := ""
		if len(sub) >= minIncludingCommodity {
			commodity = sub[4]
		}

		if !isDuplicateResult(existing, "", intercept, grade) {
			results = append(results, domain.DrillResult{
				HoleID:     "", // sub-intervals often don't restate the hole ID
				Commodity:  commodity,
				InterceptM: intercept,
				Grade:      grade,
				Unit:       unit,
			})
		}
	}
	return results
}

// determineDrillConfidence returns the confidence level based on results and hole IDs.
func determineDrillConfidence(
	results []domain.DrillResult, holeIDs []string,
) (finalResults []domain.DrillResult, confidence string) {
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
		return unitGramsPerTonne
	case unitGramsPerTonne, "%", "ppm", "oz/t":
		return strings.ToLower(strings.TrimSpace(unit))
	default:
		return strings.ToLower(strings.TrimSpace(unit))
	}
}

// isDuplicateResult checks if a result with the same intercept and grade already exists.
// Uses formatted string comparison to avoid float64 exact-equality issues from
// different computation paths (e.g. math.Abs vs direct parsing).
func isDuplicateResult(results []domain.DrillResult, holeID string, intercept, grade float64) bool {
	iStr := strconv.FormatFloat(intercept, 'f', 2, 64)
	gStr := strconv.FormatFloat(grade, 'f', 4, 64)
	for _, r := range results {
		rI := strconv.FormatFloat(r.InterceptM, 'f', 2, 64)
		rG := strconv.FormatFloat(r.Grade, 'f', 4, 64)
		if rI == iStr && rG == gStr {
			if holeID == "" || r.HoleID == holeID {
				return true
			}
		}
	}
	return false
}
