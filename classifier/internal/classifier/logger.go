package classifier

// Logger defines the logging interface used by classifiers
// This allows for flexible logging implementations (zap, logrus, etc.)
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}
