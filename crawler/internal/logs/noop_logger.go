package logs

import "context"

// noopJobLogger is a no-op implementation of JobLogger.
type noopJobLogger struct{}

// NoopJobLogger returns a JobLogger that does nothing.
// Useful as a fallback when no logger is configured.
func NoopJobLogger() JobLogger {
	return &noopJobLogger{}
}

func (n *noopJobLogger) Info(_ Category, _ string, _ ...Field)  {}
func (n *noopJobLogger) Warn(_ Category, _ string, _ ...Field)  {}
func (n *noopJobLogger) Error(_ Category, _ string, _ ...Field) {}
func (n *noopJobLogger) Debug(_ Category, _ string, _ ...Field) {}
func (n *noopJobLogger) JobStarted(_, _ string)                 {}
func (n *noopJobLogger) JobCompleted(_ *JobSummary)             {}
func (n *noopJobLogger) JobFailed(_ error)                      {}
func (n *noopJobLogger) StartHeartbeat(_ context.Context)       {}
func (n *noopJobLogger) IsDebugEnabled() bool                   { return false }
func (n *noopJobLogger) IsTraceEnabled() bool                   { return false }
func (n *noopJobLogger) WithFields(_ ...Field) JobLogger        { return n }
func (n *noopJobLogger) BuildSummary() *JobSummary              { return &JobSummary{} }
func (n *noopJobLogger) Flush() error                           { return nil }

// Ensure noopJobLogger implements JobLogger
var _ JobLogger = (*noopJobLogger)(nil)
