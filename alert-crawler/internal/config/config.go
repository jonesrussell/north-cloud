// Package config provides configuration management for the alert-crawler service.
//
// Precedence (highest to lowest):
//
//  1. Environment variables (env struct tags)
//  2. YAML file values
//  3. SetDefaults (code-level defaults)
//
// # RR-007 Pitfall
//
// Non-empty values in config.yml silently prevent SetDefaults from applying.
// Fields owned by SetDefaults MUST be left blank or omitted in config.yml.
// Use environment variables for per-environment overrides.
package config

import (
	"fmt"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
)

// ServiceConfig holds basic service identity configuration.
type ServiceConfig struct {
	Name string `env:"SERVICE_NAME" yaml:"name"`
}

// DatabaseConfig holds SQLite catalogue database configuration.
type DatabaseConfig struct {
	Path           string `env:"ALERT_DB_PATH"            yaml:"path"`
	MigrationsPath string `env:"ALERT_DB_MIGRATIONS_PATH" yaml:"migrations_path"`
}

// ESConfig holds Elasticsearch index configuration for alert-crawler.
type ESConfig struct {
	URL   string `env:"ELASTICSEARCH_URL" yaml:"url"`
	Index string `env:"ALERT_ES_INDEX"    yaml:"index"`
}

// RedisConfig holds Redis pub/sub configuration for alert-crawler.
type RedisConfig struct {
	URL     string `env:"REDIS_URL"           yaml:"url"`
	Channel string `env:"ALERT_REDIS_CHANNEL" yaml:"channel"`
}

// SeverityConfig maps hazard keyword strings to domain severity levels.
// The Table field drives the keyword-scoring step in the severity package.
type SeverityConfig struct {
	Table map[string]domain.Severity `yaml:"table"`
}

// ObservabilityConfig holds structured logging configuration.
type ObservabilityConfig struct {
	LogLevel  string `env:"LOG_LEVEL"  yaml:"log_level"`
	LogFormat string `env:"LOG_FORMAT" yaml:"log_format"`
}

// Config is the top-level configuration for alert-crawler.
//
// Load via [Load], which calls infrastructure/config.LoadWithDefaults and
// enforces the env > YAML > SetDefaults precedence chain.
type Config struct {
	Service       ServiceConfig        `yaml:"service"`
	Sources       []domain.AlertSource `yaml:"sources"`
	Database      DatabaseConfig       `yaml:"database"`
	Elasticsearch ESConfig             `yaml:"elasticsearch"`
	Redis         RedisConfig          `yaml:"redis"`
	Severity      SeverityConfig       `yaml:"severity"`
	Observability ObservabilityConfig  `yaml:"observability"`
}

// Load reads config from path, applies code defaults via SetDefaults, then
// re-applies environment variable overrides so that env always wins.
//
// Precedence: env vars > YAML file > SetDefaults.
// See the package-level doc and RR-007 in alert-crawler/CLAUDE.md for the
// config.yml pitfall: fields that have non-empty YAML values prevent
// SetDefaults from running — leave SetDefaults-owned fields blank in config.yml.
func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults[Config](path, SetDefaults)
	if err != nil {
		return nil, fmt.Errorf("load alert-crawler config: %w", err)
	}

	return cfg, nil
}
