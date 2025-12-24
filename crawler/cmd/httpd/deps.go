package httpd

import (
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/spf13/viper"
)

// newCommandDeps creates CommandDeps by loading config and creating logger.
func newCommandDeps() (*CommandDeps, error) {
	// Initialize config first
	if err := initConfig(); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	// Load config
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Create logger
	log, err := createLogger()
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	deps := &CommandDeps{
		Logger: log,
		Config: cfg,
	}

	if validateErr := deps.validate(); validateErr != nil {
		return nil, fmt.Errorf("validate deps: %w", validateErr)
	}

	return deps, nil
}

// loadConfig loads configuration from the config package.
func loadConfig() (config.Interface, error) {
	return config.LoadConfig()
}

// createLogger creates a logger instance from Viper configuration.
func createLogger() (logger.Interface, error) {
	logLevel := normalizeLogLevel(viper.GetString("logger.level"))
	logCfg := &logger.Config{
		Level:       logger.Level(logLevel),
		Development: viper.GetBool("logger.development"),
		Encoding:    viper.GetString("logger.encoding"),
		OutputPaths: viper.GetStringSlice("logger.output_paths"),
		EnableColor: viper.GetBool("logger.enable_color"),
	}
	return logger.New(logCfg)
}

// normalizeLogLevel normalizes log level string.
func normalizeLogLevel(level string) string {
	if level == "" {
		return "info"
	}
	return strings.ToLower(level)
}

// validate ensures all required dependencies are present.
func (d *CommandDeps) validate() error {
	if d.Logger == nil {
		return errLoggerRequired
	}
	if d.Config == nil {
		return errConfigRequired
	}
	return nil
}
