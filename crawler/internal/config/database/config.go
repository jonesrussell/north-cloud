// Package database provides database configuration management.
package database

// Default configuration values
const (
	DefaultHost    = "localhost"
	DefaultPort    = "5432"
	DefaultUser    = "postgres"
	DefaultDBName  = "crawler"
	DefaultSSLMode = "disable"
)

// Config represents database configuration settings.
type Config struct {
	Host     string `env:"POSTGRES_CRAWLER_HOST"     yaml:"host"`
	Port     string `env:"POSTGRES_CRAWLER_PORT"     yaml:"port"`
	User     string `env:"POSTGRES_CRAWLER_USER"     yaml:"user"`
	Password string `env:"POSTGRES_CRAWLER_PASSWORD" yaml:"password"`
	DBName   string `env:"POSTGRES_CRAWLER_DB"       yaml:"dbname"`
	SSLMode  string `env:"POSTGRES_CRAWLER_SSLMODE"  yaml:"sslmode"`
}

// NewConfig creates a new Config instance with default values.
func NewConfig() *Config {
	return &Config{
		Host:    DefaultHost,
		Port:    DefaultPort,
		User:    DefaultUser,
		DBName:  DefaultDBName,
		SSLMode: DefaultSSLMode,
	}
}
