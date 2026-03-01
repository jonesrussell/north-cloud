package config_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Address:      ":8077",
			ReadTimeout:  "10s",
			WriteTimeout: "30s",
		},
		Database: config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "social_publisher",
			Password: "secret",
			DBName:   "social_publisher",
			SSLMode:  "disable",
		},
		Redis: config.RedisConfig{
			URL: "localhost:6379",
		},
		Service: config.ServiceConfig{
			RetryInterval:    "30s",
			ScheduleInterval: "60s",
			MaxRetries:       3,
			BatchSize:        50,
		},
	}
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_MissingDatabase(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Address: ":8077"},
		Redis:  config.RedisConfig{URL: "localhost:6379"},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database")
}

func TestConfig_Validate_MissingRedis(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Address: ":8077"},
		Database: config.DatabaseConfig{
			Host: "localhost", Port: 5432, User: "u", Password: "p", DBName: "db",
		},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
}
