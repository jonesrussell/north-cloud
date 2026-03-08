package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// obituaryKeywords are phrases whose presence (case-insensitive) strongly
// indicates that the page is an obituary. Requiring 2+ matches avoids
// false positives from pages that incidentally mention one memorial term.
var obituaryKeywords = []string{
	"passed away",
	"survived by",
	"predeceased",
	"in loving memory",
	"memorial service",
	"funeral",
	"obituary",
	"condolences",
	"celebration of life",
	"rest in peace",
}

// obituaryCrimeSuppressors are phrases that, when present, suppress
// obituary classification. Crime articles often mention death ("passed
// away") alongside investigation language; this negative-keyword check
// prevents misclassifying crime reports as obituaries.
var obituaryCrimeSuppressors = []string{
	"police said",
	"charged with",
	"investigation",
	"suspect",
	"arrested",
	"under investigation",
	"crime",
}

// classifyFromObituaryKeywords checks title + raw_text for obituary-related
// keywords. Returns ContentTypeObituary with confidence 0.80 when at least
// 2 keyword matches are found AND no crime suppressor keywords are present.
// Returns nil if no obituary signal is detected or if crime language suppresses it.
func (c *ContentTypeClassifier) classifyFromObituaryKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	// Crime suppression: if any crime keyword is present, bail out early
	for _, suppressor := range obituaryCrimeSuppressors {
		if strings.Contains(combinedText, suppressor) {
			c.logger.Debug("Obituary suppressed by crime keyword",
				infralogger.String("content_id", raw.ID),
				infralogger.String("suppressor", suppressor),
			)
			return nil
		}
	}

	matches := 0

	for _, kw := range obituaryKeywords {
		if strings.Contains(combinedText, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			c.logger.Debug("Obituary detected via keyword heuristic",
				infralogger.String("content_id", raw.ID),
				infralogger.Int("keyword_matches", matches),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeObituary,
				Confidence: keywordHeuristicConfidence,
				Method:     "keyword_heuristic",
				Reason:     "Obituary keywords detected in content",
			}
		}
	}

	return nil
}
