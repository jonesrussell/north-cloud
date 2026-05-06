package config

import (
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// defaultServiceName is the canonical service identity used in logs and metrics.
const defaultServiceName = "alert-crawler"

// defaultDBPath is the SQLite catalogue path inside the container.
const defaultDBPath = "/app/data/state.db"

// defaultESURL is the Elasticsearch base URL in the Docker network.
const defaultESURL = "http://elasticsearch:9200"

// defaultESIndex is the Elasticsearch index for community alerts.
const defaultESIndex = "community_alerts"

// defaultRedisURL is the Redis connection URL in the Docker network.
const defaultRedisURL = "redis://redis:6379"

// defaultRedisChannel is the Redis pub/sub channel for alert lifecycle events.
const defaultRedisChannel = "community_alerts:lifecycle"

// defaultLogLevel is the structured log verbosity.
const defaultLogLevel = "info"

// defaultLogFormat is the structured log format.
const defaultLogFormat = "json"

// defaultPollInterval is the minimum poll interval per FR-001 (30m ≤ interval ≤ 60m).
const defaultPollInterval = 30 * time.Minute

// defaultExpiry is the default alert TTL per TC-009 (30 days).
const defaultExpiry = 720 * time.Hour

// SetDefaults fills in sensible defaults for every field that config.yml
// intentionally leaves blank.
//
// RR-007: any field listed here MUST be omitted or blank in config.yml.
// A non-empty YAML value takes precedence over SetDefaults, so the code
// default would never be reached.
func SetDefaults(c *Config) {
	applyServiceDefaults(c)
	applyDatabaseDefaults(c)
	applyESDefaults(c)
	applyRedisDefaults(c)
	applyObservabilityDefaults(c)
	applySourcesDefaults(c)
	applySeverityDefaults(c)
}

func applyServiceDefaults(c *Config) {
	if c.Service.Name == "" {
		c.Service.Name = defaultServiceName
	}
}

func applyDatabaseDefaults(c *Config) {
	if c.Database.Path == "" {
		c.Database.Path = defaultDBPath
	}
}

func applyESDefaults(c *Config) {
	if c.Elasticsearch.URL == "" {
		c.Elasticsearch.URL = defaultESURL
	}

	if c.Elasticsearch.Index == "" {
		c.Elasticsearch.Index = defaultESIndex
	}
}

func applyRedisDefaults(c *Config) {
	if c.Redis.URL == "" {
		c.Redis.URL = defaultRedisURL
	}

	if c.Redis.Channel == "" {
		c.Redis.Channel = defaultRedisChannel
	}
}

func applyObservabilityDefaults(c *Config) {
	if c.Observability.LogLevel == "" {
		c.Observability.LogLevel = defaultLogLevel
	}

	if c.Observability.LogFormat == "" {
		c.Observability.LogFormat = defaultLogFormat
	}
}

func applySourcesDefaults(c *Config) {
	if len(c.Sources) == 0 {
		c.Sources = defaultSources()
		return
	}

	for i := range c.Sources {
		applySourceDefaults(&c.Sources[i])
	}
}

func applySourceDefaults(s *domain.AlertSource) {
	if s.PollInterval == 0 {
		s.PollInterval = defaultPollInterval
	}

	if s.DefaultExpiry == 0 {
		s.DefaultExpiry = defaultExpiry
	}

	if s.DefaultCategory == "" {
		s.DefaultCategory = domain.CategoryHarmReduction
	}
}

// defaultSources returns the built-in source list used when config.yml
// carries no sources. The MHRN feed covers Treaty 1 / Manitoba scope.
func defaultSources() []domain.AlertSource {
	return []domain.AlertSource{
		{
			ID:                  "mhrn",
			Name:                "Manitoba Harm Reduction Network",
			FeedURL:             "https://www.safersites.ca/drugalerts.rss",
			AcquisitionStrategy: domain.AcquisitionRSS,
			PollInterval:        defaultPollInterval,
			DefaultCategory:     domain.CategoryHarmReduction,
			DefaultScope:        []string{"treaty:1", "canada:manitoba"},
			DefaultExpiry:       defaultExpiry,
			Enabled:             true,
		},
	}
}

// defaultSeverityTable returns the baseline hazard-keyword → severity mapping.
// Keywords are lowercased for case-insensitive matching in the severity package.
func defaultSeverityTable() map[string]domain.Severity {
	return map[string]domain.Severity{
		"carfentanil":   domain.SeverityCritical,
		"nitazenes":     domain.SeverityHigh,
		"medetomidine":  domain.SeverityHigh,
		"xylazine":      domain.SeverityHigh,
		"fentanyl":      domain.SeverityHigh,
		"benzodiazepine": domain.SeverityHigh,
	}
}

func applySeverityDefaults(c *Config) {
	if len(c.Severity.Table) == 0 {
		c.Severity.Table = defaultSeverityTable()
	}
}
