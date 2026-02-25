// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"regexp"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
)

// CrawlContext holds the source config fetched once per crawl for reuse by link handling.
type CrawlContext struct {
	SourceID        string
	Source          *configtypes.Source
	ContentPatterns []*regexp.Regexp // Compiled patterns for content URL detection
}
