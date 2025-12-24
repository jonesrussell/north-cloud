// Package minio provides MinIO configuration for HTML archiving.
package minio

import (
	"errors"
	"time"

	"github.com/spf13/viper"
)

// Config represents MinIO configuration for HTML archiving.
type Config struct {
	// Enabled toggles HTML archiving on/off
	Enabled bool `yaml:"enabled"`
	// Endpoint is the MinIO server address (e.g., "minio:9000")
	Endpoint string `yaml:"endpoint"`
	// AccessKey for MinIO authentication
	AccessKey string `yaml:"access_key"`
	// SecretKey for MinIO authentication
	SecretKey string `yaml:"secret_key"`
	// UseSSL enables HTTPS for MinIO connections
	UseSSL bool `yaml:"use_ssl"`
	// Bucket is the main bucket for HTML archives
	Bucket string `yaml:"bucket"`
	// MetadataBucket is the bucket for metadata JSON files
	MetadataBucket string `yaml:"metadata_bucket"`
	// UploadAsync enables non-blocking uploads via worker queue
	UploadAsync bool `yaml:"upload_async"`
	// UploadTimeout is the timeout for upload operations
	UploadTimeout time.Duration `yaml:"upload_timeout"`
	// MaxRetries is the maximum number of retry attempts for failed uploads
	MaxRetries int `yaml:"max_retries"`
	// Compression enables gzip compression before upload (future enhancement)
	Compression bool `yaml:"compression"`
	// FailSilently continues crawling even if archiving fails
	FailSilently bool `yaml:"fail_silently"`
}

const (
	// defaultUploadTimeout is the default timeout for upload operations.
	defaultUploadTimeout = 30 * time.Second
	// defaultMaxRetries is the default maximum number of retry attempts.
	defaultMaxRetries = 3
)

// NewConfig returns a new MinIO configuration with default values.
func NewConfig() *Config {
	return &Config{
		Enabled:        false,
		Endpoint:       "localhost:9000",
		UseSSL:         false,
		Bucket:         "html-archives",
		MetadataBucket: "crawler-metadata",
		UploadAsync:    true,
		UploadTimeout:  defaultUploadTimeout,
		MaxRetries:     defaultMaxRetries,
		Compression:    false,
		FailSilently:   true,
	}
}

// LoadFromViper loads MinIO configuration from Viper with environment variable overrides.
func LoadFromViper(v *viper.Viper) *Config {
	cfg := NewConfig()

	// Load from config file
	if v.IsSet("minio.enabled") {
		cfg.Enabled = v.GetBool("minio.enabled")
	}
	if v.IsSet("minio.endpoint") {
		cfg.Endpoint = v.GetString("minio.endpoint")
	}
	if v.IsSet("minio.access_key") {
		cfg.AccessKey = v.GetString("minio.access_key")
	}
	if v.IsSet("minio.secret_key") {
		cfg.SecretKey = v.GetString("minio.secret_key")
	}
	if v.IsSet("minio.use_ssl") {
		cfg.UseSSL = v.GetBool("minio.use_ssl")
	}
	if v.IsSet("minio.bucket") {
		cfg.Bucket = v.GetString("minio.bucket")
	}
	if v.IsSet("minio.metadata_bucket") {
		cfg.MetadataBucket = v.GetString("minio.metadata_bucket")
	}
	if v.IsSet("minio.upload_async") {
		cfg.UploadAsync = v.GetBool("minio.upload_async")
	}
	if v.IsSet("minio.upload_timeout") {
		cfg.UploadTimeout = v.GetDuration("minio.upload_timeout")
	}
	if v.IsSet("minio.max_retries") {
		cfg.MaxRetries = v.GetInt("minio.max_retries")
	}
	if v.IsSet("minio.compression") {
		cfg.Compression = v.GetBool("minio.compression")
	}
	if v.IsSet("minio.fail_silently") {
		cfg.FailSilently = v.GetBool("minio.fail_silently")
	}

	// Environment variable overrides
	if v.IsSet("CRAWLER_MINIO_ENABLED") {
		cfg.Enabled = v.GetBool("CRAWLER_MINIO_ENABLED")
	}
	if v.IsSet("CRAWLER_MINIO_ENDPOINT") {
		cfg.Endpoint = v.GetString("CRAWLER_MINIO_ENDPOINT")
	}
	if v.IsSet("CRAWLER_MINIO_ACCESS_KEY") {
		cfg.AccessKey = v.GetString("CRAWLER_MINIO_ACCESS_KEY")
	}
	if v.IsSet("CRAWLER_MINIO_SECRET_KEY") {
		cfg.SecretKey = v.GetString("CRAWLER_MINIO_SECRET_KEY")
	}
	if v.IsSet("CRAWLER_MINIO_USE_SSL") {
		cfg.UseSSL = v.GetBool("CRAWLER_MINIO_USE_SSL")
	}
	if v.IsSet("CRAWLER_MINIO_BUCKET") {
		cfg.Bucket = v.GetString("CRAWLER_MINIO_BUCKET")
	}
	if v.IsSet("CRAWLER_MINIO_METADATA_BUCKET") {
		cfg.MetadataBucket = v.GetString("CRAWLER_MINIO_METADATA_BUCKET")
	}

	return cfg
}

// Validate validates the MinIO configuration.
func (c *Config) Validate() error {
	if !c.Enabled {
		return nil // No validation needed if disabled
	}

	if c.Endpoint == "" {
		return errors.New("minio endpoint required when enabled")
	}
	if c.AccessKey == "" {
		return errors.New("minio access_key required when enabled")
	}
	if c.SecretKey == "" {
		return errors.New("minio secret_key required when enabled")
	}
	if c.Bucket == "" {
		return errors.New("minio bucket required when enabled")
	}
	if c.MetadataBucket == "" {
		return errors.New("minio metadata_bucket required when enabled")
	}
	if c.UploadTimeout <= 0 {
		return errors.New("minio upload_timeout must be greater than 0")
	}
	if c.MaxRetries < 0 {
		return errors.New("minio max_retries must be non-negative")
	}

	return nil
}
