package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// classifyFromNeedSignalKeywords checks title + raw_text for need-signal
// keywords derived from signalCategoryKeywords (the single source of truth).
// Returns ContentTypeNeedSignal with confidence 0.80 when at least
// minKeywordMatches are found.
// Returns nil if no need signal is detected.
func (c *ContentTypeClassifier) classifyFromNeedSignalKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	matches := 0

	for _, kw := range allNeedSignalKeywords() {
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
