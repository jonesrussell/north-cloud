// Package metrics provides metrics collection and reporting functionality.
package metrics

import (
	"time"
)

// WordCountBucketCount is the number of buckets in the word count histogram.
const WordCountBucketCount = 7

// wordCountBucketCount is the internal alias kept for backward compatibility within this package.
const wordCountBucketCount = WordCountBucketCount

// WordCountBuckets defines the upper bounds for word count histogram buckets.
// Intervals are half-open (exclusive upper bound): [0,50), [50,100), [100,200),
// [200,500), [500,1000), [1000,2000), [2000,∞).
var WordCountBuckets = [wordCountBucketCount]int{50, 100, 200, 500, 1000, 2000, -1}

// Metrics holds the processing metrics.
type Metrics struct {
	// ProcessedCount is the number of items processed.
	ProcessedCount int64
	// ErrorCount is the number of processing errors.
	ErrorCount int64
	// LastProcessedTime is the time of the last successful processing.
	LastProcessedTime time.Time
	// ProcessingDuration is the total time spent processing.
	ProcessingDuration time.Duration
	// StartTime is when the metrics collection began.
	StartTime time.Time
	// CurrentSource is the current source being processed.
	CurrentSource string
	// SuccessfulRequests is the number of successful HTTP requests.
	SuccessfulRequests int64
	// FailedRequests is the number of failed HTTP requests.
	FailedRequests int64
	// RateLimitedRequests is the number of rate-limited requests.
	RateLimitedRequests int64
	// URLsSkipped is the number of URLs skipped by the URL pre-filter.
	URLsSkipped int64
	// TemplateExtractions is the number of pages where a CMS template
	// (template registry or HTML detection) provided the extraction selectors.
	TemplateExtractions int64

	// PagesByType counts indexed pages broken down by detected page type
	// (article, listing, stub, other).
	PagesByType map[string]int64
	// ExtractionByMethod counts indexed pages broken down by the extraction
	// method that produced usable content (selector, template, heuristic,
	// readability).
	ExtractionByMethod map[string]int64
	// ExtractionSkipped counts pages that were skipped before indexing,
	// broken down by reason (url_filter, page_type, quality_gate).
	ExtractionSkipped map[string]int64
	// WordCountHistogram counts indexed pages per word-count bucket.
	// Index i covers words in [ WordCountBuckets[i-1], WordCountBuckets[i] ).
	// Index 0 is [0, 50), index 6 is [2000, ∞).
	WordCountHistogram [wordCountBucketCount]int64
}

// pageTypeMapSize is the number of distinct page type labels.
const pageTypeMapSize = 4

// extractionMethodMapSize is the number of distinct extraction method labels.
const extractionMethodMapSize = 4

// extractionSkipReasonMapSize is the number of distinct skip reason labels.
const extractionSkipReasonMapSize = 3

// NewMetrics creates a new Metrics instance with default values.
func NewMetrics() *Metrics {
	return &Metrics{
		ProcessedCount:      0,
		ErrorCount:          0,
		LastProcessedTime:   time.Time{},
		ProcessingDuration:  0,
		StartTime:           time.Now(),
		CurrentSource:       "",
		SuccessfulRequests:  0,
		FailedRequests:      0,
		RateLimitedRequests: 0,
		PagesByType:         make(map[string]int64, pageTypeMapSize),
		ExtractionByMethod:  make(map[string]int64, extractionMethodMapSize),
		ExtractionSkipped:   make(map[string]int64, extractionSkipReasonMapSize),
	}
}

// WordCountBucketIndex returns the histogram bucket index for the given word count.
// Bucket 0: [0,50), 1: [50,100), 2: [100,200), 3: [200,500),
// 4: [500,1000), 5: [1000,2000), 6: [2000,∞).
func WordCountBucketIndex(wordCount int) int {
	for i, upper := range WordCountBuckets {
		if upper == -1 || wordCount < upper {
			return i
		}
	}
	return wordCountBucketCount - 1
}
