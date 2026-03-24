package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestSetupEventConsumer_RedisDisabled(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		redisConfig: &config.RedisConfig{
			Enabled: false,
		},
		sourceManagerConfig: &config.SourceManagerConfig{
			URL: "http://localhost:8050",
		},
	}

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: cfg,
	}

	consumer := bootstrap.SetupEventConsumerForTest(deps, nil, nil)
	if consumer != nil {
		t.Error("expected nil consumer when Redis is disabled")
	}
}

func TestSetupEventConsumer_RedisNilConfig(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{
		redisConfig: &config.RedisConfig{
			Enabled: true,
			Address: "", // empty address will fail connection
		},
		sourceManagerConfig: &config.SourceManagerConfig{
			URL: "http://localhost:8050",
		},
	}

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: cfg,
	}

	// Redis connection will fail, consumer should be nil
	consumer := bootstrap.SetupEventConsumerForTest(deps, nil, nil)
	if consumer != nil {
		t.Error("expected nil consumer when Redis connection fails")
	}
}

func TestBuildSharedProxyPool_Disabled(t *testing.T) {
	t.Parallel()

	cfg := &mockConfig{}

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: cfg,
	}

	if !bootstrap.BuildSharedProxyPoolIsNilForTest(deps) {
		t.Error("expected nil pool when proxy is disabled")
	}
}
