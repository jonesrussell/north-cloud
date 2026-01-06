package config

import (
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
)

// Config holds all configuration for the classifier service.
type Config struct {
	Service        ServiceConfig        `yaml:"service"`
	Database       DatabaseConfig       `yaml:"database"`
	Elasticsearch  ElasticsearchConfig  `yaml:"elasticsearch"`
	Redis          RedisConfig          `yaml:"redis"`
	Logging        LoggingConfig        `yaml:"logging"`
	Classification ClassificationConfig `yaml:"classification"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name              string        `yaml:"name"`
	Version           string        `yaml:"version"`
	Port              int           `yaml:"port" env:"CLASSIFIER_PORT"`
	Debug             bool          `yaml:"debug" env:"APP_DEBUG"`
	Enabled           bool          `yaml:"enabled"`
	Concurrency       int           `yaml:"concurrency" env:"CLASSIFIER_CONCURRENCY"`
	BatchSize         int           `yaml:"batch_size"`
	PollInterval      time.Duration `yaml:"poll_interval"`
	MinQualityScore   int           `yaml:"min_quality_score"`
	MinWordCount      int           `yaml:"min_word_count"`
	MinArticleWordCnt int           `yaml:"min_article_word_count"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host            string        `yaml:"host" env:"POSTGRES_HOST"`
	Port            int           `yaml:"port" env:"POSTGRES_PORT"`
	User            string        `yaml:"user" env:"POSTGRES_USER"`
	Password        string        `yaml:"password" env:"POSTGRES_PASSWORD"`
	Database        string        `yaml:"database" env:"POSTGRES_DB"`
	SSLMode         string        `yaml:"sslmode" env:"POSTGRES_SSLMODE"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdleConns    int           `yaml:"max_idle_connections"`
	ConnMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	URL                     string        `yaml:"url" env:"ELASTICSEARCH_URL"`
	Username                string        `yaml:"username"`
	Password                string        `yaml:"password"`
	MaxRetries              int           `yaml:"max_retries"`
	Timeout                 time.Duration `yaml:"timeout"`
	RawContentSuffix        string        `yaml:"raw_content_suffix"`
	ClassifiedContentSuffix string        `yaml:"classified_content_suffix"`
}

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	URL                    string        `yaml:"url" env:"REDIS_URL"`
	Password               string        `yaml:"password"`
	Database               int           `yaml:"database"`
	MaxRetries             int           `yaml:"max_retries"`
	Timeout                time.Duration `yaml:"timeout"`
	ChannelNewContent      string        `yaml:"channel_new_content"`
	ChannelClassified      string        `yaml:"channel_classified"`
	ClassificationCacheTTL time.Duration `yaml:"classification_cache_ttl"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
	Output string `yaml:"output"`
}

// ClassificationConfig holds classification settings.
type ClassificationConfig struct {
	ContentType      ContentTypeConfig      `yaml:"content_type"`
	Quality          QualityConfig          `yaml:"quality"`
	Topic            TopicConfig            `yaml:"topic"`
	SourceReputation SourceReputationConfig `yaml:"source_reputation"`
}

// ContentTypeConfig holds content type detection settings.
type ContentTypeConfig struct {
	Enabled             bool    `yaml:"enabled"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
}

// QualityConfig holds quality scoring settings.
type QualityConfig struct {
	Enabled           bool    `yaml:"enabled"`
	WordCountWeight   float64 `yaml:"word_count_weight"`
	MetadataWeight    float64 `yaml:"metadata_weight"`
	RichnessWeight    float64 `yaml:"richness_weight"`
	ReadabilityWeight float64 `yaml:"readability_weight"`
}

// TopicConfig holds topic classification settings.
type TopicConfig struct {
	Enabled             bool    `yaml:"enabled"`
	ConfidenceThreshold float64 `yaml:"confidence_threshold"`
	MaxTopics           int     `yaml:"max_topics"`
}

// SourceReputationConfig holds source reputation settings.
type SourceReputationConfig struct {
	Enabled                    bool `yaml:"enabled"`
	DefaultScore               int  `yaml:"default_score"`
	UpdateOnEachClassification bool `yaml:"update_on_each_classification"`
}

// Load loads configuration from the specified path.
func Load(path string) (*Config, error) {
	return infraconfig.LoadWithDefaults[Config](path, setDefaults)
}

// setDefaults applies default values to the config.
func setDefaults(cfg *Config) {
	// Service defaults
	if cfg.Service.Name == "" {
		cfg.Service.Name = "classifier"
	}
	if cfg.Service.Version == "" {
		cfg.Service.Version = "1.0.0"
	}
	if cfg.Service.Port == 0 {
		cfg.Service.Port = 8070
	}
	if cfg.Service.Concurrency == 0 {
		cfg.Service.Concurrency = 10
	}
	if cfg.Service.BatchSize == 0 {
		cfg.Service.BatchSize = 100
	}
	if cfg.Service.PollInterval == 0 {
		cfg.Service.PollInterval = 30 * time.Second
	}
	if cfg.Service.MinQualityScore == 0 {
		cfg.Service.MinQualityScore = 50
	}
	if cfg.Service.MinWordCount == 0 {
		cfg.Service.MinWordCount = 100
	}

	// Database defaults
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "postgres"
	}
	if cfg.Database.Database == "" {
		cfg.Database.Database = "classifier"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Database.MaxConnections == 0 {
		cfg.Database.MaxConnections = 25
	}
	if cfg.Database.MaxIdleConns == 0 {
		cfg.Database.MaxIdleConns = 5
	}
	if cfg.Database.ConnMaxLifetime == 0 {
		cfg.Database.ConnMaxLifetime = time.Hour
	}

	// Elasticsearch defaults
	if cfg.Elasticsearch.URL == "" {
		cfg.Elasticsearch.URL = "http://localhost:9200"
	}
	if cfg.Elasticsearch.MaxRetries == 0 {
		cfg.Elasticsearch.MaxRetries = 3
	}
	if cfg.Elasticsearch.Timeout == 0 {
		cfg.Elasticsearch.Timeout = 30 * time.Second
	}
	if cfg.Elasticsearch.RawContentSuffix == "" {
		cfg.Elasticsearch.RawContentSuffix = "_raw_content"
	}
	if cfg.Elasticsearch.ClassifiedContentSuffix == "" {
		cfg.Elasticsearch.ClassifiedContentSuffix = "_classified_content"
	}

	// Redis defaults
	if cfg.Redis.URL == "" {
		cfg.Redis.URL = "localhost:6379"
	}
	if cfg.Redis.MaxRetries == 0 {
		cfg.Redis.MaxRetries = 3
	}
	if cfg.Redis.Timeout == 0 {
		cfg.Redis.Timeout = 5 * time.Second
	}
	if cfg.Redis.ClassificationCacheTTL == 0 {
		cfg.Redis.ClassificationCacheTTL = 24 * time.Hour
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.Format == "" {
		cfg.Logging.Format = "json"
	}

	// Classification defaults
	if cfg.Classification.Quality.WordCountWeight == 0 {
		cfg.Classification.Quality.WordCountWeight = 0.25
	}
	if cfg.Classification.Quality.MetadataWeight == 0 {
		cfg.Classification.Quality.MetadataWeight = 0.25
	}
	if cfg.Classification.Quality.RichnessWeight == 0 {
		cfg.Classification.Quality.RichnessWeight = 0.25
	}
	if cfg.Classification.Quality.ReadabilityWeight == 0 {
		cfg.Classification.Quality.ReadabilityWeight = 0.25
	}
	if cfg.Classification.SourceReputation.DefaultScore == 0 {
		cfg.Classification.SourceReputation.DefaultScore = 50
	}
	if cfg.Classification.Topic.MaxTopics == 0 {
		cfg.Classification.Topic.MaxTopics = 5
	}
}
