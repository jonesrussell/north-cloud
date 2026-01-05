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
	DefaultDBName  = "crawler"
	DefaultSSLMode = "disable"
)

// Config represents database configuration settings.
type Config struct {
	Host     string `env:"DB_HOST"     yaml:"host"`
	Port     string `env:"DB_PORT"     yaml:"port"`
	User     string `env:"DB_USER"     yaml:"user"`
	Password string `env:"DB_PASSWORD" yaml:"password"`
	DBName   string `env:"DB_NAME"     yaml:"dbname"`
	SSLMode  string `env:"DB_SSLMODE"  yaml:"sslmode"`
}

// getConfigValue retrieves a configuration value from environment or Viper, with a default fallback.
func getConfigValue(envKey, viperKey, defaultValue string, v *viper.Viper) string {
	if val := os.Getenv(envKey); val != "" {
		return val
	}
	if val := v.GetString(viperKey); val != "" {
		return val
	}
	return defaultValue
}

// LoadFromViper loads database configuration from Viper and environment variables.
// Environment variables take precedence over Viper configuration.
func LoadFromViper(v *viper.Viper) *Config {
	return &Config{
		Host:     getConfigValue("DB_HOST", "database.host", DefaultHost, v),
		Port:     getConfigValue("DB_PORT", "database.port", DefaultPort, v),
		User:     getConfigValue("DB_USER", "database.user", DefaultUser, v),
		Password: getConfigValue("DB_PASSWORD", "database.password", "", v),
		DBName:   getConfigValue("DB_NAME", "database.dbname", DefaultDBName, v),
		SSLMode:  getConfigValue("DB_SSLMODE", "database.sslmode", DefaultSSLMode, v),
	}
}
