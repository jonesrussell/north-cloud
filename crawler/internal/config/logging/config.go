package logging

// Config holds logging-specific configuration settings.
type Config struct {
	// Level is the logging level (debug, info, warn, error)
	Level string `yaml:"level"`
	// Encoding is the log encoding format (json, console)
	Encoding string `yaml:"encoding"`
	// Output is the log output destination (stdout, stderr, file)
	Output string `yaml:"output"`
	// File is the log file path (only used when output is file)
	File string `yaml:"file"`
	// Debug enables debug mode for additional logging
	Debug bool `yaml:"debug"`
	// Caller enables caller information in logs
	Caller bool `yaml:"caller"`
	// Stacktrace enables stacktrace in error logs
	Stacktrace bool `yaml:"stacktrace"`
	// MaxSize is the maximum size of the log file in megabytes
	MaxSize int `yaml:"max_size"`
	// MaxBackups is the maximum number of old log files to retain
	MaxBackups int `yaml:"max_backups"`
	// MaxAge is the maximum number of days to retain old log files
	MaxAge int `yaml:"max_age"`
	// Compress determines if the rotated log files should be compressed
	Compress bool `yaml:"compress"`
}
