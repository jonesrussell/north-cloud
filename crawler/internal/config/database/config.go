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
