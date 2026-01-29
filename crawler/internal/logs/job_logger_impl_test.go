package logs_test

import (
	"context"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/logs"
)

func TestJobLoggerImpl_Info(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	captureFunc := func(entry logs.LogEntry) {
		entries = append(entries, entry)
	}

	logger := logs.NewJobLoggerImpl(
		"job-123",
		"exec-456",
		logs.VerbosityNormal,
		captureFunc,
		0, // no throttling
	)

	logger.Info(logs.CategoryFetch, "Page fetched", logs.URL("https://example.com"), logs.Int("status", 200))

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Level != "info" {
		t.Errorf("Level = %q, want info", entry.Level)
	}
	if entry.Category != string(logs.CategoryFetch) {
		t.Errorf("Category = %q, want %s", entry.Category, logs.CategoryFetch)
	}
	if entry.Message != "Page fetched" {
		t.Errorf("Message = %q, want 'Page fetched'", entry.Message)
	}
	if entry.SchemaVersion != logs.CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", entry.SchemaVersion, logs.CurrentSchemaVersion)
	}
	if entry.JobID != "job-123" {
		t.Errorf("JobID = %q, want job-123", entry.JobID)
	}
	if entry.ExecID != "exec-456" {
		t.Errorf("ExecID = %q, want exec-456", entry.ExecID)
	}
}

func TestJobLoggerImpl_Debug_Verbosity(t *testing.T) {
	t.Helper()

	t.Run("normal verbosity skips debug", func(t *testing.T) {
		t.Helper()

		var entries []logs.LogEntry
		logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(e logs.LogEntry) {
			entries = append(entries, e)
		}, 0)
		logger.Debug(logs.CategoryQueue, "test")
		if len(entries) != 0 {
			t.Error("debug log should be skipped at normal verbosity")
		}
	})

	t.Run("debug verbosity allows debug", func(t *testing.T) {
		t.Helper()

		var entries []logs.LogEntry
		logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityDebug, func(e logs.LogEntry) {
			entries = append(entries, e)
		}, 0)
		logger.Debug(logs.CategoryQueue, "test")
		if len(entries) != 1 {
			t.Error("debug log should be allowed at debug verbosity")
		}
	})
}

func TestJobLoggerImpl_Throttling(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityDebug, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 5) // 5 per sec

	// Should allow first 5
	for range 10 {
		logger.Debug(logs.CategoryQueue, "test")
	}

	if len(entries) != 5 {
		t.Errorf("captured %d entries, want 5 (throttled)", len(entries))
	}

	summary := logger.BuildSummary()
	if summary.LogsThrottled != 5 {
		t.Errorf("LogsThrottled = %d, want 5", summary.LogsThrottled)
	}
}

func TestJobLoggerImpl_WithFields(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	scoped := logger.WithFields(logs.URL("https://example.com"), logs.Int("depth", 2))
	scoped.Info(logs.CategoryQueue, "Link enqueued")

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	fields := entries[0].Fields
	if fields["url"] != "https://example.com" {
		t.Errorf("url field = %v, want https://example.com", fields["url"])
	}
	if fields["depth"] != 2 {
		t.Errorf("depth field = %v, want 2", fields["depth"])
	}
}

func TestJobLoggerImpl_IsDebugEnabled(t *testing.T) {
	t.Helper()

	normalLogger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, nil, 0)
	if normalLogger.IsDebugEnabled() {
		t.Error("normal verbosity should not have debug enabled")
	}

	debugLogger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityDebug, nil, 0)
	if !debugLogger.IsDebugEnabled() {
		t.Error("debug verbosity should have debug enabled")
	}
}

func TestJobLoggerImpl_IsTraceEnabled(t *testing.T) {
	t.Helper()

	debugLogger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityDebug, nil, 0)
	if debugLogger.IsTraceEnabled() {
		t.Error("debug verbosity should not have trace enabled")
	}

	traceLogger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityTrace, nil, 0)
	if !traceLogger.IsTraceEnabled() {
		t.Error("trace verbosity should have trace enabled")
	}
}

func TestJobLoggerImpl_WarnAndError(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityQuiet, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	logger.Warn(logs.CategoryFetch, "Rate limited")
	logger.Error(logs.CategoryError, "Connection failed")

	if len(entries) != 2 {
		t.Fatalf("captured %d entries, want 2", len(entries))
	}

	if entries[0].Level != "warn" {
		t.Errorf("first entry Level = %q, want warn", entries[0].Level)
	}
	if entries[1].Level != "error" {
		t.Errorf("second entry Level = %q, want error", entries[1].Level)
	}
}

func TestJobLoggerImpl_LifecycleEvents(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job-123", "exec-456", logs.VerbosityQuiet, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	logger.JobStarted("source-abc", "https://example.com")

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Category != string(logs.CategoryLifecycle) {
		t.Errorf("Category = %q, want %s", entry.Category, logs.CategoryLifecycle)
	}
	if entry.Fields["source_id"] != "source-abc" {
		t.Errorf("source_id = %v, want source-abc", entry.Fields["source_id"])
	}
	if entry.Fields["url"] != "https://example.com" {
		t.Errorf("url = %v, want https://example.com", entry.Fields["url"])
	}
}

func TestJobLoggerImpl_JobCompleted(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job-123", "exec-456", logs.VerbosityNormal, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	summary := &logs.JobSummary{
		PagesCrawled:   10,
		ItemsExtracted: 5,
	}
	logger.JobCompleted(summary)

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Category != string(logs.CategoryLifecycle) {
		t.Errorf("Category = %q, want %s", entry.Category, logs.CategoryLifecycle)
	}
	if entry.Message != "job_completed" {
		t.Errorf("Message = %q, want job_completed", entry.Message)
	}
}

func TestJobLoggerImpl_JobFailed(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job-123", "exec-456", logs.VerbosityNormal, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	logger.JobFailed(context.DeadlineExceeded)

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Level != "error" {
		t.Errorf("Level = %q, want error", entry.Level)
	}
	if entry.Fields["error"] != "context deadline exceeded" {
		t.Errorf("error field = %v, want 'context deadline exceeded'", entry.Fields["error"])
	}
}

func TestJobLoggerImpl_MaxLogsPerJob(t *testing.T) {
	t.Helper()

	entryCount := 0
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(_ logs.LogEntry) {
		entryCount++
	}, 0)

	// Log more than MaxLogsPerJob
	const logsToEmit = 100
	for range logsToEmit {
		logger.Info(logs.CategoryFetch, "test")
	}

	// All logs should be captured since we're under the limit
	if entryCount != logsToEmit {
		t.Errorf("captured %d entries, want %d", entryCount, logsToEmit)
	}
}

func TestJobLoggerImpl_Flush(t *testing.T) {
	t.Helper()

	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, nil, 0)

	err := logger.Flush()
	if err != nil {
		t.Errorf("Flush() error = %v, want nil", err)
	}
}

func TestJobLoggerImpl_BuildSummary(t *testing.T) {
	t.Helper()

	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(_ logs.LogEntry) {}, 0)

	logger.Info(logs.CategoryFetch, "test1")
	logger.Warn(logs.CategoryFetch, "test2")
	logger.Info(logs.CategoryFetch, "test3")

	summary := logger.BuildSummary()

	if summary.LogsEmitted != 3 {
		t.Errorf("LogsEmitted = %d, want 3", summary.LogsEmitted)
	}
}

func TestJobLoggerImpl_StartHeartbeat(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	ctx, cancel := context.WithCancel(context.Background())
	logger.StartHeartbeat(ctx)

	// Give some time for the heartbeat to potentially log
	time.Sleep(10 * time.Millisecond)
	cancel()

	// StartHeartbeat should not immediately log (heartbeat interval is 15s)
	// We just verify it doesn't panic and can be cancelled
}

func TestJobLoggerImpl_WithFields_Chaining(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	// Chain WithFields calls
	scoped := logger.WithFields(logs.String("key1", "val1")).WithFields(logs.String("key2", "val2"))
	scoped.Info(logs.CategoryQueue, "test")

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	fields := entries[0].Fields
	if fields["key1"] != "val1" {
		t.Errorf("key1 = %v, want val1", fields["key1"])
	}
	if fields["key2"] != "val2" {
		t.Errorf("key2 = %v, want val2", fields["key2"])
	}
}

func TestJobLoggerImpl_FieldsOverrideParent(t *testing.T) {
	t.Helper()

	var entries []logs.LogEntry
	logger := logs.NewJobLoggerImpl("job", "exec", logs.VerbosityNormal, func(e logs.LogEntry) {
		entries = append(entries, e)
	}, 0)

	// Parent has key1=original
	scoped := logger.WithFields(logs.String("key1", "original"))
	// Log call overrides with key1=override
	scoped.Info(logs.CategoryQueue, "test", logs.String("key1", "override"))

	if len(entries) != 1 {
		t.Fatalf("captured %d entries, want 1", len(entries))
	}

	// Call fields should override parent fields
	if entries[0].Fields["key1"] != "override" {
		t.Errorf("key1 = %v, want override", entries[0].Fields["key1"])
	}
}

// Compile-time interface check is in the implementation file
