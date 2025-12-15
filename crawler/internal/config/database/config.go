// Package database provides database configuration management.
package database

import (
	"os"

	"github.com/spf13/viper"
)

// Default configuration values
const (
	DefaultHost    = "localhost"
	DefaultPort    = "5432"
	DefaultUser    = "postgres"
	DefaultDBName  = "gocrawl"
	DefaultSSLMode = "disable"
)

// Config represents database configuration settings.
type Config struct {
	Host     string `yaml:"host" env:"DB_HOST"`
	Port     string `yaml:"port" env:"DB_PORT"`
	User     string `yaml:"user" env:"DB_USER"`
	Password string `yaml:"password" env:"DB_PASSWORD"`
	DBName   string `yaml:"dbname" env:"DB_NAME"`
	SSLMode  string `yaml:"sslmode" env:"DB_SSLMODE"`
}

// LoadFromViper loads database configuration from Viper and environment variables.
// Environment variables take precedence over Viper configuration.
func LoadFromViper(v *viper.Viper) *Config {
	cfg := &Config{}

	// Load from environment variables first (highest priority)
	if host := os.Getenv("DB_HOST"); host != "" {
		cfg.Host = host
	} else {
		cfg.Host = v.GetString("database.host")
	}
	if cfg.Host == "" {
		cfg.Host = DefaultHost
	}

	if port := os.Getenv("DB_PORT"); port != "" {
		cfg.Port = port
	} else {
		cfg.Port = v.GetString("database.port")
	}
	if cfg.Port == "" {
		cfg.Port = DefaultPort
	}

	if user := os.Getenv("DB_USER"); user != "" {
		cfg.User = user
	} else {
		cfg.User = v.GetString("database.user")
	}
	if cfg.User == "" {
		cfg.User = DefaultUser
	}

	if password := os.Getenv("DB_PASSWORD"); password != "" {
		cfg.Password = password
	} else {
		cfg.Password = v.GetString("database.password")
	}

	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		cfg.DBName = dbName
	} else {
		cfg.DBName = v.GetString("database.dbname")
	}
	if cfg.DBName == "" {
		cfg.DBName = DefaultDBName
	}

	if sslMode := os.Getenv("DB_SSLMODE"); sslMode != "" {
		cfg.SSLMode = sslMode
	} else {
		cfg.SSLMode = v.GetString("database.sslmode")
	}
	if cfg.SSLMode == "" {
		cfg.SSLMode = DefaultSSLMode
	}

	return cfg
}
