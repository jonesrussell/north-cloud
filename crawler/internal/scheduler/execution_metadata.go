package scheduler

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

// crawlMetricsKey is the JSONB key for crawl metrics in execution metadata.
const crawlMetricsKey = "crawl_metrics"

// BuildExecutionMetadata converts a JobSummary into a domain.JSONBMap
// for storage in the execution's metadata JSONB column.
func BuildExecutionMetadata(summary *logs.JobSummary) domain.JSONBMap {
	if summary == nil {
		return nil
	}

	metrics := make(map[string]any)

	// Status codes
	if len(summary.StatusCodes) > 0 {
		metrics["status_codes"] = summary.StatusCodes
	}

	// Request counters
	metrics["requests_total"] = summary.RequestsTotal
	metrics["requests_failed"] = summary.RequestsFailed
	metrics["bytes_downloaded"] = summary.BytesFetched

	// Blocking/rate limiting
	if summary.CloudflareBlocks > 0 {
		metrics["cloudflare_blocks"] = summary.CloudflareBlocks
	}
	if summary.RateLimits > 0 {
		metrics["rate_limits"] = summary.RateLimits
	}

	// Error categories
	if len(summary.ErrorCategories) > 0 {
		metrics["error_categories"] = summary.ErrorCategories
	}

	// Top errors
	if len(summary.TopErrors) > 0 {
		metrics["top_errors"] = summary.TopErrors
	}

	// Response time stats
	if summary.RequestsTotal > 0 {
		responseTime := map[string]float64{
			"avg_ms": summary.ResponseTimeAvgMs,
			"min_ms": summary.ResponseTimeMinMs,
			"max_ms": summary.ResponseTimeMaxMs,
		}
		metrics["response_time"] = responseTime
	}

	// Skip reasons
	skipped := BuildSkippedMap(summary)
	if len(skipped) > 0 {
		metrics["skipped"] = skipped
	}

	return domain.JSONBMap{crawlMetricsKey: metrics}
}

// BuildSkippedMap extracts non-zero skip counters into a map.
func BuildSkippedMap(summary *logs.JobSummary) map[string]int64 {
	skipped := make(map[string]int64)
	if summary.SkippedNonHTML > 0 {
		skipped["non_html"] = summary.SkippedNonHTML
	}
	if summary.SkippedMaxDepth > 0 {
		skipped["max_depth"] = summary.SkippedMaxDepth
	}
	if summary.SkippedRobotsTxt > 0 {
		skipped["robots_txt"] = summary.SkippedRobotsTxt
	}
	return skipped
}
