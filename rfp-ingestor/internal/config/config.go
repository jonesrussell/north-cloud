package config

import (
	"fmt"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServiceName    = "rfp-ingestor"
	defaultServiceVersion = "0.1.0"
	defaultServicePort    = 8095
	defaultLogLevel       = "info"
	defaultLogFormat      = "json"
)

// Default Elasticsearch configuration values.
const (
	defaultESURL      = "http://localhost:9200"
	defaultESIndex    = "rfp_classified_content"
	defaultESBulkSize = 500
)

// Default ingestion configuration values.
const (
	defaultPollIntervalMinutes = 120
	defaultReconcileCron       = "0 9 * * *"
)

// Default CanadaBuys feed URLs.
const (
	defaultNewFeedURL     = "https://canadabuys.canada.ca/opendata/pub/newTenderNotice-nouvelAvisAppelOffres.csv"
	defaultOpenFeedURL    = "https://canadabuys.canada.ca/opendata/pub/openTenderNotice-ouvertAvisAppelOffres.csv"
	defaultArchiveFeedURL = "https://canadabuys.canada.ca/opendata/pub/tenderNoticeComplete-avisAppelOffresComplet.csv"
)

// Config holds all configuration for the rfp-ingestor service.
type Config struct {
	Service       ServiceConfig       `yaml:"service"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Feeds         FeedsConfig         `yaml:"feeds"`
	Ingestion     IngestionConfig     `yaml:"ingestion"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ServiceConfig holds service identity and runtime settings.
type ServiceConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Port    int    `env:"RFP_INGESTOR_PORT" yaml:"port"`
	Debug   bool   `env:"APP_DEBUG"         yaml:"debug"`
}

// ElasticsearchConfig holds Elasticsearch connection and indexing settings.
type ElasticsearchConfig struct {
	URL      string `env:"ELASTICSEARCH_URL" yaml:"url"`
	Index    string `env:"ES_RFP_INDEX"      yaml:"index"`
	BulkSize int    `yaml:"bulk_size"`
}

// FeedsConfig holds the CanadaBuys CSV feed URLs.
type FeedsConfig struct {
	NewURL     string `env:"CANADABUYS_NEW_URL"     yaml:"new_url"`
	OpenURL    string `env:"CANADABUYS_OPEN_URL"    yaml:"open_url"`
	ArchiveURL string `env:"CANADABUYS_ARCHIVE_URL" yaml:"archive_url"`
}

// IngestionConfig holds scheduling and reconciliation settings.
type IngestionConfig struct {
	PollIntervalMinutes int    `env:"RFP_POLL_INTERVAL_MINUTES" yaml:"poll_interval_minutes"`
	ReconcileCron       string `env:"RFP_RECONCILE_CRON"        yaml:"reconcile_cron"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// Load loads configuration from a YAML file, applies defaults, then env overrides.
func Load(path string) (*Config, error) {
	cfg, loadErr := infraconfig.LoadWithDefaults(path, setDefaults)
	if loadErr != nil {
		return nil, fmt.Errorf("load config: %w", loadErr)
	}

	if validateErr := cfg.Validate(); validateErr != nil {
		return nil, fmt.Errorf("validate config: %w", validateErr)
	}

	return cfg, nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	if err := infraconfig.ValidatePort("service.port", c.Service.Port); err != nil {
		return err
	}

	if c.Elasticsearch.URL == "" {
		return &infraconfig.ValidationError{Field: "elasticsearch.url", Message: "is required"}
	}

	if c.Elasticsearch.Index == "" {
		return &infraconfig.ValidationError{Field: "elasticsearch.index", Message: "is required"}
	}

	return nil
}

// setDefaults applies default values to all configuration sections.
func setDefaults(cfg *Config) {
	setServiceDefaults(&cfg.Service)
	setElasticsearchDefaults(&cfg.Elasticsearch)
	setFeedsDefaults(&cfg.Feeds)
	setIngestionDefaults(&cfg.Ingestion)
	setLoggingDefaults(&cfg.Logging)
}

func setServiceDefaults(s *ServiceConfig) {
	if s.Name == "" {
		s.Name = defaultServiceName
	}

	if s.Version == "" {
		s.Version = defaultServiceVersion
	}

	if s.Port == 0 {
		s.Port = defaultServicePort
	}
}

func setElasticsearchDefaults(e *ElasticsearchConfig) {
	if e.URL == "" {
		e.URL = defaultESURL
	}

	if e.Index == "" {
		e.Index = defaultESIndex
	}

	if e.BulkSize == 0 {
		e.BulkSize = defaultESBulkSize
	}
}

func setFeedsDefaults(f *FeedsConfig) {
	if f.NewURL == "" {
		f.NewURL = defaultNewFeedURL
	}

	if f.OpenURL == "" {
		f.OpenURL = defaultOpenFeedURL
	}

	if f.ArchiveURL == "" {
		f.ArchiveURL = defaultArchiveFeedURL
	}
}

func setIngestionDefaults(i *IngestionConfig) {
	if i.PollIntervalMinutes == 0 {
		i.PollIntervalMinutes = defaultPollIntervalMinutes
	}

	if i.ReconcileCron == "" {
		i.ReconcileCron = defaultReconcileCron
	}
}

func setLoggingDefaults(l *LoggingConfig) {
	if l.Level == "" {
		l.Level = defaultLogLevel
	}

	if l.Format == "" {
		l.Format = defaultLogFormat
	}
}
