package logs_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestCategoryString(t *testing.T) {
	t.Helper()

	tests := []struct {
		category logs.Category
		expected string
	}{
		{logs.CategoryLifecycle, "crawler.lifecycle"},
		{logs.CategoryFetch, "crawler.fetch"},
		{logs.CategoryExtract, "crawler.extract"},
		{logs.CategoryError, "crawler.error"},
		{logs.CategoryRateLimit, "crawler.rate_limit"},
		{logs.CategoryQueue, "crawler.queue"},
		{logs.CategoryMetrics, "crawler.metrics"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.String(); got != tt.expected {
				t.Errorf("Category.String() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestCategoryShortName(t *testing.T) {
	t.Helper()

	tests := []struct {
		category logs.Category
		expected string
	}{
		{logs.CategoryLifecycle, "lifecycle"},
		{logs.CategoryFetch, "fetch"},
		{logs.CategoryError, "error"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.category.ShortName(); got != tt.expected {
				t.Errorf("Category.ShortName() = %q, want %q", got, tt.expected)
			}
		})
	}
}
