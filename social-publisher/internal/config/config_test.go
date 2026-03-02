package config_test

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validTestKey() string {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return hex.EncodeToString(key)
}

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
		Encryption: config.EncryptionConfig{Key: validTestKey()},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestConfig_Validate_MissingDatabase(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Address: ":8077"},
		Redis:  config.RedisConfig{URL: "localhost:6379"},
	}
	err := cfg.Validate()
	require.Error(t, err)
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis")
}

func TestConfig_Validate_MissingEncryptionKey(t *testing.T) {
	cfg := &config.Config{
		Database: config.DatabaseConfig{Host: "localhost", DBName: "db"},
		Redis:    config.RedisConfig{URL: "localhost:6379"},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "encryption key is required")
}

func TestConfig_Validate_InvalidEncryptionKeyLength(t *testing.T) {
	cfg := &config.Config{
		Database:   config.DatabaseConfig{Host: "localhost", DBName: "db"},
		Redis:      config.RedisConfig{URL: "localhost:6379"},
		Encryption: config.EncryptionConfig{Key: "tooshort"},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "64-character hex string")
}

func TestConfig_Validate_InvalidEncryptionKeyHex(t *testing.T) {
	cfg := &config.Config{
		Database:   config.DatabaseConfig{Host: "localhost", DBName: "db"},
		Redis:      config.RedisConfig{URL: "localhost:6379"},
		Encryption: config.EncryptionConfig{Key: strings.Repeat("zz", 32)},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid encryption key")
}
