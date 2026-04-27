// Package config loads and validates the signal-producer configuration.
//
// Loading uses infrastructure/config (YAML + environment-variable overrides);
// validation enforces the rules from data-model.md so a misconfigured run
// fails fast — before any network call to ES or Waaseyaa.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	infraconfig "github.com/jonesrussell/north-cloud/infrastructure/config"
)

// Bounds for the validation rules in data-model.md.
const (
	minBatchSize          = 1
	maxBatchSize          = 500
	minQualityScoreFloor  = 0
	maxQualityScoreCeil   = 100
	defaultBatchSize      = 50
	defaultMinQuality     = 40
	defaultLookbackBuffer = 5 * time.Minute
	defaultCheckpointFile = "/var/lib/signal-producer/checkpoint.json"
	defaultIndex          = "*_classified_content"
)

// Config is the on-disk shape loaded by infrastructure/config. The `env` tags
// are the override surface (env always wins). YAML keys mirror them.
type Config struct {
	Waaseyaa      WaaseyaaConfig      `yaml:"waaseyaa"`
	Elasticsearch ElasticsearchConfig `yaml:"elasticsearch"`
	Schedule      ScheduleConfig      `yaml:"schedule"`
	Checkpoint    CheckpointConfig    `yaml:"checkpoint"`
}

// WaaseyaaConfig groups receiver-side knobs.
type WaaseyaaConfig struct {
	URL             string `yaml:"url"               env:"WAASEYAA_URL"`
	APIKey          string `yaml:"api_key"           env:"WAASEYAA_API_KEY"`
	BatchSize       int    `yaml:"batch_size"        env:"WAASEYAA_BATCH_SIZE"`
	MinQualityScore int    `yaml:"min_quality_score" env:"WAASEYAA_MIN_QUALITY_SCORE"`
}

// ElasticsearchConfig groups source-side knobs.
type ElasticsearchConfig struct {
	URL     string   `yaml:"url"     env:"ES_URL"`
	Indexes []string `yaml:"indexes" env:"ES_INDEXES"`
}

// ScheduleConfig groups timing knobs.
type ScheduleConfig struct {
	LookbackBuffer time.Duration `yaml:"lookback_buffer" env:"SCHEDULE_LOOKBACK_BUFFER"`
}

// CheckpointConfig groups persistence knobs.
type CheckpointConfig struct {
	File string `yaml:"file" env:"CHECKPOINT_FILE"`
}

// Load reads the YAML config at path, applies env overrides, fills defaults,
// and validates. The returned Config is safe to hand to the producer.
//
// If path is empty, only env overrides + defaults are applied (no YAML).
func Load(path string) (*Config, error) {
	cfg := &Config{}
	if path != "" {
		loaded, err := infraconfig.Load[Config](path)
		if err != nil {
			return nil, fmt.Errorf("config: load %s: %w", path, err)
		}
		cfg = loaded
	} else {
		// No YAML — apply env overrides directly to the empty struct.
		if err := infraconfig.ApplyEnvOverrides(cfg); err != nil {
			return nil, fmt.Errorf("config: apply env overrides: %w", err)
		}
	}

	applyDefaults(cfg)

	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// applyDefaults fills empty fields with the documented defaults from
// config.yml.example so an operator can run with only the secrets set.
func applyDefaults(cfg *Config) {
	if cfg.Waaseyaa.BatchSize == 0 {
		cfg.Waaseyaa.BatchSize = defaultBatchSize
	}
	if cfg.Waaseyaa.MinQualityScore == 0 {
		cfg.Waaseyaa.MinQualityScore = defaultMinQuality
	}
	if cfg.Schedule.LookbackBuffer == 0 {
		cfg.Schedule.LookbackBuffer = defaultLookbackBuffer
	}
	if cfg.Checkpoint.File == "" {
		cfg.Checkpoint.File = defaultCheckpointFile
	}
	if len(cfg.Elasticsearch.Indexes) == 0 {
		cfg.Elasticsearch.Indexes = []string{defaultIndex}
	}
}

// Validate enforces the data-model.md rules. Returns the first error
// encountered so the operator sees a single clear message.
func Validate(cfg *Config) error {
	if err := validateWaaseyaa(cfg.Waaseyaa); err != nil {
		return err
	}
	if err := validateElasticsearch(cfg.Elasticsearch); err != nil {
		return err
	}
	if err := validateSchedule(cfg.Schedule); err != nil {
		return err
	}
	return validateCheckpoint(cfg.Checkpoint)
}

func validateWaaseyaa(w WaaseyaaConfig) error {
	if w.URL == "" {
		return errors.New("config: waaseyaa.url is required")
	}
	parsed, err := url.Parse(w.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("config: waaseyaa.url is not a valid URL: %q", w.URL)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("config: waaseyaa.url scheme must be http or https: %q", parsed.Scheme)
	}
	if w.APIKey == "" {
		return errors.New("config: waaseyaa.api_key is required")
	}
	if w.BatchSize < minBatchSize || w.BatchSize > maxBatchSize {
		return fmt.Errorf("config: waaseyaa.batch_size must be in [%d,%d], got %d",
			minBatchSize, maxBatchSize, w.BatchSize)
	}
	if w.MinQualityScore < minQualityScoreFloor || w.MinQualityScore > maxQualityScoreCeil {
		return fmt.Errorf("config: waaseyaa.min_quality_score must be in [%d,%d], got %d",
			minQualityScoreFloor, maxQualityScoreCeil, w.MinQualityScore)
	}
	return nil
}

func validateElasticsearch(e ElasticsearchConfig) error {
	if e.URL == "" {
		return errors.New("config: elasticsearch.url is required")
	}
	parsed, err := url.Parse(e.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("config: elasticsearch.url is not a valid URL: %q", e.URL)
	}
	if len(e.Indexes) == 0 {
		return errors.New("config: elasticsearch.indexes must not be empty")
	}
	return nil
}

func validateSchedule(s ScheduleConfig) error {
	if s.LookbackBuffer <= 0 {
		return fmt.Errorf("config: schedule.lookback_buffer must be > 0, got %s", s.LookbackBuffer)
	}
	return nil
}

func validateCheckpoint(c CheckpointConfig) error {
	if c.File == "" {
		return errors.New("config: checkpoint.file is required")
	}
	dir := filepath.Dir(c.File)
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("config: checkpoint.file parent dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("config: checkpoint.file parent %q is not a directory", dir)
	}
	return nil
}
