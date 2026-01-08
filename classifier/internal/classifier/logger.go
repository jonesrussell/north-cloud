package classifier

// Logger defines the logging interface used by classifiers
// This allows for flexible logging implementations (zap, logrus, etc.)
type Logger interface {
	Debug(msg string, keysAndValues ...any)
	Info(msg string, keysAndValues ...any)
	Warn(msg string, keysAndValues ...any)
	Error(msg string, keysAndValues ...any)
}
