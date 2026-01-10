// Package logger provides a unified structured logging interface for all North Cloud services.
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

// Config represents the logger configuration.
type Config struct {
	// Level is the minimum logging level (debug, info, warn, error, fatal).
	Level string `env:"LOG_LEVEL" yaml:"level"`
	// Format is the output format. Always uses "json" for consistency.
	// The "console" format is deprecated and will be ignored.
	Format string `env:"LOG_FORMAT" yaml:"format"`
	// Development enables development mode with prettier output.
	Development bool `yaml:"development"`
	// OutputPaths is a list of URLs or file paths to write logging output to.
	OutputPaths []string `yaml:"output_paths"`
}

// Default configuration values.
const (
	// DefaultLevel is the default logging level.
	DefaultLevel = "info"
	// DefaultFormat is the default log format.
	DefaultFormat = "json"
)

// Default output paths.
var (
	// DefaultOutputPaths is the default list of paths to write log output to.
	DefaultOutputPaths = []string{"stdout"}
)

// SetDefaults applies default values to the config if not set.
func (c *Config) SetDefaults() {
	if c.Level == "" {
		c.Level = DefaultLevel
	}
	if c.Format == "" {
		c.Format = DefaultFormat
	}
	if len(c.OutputPaths) == 0 {
		c.OutputPaths = DefaultOutputPaths
	}
}
