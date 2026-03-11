package logger

// NoOpLogger is a logger that does nothing.
// Use this for testing or when logging should be disabled.
type NoOpLogger struct{}

// NewNop creates a new no-op logger instance.
func NewNop() Logger {
	return &NoOpLogger{}
}

// Debug does nothing.
func (l *NoOpLogger) Debug(msg string, fields ...Field) {}

// Info does nothing.
func (l *NoOpLogger) Info(msg string, fields ...Field) {}

// Warn does nothing.
func (l *NoOpLogger) Warn(msg string, fields ...Field) {}

// Error does nothing.
func (l *NoOpLogger) Error(msg string, fields ...Field) {}

// Fatal does nothing (does not exit in no-op mode).
func (l *NoOpLogger) Fatal(msg string, fields ...Field) {}

// With returns the same no-op logger.
func (l *NoOpLogger) With(fields ...Field) Logger {
	return l
}

// Sync does nothing and returns nil.
func (l *NoOpLogger) Sync() error {
	return nil
}
