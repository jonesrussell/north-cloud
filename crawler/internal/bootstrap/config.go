// Package bootstrap handles application initialization and lifecycle management
// for the crawler service.
package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	infraconfig "github.com/north-cloud/infrastructure/config"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// === Errors ===

var (
	// errLoggerRequired is returned when CommandDeps.Logger is nil.
	errLoggerRequired = errors.New("logger is required")
	// errConfigRequired is returned when CommandDeps.Config is nil.
	errConfigRequired = errors.New("config is required")
)

// === Types ===

// CommandDeps holds common dependencies for the HTTP server.
type CommandDeps struct {
	Logger infralogger.Logger
	Config config.Interface
}

// === Config Loading ===

// NewCommandDeps creates CommandDeps by loading config and creating logger.
func NewCommandDeps() (*CommandDeps, error) {
	// Load config
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Create logger from config
	log, err := CreateLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	// Add service name to all log entries
	log = log.With(infralogger.String("service", "crawler"))

	deps := &CommandDeps{
		Logger: log,
		Config: cfg,
	}

	if validateErr := deps.Validate(); validateErr != nil {
		return nil, fmt.Errorf("validate deps: %w", validateErr)
	}

	return deps, nil
}

// LoadConfig loads configuration from the config package.
func LoadConfig() (config.Interface, error) {
	configPath := infraconfig.GetConfigPath("config.yml")
	return config.Load(configPath)
}

// CreateLogger creates a logger instance from configuration using infrastructure logger.
func CreateLogger(cfg config.Interface) (infralogger.Logger, error) {
	loggingCfg := cfg.GetLoggingConfig()

	logLevel := normalizeLogLevel(loggingCfg.Level)
	if logLevel == "" {
		logLevel = "info"
	}

	// Determine if we're in development mode
	appEnv := loggingCfg.Env
	if appEnv == "" {
		appEnv = "production"
	}
	isDev := appEnv == "development"
	appDebug := loggingCfg.Debug

	// Override log level if APP_DEBUG is set
	if appDebug {
		logLevel = "debug"
	}

	// Determine encoding based on environment
	encoding := loggingCfg.Format
	if encoding == "" {
		if isDev {
			encoding = "console"
		} else {
			encoding = "json"
		}
	}

	return infralogger.New(infralogger.Config{
		Level:       logLevel,
		Format:      encoding,
		Development: isDev || appDebug,
	})
}

// normalizeLogLevel normalizes log level string.
func normalizeLogLevel(level string) string {
	if level == "" {
		return "info"
	}
	return strings.ToLower(level)
}

// Validate ensures all required dependencies are present.
func (d *CommandDeps) Validate() error {
	if d.Logger == nil {
		return errLoggerRequired
	}
	if d.Config == nil {
		return errConfigRequired
	}
	return nil
}
