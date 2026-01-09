package config

import (
	"strconv"
	"time"
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
		c.Port = 8080
	}
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 30 * time.Second
	}
	if c.IdleTimeout == 0 {
		c.IdleTimeout = 60 * time.Second
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
		c.Port = 5432
	}
	if c.SSLMode == "" {
		c.SSLMode = "disable"
	}
	if c.MaxConnections == 0 {
		c.MaxConnections = 25
	}
	if c.MaxIdleConns == 0 {
		c.MaxIdleConns = 5
	}
	if c.ConnMaxLifetime == 0 {
		c.ConnMaxLifetime = 5 * time.Minute
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
		c.URL = "http://localhost:9200"
	}
	if c.MaxRetries == 0 {
		c.MaxRetries = 3
	}
	if c.Timeout == 0 {
		c.Timeout = 30 * time.Second
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
		c.URL = "localhost:6379"
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
		c.Level = "info"
	}
	if c.Format == "" {
		c.Format = "json"
	}
}

// formatPort converts a port number to string.
func formatPort(port int) string {
	return strconv.Itoa(port)
}
