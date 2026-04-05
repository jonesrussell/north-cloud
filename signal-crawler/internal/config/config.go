// Package config provides configuration management for the signal-crawler service.
// It handles loading, validation, and defaults for configuration values from both
// YAML files and environment variables using infrastructure/config.
package config

import (
	"errors"
	"fmt"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
)

// defaultGCJobsURL is the default GC Jobs search URL for cloud/devops/infrastructure roles.
const defaultGCJobsURL = "https://emploisfp-psjobs.cfp-psc.gc.ca/psrs-srfp/applicant/page2440" +
	"?selectionProcessNumber=&officialLanguage=E" +
	"&title=cloud+OR+devops+OR+infrastructure+OR+platform"

// NorthOpsConfig holds connection configuration for the NorthOps ingest API.
type NorthOpsConfig struct {
	URL    string `env:"NORTHOPS_URL"     yaml:"url"`
	APIKey string `env:"PIPELINE_API_KEY" yaml:"api_key"`
}

// DedupConfig holds deduplication store configuration.
type DedupConfig struct {
	DBPath string `env:"SIGNAL_DB_PATH" yaml:"db_path"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// HNConfig holds Hacker News adapter configuration.
type HNConfig struct {
	MaxItems int    `env:"HN_MAX_ITEMS" yaml:"max_items"`
	BaseURL  string `env:"HN_BASE_URL"  yaml:"base_url"`
}

// FundingConfig holds funding adapter configuration.
type FundingConfig struct {
	URLs []string `yaml:"urls"`
}

// RendererConfig holds Playwright renderer configuration.
type RendererConfig struct {
	URL     string `env:"RENDERER_URL"     yaml:"url"`
	Enabled bool   `env:"RENDERER_ENABLED" yaml:"enabled"`
}

// JobsConfig holds job board adapter configuration.
type JobsConfig struct {
	RemoteOKURL   string `env:"JOBS_REMOTEOK_URL"    yaml:"remoteok_url"`
	WWRURL        string `env:"JOBS_WWR_URL"          yaml:"wwr_url"`
	HNMaxComments int    `env:"JOBS_HN_MAX_COMMENTS"  yaml:"hn_max_comments"`
	GCJobsURL     string `env:"JOBS_GCJOBS_URL"       yaml:"gcjobs_url"`
	WorkBCURL     string `env:"JOBS_WORKBC_URL"        yaml:"workbc_url"`
}

// Config is the top-level configuration for signal-crawler.
type Config struct {
	NorthOps NorthOpsConfig `yaml:"northops"`
	Dedup    DedupConfig    `yaml:"dedup"`
	Logging  LoggingConfig  `yaml:"logging"`
	HN       HNConfig       `yaml:"hn"`
	Funding  FundingConfig  `yaml:"funding"`
	Renderer RendererConfig `yaml:"renderer"`
	Jobs     JobsConfig     `yaml:"jobs"`
}

// Validate checks that all required fields are present.
func (c *Config) Validate() error {
	if c.NorthOps.URL == "" {
		return errors.New("northops_url is required")
	}
	if c.NorthOps.APIKey == "" {
		return errors.New("api_key is required")
	}
	if c.Dedup.DBPath == "" {
		return errors.New("db_path is required")
	}
	return nil
}

// SetDefaults fills in default values for optional fields.
func SetDefaults(cfg *Config) {
	if cfg.Dedup.DBPath == "" {
		cfg.Dedup.DBPath = "data/seen.db"
	}
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}
	if cfg.HN.MaxItems == 0 {
		cfg.HN.MaxItems = 200
	}
	if cfg.HN.BaseURL == "" {
		cfg.HN.BaseURL = "https://hacker-news.firebaseio.com"
	}
	if len(cfg.Funding.URLs) == 0 {
		cfg.Funding.URLs = []string{"https://otf.ca/funded-grants"}
	}
	if cfg.Renderer.URL == "" {
		cfg.Renderer.URL = "http://localhost:8095"
	}
	if cfg.Jobs.RemoteOKURL == "" {
		cfg.Jobs.RemoteOKURL = "https://remoteok.com/api"
	}
	if cfg.Jobs.WWRURL == "" {
		cfg.Jobs.WWRURL = "https://weworkremotely.com/categories/remote-devops-sysadmin-jobs"
	}
	if cfg.Jobs.HNMaxComments == 0 {
		cfg.Jobs.HNMaxComments = 200
	}
	if cfg.Jobs.GCJobsURL == "" {
		cfg.Jobs.GCJobsURL = defaultGCJobsURL
	}
	if cfg.Jobs.WorkBCURL == "" {
		cfg.Jobs.WorkBCURL = "https://www.workbc.ca/find-jobs/browse-jobs?searchTerm=cloud+engineer+OR+devops+OR+platform"
	}
}

// Load reads config from path, applies defaults, then applies env overrides.
func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults[Config](path, SetDefaults)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
