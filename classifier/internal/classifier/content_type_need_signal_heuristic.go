package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
	"github.com/jonesrussell/north-cloud/infrastructure/signal"
)

// classifyFromNeedSignalKeywords checks title + raw_text against the shared
// need-signal keyword list and the unified threshold contract in
// infrastructure/signal (docs/specs/lead-pipeline.md). Both this path and
// signal-crawler's scoring gate delegate to the same helper so the two sides
// cannot drift.
func (c *ContentTypeClassifier) classifyFromNeedSignalKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	ok, conf, matches := signal.Evaluate(combinedText, allNeedSignalKeywords())
	if !ok {
		return nil
	}

	c.logger.Debug("Need signal detected via keyword heuristic",
		infralogger.String("content_id", raw.ID),
		infralogger.Int("keyword_matches", matches),
	)
	return &ContentTypeResult{
		Type:       domain.ContentTypeNeedSignal,
		Confidence: conf,
		Method:     "keyword_heuristic",
		Reason:     "Need signal keywords detected in content",
	}
}
