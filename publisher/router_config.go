package main

import (
	"log"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
)

const (
	defaultBatchSize = 100
)

// RouterConfig holds configuration for the router service
type RouterConfig struct {
	Database      database.Config
	ESURL         string
	RedisAddr     string
	RedisPassword string
	CheckInterval time.Duration
	BatchSize     int
}

// LoadRouterConfig loads configuration from environment variables
func LoadRouterConfig() RouterConfig {
	checkIntervalStr := getEnv("PUBLISHER_ROUTER_CHECK_INTERVAL", "5m")
	checkInterval, parseErr := time.ParseDuration(checkIntervalStr)
	if parseErr != nil {
		log.Fatalf("Invalid check interval: %v", parseErr)
	}

	return RouterConfig{
		Database: database.Config{
			Host:     getEnv("POSTGRES_PUBLISHER_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PUBLISHER_PORT", "5432"),
			User:     getEnv("POSTGRES_PUBLISHER_USER", "postgres"),
			Password: getEnv("POSTGRES_PUBLISHER_PASSWORD", ""),
			DBName:   getEnv("POSTGRES_PUBLISHER_DB", "publisher"),
			SSLMode:  getEnv("POSTGRES_PUBLISHER_SSLMODE", "disable"),
		},
		ESURL:         getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		CheckInterval: checkInterval,
		BatchSize:     getEnvInt("PUBLISHER_ROUTER_BATCH_SIZE", defaultBatchSize),
	}
}
