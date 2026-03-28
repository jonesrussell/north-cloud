package config

import (
	"time"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
)

// Default configuration values.
const (
	defaultServiceName               = "classifier"
	defaultServiceVersion            = "1.0.0"
	defaultServicePort               = 8070
	defaultConcurrency               = 10
	defaultBatchSize                 = 100
	defaultPollIntervalSec           = 30
	defaultMinQualityScore           = 50
	defaultMinWordCount              = 100
	defaultDBHost                    = "localhost"
	defaultDBPort                    = 5432
	defaultDBUser                    = "postgres"
	defaultDBName                    = "classifier"
	defaultDBSSLMode                 = "disable"
	defaultDBMaxConns                = 25
	defaultDBMaxIdleConns            = 5
	defaultESURL                     = "http://localhost:9200"
	defaultESMaxRetries              = 3
	defaultESTimeoutSec              = 30
	defaultESRawSuffix               = "_raw_content"
	defaultESClassifiedSuffix        = "_classified_content"
	defaultRedisURL                  = "localhost:6379"
	defaultRedisMaxRetries           = 3
	defaultRedisTimeoutSec           = 5
	defaultCacheTTLHours             = 24
	defaultLogLevel                  = "info"
	defaultLogFormat                 = "json"
	defaultQualityWeight             = 0.25
	defaultReputationScore           = 50
	defaultMaxTopics                 = 5
	defaultCrimeMLServiceURL         = "http://crime-ml:8076"
	defaultCoforgeMLServiceURL       = "http://coforge-ml:8078"
	defaultEntertainmentMLServiceURL = "http://entertainment-ml:8079"
	defaultIndigenousMLServiceURL    = "http://indigenous-ml:8081"
	defaultMiningMLServiceURL        = "http://mining-ml:8077"
	defaultQualityGateThreshold      = 40
)

// Config holds all configuration for the classifier service.
type Config struct {
	Service        ServiceConfig        `yaml:"service"`
	Database       DatabaseConfig       `yaml:"database"`
	Elasticsearch  ElasticsearchConfig  `yaml:"elasticsearch"`
	Redis          RedisConfig          `yaml:"redis"`
	Logging        LoggingConfig        `yaml:"logging"`
	Classification ClassificationConfig `yaml:"classification"`
	Auth           AuthConfig           `yaml:"auth"`
}

// ServiceConfig holds service-level configuration.
type ServiceConfig struct {
	Name              string        `yaml:"name"`
	Version           string        `yaml:"version"`
	Port              int           `env:"CLASSIFIER_PORT"         yaml:"port"`
	Debug             bool          `env:"APP_DEBUG"               yaml:"debug"`
	Enabled           bool          `yaml:"enabled"`
	Concurrency       int           `env:"CLASSIFIER_CONCURRENCY"  yaml:"concurrency"`
	BatchSize         int           `yaml:"batch_size"`
	PollInterval      time.Duration `yaml:"poll_interval"`
	MinQualityScore   int           `yaml:"min_quality_score"`
	MinWordCount      int           `yaml:"min_word_count"`
	MinArticleWordCnt int           `yaml:"min_article_word_count"`
	PipelineURL       string        `env:"PIPELINE_URL"            yaml:"pipeline_url"`
}

// DatabaseConfig holds database configuration.
type DatabaseConfig struct {
	Host            string        `env:"POSTGRES_HOST"            yaml:"host"`
	Port            int           `env:"POSTGRES_PORT"            yaml:"port"`
	User            string        `env:"POSTGRES_USER"            yaml:"user"`
	Password        string        `env:"POSTGRES_PASSWORD"        yaml:"password"`
	Database        string        `env:"POSTGRES_DB"              yaml:"database"`
	SSLMode         string        `env:"POSTGRES_SSLMODE"         yaml:"sslmode"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdleConns    int           `yaml:"max_idle_connections"`
	ConnMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	URL                     string        `env:"ELASTICSEARCH_URL"          yaml:"url"`
	Username                string        `yaml:"username"`
	Password                string        `yaml:"password"`
	MaxRetries              int           `yaml:"max_retries"`
	Timeout                 time.Duration `yaml:"timeout"`
	RawContentSuffix        string        `yaml:"raw_content_suffix"`
	ClassifiedContentSuffix string        `yaml:"classified_content_suffix"`
}

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	URL                    string        `env:"REDIS_URL"                 yaml:"url"`
	Password               string        `env:"REDIS_PASSWORD"            yaml:"password"`
	Database               int           `yaml:"database"`
	MaxRetries             int           `yaml:"max_retries"`
	Timeout                time.Duration `yaml:"timeout"`
	ChannelNewContent      string        `yaml:"channel_new_content"`
	ChannelClassified      string        `yaml:"channel_classified"`
	ClassificationCacheTTL time.Duration `yaml:"classification_cache_ttl"`
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
	Output string `yaml:"output"`
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	JWTSecret      string `env:"AUTH_JWT_SECRET"      yaml:"jwt_secret"`
	InternalSecret string `env:"AUTH_INTERNAL_SECRET" yaml:"internal_secret"`
}

// SidecarConfig holds enabled flag and ML service URL for one optional classifier sidecar.
// Used in classification.sidecar_registry (keyed by sidecar name, e.g. "crime", "mining").
type SidecarConfig struct {
	Enabled      bool   `yaml:"enabled"`
	MLServiceURL string `yaml:"ml_service_url"`
}

// ClassificationConfig holds classification settings.
type ClassificationConfig struct {
	ContentType      ContentTypeConfig          `yaml:"content_type"`
	Quality          QualityConfig              `yaml:"quality"`
	Topic            TopicConfig                `yaml:"topic"`
	SourceReputation SourceReputationConfig     `yaml:"source_reputation"`
	Crime            CrimeConfig                `yaml:"crime"`
	Mining           MiningConfig               `yaml:"mining"`
	Coforge          CoforgeConfig              `yaml:"coforge"`
	Entertainment    EntertainmentConfig        `yaml:"entertainment"`
	Indigenous       IndigenousConfig           `yaml:"indigenous"`
	Recipe           RecipeExtractionConfig     `yaml:"recipe"`
	Job              JobExtractionConfig        `yaml:"job"`
	RFP              RFPExtractionConfig        `yaml:"rfp"`
	NeedSignal       NeedSignalExtractionConfig `yaml:"need_signal"`
	DrillExtraction  DrillExtractionConfig      `yaml:"drill_extraction"`
	QualityGate      QualityGateConfig          `yaml:"quality_gate"`
	// SidecarRegistry maps sidecar name (e.g. "crime", "mining") to enabled + URL.
	// Built from Crime/Mining/... named configs when absent in YAML.
	// NOTE: Currently populated by setClassificationDefaults but not yet consumed by the bootstrap
	// or classifier — the named fields (Crime, Mining, etc.) remain authoritative.
	// TODO: when declarative registry-driven dispatch is implemented, this will replace named fields.
	SidecarRegistry map[string]SidecarConfig `yaml:"sidecar_registry"`
	// SidecarRegistryFromYAML is true when sidecar_registry was explicitly set in the YAML config.
	// It has no runtime effect but triggers a startup warning so operators know the field is inoperative.
	SidecarRegistryFromYAML bool `yaml:"-"` // not loaded from YAML; set by setClassificationDefaults
	// Routing maps route key (e.g. "article", "article:event") to sidecar names to run. Optional; default matches current behavior.
	Routing map[string][]string `yaml:"routing"`
}

// IndigenousConfig holds Indigenous hybrid classification settings.
type IndigenousConfig struct {
	Enabled      bool   `env:"INDIGENOUS_ENABLED"        yaml:"enabled"`
	MLServiceURL string `env:"INDIGENOUS_ML_SERVICE_URL" yaml:"ml_service_url"`
}

// CrimeConfig holds Crime hybrid classification settings.
type CrimeConfig struct {
	Enabled      bool   `env:"CRIME_ENABLED"        yaml:"enabled"`
	MLServiceURL string `env:"CRIME_ML_SERVICE_URL" yaml:"ml_service_url"`
}

// MiningConfig holds Mining hybrid classification settings.
type MiningConfig struct {
	Enabled      bool   `env:"MINING_ENABLED"        yaml:"enabled"`
	MLServiceURL string `env:"MINING_ML_SERVICE_URL" yaml:"ml_service_url"`
}

// CoforgeConfig holds Coforge hybrid classification settings.
type CoforgeConfig struct {
	Enabled      bool   `env:"COFORGE_ENABLED"        yaml:"enabled"`
	MLServiceURL string `env:"COFORGE_ML_SERVICE_URL" yaml:"ml_service_url"`
}

// EntertainmentConfig holds Entertainment hybrid classification settings.
type EntertainmentConfig struct {
	Enabled      bool   `env:"ENTERTAINMENT_ENABLED"        yaml:"enabled"`
	MLServiceURL string `env:"ENTERTAINMENT_ML_SERVICE_URL" yaml:"ml_service_url"`
}

// RecipeExtractionConfig holds recipe extraction settings.
type RecipeExtractionConfig struct {
	Enabled bool `env:"RECIPE_ENABLED" yaml:"enabled"`
}

// JobExtractionConfig holds job extraction settings.
type JobExtractionConfig struct {
	Enabled bool `env:"JOB_ENABLED" yaml:"enabled"`
}

// RFPExtractionConfig holds RFP extraction settings.
type RFPExtractionConfig struct {
	Enabled bool `env:"RFP_ENABLED" yaml:"enabled"`
}

// NeedSignalExtractionConfig holds need signal extraction settings.
type NeedSignalExtractionConfig struct {
	Enabled bool `env:"NEED_SIGNAL_ENABLED" yaml:"enabled"`
}

// DrillExtractionConfig holds drill results extraction settings.
type DrillExtractionConfig struct {
	Enabled          bool   `env:"DRILL_EXTRACTION_ENABLED" yaml:"enabled"`
	LLMFallback      bool   `env:"DRILL_LLM_FALLBACK"       yaml:"llm_fallback"`
	AnthropicKey     string `env:"ANTHROPIC_API_KEY"        yaml:"anthropic_api_key"`
	AnthropicModel   string `yaml:"anthropic_model"`
	AnthropicBaseURL string `yaml:"anthropic_base_url"`
	MaxBodyChars     int    `yaml:"max_body_chars"`
}

// QualityGateConfig holds quality gate settings.
type QualityGateConfig struct {
	Enabled   bool `env:"CLASSIFIER_QUALITY_GATE_ENABLED"   yaml:"enabled"`
	Threshold int  `env:"CLASSIFIER_QUALITY_GATE_THRESHOLD" yaml:"threshold"`
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
	setServiceDefaults(&cfg.Service)
	setDatabaseDefaults(&cfg.Database)
	setElasticsearchDefaults(&cfg.Elasticsearch)
	setRedisDefaults(&cfg.Redis)
	setLoggingDefaults(&cfg.Logging)
	setClassificationDefaults(&cfg.Classification)
	// Auth defaults are handled by env tags - no explicit defaults needed
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
	if s.Concurrency == 0 {
		s.Concurrency = defaultConcurrency
	}
	if s.BatchSize == 0 {
		s.BatchSize = defaultBatchSize
	}
	if s.PollInterval == 0 {
		s.PollInterval = defaultPollIntervalSec * time.Second
	}
	if s.MinQualityScore == 0 {
		s.MinQualityScore = defaultMinQualityScore
	}
	if s.MinWordCount == 0 {
		s.MinWordCount = defaultMinWordCount
	}
}

func setDatabaseDefaults(d *DatabaseConfig) {
	if d.Host == "" {
		d.Host = defaultDBHost
	}
	if d.Port == 0 {
		d.Port = defaultDBPort
	}
	if d.User == "" {
		d.User = defaultDBUser
	}
	if d.Database == "" {
		d.Database = defaultDBName
	}
	if d.SSLMode == "" {
		d.SSLMode = defaultDBSSLMode
	}
	if d.MaxConnections == 0 {
		d.MaxConnections = defaultDBMaxConns
	}
	if d.MaxIdleConns == 0 {
		d.MaxIdleConns = defaultDBMaxIdleConns
	}
	if d.ConnMaxLifetime == 0 {
		d.ConnMaxLifetime = time.Hour
	}
}

func setElasticsearchDefaults(e *ElasticsearchConfig) {
	if e.URL == "" {
		e.URL = defaultESURL
	}
	if e.MaxRetries == 0 {
		e.MaxRetries = defaultESMaxRetries
	}
	if e.Timeout == 0 {
		e.Timeout = defaultESTimeoutSec * time.Second
	}
	if e.RawContentSuffix == "" {
		e.RawContentSuffix = defaultESRawSuffix
	}
	if e.ClassifiedContentSuffix == "" {
		e.ClassifiedContentSuffix = defaultESClassifiedSuffix
	}
}

func setRedisDefaults(r *RedisConfig) {
	if r.URL == "" {
		r.URL = defaultRedisURL
	}
	if r.MaxRetries == 0 {
		r.MaxRetries = defaultRedisMaxRetries
	}
	if r.Timeout == 0 {
		r.Timeout = defaultRedisTimeoutSec * time.Second
	}
	if r.ClassificationCacheTTL == 0 {
		r.ClassificationCacheTTL = defaultCacheTTLHours * time.Hour
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

func setClassificationDefaults(c *ClassificationConfig) {
	if c.Quality.WordCountWeight == 0 {
		c.Quality.WordCountWeight = defaultQualityWeight
	}
	if c.Quality.MetadataWeight == 0 {
		c.Quality.MetadataWeight = defaultQualityWeight
	}
	if c.Quality.RichnessWeight == 0 {
		c.Quality.RichnessWeight = defaultQualityWeight
	}
	if c.Quality.ReadabilityWeight == 0 {
		c.Quality.ReadabilityWeight = defaultQualityWeight
	}
	if c.SourceReputation.DefaultScore == 0 {
		c.SourceReputation.DefaultScore = defaultReputationScore
	}
	if c.Topic.MaxTopics == 0 {
		c.Topic.MaxTopics = defaultMaxTopics
	}
	// Crime defaults: disabled by default, but set ML URL
	if c.Crime.MLServiceURL == "" {
		c.Crime.MLServiceURL = defaultCrimeMLServiceURL
	}
	// Coforge defaults: disabled by default, but set ML URL
	if c.Coforge.MLServiceURL == "" {
		c.Coforge.MLServiceURL = defaultCoforgeMLServiceURL
	}
	// Entertainment defaults: disabled by default, but set ML URL
	if c.Entertainment.MLServiceURL == "" {
		c.Entertainment.MLServiceURL = defaultEntertainmentMLServiceURL
	}
	// Indigenous defaults: disabled by default, but set ML URL
	if c.Indigenous.MLServiceURL == "" {
		c.Indigenous.MLServiceURL = defaultIndigenousMLServiceURL
	}
	// Mining defaults: disabled by default, but set ML URL
	if c.Mining.MLServiceURL == "" {
		c.Mining.MLServiceURL = defaultMiningMLServiceURL
	}
	// DrillExtraction defaults: disabled by default for safety
	if c.DrillExtraction.AnthropicModel == "" {
		c.DrillExtraction.AnthropicModel = "claude-haiku-4-5"
	}
	if c.DrillExtraction.AnthropicBaseURL == "" {
		c.DrillExtraction.AnthropicBaseURL = "https://api.anthropic.com"
	}
	if c.DrillExtraction.MaxBodyChars == 0 {
		c.DrillExtraction.MaxBodyChars = 4000
	}
	// QualityGate defaults: disabled by default for safe rollout
	if c.QualityGate.Threshold == 0 {
		c.QualityGate.Threshold = defaultQualityGateThreshold
	}
	// Routing: if absent, use default routing table (article -> all; article:event -> location; article:blotter -> crime; article:report -> none)
	if c.Routing == nil {
		c.Routing = getDefaultRouting()
	}
	// SidecarRegistry: if explicitly set in YAML, mark it so callers can warn; otherwise build from named fields.
	if c.SidecarRegistry != nil {
		c.SidecarRegistryFromYAML = true
	} else {
		c.SidecarRegistry = getDefaultSidecarRegistry(c)
	}
}

// SetDefaults applies all defaults to cfg. Call this when constructing a Config without Load
// (e.g. in test helpers or fallback paths that cannot read a config file).
func SetDefaults(cfg *Config) {
	setDefaults(cfg)
}

// getDefaultRouting returns the default content-type → sidecars mapping (current behavior).
func getDefaultRouting() map[string][]string {
	return map[string][]string{
		"article":              {"crime", "mining", "coforge", "entertainment", "indigenous", "location"},
		"article:event":        {"location"},
		"article:event_report": {"location"},
		"article:blotter":      {"crime"},
		"article:report":       {},
	}
}

// getDefaultSidecarRegistry builds sidecar_registry from existing Crime, Mining, ... config blocks.
func getDefaultSidecarRegistry(c *ClassificationConfig) map[string]SidecarConfig {
	return map[string]SidecarConfig{
		"crime":         {Enabled: c.Crime.Enabled, MLServiceURL: c.Crime.MLServiceURL},
		"mining":        {Enabled: c.Mining.Enabled, MLServiceURL: c.Mining.MLServiceURL},
		"coforge":       {Enabled: c.Coforge.Enabled, MLServiceURL: c.Coforge.MLServiceURL},
		"entertainment": {Enabled: c.Entertainment.Enabled, MLServiceURL: c.Entertainment.MLServiceURL},
		"indigenous":    {Enabled: c.Indigenous.Enabled, MLServiceURL: c.Indigenous.MLServiceURL},
		"location":      {Enabled: true, MLServiceURL: ""}, // in-process, no URL
	}
}
