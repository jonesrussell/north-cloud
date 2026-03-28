package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// needSignalKeywords are phrases whose presence (case-insensitive) indicates
// that the page content signals an organization may need web development
// services. Requiring 2+ matches avoids false positives.
var needSignalKeywords = []string{
	// Outdated websites / tech migrations
	"drupal 7",
	"site migration",
	"website migration",
	"legacy website",
	"website overhaul",
	"website redesign",
	"site redesign",
	"outdated website",
	"wordpress migration",
	"joomla migration",
	"platform migration",

	// Funding and grants
	"funding announcement",
	"grant funding",
	"digital transformation",
	"website modernization",
	"technology modernization",
	"infrastructure funding",
	"capital funding",

	// Job postings for developers
	"web developer",
	"frontend developer",
	"full stack developer",
	"website development",
	"seeking a developer",
	"hiring a developer",

	// New programs / expansions
	"new program launch",
	"program expansion",
	"service expansion",
	"digital strategy",
	"online presence",

	// Technology needs
	"accessibility compliance",
	"wcag compliance",
	"digital services",
	"web application",
	"content management system",
}

// classifyFromNeedSignalKeywords checks title + raw_text for need-signal
// keywords. Returns ContentTypeNeedSignal with confidence 0.80 when at least
// minKeywordMatches are found.
// Returns nil if no need signal is detected.
func (c *ContentTypeClassifier) classifyFromNeedSignalKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	matches := 0

	for _, kw := range needSignalKeywords {
		if strings.Contains(combinedText, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			c.logger.Debug("Need signal detected via keyword heuristic",
				infralogger.String("content_id", raw.ID),
				infralogger.Int("keyword_matches", matches),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeNeedSignal,
				Confidence: keywordHeuristicConfidence,
				Method:     "keyword_heuristic",
				Reason:     "Need signal keywords detected in content",
			}
		}
	}

	return nil
}
