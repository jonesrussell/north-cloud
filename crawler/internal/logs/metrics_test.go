package logs_test

import (
	"testing"

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
