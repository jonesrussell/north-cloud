package bootstrap

import (
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// MapExtractedToRawContentForTest exposes mapExtractedToRawContent for testing.
func MapExtractedToRawContentForTest(
	content *fetcher.ExtractedContent,
	sourceName string,
	logger infralogger.Logger,
) *storage.RawContent {
	return mapExtractedToRawContent(content, sourceName, logger)
}

// ParsePublishedDateForTest exposes parsePublishedDate for testing.
func ParsePublishedDateForTest(raw string) (time.Time, bool) {
	return parsePublishedDate(raw)
}
