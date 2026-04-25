package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jonesrussell/north-cloud/signal-crawler/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_MissingNorthOpsURL(t *testing.T) {
	cfg := &config.Config{
		NorthOps: config.NorthOpsConfig{
			URL:    "",
			APIKey: "test-key",
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "northops_url")
}

func TestValidate_MissingAPIKey(t *testing.T) {
	cfg := &config.Config{
		NorthOps: config.NorthOpsConfig{
			URL:    "https://northops.example.com",
			APIKey: "",
		},
	}
	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "api_key")
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &config.Config{
		NorthOps: config.NorthOpsConfig{
			URL:    "https://northops.example.com",
			APIKey: "test-key",
		},
		Dedup: config.DedupConfig{DBPath: "data/seen.db"},
	}
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestDefaults(t *testing.T) {
	cfg := &config.Config{}
	config.SetDefaults(cfg)

	assert.Equal(t, "data/seen.db", cfg.Dedup.DBPath)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.False(t, cfg.Jobs.GCJobsDisabled)
}

func TestConfig_Validate_EmptyDBPath(t *testing.T) {
	cfg := &config.Config{
		NorthOps: config.NorthOpsConfig{URL: "https://northops.ca", APIKey: "key"},
		Dedup:    config.DedupConfig{DBPath: ""},
	}
	config.SetDefaults(cfg)
	err := cfg.Validate()
	require.NoError(t, err)

	cfg.Dedup.DBPath = ""
	err = cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db_path")
}

func TestLoad_JobsGCJobsDisabledEnvOverride(t *testing.T) {
	t.Setenv("JOBS_GCJOBS_DISABLED", "true")

	path := filepath.Join(t.TempDir(), "config.yml")
	err := os.WriteFile(path, []byte(`northops:
  url: ""
  api_key: ""
dedup:
  db_path: ""
jobs:
  gcjobs_disabled: false
`), 0o644)
	require.NoError(t, err)

	cfg, err := config.Load(path)
	require.NoError(t, err)
	assert.True(t, cfg.Jobs.GCJobsDisabled)
}
