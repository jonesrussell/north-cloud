package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetDefaults_ServerAddress(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	assert.Equal(t, DefaultServerAddress, cfg.Server.Address)
}

func TestSetDefaults_ServerTimeouts(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	assert.Equal(t, DefaultReadTimeoutSeconds*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeoutSeconds*time.Second, cfg.Server.WriteTimeout)
}

func TestSetDefaults_ServiceDefaults(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	assert.Equal(t, 5*time.Minute, cfg.Service.CheckInterval)
	assert.Equal(t, 100, cfg.Service.BatchSize)
}

func TestSetDefaults_SourcesTimeout(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	assert.Equal(t, 5*time.Second, cfg.Sources.Timeout)
}

func TestSetDefaults_DatabaseDefaults(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	assert.Equal(t, "localhost", cfg.Database.Host)
	assert.Equal(t, "5432", cfg.Database.Port)
	assert.Equal(t, "postgres", cfg.Database.User)
	assert.Equal(t, "publisher", cfg.Database.DBName)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
}

func TestSetDefaults_ClassifiedContentSuffix(t *testing.T) {
	t.Helper()

	cfg := &Config{Service: ServiceConfig{UseClassifiedContent: true}}
	SetDefaults(cfg)

	assert.Equal(t, "_classified_content", cfg.Service.IndexSuffix)
}

func TestSetDefaults_ArticlesSuffix(t *testing.T) {
	t.Helper()

	cfg := &Config{Service: ServiceConfig{UseClassifiedContent: false}}
	SetDefaults(cfg)

	assert.Equal(t, "_articles", cfg.Service.IndexSuffix)
}

func TestSetDefaults_MinQualityScore(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	assert.Equal(t, 50, cfg.Service.MinQualityScore)
}

func TestSetDefaults_CORSOrigins(t *testing.T) {
	t.Helper()

	cfg := &Config{}
	SetDefaults(cfg)

	require.Len(t, cfg.Server.CORSOrigins, 3)
	assert.Contains(t, cfg.Server.CORSOrigins, "http://localhost:3000")
	assert.Contains(t, cfg.Server.CORSOrigins, "http://localhost:3001")
	assert.Contains(t, cfg.Server.CORSOrigins, "http://localhost:3002")
}

func TestSetDefaults_DoesNotOverrideExistingValues(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Server: ServerConfig{
			Address:      ":9090",
			ReadTimeout:  20 * time.Second,
			WriteTimeout: 60 * time.Second,
			CORSOrigins:  []string{"http://custom.example.com"},
		},
		Service: ServiceConfig{
			CheckInterval:   10 * time.Minute,
			BatchSize:       200,
			MinQualityScore: 75,
			IndexSuffix:     "_custom",
		},
		Database: DatabaseConfig{
			Host:    "db.example.com",
			Port:    "5433",
			User:    "admin",
			DBName:  "custom_db",
			SSLMode: "require",
		},
		Sources: SourcesConfig{
			Timeout: 10 * time.Second,
		},
	}

	SetDefaults(cfg)

	assert.Equal(t, ":9090", cfg.Server.Address)
	assert.Equal(t, 20*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 60*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, []string{"http://custom.example.com"}, cfg.Server.CORSOrigins)
	assert.Equal(t, 10*time.Minute, cfg.Service.CheckInterval)
	assert.Equal(t, 200, cfg.Service.BatchSize)
	assert.Equal(t, 75, cfg.Service.MinQualityScore)
	assert.Equal(t, "_custom", cfg.Service.IndexSuffix)
	assert.Equal(t, "db.example.com", cfg.Database.Host)
	assert.Equal(t, "5433", cfg.Database.Port)
	assert.Equal(t, "admin", cfg.Database.User)
	assert.Equal(t, "custom_db", cfg.Database.DBName)
	assert.Equal(t, "require", cfg.Database.SSLMode)
	assert.Equal(t, 10*time.Second, cfg.Sources.Timeout)
}

func TestServerConfig_Validate_DefaultAddress(t *testing.T) {
	t.Helper()

	sc := &ServerConfig{}
	err := sc.Validate()

	require.NoError(t, err)
	assert.Equal(t, DefaultServerAddress, sc.Address)
}

func TestServerConfig_Validate_PrependColon(t *testing.T) {
	t.Helper()

	sc := &ServerConfig{Address: "9090"}
	err := sc.Validate()

	require.NoError(t, err)
	assert.Equal(t, ":9090", sc.Address)
}

func TestServerConfig_Validate_AlreadyHasColon(t *testing.T) {
	t.Helper()

	sc := &ServerConfig{Address: ":8080"}
	err := sc.Validate()

	require.NoError(t, err)
	assert.Equal(t, ":8080", sc.Address)
}

func TestServerConfig_Validate_DefaultTimeouts(t *testing.T) {
	t.Helper()

	sc := &ServerConfig{}
	err := sc.Validate()

	require.NoError(t, err)
	assert.Equal(t, DefaultReadTimeoutSeconds*time.Second, sc.ReadTimeout)
	assert.Equal(t, DefaultWriteTimeoutSeconds*time.Second, sc.WriteTimeout)
}

func TestServerConfig_Validate_PreservesExistingTimeouts(t *testing.T) {
	t.Helper()

	sc := &ServerConfig{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 45 * time.Second,
	}
	err := sc.Validate()

	require.NoError(t, err)
	assert.Equal(t, 15*time.Second, sc.ReadTimeout)
	assert.Equal(t, 45*time.Second, sc.WriteTimeout)
}

func TestConfig_Validate_Valid(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
	}

	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_MissingElasticsearchURL(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Redis:   RedisConfig{URL: "redis://localhost:6379"},
		Service: ServiceConfig{CheckInterval: 5 * time.Minute},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "elasticsearch.url is required")
}

func TestConfig_Validate_MissingRedisURL(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "redis.url is required")
}

func TestConfig_Validate_NonPositiveCheckInterval(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 0},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service.check_interval must be positive")
}

func TestConfig_Validate_SourcesEnabledWithoutURL(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
		Sources:       SourcesConfig{Enabled: true, URL: ""},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "sources.url is required when sources.enabled is true")
}

func TestConfig_Validate_SourcesEnabledWithURL(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
		Sources:       SourcesConfig{Enabled: true, URL: "http://sources:8080"},
	}

	assert.NoError(t, cfg.Validate())
}

func TestConfig_Validate_EmptyCityName(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
		Cities: []CityConfig{
			{Name: "Thunder Bay", Index: "thunder_bay"},
			{Name: "", Index: "empty"},
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cities[1].name is required")
}

func TestConfig_Validate_ValidCities(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
		Cities: []CityConfig{
			{Name: "Thunder Bay", Index: "thunder_bay"},
			{Name: "Ottawa", Index: "ottawa"},
		},
	}

	assert.NoError(t, cfg.Validate())
}

func TestLoadWithSources_SourcesDisabled(t *testing.T) {
	t.Helper()

	cfg := &Config{
		Elasticsearch: ElasticsearchConfig{URL: "http://localhost:9200"},
		Redis:         RedisConfig{URL: "redis://localhost:6379"},
		Service:       ServiceConfig{CheckInterval: 5 * time.Minute},
		Sources:       SourcesConfig{Enabled: false},
		Cities: []CityConfig{
			{Name: "Thunder Bay"},
		},
	}

	// With sources disabled, cities should remain unchanged
	assert.Len(t, cfg.Cities, 1)
	assert.Equal(t, "Thunder Bay", cfg.Cities[0].Name)
}

func TestConstants(t *testing.T) {
	t.Helper()

	assert.Equal(t, 10, DefaultReadTimeoutSeconds)
	assert.Equal(t, 30, DefaultWriteTimeoutSeconds)
	assert.Equal(t, 30, DefaultShutdownTimeoutSeconds)
	assert.Equal(t, ":8070", DefaultServerAddress)
}
