package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	infraconfig "github.com/north-cloud/infrastructure/config"
	infracontext "github.com/north-cloud/infrastructure/context"
)

const (
	// DefaultReadTimeoutSeconds is the default read timeout in seconds
	DefaultReadTimeoutSeconds = 10
	// DefaultWriteTimeoutSeconds is the default write timeout in seconds
	DefaultWriteTimeoutSeconds = 30
	// DefaultShutdownTimeoutSeconds is the default shutdown timeout in seconds
	DefaultShutdownTimeoutSeconds = 30
	// DefaultServerAddress is the default server listen address
	DefaultServerAddress = ":8070"
)

type Config struct {
	Debug         bool                `env:"APP_DEBUG"      yaml:"debug"` // Application debug mode (controls log level and format)
	Server        ServerConfig        `yaml:"server"`
	Database      DatabaseConfig      `yaml:"database"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Redis         RedisConfig         `yaml:"redis"`
	Service       ServiceConfig       `yaml:"service"`
	Cities        []CityConfig        `yaml:"cities"`
	Sources       SourcesConfig       `yaml:"sources"` // Optional: Sources service configuration
	Auth          AuthConfig          `yaml:"auth"`
}

type DatabaseConfig struct {
	Host     string `env:"POSTGRES_PUBLISHER_HOST"     yaml:"host"`
	Port     string `env:"POSTGRES_PUBLISHER_PORT"     yaml:"port"`
	User     string `env:"POSTGRES_PUBLISHER_USER"     yaml:"user"`
	Password string `env:"POSTGRES_PUBLISHER_PASSWORD" yaml:"password"`
	DBName   string `env:"POSTGRES_PUBLISHER_DB"       yaml:"dbname"`
	SSLMode  string `env:"POSTGRES_PUBLISHER_SSLMODE"  yaml:"sslmode"`
}

type ElasticsearchConfig struct {
	URL      string `env:"ES_URL"    yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type RedisConfig struct {
	URL      string `env:"REDIS_URL"      yaml:"url"`
	Password string `env:"REDIS_PASSWORD" yaml:"password"`
	DB       int    `yaml:"db"`
}

type AuthConfig struct {
	JWTSecret string `env:"AUTH_JWT_SECRET" yaml:"jwt_secret"`
}

type ServiceConfig struct {
	CheckInterval        time.Duration `env:"PUBLISHER_ROUTER_CHECK_INTERVAL" yaml:"check_interval"`
	BatchSize            int           `env:"PUBLISHER_ROUTER_BATCH_SIZE"     yaml:"batch_size"`
	UseClassifiedContent bool          `yaml:"use_classified_content"` // Use classified_content indexes instead of articles
	MinQualityScore      int           `yaml:"min_quality_score"`      // Minimum quality score for classified content (0-100)
	IndexSuffix          string        `yaml:"index_suffix"`           // Index suffix (_articles or _classified_content)
}

type CityConfig struct {
	Name  string `yaml:"name"`
	Index string `yaml:"index"`
}

type SourcesConfig struct {
	URL     string        `env:"SOURCES_URL"     yaml:"url"`     // Sources service API URL (e.g., "http://localhost:8080")
	Timeout time.Duration `yaml:"timeout"`                       // Request timeout (default: 5s)
	Enabled bool          `env:"SOURCES_ENABLED" yaml:"enabled"` // Enable fetching cities from sources service
}

type ServerConfig struct {
	Address      string        `env:"PUBLISHER_PORT" yaml:"address"` // e.g., DefaultServerAddress or port number
	ReadTimeout  time.Duration `yaml:"read_timeout"`                 // Default: 10s
	WriteTimeout time.Duration `yaml:"write_timeout"`                // Default: 30s
	CORSOrigins  []string      `env:"CORS_ORIGINS"   yaml:"cors_origins"`
}

// Validate checks if the server configuration is valid and sets defaults.
func (c *ServerConfig) Validate() error {
	// Handle port - can be ":PORT" or just port number
	if c.Address == "" {
		c.Address = DefaultServerAddress
	} else if c.Address != "" && !strings.HasPrefix(c.Address, ":") {
		// If just a port number, prepend ":"
		c.Address = ":" + c.Address
	}
	if c.ReadTimeout <= 0 {
		c.ReadTimeout = DefaultReadTimeoutSeconds * time.Second
	}
	if c.WriteTimeout <= 0 {
		c.WriteTimeout = DefaultWriteTimeoutSeconds * time.Second
	}
	return nil
}

// Validate checks if the configuration is valid and returns an error if not.
func (c *Config) Validate() error {
	if c.Elasticsearch.URL == "" {
		return errors.New("elasticsearch.url is required")
	}
	if c.Redis.URL == "" {
		return errors.New("redis.url is required")
	}
	if c.Service.CheckInterval <= 0 {
		return fmt.Errorf("service.check_interval must be positive, got %v", c.Service.CheckInterval)
	}
	if c.Sources.Enabled && c.Sources.URL == "" {
		return errors.New("sources.url is required when sources.enabled is true")
	}
	for i, city := range c.Cities {
		if city.Name == "" {
			return fmt.Errorf("cities[%d].name is required", i)
		}
	}
	return nil
}

// SetDefaults sets default values for configuration fields
func SetDefaults(cfg *Config) {
	if cfg.Server.Address == "" {
		cfg.Server.Address = DefaultServerAddress
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = DefaultReadTimeoutSeconds * time.Second
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = DefaultWriteTimeoutSeconds * time.Second
	}
	if cfg.Service.CheckInterval == 0 {
		cfg.Service.CheckInterval = 5 * time.Minute
	}
	if cfg.Service.BatchSize == 0 {
		cfg.Service.BatchSize = 100
	}
	if cfg.Sources.Timeout == 0 {
		cfg.Sources.Timeout = 5 * time.Second
	}
	// Database defaults
	if cfg.Database.Host == "" {
		cfg.Database.Host = "localhost"
	}
	if cfg.Database.Port == "" {
		cfg.Database.Port = "5432"
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "postgres"
	}
	if cfg.Database.DBName == "" {
		cfg.Database.DBName = "publisher"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	// Classified content defaults
	if cfg.Service.MinQualityScore == 0 {
		cfg.Service.MinQualityScore = 50 // Default minimum quality score
	}
	if cfg.Service.IndexSuffix == "" {
		if cfg.Service.UseClassifiedContent {
			cfg.Service.IndexSuffix = "_classified_content"
		} else {
			cfg.Service.IndexSuffix = "_articles"
		}
	}
	// Set default CORS origins if not provided
	if len(cfg.Server.CORSOrigins) == 0 {
		cfg.Server.CORSOrigins = []string{
			"http://localhost:3000", // Legacy dashboard frontend
			"http://localhost:3001", // Crawler frontend
			"http://localhost:3002", // Unified dashboard frontend
		}
	}
}

func Load(path string) (*Config, error) {
	cfg, err := infraconfig.LoadWithDefaults(path, SetDefaults)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Validate server config (handles port formatting)
	if err := cfg.Server.Validate(); err != nil {
		return nil, fmt.Errorf("server config validation: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// LoadWithSources loads configuration and optionally fetches cities from sources service.
// If sources service is enabled and cities are fetched successfully, they override the config file cities.
func LoadWithSources(path string, sourcesClient interface {
	GetCities(context.Context) ([]CityConfig, error)
}) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}

	// If sources service is enabled, try to fetch cities
	if cfg.Sources.Enabled && sourcesClient != nil {
		ctx, cancel := infracontext.WithTimeout(cfg.Sources.Timeout)
		defer cancel()

		cities, err := sourcesClient.GetCities(ctx)
		if err != nil {
			// Log warning but don't fail - fallback to config file cities
			// In production, you might want to use a logger here
			_ = err
			return cfg, nil
		}

		if len(cities) > 0 {
			cfg.Cities = cities
		}
	}

	return cfg, nil
}
