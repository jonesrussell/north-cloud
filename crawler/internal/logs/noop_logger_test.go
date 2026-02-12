package logs_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestNoopJobLogger(t *testing.T) {
	t.Helper()

	logger := logs.NoopJobLogger()

	// Should not panic
	logger.Info(logs.CategoryFetch, "test")
	logger.Warn(logs.CategoryError, "test")
	logger.Error(logs.CategoryError, "test")
	logger.Debug(logs.CategoryQueue, "test")
	logger.JobStarted("source", "url")
	logger.JobCompleted(&logs.JobSummary{})
	logger.JobFailed(nil)
	logger.StartHeartbeat(context.Background())

	// Visibility metrics should not panic
	logger.RecordStatusCode(200)
	logger.RecordResponseTime(100 * time.Millisecond)
	logger.RecordBytes(1024)
	logger.IncrementCloudflare()
	logger.IncrementRateLimit()
	logger.IncrementRequestsTotal()
	logger.IncrementRequestsFailed()
	logger.IncrementSkippedNonHTML()
	logger.IncrementSkippedMaxDepth()
	logger.IncrementSkippedRobotsTxt()
	logger.RecordErrorCategory("timeout")

	if logger.IsDebugEnabled() {
		t.Error("NoopJobLogger.IsDebugEnabled() should return false")
	}

	if logger.IsTraceEnabled() {
		t.Error("NoopJobLogger.IsTraceEnabled() should return false")
	}

	scoped := logger.WithFields(logs.String("key", "value"))
	if scoped == nil {
		t.Error("NoopJobLogger.WithFields() should return non-nil")
	}

	summary := logger.BuildSummary()
	if summary == nil {
		t.Error("NoopJobLogger.BuildSummary() should return non-nil")
	}

	if err := logger.Flush(); err != nil {
		t.Errorf("NoopJobLogger.Flush() unexpected error: %v", err)
	}
}
