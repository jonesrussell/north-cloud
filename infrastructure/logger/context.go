package logger

import (
	"context"
	"sync"
)

type ctxKey struct{}

// WithContext returns a new context with the given logger stored in it.
func WithContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the logger from the context.
// Returns a stderr-backed fallback logger if none is found, ensuring errors
// are never silently discarded. Callers in non-HTTP contexts (background
// goroutines, startup code) should pass a logger explicitly rather than
// relying on context.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}

	return fallbackLogger()
}

var (
	fallbackLog  Logger
	fallbackOnce sync.Once
)

// fallbackLogger returns a shared warn-level logger that writes to stderr.
// It is used when no logger is found in the context, ensuring log output
// is never silently discarded.
func fallbackLogger() Logger {
	fallbackOnce.Do(func() {
		l, err := New(Config{
			Level:       "warn",
			OutputPaths: []string{"stderr"},
		})
		if err != nil {
			// Last resort: if we can't create a real logger, use no-op
			// to avoid panicking. This should never happen in practice.
			l = NewNop()
		}
		fallbackLog = l
	})

	return fallbackLog
}
