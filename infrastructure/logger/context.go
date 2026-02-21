package logger

import "context"

type ctxKey struct{}

// WithContext returns a new context with the given logger stored in it.
func WithContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext retrieves the logger from the context.
// Returns a no-op logger if none is found.
func FromContext(ctx context.Context) Logger {
	if l, ok := ctx.Value(ctxKey{}).(Logger); ok {
		return l
	}

	return NewNop()
}
