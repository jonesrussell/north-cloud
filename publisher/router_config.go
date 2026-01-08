package main

import (
	"log"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	infraconfig "github.com/north-cloud/infrastructure/config"
)

const (
	defaultCheckInterval = 5 * time.Minute
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

// LoadRouterConfig loads configuration from config file with env var overrides
func LoadRouterConfig() RouterConfig {
	// Load main config
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - create default config if file doesn't exist
		log.Printf("Warning: Failed to load config file (%s), using defaults: %v", configPath, err)
		cfg = &config.Config{}
		// Apply defaults manually
		if cfg.Service.CheckInterval == 0 {
			cfg.Service.CheckInterval = defaultCheckInterval
		}
		if cfg.Service.BatchSize == 0 {
			cfg.Service.BatchSize = 100
		}
		if validateErr := cfg.Validate(); validateErr != nil {
			log.Fatalf("Invalid default configuration: %v", validateErr)
		}
	}

	// Convert main config to RouterConfig
	return RouterConfig{
		Database: database.Config{
			Host:     cfg.Database.Host,
			Port:     cfg.Database.Port,
			User:     cfg.Database.User,
			Password: cfg.Database.Password,
			DBName:   cfg.Database.DBName,
			SSLMode:  cfg.Database.SSLMode,
		},
		ESURL:         cfg.Elasticsearch.URL,
		RedisAddr:     cfg.Redis.URL,
		RedisPassword: cfg.Redis.Password,
		CheckInterval: cfg.Service.CheckInterval,
		BatchSize:     cfg.Service.BatchSize,
	}
}
