package logs_test

import (
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestLogMetrics_Counters(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	m.IncrementPagesDiscovered()
	m.IncrementPagesDiscovered()
	m.IncrementPagesCrawled()
	m.IncrementItemsExtracted()
	m.IncrementErrors()
	m.IncrementLogsEmitted()
	m.IncrementThrottled()

	summary := m.BuildSummary()

	if summary.PagesDiscovered != 2 {
		t.Errorf("PagesDiscovered = %d, want 2", summary.PagesDiscovered)
	}
	if summary.PagesCrawled != 1 {
		t.Errorf("PagesCrawled = %d, want 1", summary.PagesCrawled)
	}
	if summary.ItemsExtracted != 1 {
		t.Errorf("ItemsExtracted = %d, want 1", summary.ItemsExtracted)
	}
	if summary.ErrorsCount != 1 {
		t.Errorf("ErrorsCount = %d, want 1", summary.ErrorsCount)
	}
	if summary.LogsEmitted != 1 {
		t.Errorf("LogsEmitted = %d, want 1", summary.LogsEmitted)
	}
	if summary.LogsThrottled != 1 {
		t.Errorf("LogsThrottled = %d, want 1", summary.LogsThrottled)
	}
}

func TestLogMetrics_StatusCodes(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	m.RecordStatusCode(200)
	m.RecordStatusCode(200)
	m.RecordStatusCode(404)

	summary := m.BuildSummary()

	if summary.StatusCodes[200] != 2 {
		t.Errorf("StatusCodes[200] = %d, want 2", summary.StatusCodes[200])
	}
	if summary.StatusCodes[404] != 1 {
		t.Errorf("StatusCodes[404] = %d, want 1", summary.StatusCodes[404])
	}
}

func TestLogMetrics_TopErrors(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	m.RecordError("timeout", "https://a.com")
	m.RecordError("timeout", "https://b.com")
	m.RecordError("connection refused", "https://c.com")

	summary := m.BuildSummary()

	if len(summary.TopErrors) != 2 {
		t.Fatalf("TopErrors length = %d, want 2", len(summary.TopErrors))
	}

	// Timeout should be first (count=2)
	if summary.TopErrors[0].Message != "timeout" || summary.TopErrors[0].Count != 2 {
		t.Errorf("TopErrors[0] = %+v, want timeout with count 2", summary.TopErrors[0])
	}
}

func TestLogMetrics_ThrottlePercent(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	// Emit 80, throttle 20 = 20% throttled
	for range 80 {
		m.IncrementLogsEmitted()
	}
	for range 20 {
		m.IncrementThrottled()
	}

	summary := m.BuildSummary()

	if summary.ThrottlePercent != 20.0 {
		t.Errorf("ThrottlePercent = %f, want 20.0", summary.ThrottlePercent)
	}
}

func TestLogMetrics_Getters(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	// Increment counters
	m.IncrementPagesCrawled()
	m.IncrementPagesCrawled()
	m.IncrementItemsExtracted()
	m.IncrementErrors()
	m.IncrementErrors()
	m.IncrementErrors()

	// Test individual getters
	if got := m.PagesCrawled(); got != 2 {
		t.Errorf("PagesCrawled() = %d, want 2", got)
	}
	if got := m.ItemsExtracted(); got != 1 {
		t.Errorf("ItemsExtracted() = %d, want 1", got)
	}
	if got := m.ErrorsCount(); got != 3 {
		t.Errorf("ErrorsCount() = %d, want 3", got)
	}
}

func TestLogMetrics_VisibilityCounters(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	m.IncrementCloudflare()
	m.IncrementCloudflare()
	m.IncrementRateLimit()
	m.IncrementRequestsTotal()
	m.IncrementRequestsTotal()
	m.IncrementRequestsTotal()
	m.IncrementRequestsFailed()
	m.IncrementSkippedNonHTML()
	m.IncrementSkippedNonHTML()
	m.IncrementSkippedMaxDepth()
	m.IncrementSkippedRobotsTxt()
	m.RecordBytes(1024)
	m.RecordBytes(2048)

	summary := m.BuildSummary()

	if summary.CloudflareBlocks != 2 {
		t.Errorf("CloudflareBlocks = %d, want 2", summary.CloudflareBlocks)
	}
	if summary.RateLimits != 1 {
		t.Errorf("RateLimits = %d, want 1", summary.RateLimits)
	}
	if summary.RequestsTotal != 3 {
		t.Errorf("RequestsTotal = %d, want 3", summary.RequestsTotal)
	}
	if summary.RequestsFailed != 1 {
		t.Errorf("RequestsFailed = %d, want 1", summary.RequestsFailed)
	}
	if summary.SkippedNonHTML != 2 {
		t.Errorf("SkippedNonHTML = %d, want 2", summary.SkippedNonHTML)
	}
	if summary.SkippedMaxDepth != 1 {
		t.Errorf("SkippedMaxDepth = %d, want 1", summary.SkippedMaxDepth)
	}
	if summary.SkippedRobotsTxt != 1 {
		t.Errorf("SkippedRobotsTxt = %d, want 1", summary.SkippedRobotsTxt)
	}

	const expectedBytes int64 = 3072
	if summary.BytesFetched != expectedBytes {
		t.Errorf("BytesFetched = %d, want %d", summary.BytesFetched, expectedBytes)
	}
}

func TestLogMetrics_ResponseTime(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	// Simulate 3 requests to populate avg
	m.IncrementRequestsTotal()
	m.IncrementRequestsTotal()
	m.IncrementRequestsTotal()

	m.RecordResponseTime(100 * time.Millisecond)
	m.RecordResponseTime(200 * time.Millisecond)
	m.RecordResponseTime(300 * time.Millisecond)

	summary := m.BuildSummary()

	// Avg = (100+200+300)/3 = 200ms
	if summary.ResponseTimeAvgMs != 200.0 {
		t.Errorf("ResponseTimeAvgMs = %f, want 200.0", summary.ResponseTimeAvgMs)
	}
	if summary.ResponseTimeMinMs != 100.0 {
		t.Errorf("ResponseTimeMinMs = %f, want 100.0", summary.ResponseTimeMinMs)
	}
	if summary.ResponseTimeMaxMs != 300.0 {
		t.Errorf("ResponseTimeMaxMs = %f, want 300.0", summary.ResponseTimeMaxMs)
	}
}

func TestLogMetrics_ResponseTime_SingleRequest(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()
	m.IncrementRequestsTotal()
	m.RecordResponseTime(150 * time.Millisecond)

	summary := m.BuildSummary()

	if summary.ResponseTimeAvgMs != 150.0 {
		t.Errorf("ResponseTimeAvgMs = %f, want 150.0", summary.ResponseTimeAvgMs)
	}
	if summary.ResponseTimeMinMs != 150.0 {
		t.Errorf("ResponseTimeMinMs = %f, want 150.0", summary.ResponseTimeMinMs)
	}
	if summary.ResponseTimeMaxMs != 150.0 {
		t.Errorf("ResponseTimeMaxMs = %f, want 150.0", summary.ResponseTimeMaxMs)
	}
}

func TestLogMetrics_ResponseTime_NoRequests(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()
	summary := m.BuildSummary()

	if summary.ResponseTimeAvgMs != 0 {
		t.Errorf("ResponseTimeAvgMs = %f, want 0", summary.ResponseTimeAvgMs)
	}
	if summary.ResponseTimeMinMs != 0 {
		t.Errorf("ResponseTimeMinMs = %f, want 0", summary.ResponseTimeMinMs)
	}
	if summary.ResponseTimeMaxMs != 0 {
		t.Errorf("ResponseTimeMaxMs = %f, want 0", summary.ResponseTimeMaxMs)
	}
}

func TestLogMetrics_ErrorCategories(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()

	m.RecordErrorCategory("timeout")
	m.RecordErrorCategory("timeout")
	m.RecordErrorCategory("network")
	m.RecordErrorCategory("http_server")

	summary := m.BuildSummary()

	if summary.ErrorCategories == nil {
		t.Fatal("ErrorCategories is nil")
	}
	if summary.ErrorCategories["timeout"] != 2 {
		t.Errorf("ErrorCategories[timeout] = %d, want 2", summary.ErrorCategories["timeout"])
	}
	if summary.ErrorCategories["network"] != 1 {
		t.Errorf("ErrorCategories[network] = %d, want 1", summary.ErrorCategories["network"])
	}
	if summary.ErrorCategories["http_server"] != 1 {
		t.Errorf("ErrorCategories[http_server] = %d, want 1", summary.ErrorCategories["http_server"])
	}
}

func TestLogMetrics_ErrorCategories_Empty(t *testing.T) {
	t.Helper()

	m := logs.NewLogMetrics()
	summary := m.BuildSummary()

	if summary.ErrorCategories != nil {
		t.Errorf("ErrorCategories should be nil when empty, got %v", summary.ErrorCategories)
	}
}
