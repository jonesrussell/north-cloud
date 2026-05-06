package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/config"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeYAML writes content to a temp file and returns its path.
func writeYAML(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "config-*.yml")
	require.NoError(t, err)

	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}

// TestLoadEmptyYAMLPopulatesDefaults verifies that an empty (or comment-only)
// YAML file causes SetDefaults to populate all expected fields.
func TestLoadEmptyYAMLPopulatesDefaults(t *testing.T) {
	path := writeYAML(t, "# empty config\n")

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "alert-crawler", cfg.Service.Name, "service.name default")
	assert.Equal(t, "/app/data/state.db", cfg.Database.Path, "database.path default")
	assert.Equal(t, "http://elasticsearch:9200", cfg.Elasticsearch.URL, "elasticsearch.url default")
	assert.Equal(t, "community_alerts", cfg.Elasticsearch.Index, "elasticsearch.index default")
	assert.Equal(t, "redis://redis:6379", cfg.Redis.URL, "redis.url default")
	assert.Equal(t, "community_alerts:lifecycle", cfg.Redis.Channel, "redis.channel default")
	assert.Equal(t, "info", cfg.Observability.LogLevel, "observability.log_level default")
	assert.Equal(t, "json", cfg.Observability.LogFormat, "observability.log_format default")

	// Default source: mhrn
	require.Len(t, cfg.Sources, 1, "expected exactly one default source")
	src := cfg.Sources[0]
	assert.Equal(t, "mhrn", src.ID)
	assert.Equal(t, domain.AcquisitionRSS, src.AcquisitionStrategy)
	assert.Equal(t, 30*time.Minute, src.PollInterval)
	assert.Equal(t, 720*time.Hour, src.DefaultExpiry)
	assert.Equal(t, []string{"treaty:1", "canada:manitoba"}, src.DefaultScope)
	assert.True(t, src.Enabled)

	// Default severity table
	require.NotEmpty(t, cfg.Severity.Table)
	assert.Equal(t, domain.SeverityCritical, cfg.Severity.Table["carfentanil"])
	assert.Equal(t, domain.SeverityHigh, cfg.Severity.Table["fentanyl"])
	assert.Equal(t, domain.SeverityHigh, cfg.Severity.Table["xylazine"])
	assert.Equal(t, domain.SeverityHigh, cfg.Severity.Table["benzodiazepine"])
	assert.Equal(t, domain.SeverityHigh, cfg.Severity.Table["nitazenes"])
	assert.Equal(t, domain.SeverityHigh, cfg.Severity.Table["medetomidine"])
}

// TestLoadYAMLOverridesDefaults checks that non-empty YAML values replace
// SetDefaults values.
func TestLoadYAMLOverridesDefaults(t *testing.T) {
	yaml := `
service:
  name: custom-name
database:
  path: /tmp/custom.db
elasticsearch:
  url: http://myES:9201
  index: my_alerts
redis:
  url: redis://myredis:6380
  channel: my_channel
observability:
  log_level: debug
  log_format: console
`
	path := writeYAML(t, yaml)

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "custom-name", cfg.Service.Name)
	assert.Equal(t, "/tmp/custom.db", cfg.Database.Path)
	assert.Equal(t, "http://myES:9201", cfg.Elasticsearch.URL)
	assert.Equal(t, "my_alerts", cfg.Elasticsearch.Index)
	assert.Equal(t, "redis://myredis:6380", cfg.Redis.URL)
	assert.Equal(t, "my_channel", cfg.Redis.Channel)
	assert.Equal(t, "debug", cfg.Observability.LogLevel)
	assert.Equal(t, "console", cfg.Observability.LogFormat)
}

// TestLoadEnvOverridesYAML verifies that env vars win over YAML values.
func TestLoadEnvOverridesYAML(t *testing.T) {
	yaml := `
elasticsearch:
  url: http://yaml-es:9200
  index: yaml_index
redis:
  url: redis://yaml-redis:6379
  channel: yaml_channel
observability:
  log_level: warn
`
	path := writeYAML(t, yaml)

	t.Setenv("ELASTICSEARCH_URL", "http://env-es:9200")
	t.Setenv("ALERT_ES_INDEX", "env_index")
	t.Setenv("REDIS_URL", "redis://env-redis:6379")
	t.Setenv("ALERT_REDIS_CHANNEL", "env_channel")
	t.Setenv("LOG_LEVEL", "error")

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "http://env-es:9200", cfg.Elasticsearch.URL, "env wins over yaml for ES URL")
	assert.Equal(t, "env_index", cfg.Elasticsearch.Index, "env wins over yaml for ES index")
	assert.Equal(t, "redis://env-redis:6379", cfg.Redis.URL, "env wins over yaml for Redis URL")
	assert.Equal(t, "env_channel", cfg.Redis.Channel, "env wins over yaml for Redis channel")
	assert.Equal(t, "error", cfg.Observability.LogLevel, "env wins over yaml for log level")
}

// TestLoadEnvOverridesDefaults verifies that env vars win over SetDefaults.
func TestLoadEnvOverridesDefaults(t *testing.T) {
	path := writeYAML(t, "# empty\n")

	t.Setenv("SERVICE_NAME", "env-service")
	t.Setenv("ALERT_DB_PATH", "/env/data.db")

	cfg, err := config.Load(path)
	require.NoError(t, err)

	assert.Equal(t, "env-service", cfg.Service.Name, "env wins over SetDefaults for service.name")
	assert.Equal(t, "/env/data.db", cfg.Database.Path, "env wins over SetDefaults for db.path")
}

// TestLoadMissingFile returns an error for a non-existent config path.
func TestLoadMissingFile(t *testing.T) {
	_, err := config.Load(filepath.Join(t.TempDir(), "nonexistent.yml"))
	assert.Error(t, err)
}
