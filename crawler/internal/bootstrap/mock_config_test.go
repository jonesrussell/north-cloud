package bootstrap_test

import (
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	dbconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/elasticsearch"
	fetcherconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/fetcher"
	logsconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/logs"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/minio"
	"github.com/jonesrussell/north-cloud/crawler/internal/config/server"
)

// mockConfig implements config.Interface for testing.
type mockConfig struct {
	loggingConfig       *config.LoggingConfig
	redisConfig         *config.RedisConfig
	feedConfig          *config.FeedConfig
	fetcherConfig       *fetcherconfig.Config
	schedulerConfig     *config.SchedulerConfig
	sourceManagerConfig *config.SourceManagerConfig
}

func (m *mockConfig) GetServerConfig() *server.Config {
	return &server.Config{Address: ":8080"}
}

func (m *mockConfig) GetCrawlerConfig() *crawlerconfig.Config {
	return &crawlerconfig.Config{}
}

func (m *mockConfig) GetElasticsearchConfig() *elasticsearch.Config {
	return &elasticsearch.Config{}
}

func (m *mockConfig) GetDatabaseConfig() *dbconfig.Config {
	return &dbconfig.Config{}
}

func (m *mockConfig) GetMinIOConfig() *minio.Config {
	return &minio.Config{}
}

func (m *mockConfig) GetLogsConfig() *logsconfig.Config {
	return &logsconfig.Config{}
}

func (m *mockConfig) GetAuthConfig() *config.AuthConfig {
	return &config.AuthConfig{}
}

func (m *mockConfig) GetLoggingConfig() *config.LoggingConfig {
	if m.loggingConfig != nil {
		return m.loggingConfig
	}
	return &config.LoggingConfig{Level: "info", Env: "production"}
}

func (m *mockConfig) GetRedisConfig() *config.RedisConfig {
	if m.redisConfig != nil {
		return m.redisConfig
	}
	return &config.RedisConfig{}
}

func (m *mockConfig) GetSourceManagerConfig() *config.SourceManagerConfig {
	if m.sourceManagerConfig != nil {
		return m.sourceManagerConfig
	}
	return &config.SourceManagerConfig{URL: "http://localhost:8050"}
}

func (m *mockConfig) GetFeedConfig() *config.FeedConfig {
	if m.feedConfig != nil {
		return m.feedConfig
	}
	return &config.FeedConfig{}
}

func (m *mockConfig) GetDiscoveryConfig() *config.DiscoveryConfig {
	return nil
}

func (m *mockConfig) GetFetcherConfig() *fetcherconfig.Config {
	if m.fetcherConfig != nil {
		return m.fetcherConfig
	}
	return &fetcherconfig.Config{}
}

func (m *mockConfig) GetSchedulerConfig() *config.SchedulerConfig {
	if m.schedulerConfig != nil {
		return m.schedulerConfig
	}
	return &config.SchedulerConfig{}
}

func (m *mockConfig) GetPipelineURL() string {
	return ""
}

func (m *mockConfig) Validate() error {
	return nil
}
