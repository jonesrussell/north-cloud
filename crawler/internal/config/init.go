package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/elasticsearch"
	"github.com/spf13/viper"
)

// Log rotation defaults
const (
	defaultLogMaxSize    = 100 // MB before rotation
	defaultLogMaxBackups = 3   // old log files to retain
	defaultLogMaxAge     = 30  // days to retain old files
)

// InitializeViper initializes Viper configuration from environment variables and config files.
// This must be called before LoadConfig() to ensure Viper is properly configured.
func InitializeViper() error {
	loadEnvFile()
	setupViper()
	setDefaults()
	readConfigFile()

	if err := bindEnvironmentVariables(); err != nil {
		return fmt.Errorf("failed to bind environment variables: %w", err)
	}

	setupDevelopmentLogging()
	return nil
}

// loadEnvFile loads .env file (ignores error if file doesn't exist).
func loadEnvFile() {
	_ = godotenv.Load()
}

// setupViper configures Viper for environment variable and config file reading.
func setupViper() {
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
}

// readConfigFile reads config file (ignores error if file doesn't exist).
func readConfigFile() {
	_ = viper.ReadInConfig()
}

// bindEnvironmentVariables binds all environment variables to config keys.
func bindEnvironmentVariables() error {
	if err := bindAppEnvVars(); err != nil {
		return fmt.Errorf("failed to bind app env vars: %w", err)
	}
	if err := bindElasticsearchEnvVars(); err != nil {
		return fmt.Errorf("failed to bind elasticsearch env vars: %w", err)
	}
	if err := bindMinIOEnvVars(); err != nil {
		return fmt.Errorf("failed to bind minio env vars: %w", err)
	}
	return nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// App defaults - production safe
	viper.SetDefault("app", map[string]any{
		"name":        "crawler",
		"version":     "1.0.0",
		"environment": "production",
		"debug":       false,
	})

	// Logger defaults - production safe
	viper.SetDefault("logger", map[string]any{
		"level":        "info",
		"development":  false,
		"encoding":     "json",
		"output_paths": []string{"stdout"},
		"enable_color": false,
		"caller":       false,
		"stacktrace":   false,
		"max_size":     defaultLogMaxSize,    // MB before rotation
		"max_backups":  defaultLogMaxBackups, // old log files to retain
		"max_age":      defaultLogMaxAge,     // days to retain old files
		"compress":     true,
	})

	// Server defaults - production safe
	viper.SetDefault("server", map[string]any{
		"address":          ":8080",
		"read_timeout":     "15s",
		"write_timeout":    "15s",
		"idle_timeout":     "60s",
		"security_enabled": true,
	})

	// Elasticsearch defaults - production safe
	viper.SetDefault("elasticsearch", map[string]any{
		"addresses": []string{"http://127.0.0.1:9200"},
		"tls": map[string]any{
			"enabled":              true,
			"insecure_skip_verify": false,
		},
		"retry": map[string]any{
			"enabled":      true,
			"initial_wait": "1s",
			"max_wait":     "30s",
			"max_retries":  crawler.DefaultMaxRetries,
		},
		"bulk_size":      elasticsearch.DefaultBulkSize,
		"flush_interval": "1s",
		"index_prefix":   "crawler",
		"discover_nodes": false,
	})

	// Crawler defaults - production safe
	viper.SetDefault("crawler", map[string]any{
		"max_depth":          crawler.DefaultMaxDepth,
		"max_concurrency":    crawler.DefaultParallelism,
		"request_timeout":    "30s",
		"user_agent":         crawler.DefaultUserAgent,
		"respect_robots_txt": true,
		"delay":              "1s",
		"random_delay":       "0s",
		"source_file":        "sources.yml",
		"debugger": map[string]any{
			"enabled": false,
			"level":   "info",
			"format":  "json",
			"output":  "stdout",
		},
		"rate_limit":  "2s",
		"parallelism": crawler.DefaultParallelism,
		"tls": map[string]any{
			"insecure_skip_verify": false,
		},
		"retry_delay":      "5s",
		"max_retries":      crawler.DefaultMaxRetries,
		"follow_redirects": true,
		"max_redirects":    crawler.DefaultMaxRedirects,
		"validate_urls":    true,
		"cleanup_interval": crawler.DefaultCleanupInterval.String(),
	})
}

// bindAppEnvVars binds application and logger environment variables to config keys.
func bindAppEnvVars() error {
	if err := viper.BindEnv("app.environment", "APP_ENV"); err != nil {
		return fmt.Errorf("failed to bind APP_ENV: %w", err)
	}
	if err := viper.BindEnv("app.debug", "APP_DEBUG"); err != nil {
		return fmt.Errorf("failed to bind APP_DEBUG: %w", err)
	}
	if err := viper.BindEnv("logger.level", "LOG_LEVEL"); err != nil {
		return fmt.Errorf("failed to bind LOG_LEVEL: %w", err)
	}
	if err := viper.BindEnv("logger.encoding", "LOG_FORMAT"); err != nil {
		return fmt.Errorf("failed to bind LOG_FORMAT: %w", err)
	}
	// Bind crawler sources API URL
	if err := viper.BindEnv("crawler.sources_api_url", "CRAWLER_SOURCES_API_URL"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_SOURCES_API_URL: %w", err)
	}
	return nil
}

// bindElasticsearchEnvVars binds Elasticsearch environment variables to config keys.
func bindElasticsearchEnvVars() error {
	// Support both ELASTICSEARCH_HOSTS and ELASTICSEARCH_ADDRESSES
	if err := viper.BindEnv("elasticsearch.addresses", "ELASTICSEARCH_HOSTS", "ELASTICSEARCH_ADDRESSES"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch addresses: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.password", "ELASTIC_PASSWORD", "ELASTICSEARCH_PASSWORD"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch password: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.tls.insecure_skip_verify", "ELASTICSEARCH_SKIP_TLS"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch TLS skip verify: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.api_key", "ELASTICSEARCH_API_KEY"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch API key: %w", err)
	}
	// Bind index_name (supports both ELASTICSEARCH_INDEX_PREFIX and ELASTICSEARCH_INDEX_NAME)
	if err := viper.BindEnv("elasticsearch.index_name",
		"ELASTICSEARCH_INDEX_PREFIX", "ELASTICSEARCH_INDEX_NAME"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch index name: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.retry.max_retries", "ELASTICSEARCH_MAX_RETRIES"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch max retries: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.retry.initial_wait", "ELASTICSEARCH_RETRY_INITIAL_WAIT"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch retry initial wait: %w", err)
	}
	if err := viper.BindEnv("elasticsearch.retry.max_wait", "ELASTICSEARCH_RETRY_MAX_WAIT"); err != nil {
		return fmt.Errorf("failed to bind Elasticsearch retry max wait: %w", err)
	}
	return nil
}

// bindMinIOEnvVars binds MinIO environment variables to config keys.
func bindMinIOEnvVars() error {
	if err := viper.BindEnv("minio.enabled", "CRAWLER_MINIO_ENABLED"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_ENABLED: %w", err)
	}
	if err := viper.BindEnv("minio.endpoint", "CRAWLER_MINIO_ENDPOINT"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_ENDPOINT: %w", err)
	}
	if err := viper.BindEnv("minio.access_key", "CRAWLER_MINIO_ACCESS_KEY"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_ACCESS_KEY: %w", err)
	}
	if err := viper.BindEnv("minio.secret_key", "CRAWLER_MINIO_SECRET_KEY"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_SECRET_KEY: %w", err)
	}
	if err := viper.BindEnv("minio.use_ssl", "CRAWLER_MINIO_USE_SSL"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_USE_SSL: %w", err)
	}
	if err := viper.BindEnv("minio.bucket", "CRAWLER_MINIO_BUCKET"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_BUCKET: %w", err)
	}
	if err := viper.BindEnv("minio.metadata_bucket", "CRAWLER_MINIO_METADATA_BUCKET"); err != nil {
		return fmt.Errorf("failed to bind CRAWLER_MINIO_METADATA_BUCKET: %w", err)
	}
	return nil
}

// setupDevelopmentLogging configures logging settings based on environment variables.
// It separates concerns: debug level (controlled by APP_DEBUG) vs development formatting (controlled by APP_ENV).
func setupDevelopmentLogging() {
	debugFlag := viper.GetBool("app.debug")
	isDev := viper.GetString("app.environment") == "development"

	// Always set debug level when APP_DEBUG=true, regardless of environment (production, staging, development)
	// This allows enabling debug logs in production for troubleshooting
	if debugFlag {
		viper.Set("logger.level", "debug")
	}

	// Set development mode features (formatting, colors, console encoding, etc.) only in development environment
	// These formatting options are separate from log level - you can have debug logs with production formatting
	if isDev {
		viper.Set("logger.development", true)
		viper.Set("logger.enable_color", true)
		viper.Set("logger.caller", true)
		viper.Set("logger.stacktrace", true)
		viper.Set("logger.encoding", "console")
	}
}
