package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// rfpKeywords are phrases whose presence (case-insensitive) strongly
// indicates that the page is an RFP or procurement document. Requiring 2+
// matches avoids false positives from pages that incidentally mention one term.
var rfpKeywords = []string{
	"request for proposal",
	"request for tender",
	"request for quotation",
	"call for tenders",
	"call for proposals",
	"invitation to tender",
	"solicitation notice",
	"submission deadline",
	"proposal deadline",
	"closing date for submissions",
	"procurement",
	"bid submission",
	"scope of work",
}

// classifyFromRFPKeywords checks title + raw_text for RFP-related
// keywords. Returns ContentTypeRFP with confidence 0.80 when at least
// 2 keyword matches are found.
// Returns nil if no RFP signal is detected.
func (c *ContentTypeClassifier) classifyFromRFPKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	matches := 0

	for _, kw := range rfpKeywords {
		if strings.Contains(combinedText, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			c.logger.Debug("RFP detected via keyword heuristic",
				infralogger.String("content_id", raw.ID),
				infralogger.Int("keyword_matches", matches),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeRFP,
				Confidence: keywordHeuristicConfidence,
				Method:     "keyword_heuristic",
				Reason:     "RFP keywords detected in content",
			}
		}
	}

	return nil
}
