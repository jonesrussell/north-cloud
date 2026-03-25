package processor

import (
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// QualityGateResult holds the output of the quality gate filter.
type QualityGateResult struct {
	Passed      []*domain.ClassifiedContent
	RejectedIDs []string // Content IDs rejected by the gate (need status update)
}

// applyQualityGate filters classified content based on quality_score and content_type.
// - quality_score >= threshold: pass through (LowQuality cleared to false)
// - quality_score < threshold AND content_type=article: pass with LowQuality=true
// - quality_score < threshold AND content_type!=article: reject
func applyQualityGate(
	cfg config.QualityGateConfig,
	contents []*domain.ClassifiedContent,
	logger infralogger.Logger,
) QualityGateResult {
	if !cfg.Enabled {
		return QualityGateResult{Passed: contents}
	}

	passed := make([]*domain.ClassifiedContent, 0, len(contents))
	rejectedIDs := make([]string, 0)
	flaggedCount := 0

	for _, content := range contents {
		if content.QualityScore >= cfg.Threshold {
			content.LowQuality = false
			passed = append(passed, content)

			continue
		}

		if content.ContentType == domain.ContentTypeArticle {
			content.LowQuality = true
			passed = append(passed, content)
			flaggedCount++

			logger.Info("Quality gate: flagged low-quality article",
				infralogger.String("url", content.URL),
				infralogger.String("source", content.SourceName),
				infralogger.String("content_type", content.ContentType),
				infralogger.Int("quality_score", content.QualityScore),
				infralogger.Int("threshold", cfg.Threshold),
				infralogger.String("reason", "below_threshold"),
			)

			continue
		}

		rejectedIDs = append(rejectedIDs, content.ID)

		logger.Info("Quality gate: rejected non-article content",
			infralogger.String("url", content.URL),
			infralogger.String("source", content.SourceName),
			infralogger.String("content_type", content.ContentType),
			infralogger.Int("quality_score", content.QualityScore),
			infralogger.Int("threshold", cfg.Threshold),
			infralogger.String("reason", "non_article_below_threshold"),
		)
	}

	rejectedCount := len(rejectedIDs)
	if flaggedCount > 0 || rejectedCount > 0 {
		logger.Info("Quality gate summary",
			infralogger.Int("total", len(contents)),
			infralogger.Int("passed", len(passed)-flaggedCount),
			infralogger.Int("flagged", flaggedCount),
			infralogger.Int("rejected", rejectedCount),
			infralogger.Int("threshold", cfg.Threshold),
		)
	}

	return QualityGateResult{
		Passed:      passed,
		RejectedIDs: rejectedIDs,
	}
}
