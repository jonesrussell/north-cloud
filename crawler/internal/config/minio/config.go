// Package minio provides MinIO configuration for HTML archiving.
package minio

import (
	"errors"
	"time"
)

// Config represents MinIO configuration for HTML archiving.
type Config struct {
	// Enabled toggles HTML archiving on/off
	Enabled bool `env:"CRAWLER_MINIO_ENABLED" yaml:"enabled"`
	// Endpoint is the MinIO server address (e.g., "minio:9000")
	Endpoint string `env:"CRAWLER_MINIO_ENDPOINT" yaml:"endpoint"`
	// AccessKey for MinIO authentication
	AccessKey string `env:"CRAWLER_MINIO_ACCESS_KEY" json:"-" yaml:"access_key"`
	// SecretKey for MinIO authentication
	SecretKey string `env:"CRAWLER_MINIO_SECRET_KEY" json:"-" yaml:"secret_key"`
	// UseSSL enables HTTPS for MinIO connections
	UseSSL bool `env:"CRAWLER_MINIO_USE_SSL" yaml:"use_ssl"`
	// Bucket is the main bucket for HTML archives
	Bucket string `env:"CRAWLER_MINIO_BUCKET" yaml:"bucket"`
	// MetadataBucket is the bucket for metadata JSON files
	MetadataBucket string `env:"CRAWLER_MINIO_METADATA_BUCKET" yaml:"metadata_bucket"`
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
