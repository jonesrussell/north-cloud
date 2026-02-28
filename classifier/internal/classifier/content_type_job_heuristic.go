package classifier

import (
	"strings"

	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// jobKeywords are phrases whose presence (case-insensitive) strongly
// indicates that the page is a job listing. Requiring 2+ matches avoids
// false positives from pages that incidentally mention one employment term.
var jobKeywords = []string{
	"apply now",
	"qualifications",
	"salary",
	"compensation",
	"job description",
	"requirements",
	"responsibilities",
	"full-time",
	"part-time",
	"resume",
	"position available",
}

// classifyFromJobKeywords checks title + raw_text for job-related
// keywords. Returns ContentTypeJob with confidence 0.80 when at least
// 2 keyword matches are found.
// Returns nil if no job signal is detected.
func (c *ContentTypeClassifier) classifyFromJobKeywords(
	raw *domain.RawContent,
) *ContentTypeResult {
	combinedText := strings.ToLower(raw.Title + " " + raw.RawText)

	matches := 0

	for _, kw := range jobKeywords {
		if strings.Contains(combinedText, kw) {
			matches++
		}
		if matches >= minKeywordMatches {
			c.logger.Debug("Job detected via keyword heuristic",
				infralogger.String("content_id", raw.ID),
				infralogger.Int("keyword_matches", matches),
			)
			return &ContentTypeResult{
				Type:       domain.ContentTypeJob,
				Confidence: keywordHeuristicConfidence,
				Method:     "keyword_heuristic",
				Reason:     "Job keywords detected in content",
			}
		}
	}

	return nil
}
