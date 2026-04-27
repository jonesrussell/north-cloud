package config

import (
	"strings"
	"testing"
	"time"
)

// validConfig returns a Config that passes Validate. Tests mutate one field at
// a time to drive each error branch.
func validConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	return &Config{
		Waaseyaa: WaaseyaaConfig{
			URL:             "https://waaseyaa.example.com",
			APIKey:          "test-key",
			BatchSize:       50,
			MinQualityScore: 40,
		},
		Elasticsearch: ElasticsearchConfig{
			URL:     "http://localhost:9200",
			Indexes: []string{"*_classified_content"},
		},
		Schedule: ScheduleConfig{
			LookbackBuffer: 5 * time.Minute,
		},
		Checkpoint: CheckpointConfig{
			File: dir + "/checkpoint.json",
		},
	}
}

func TestValidate_Happy(t *testing.T) {
	cfg := validConfig(t)
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidate_Errors(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{
			name:   "waaseyaa url missing",
			mutate: func(c *Config) { c.Waaseyaa.URL = "" },
			want:   "waaseyaa.url is required",
		},
		{
			name:   "waaseyaa url malformed",
			mutate: func(c *Config) { c.Waaseyaa.URL = "::nope" },
			want:   "is not a valid URL",
		},
		{
			name:   "waaseyaa url wrong scheme",
			mutate: func(c *Config) { c.Waaseyaa.URL = "ftp://x.example.com" },
			want:   "scheme must be http or https",
		},
		{
			name:   "waaseyaa api key missing",
			mutate: func(c *Config) { c.Waaseyaa.APIKey = "" },
			want:   "waaseyaa.api_key is required",
		},
		{
			name:   "batch size below floor",
			mutate: func(c *Config) { c.Waaseyaa.BatchSize = 0 },
			want:   "batch_size must be in",
		},
		{
			name:   "batch size above ceil",
			mutate: func(c *Config) { c.Waaseyaa.BatchSize = 501 },
			want:   "batch_size must be in",
		},
		{
			name:   "quality score above ceil",
			mutate: func(c *Config) { c.Waaseyaa.MinQualityScore = 101 },
			want:   "min_quality_score must be in",
		},
		{
			name:   "es url missing",
			mutate: func(c *Config) { c.Elasticsearch.URL = "" },
			want:   "elasticsearch.url is required",
		},
		{
			name:   "es indexes empty",
			mutate: func(c *Config) { c.Elasticsearch.Indexes = nil },
			want:   "elasticsearch.indexes must not be empty",
		},
		{
			name:   "lookback buffer zero",
			mutate: func(c *Config) { c.Schedule.LookbackBuffer = 0 },
			want:   "lookback_buffer must be > 0",
		},
		{
			name:   "checkpoint file missing",
			mutate: func(c *Config) { c.Checkpoint.File = "" },
			want:   "checkpoint.file is required",
		},
		{
			name:   "checkpoint dir does not exist",
			mutate: func(c *Config) { c.Checkpoint.File = "/nonexistent/dir/checkpoint.json" },
			want:   "checkpoint.file parent dir",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validConfig(t)
			tc.mutate(cfg)
			err := Validate(cfg)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got %q", tc.want, err.Error())
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)
	if cfg.Waaseyaa.BatchSize != defaultBatchSize {
		t.Errorf("BatchSize = %d, want %d", cfg.Waaseyaa.BatchSize, defaultBatchSize)
	}
	if cfg.Waaseyaa.MinQualityScore != defaultMinQuality {
		t.Errorf("MinQualityScore = %d, want %d", cfg.Waaseyaa.MinQualityScore, defaultMinQuality)
	}
	if cfg.Schedule.LookbackBuffer != defaultLookbackBuffer {
		t.Errorf("LookbackBuffer = %s, want %s", cfg.Schedule.LookbackBuffer, defaultLookbackBuffer)
	}
	if cfg.Checkpoint.File != defaultCheckpointFile {
		t.Errorf("Checkpoint.File = %q, want %q", cfg.Checkpoint.File, defaultCheckpointFile)
	}
	if len(cfg.Elasticsearch.Indexes) != 1 || cfg.Elasticsearch.Indexes[0] != defaultIndex {
		t.Errorf("Indexes = %v, want [%s]", cfg.Elasticsearch.Indexes, defaultIndex)
	}
}
