package config

import (
	"errors"
	"fmt"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Common validation errors.
var (
	ErrRequired     = errors.New("field is required")
	ErrInvalidPort  = errors.New("invalid port number")
	ErrInvalidURL   = errors.New("invalid URL")
	ErrInvalidLevel = errors.New("invalid log level")
)

// ValidateRequired checks if a string field is not empty.
func ValidateRequired(field, value string) error {
	if value == "" {
		return &ValidationError{Field: field, Message: "is required"}
	}
	return nil
}

// ValidatePort checks if a port number is valid.
func ValidatePort(field string, port int) error {
	if port < 1 || port > 65535 {
		return &ValidationError{Field: field, Message: "must be between 1 and 65535"}
	}
	return nil
}

// ValidateLogLevel checks if a log level is valid.
func ValidateLogLevel(level string) error {
	switch level {
	case "debug", "info", "warn", "warning", "error", "fatal":
		return nil
	default:
		return &ValidationError{Field: "level", Message: "must be one of: debug, info, warn, error, fatal"}
	}
}

// ValidateLogFormat checks if a log format is valid.
func ValidateLogFormat(format string) error {
	switch format {
	case "json", "console":
		return nil
	default:
		return &ValidationError{Field: "format", Message: "must be one of: json, console"}
	}
}

// Validator is an interface for types that can validate themselves.
type Validator interface {
	Validate() error
}

// Validate calls the Validate method on cfg if it implements Validator.
func Validate(cfg any) error {
	if v, ok := cfg.(Validator); ok {
		return v.Validate()
	}
	return nil
}

// ValidateServerConfig validates a ServerConfig.
func (c *ServerConfig) Validate() error {
	if c.Port != 0 {
		if err := ValidatePort("server.port", c.Port); err != nil {
			return err
		}
	}
	return nil
}

// ValidateDatabaseConfig validates a DatabaseConfig.
func (c *DatabaseConfig) Validate() error {
	if c.Host == "" {
		return &ValidationError{Field: "database.host", Message: "is required"}
	}
	if err := ValidatePort("database.port", c.Port); err != nil {
		return err
	}
	if c.User == "" {
		return &ValidationError{Field: "database.user", Message: "is required"}
	}
	if c.Database == "" {
		return &ValidationError{Field: "database.database", Message: "is required"}
	}
	return nil
}

// ValidateElasticsearchConfig validates an ElasticsearchConfig.
func (c *ElasticsearchConfig) Validate() error {
	if c.URL == "" {
		return &ValidationError{Field: "elasticsearch.url", Message: "is required"}
	}
	return nil
}

// ValidateLoggingConfig validates a LoggingConfig.
func (c *LoggingConfig) Validate() error {
	if c.Level != "" {
		if err := ValidateLogLevel(c.Level); err != nil {
			return err
		}
	}
	if c.Format != "" {
		if err := ValidateLogFormat(c.Format); err != nil {
			return err
		}
	}
	return nil
}
