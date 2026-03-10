// Package rawcontent provides extraction and indexing of raw content from HTML.
package rawcontent

import "github.com/jonesrussell/north-cloud/crawler/internal/metrics"

// Extraction method label constants for crawler_extraction_method counter.
const (
	extractionMethodSelector    = "selector"
	extractionMethodTemplate    = "template"
	extractionMethodHeuristic   = "heuristic"
	extractionMethodReadability = "readability"
)

// Skip reason label constants for crawler_extraction_skipped counter.
const (
	skipReasonURLFilter   = "url_filter"
	skipReasonPageType    = "page_type"
	skipReasonQualityGate = "quality_gate"
)

// ExtractionQualityMetrics is a point-in-time snapshot of extraction quality
// counters collected by RawContentService.
type ExtractionQualityMetrics struct {
	// PagesByType counts indexed pages by page-type label
	// (article, listing, stub, other).
	PagesByType map[string]int64
	// ExtractionByMethod counts indexed pages by the extraction method that
	// produced usable content (selector, template, heuristic, readability).
	ExtractionByMethod map[string]int64
	// ExtractionSkipped counts skipped pages by reason
	// (url_filter, page_type, quality_gate).
	ExtractionSkipped map[string]int64
	// WordCountHistogram counts indexed pages per word-count bucket using
	// the same bucket bounds as metrics.WordCountBuckets.
	WordCountHistogram [metrics.WordCountBucketCount]int64
}
