package logger_test

import (
	"context"
	"testing"

	"github.com/north-cloud/infrastructure/logger"
)

func TestWithContext_FromContext_RoundTrip(t *testing.T) {
	t.Parallel()

	nop := logger.NewNop()
	ctx := logger.WithContext(context.Background(), nop)
	got := logger.FromContext(ctx)

	if got != nop {
		t.Errorf("FromContext returned %v, want the same logger instance %v", got, nop)
	}
}

func TestFromContext_NoLogger_ReturnsFallback(t *testing.T) {
	t.Parallel()

	got := logger.FromContext(context.Background())
	if got == nil {
		t.Fatal("FromContext on empty context returned nil, want non-nil fallback logger")
	}
}

func TestFromContext_FallbackIsUsable(t *testing.T) {
	t.Parallel()

	fallback := logger.FromContext(context.Background())

	// These calls must not panic. The fallback logger is warn-level,
	// so Debug/Info will be filtered, but the calls must still succeed.
	fallback.Debug("debug message")
	fallback.Info("info message")
	fallback.Warn("warn message")
	fallback.Error("error message")
	fallback.Warn("message with field", logger.String("key", "value"))
}

func TestWithContext_OverwritesPrevious(t *testing.T) {
	t.Parallel()

	// Use real loggers so each allocation has a distinct pointer
	// (NewNop returns *NoOpLogger{} which is a zero-size struct;
	// Go may intern those to the same address).
	first := mustTestLogger(t)
	second := mustTestLogger(t)

	ctx := logger.WithContext(context.Background(), first)
	ctx = logger.WithContext(ctx, second)

	got := logger.FromContext(ctx)
	if got != second {
		t.Error("FromContext returned the first logger, want the second (overwritten) logger")
	}
}

func TestFromContext_WithFieldsPreserved(t *testing.T) {
	t.Parallel()

	// Use a real logger because NoOpLogger.With() returns the same pointer,
	// making identity checks meaningless for that type.
	base := mustTestLogger(t)
	enriched := base.With(logger.String("service", "test-svc"), logger.String("request_id", "abc-123"))

	ctx := logger.WithContext(context.Background(), enriched)
	got := logger.FromContext(ctx)

	if got != enriched {
		t.Error("FromContext did not return the enriched logger with fields preserved")
	}

	// Verify the enriched logger is distinct from the base.
	if got == base {
		t.Error("enriched logger is the same pointer as base, With() should create a new instance")
	}

	// Verify the enriched logger is usable and does not panic when logging.
	got.Info("should carry service and request_id fields")
}

func TestFromContext_FallbackConsistency(t *testing.T) {
	t.Parallel()

	// Multiple calls to FromContext on empty contexts must return
	// the same fallback instance (singleton via sync.Once).
	a := logger.FromContext(context.Background())
	b := logger.FromContext(context.Background())

	requireLogger(t, a)
	requireLogger(t, b)

	if a != b {
		t.Error("FromContext returned different fallback instances, want the same singleton")
	}
}

// mustTestLogger creates a real logger for testing, failing the test on error.
func mustTestLogger(t *testing.T) logger.Logger {
	t.Helper()

	l, err := logger.New(logger.Config{
		Level:       "warn",
		OutputPaths: []string{"stderr"},
	})
	if err != nil {
		t.Fatalf("failed to create test logger: %v", err)
	}

	return l
}

// requireLogger is a test helper that fails the test if the logger is nil.
func requireLogger(t *testing.T, l logger.Logger) {
	t.Helper()

	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}
