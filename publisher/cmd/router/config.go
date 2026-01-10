package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jonesrussell/north-cloud/publisher/internal/config"
	"github.com/jonesrussell/north-cloud/publisher/internal/database"
	infraconfig "github.com/north-cloud/infrastructure/config"
)

const (
	defaultCheckInterval = 5 * time.Minute
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

// LoadConfig loads configuration from config file with env var overrides
func LoadConfig() ServiceConfig {
	// Load main config
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - create default config if file doesn't exist
		// Use fmt for config loading errors since logger isn't initialized yet
		fmt.Printf("Warning: Failed to load config file (%s), using defaults: %v\n", configPath, err)
		cfg = &config.Config{}
		// Apply defaults manually
		if cfg.Service.CheckInterval == 0 {
			cfg.Service.CheckInterval = defaultCheckInterval
		}
		if cfg.Service.BatchSize == 0 {
			cfg.Service.BatchSize = 100
		}
		if validateErr := cfg.Validate(); validateErr != nil {
			fmt.Fprintf(os.Stderr, "Invalid default configuration: %v\n", validateErr)
			os.Exit(1)
		}
	}

	// Convert main config to ServiceConfig
	return ServiceConfig{
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
