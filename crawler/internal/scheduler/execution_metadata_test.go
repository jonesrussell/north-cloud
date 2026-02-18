package scheduler_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/scheduler"
)

const crawlMetricsKey = "crawl_metrics"

func TestBuildExecutionMetadata_NilSummary(t *testing.T) {
	t.Helper()

	result := scheduler.BuildExecutionMetadata(nil)
	if result != nil {
		t.Errorf("expected nil for nil summary, got %v", result)
	}
}

func TestBuildExecutionMetadata_BasicFields(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{
		RequestsTotal:  10,
		RequestsFailed: 2,
		BytesFetched:   4096,
		StatusCodes: map[int]int64{
			200: 8,
			404: 2,
		},
	}

	result := scheduler.BuildExecutionMetadata(summary)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	metrics, ok := result[crawlMetricsKey].(map[string]any)
	if !ok {
		t.Fatalf("expected crawl_metrics map, got %T", result[crawlMetricsKey])
	}

	if metrics["requests_total"] != int64(10) {
		t.Errorf("requests_total = %v, want 10", metrics["requests_total"])
	}
	if metrics["requests_failed"] != int64(2) {
		t.Errorf("requests_failed = %v, want 2", metrics["requests_failed"])
	}

	const expectedBytes int64 = 4096
	if metrics["bytes_downloaded"] != expectedBytes {
		t.Errorf("bytes_downloaded = %v, want %d", metrics["bytes_downloaded"], expectedBytes)
	}

	statusCodes, scOK := metrics["status_codes"].(map[int]int64)
	if !scOK {
		t.Fatalf("expected status_codes map, got %T", metrics["status_codes"])
	}
	if statusCodes[200] != 8 {
		t.Errorf("status_codes[200] = %d, want 8", statusCodes[200])
	}
}

func TestBuildExecutionMetadata_CloudflareAndRateLimits(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{
		CloudflareBlocks: 3,
		RateLimits:       1,
	}

	result := scheduler.BuildExecutionMetadata(summary)
	metrics := result[crawlMetricsKey].(map[string]any)

	if metrics["cloudflare_blocks"] != int64(3) {
		t.Errorf("cloudflare_blocks = %v, want 3", metrics["cloudflare_blocks"])
	}
	if metrics["rate_limits"] != int64(1) {
		t.Errorf("rate_limits = %v, want 1", metrics["rate_limits"])
	}
}

func TestBuildExecutionMetadata_OmitsZeroOptionalFields(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{}

	result := scheduler.BuildExecutionMetadata(summary)
	metrics := result[crawlMetricsKey].(map[string]any)

	if _, exists := metrics["cloudflare_blocks"]; exists {
		t.Error("cloudflare_blocks should be omitted when zero")
	}
	if _, exists := metrics["rate_limits"]; exists {
		t.Error("rate_limits should be omitted when zero")
	}
	if _, exists := metrics["error_categories"]; exists {
		t.Error("error_categories should be omitted when empty")
	}
	if _, exists := metrics["top_errors"]; exists {
		t.Error("top_errors should be omitted when empty")
	}
	if _, exists := metrics["response_time"]; exists {
		t.Error("response_time should be omitted when no requests")
	}
	if _, exists := metrics["skipped"]; exists {
		t.Error("skipped should be omitted when all zero")
	}
}

func TestBuildExecutionMetadata_ResponseTime(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{
		RequestsTotal:     5,
		ResponseTimeAvgMs: 234.5,
		ResponseTimeMinMs: 89.2,
		ResponseTimeMaxMs: 1523.7,
	}

	result := scheduler.BuildExecutionMetadata(summary)
	metrics := result[crawlMetricsKey].(map[string]any)

	rt, ok := metrics["response_time"].(map[string]float64)
	if !ok {
		t.Fatalf("expected response_time map, got %T", metrics["response_time"])
	}

	const expectedAvg = 234.5
	if rt["avg_ms"] != expectedAvg {
		t.Errorf("avg_ms = %f, want %f", rt["avg_ms"], expectedAvg)
	}

	const expectedMin = 89.2
	if rt["min_ms"] != expectedMin {
		t.Errorf("min_ms = %f, want %f", rt["min_ms"], expectedMin)
	}

	const expectedMax = 1523.7
	if rt["max_ms"] != expectedMax {
		t.Errorf("max_ms = %f, want %f", rt["max_ms"], expectedMax)
	}
}

func TestBuildExecutionMetadata_SkipReasons(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{
		SkippedNonHTML:  12,
		SkippedMaxDepth: 3,
	}

	result := scheduler.BuildExecutionMetadata(summary)
	metrics := result[crawlMetricsKey].(map[string]any)

	skipped, ok := metrics["skipped"].(map[string]int64)
	if !ok {
		t.Fatalf("expected skipped map, got %T", metrics["skipped"])
	}

	const expectedNonHTML int64 = 12
	if skipped["non_html"] != expectedNonHTML {
		t.Errorf("non_html = %d, want %d", skipped["non_html"], expectedNonHTML)
	}

	const expectedMaxDepth int64 = 3
	if skipped["max_depth"] != expectedMaxDepth {
		t.Errorf("max_depth = %d, want %d", skipped["max_depth"], expectedMaxDepth)
	}

	if _, exists := skipped["robots_txt"]; exists {
		t.Error("robots_txt should be omitted when zero")
	}
}

func TestBuildExecutionMetadata_ErrorCategories(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{
		ErrorCategories: map[string]int64{
			"timeout":     2,
			"http_server": 1,
		},
	}

	result := scheduler.BuildExecutionMetadata(summary)
	metrics := result[crawlMetricsKey].(map[string]any)

	cats, ok := metrics["error_categories"].(map[string]int64)
	if !ok {
		t.Fatalf("expected error_categories map, got %T", metrics["error_categories"])
	}
	if cats["timeout"] != 2 {
		t.Errorf("timeout = %d, want 2", cats["timeout"])
	}
	if cats["http_server"] != 1 {
		t.Errorf("http_server = %d, want 1", cats["http_server"])
	}
}

func TestBuildExecutionMetadata_ExtractionQuality(t *testing.T) {
	t.Helper()

	summary := &logs.JobSummary{
		ItemsExtracted:           10,
		ItemsExtractedEmptyTitle: 2,
		ItemsExtractedEmptyBody:  1,
	}

	result := scheduler.BuildExecutionMetadata(summary)
	metrics := result[crawlMetricsKey].(map[string]any)

	eq, ok := metrics["extraction_quality"].(map[string]int64)
	if !ok {
		t.Fatalf("expected extraction_quality map, got %T", metrics["extraction_quality"])
	}
	if eq["items_indexed"] != 10 {
		t.Errorf("items_indexed = %d, want 10", eq["items_indexed"])
	}
	if eq["empty_title_count"] != 2 {
		t.Errorf("empty_title_count = %d, want 2", eq["empty_title_count"])
	}
	if eq["empty_body_count"] != 1 {
		t.Errorf("empty_body_count = %d, want 1", eq["empty_body_count"])
	}
}
