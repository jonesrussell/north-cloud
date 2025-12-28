package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/database"
)

// ServiceConfig holds all configuration for the router service
type ServiceConfig struct {
	Database      database.Config
	ESURL         string
	RedisAddr     string
	RedisPassword string
	CheckInterval time.Duration
	BatchSize     int
}

// LoadConfig loads configuration from environment variables
func LoadConfig() ServiceConfig {
	checkIntervalStr := getEnv("PUBLISHER_ROUTER_CHECK_INTERVAL", "5m")
	checkInterval, parseErr := time.ParseDuration(checkIntervalStr)
	if parseErr != nil {
		log.Fatalf("Invalid check interval: %v", parseErr)
	}

	return ServiceConfig{
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

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		intValue, parseErr := strconv.Atoi(value)
		if parseErr == nil {
			return intValue
		}
	}
	return defaultValue
}
