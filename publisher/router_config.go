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
	defaultPollInterval      = 30 * time.Second
	defaultDiscoveryInterval = 5 * time.Minute
)

// RouterConfig holds configuration for the router service
type RouterConfig struct {
	Database          database.Config
	ESURL             string
	RedisAddr         string
	RedisPassword     string
	PollInterval      time.Duration
	DiscoveryInterval time.Duration
	BatchSize         int
}

// LoadRouterConfig loads configuration from config file with env var overrides
func LoadRouterConfig() RouterConfig {
	// Load main config
	configPath := infraconfig.GetConfigPath("config.yml")
	cfg, err := config.Load(configPath)
	if err != nil {
		// Config file is optional - try loading from environment variables only
		// Use fmt for config loading errors since logger isn't initialized yet
		fmt.Printf("Warning: Failed to load config file (%s), trying environment variables: %v\n", configPath, err)
		cfg = &config.Config{}
		// Apply environment variables to the config
		if envErr := infraconfig.ApplyEnvOverrides(cfg); envErr != nil {
			fmt.Fprintf(os.Stderr, "Failed to apply environment overrides: %v\n", envErr)
		}
		// Apply defaults for any missing values
		config.SetDefaults(cfg)
		if validateErr := cfg.Validate(); validateErr != nil {
			fmt.Fprintf(os.Stderr, "Invalid configuration from environment: %v\n", validateErr)
			os.Exit(1)
		}
	}

	// Map CheckInterval to PollInterval for Routing V2
	pollInterval := cfg.Service.CheckInterval
	if pollInterval == 0 {
		pollInterval = defaultPollInterval
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
		ESURL:             cfg.Elasticsearch.URL,
		RedisAddr:         cfg.Redis.URL,
		RedisPassword:     cfg.Redis.Password,
		PollInterval:      pollInterval,
		DiscoveryInterval: defaultDiscoveryInterval,
		BatchSize:         cfg.Service.BatchSize,
	}
}
