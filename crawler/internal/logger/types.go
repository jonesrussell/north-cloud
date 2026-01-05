// Package logger provides logging functionality for the application.
package logger

// Level represents the logging level.
type Level string

const (
	// DebugLevel logs debug messages.
	DebugLevel Level = "debug"
	// InfoLevel logs info messages.
	InfoLevel Level = "info"
	// WarnLevel logs warning messages.
	WarnLevel Level = "warn"
	// ErrorLevel logs error messages.
	ErrorLevel Level = "error"
	// FatalLevel logs fatal messages and exits.
	FatalLevel Level = "fatal"
)

// ANSI color codes for log levels
const (
	ColorDebug = "\033[36m" // Cyan
	ColorInfo  = "\033[32m" // Green
	ColorWarn  = "\033[33m" // Yellow
	ColorError = "\033[31m" // Red
	ColorFatal = "\033[35m" // Magenta
	ColorReset = "\033[0m"  // Reset
)

// Config represents the logger configuration.
type Config struct {
	// Level is the minimum logging level.
	Level Level `json:"level" yaml:"level"`
	// Development enables development mode.
	Development bool `json:"development" yaml:"development"`
	// Encoding sets the logger's encoding.
	Encoding string `json:"encoding" yaml:"encoding"`
	// OutputPaths is a list of URLs or file paths to write logging output to.
	OutputPaths []string `json:"outputPaths" yaml:"outputPaths"`
	// ErrorOutputPaths is a list of URLs to write internal logger errors to.
	ErrorOutputPaths []string `json:"errorOutputPaths" yaml:"errorOutputPaths"`
	// EnableColor enables colored output in development mode.
	EnableColor bool `json:"enableColor" yaml:"enableColor"`
}
