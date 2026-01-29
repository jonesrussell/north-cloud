package logs

import "strings"

// Category defines the type of log event for filtering.
type Category string

const (
	CategoryLifecycle Category = "crawler.lifecycle"
	CategoryFetch     Category = "crawler.fetch"
	CategoryExtract   Category = "crawler.extract"
	CategoryError     Category = "crawler.error"
	CategoryRateLimit Category = "crawler.rate_limit"
	CategoryQueue     Category = "crawler.queue"
	CategoryMetrics   Category = "crawler.metrics"
)

// String returns the category as a string.
func (c Category) String() string {
	return string(c)
}

// ShortName returns the category without the "crawler." prefix.
func (c Category) ShortName() string {
	return strings.TrimPrefix(string(c), "crawler.")
}

// AllCategories returns all valid categories.
func AllCategories() []Category {
	return []Category{
		CategoryLifecycle,
		CategoryFetch,
		CategoryExtract,
		CategoryError,
		CategoryRateLimit,
		CategoryQueue,
		CategoryMetrics,
	}
}
