package config

import (
	"strconv"
	"time"
)

const (
	defaultServerPort         = 8080
	defaultServerReadTimeout  = 30 * time.Second
	defaultServerWriteTimeout = 30 * time.Second
	defaultServerIdleTimeout  = 60 * time.Second

	defaultDatabasePort            = 5432
	defaultDatabaseSSLMode         = "disable"
	defaultDatabaseMaxConnections  = 25
	defaultDatabaseMaxIdleConns    = 5
	defaultDatabaseConnMaxLifetime = 5 * time.Minute

	defaultElasticsearchURL        = "http://localhost:9200"
	defaultElasticsearchMaxRetries = 3
	defaultElasticsearchTimeout    = 30 * time.Second

	defaultRedisURL = "localhost:6379"

	defaultLogLevel  = "info"
	defaultLogFormat = "json"
)

// ServerConfig holds HTTP server configuration.
type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
}

// Address returns the server address in host:port format.
func (c *ServerConfig) Address() string {
	if c.Host == "" {
		return ":" + formatPort(c.Port)
	}
	return c.Host + ":" + formatPort(c.Port)
}

// SetDefaults applies default values for ServerConfig.
func (c *ServerConfig) SetDefaults() {
	if c.Port == 0 {
		c.Port = defaultServerPort
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = defaultServerReadTimeout
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = defaultServerWriteTimeout
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = defaultServerIdleTimeout
	}
}

// DatabaseConfig holds PostgreSQL database configuration.
type DatabaseConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	User            string        `yaml:"user"`
	Password        string        `yaml:"password"`
	Database        string        `yaml:"database"`
	SSLMode         string        `yaml:"sslmode"`
	MaxConnections  int           `yaml:"max_connections"`
	MaxIdleConns    int           `yaml:"max_idle_connections"`
	ConnMaxLifetime time.Duration `yaml:"connection_max_lifetime"`
}

// DSN returns the PostgreSQL connection string.
func (c *DatabaseConfig) DSN() string {
	return "host=" + c.Host +
		" port=" + formatPort(c.Port) +
		" user=" + c.User +
		" password=" + c.Password +
		" dbname=" + c.Database +
		" sslmode=" + c.SSLMode
}

// SetDefaults applies default values for DatabaseConfig.
func (c *DatabaseConfig) SetDefaults() {
	if c.Port == 0 {
		c.Port = defaultDatabasePort
	}
	if c.SSLMode == "" {
		c.SSLMode = defaultDatabaseSSLMode
	}
	if c.MaxConnections == 0 {
		c.MaxConnections = defaultDatabaseMaxConnections
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = defaultDatabaseMaxIdleConns
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = defaultDatabaseConnMaxLifetime
	}
}

// ElasticsearchConfig holds Elasticsearch configuration.
type ElasticsearchConfig struct {
	URL        string        `env:"ELASTICSEARCH_URL" yaml:"url"`
	Username   string        `yaml:"username"`
	Password   string        `yaml:"password"`
	MaxRetries int           `yaml:"max_retries"`
	Timeout    time.Duration `yaml:"timeout"`
}

// SetDefaults applies default values for ElasticsearchConfig.
func (c *ElasticsearchConfig) SetDefaults() {
	if c.URL == "" {
		c.URL = defaultElasticsearchURL
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = defaultElasticsearchMaxRetries
	}
	if c.Timeout == 0 {
		c.Timeout = defaultElasticsearchTimeout
	}
}

// RedisConfig holds Redis configuration.
type RedisConfig struct {
	URL      string `env:"REDIS_URL" yaml:"url"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// SetDefaults applies default values for RedisConfig.
func (c *RedisConfig) SetDefaults() {
	if c.URL == "" {
		c.URL = defaultRedisURL
	}
}

// LoggingConfig holds logging configuration.
type LoggingConfig struct {
	Level  string `env:"LOG_LEVEL"  yaml:"level"`
	Format string `env:"LOG_FORMAT" yaml:"format"`
}

// SetDefaults applies default values for LoggingConfig.
func (c *LoggingConfig) SetDefaults() {
	if c.Level == "" {
		c.Level = defaultLogLevel
	}
	if c.Format == "" {
		c.Format = defaultLogFormat
	}
}

// formatPort converts a port number to string.
func formatPort(port int) string {
	return strconv.Itoa(port)
}
