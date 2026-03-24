package processor

import (
	"github.com/jonesrussell/north-cloud/classifier/internal/config"
	"github.com/jonesrussell/north-cloud/classifier/internal/domain"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

// applyQualityGate filters classified content based on quality_score and content_type.
// - quality_score >= threshold: pass through
// - quality_score < threshold AND content_type=article: pass with LowQuality=true
// - quality_score < threshold AND content_type!=article: reject
func applyQualityGate(
	cfg config.QualityGateConfig,
	contents []*domain.ClassifiedContent,
	logger infralogger.Logger,
) []*domain.ClassifiedContent {
	if !cfg.Enabled {
		return contents
	}

	passed := make([]*domain.ClassifiedContent, 0, len(contents))

	for _, content := range contents {
		if content.QualityScore >= cfg.Threshold {
			passed = append(passed, content)
			continue
		}

		if content.ContentType == domain.ContentTypeArticle {
			content.LowQuality = true
			passed = append(passed, content)

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

		logger.Info("Quality gate: rejected non-article content",
			infralogger.String("url", content.URL),
			infralogger.String("source", content.SourceName),
			infralogger.String("content_type", content.ContentType),
			infralogger.Int("quality_score", content.QualityScore),
			infralogger.Int("threshold", cfg.Threshold),
			infralogger.String("reason", "non_article_below_threshold"),
		)
	}

	return passed
}
