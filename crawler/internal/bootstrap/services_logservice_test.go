package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	logsconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/logs"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestSetupLogService_DefaultConfig(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{},
	}

	broker := bootstrap.SetupSSEForTest(deps)
	defer func() {
		_ = broker.Stop()
	}()

	result := bootstrap.SetupLogServiceForTest(deps, broker)
	if result.Service == nil {
		t.Fatal("expected non-nil log service")
	}
	if result.RedisWriter != nil {
		t.Error("expected nil Redis writer when Redis is not enabled")
	}
}

func TestSetupLogService_RedisEnabled_NoConnection(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigWithLogs{
		mockConfig: mockConfig{
			redisConfig: &config.RedisConfig{
				Enabled: true,
				Address: "localhost:59999", // no Redis here
			},
		},
		logsConfig: &logsconfig.Config{
			Enabled:        true,
			BufferSize:     100,
			RedisEnabled:   true,
			RedisKeyPrefix: "test:",
		},
	}

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: cfg,
	}

	broker := bootstrap.SetupSSEForTest(deps)
	defer func() {
		_ = broker.Stop()
	}()

	result := bootstrap.SetupLogServiceForTest(deps, broker)
	if result.Service == nil {
		t.Fatal("expected non-nil log service even when Redis fails")
	}
}

func TestCreateFeedPoller_Disabled(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{
			feedConfig: &config.FeedConfig{
				Enabled: false,
			},
		},
	}

	if !bootstrap.CreateFeedPollerIsNilForTest(deps) {
		t.Error("expected nil feed poller when disabled")
	}
}

func TestCreateFeedDiscoverer_Disabled(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{
			feedConfig: &config.FeedConfig{
				DiscoveryEnabled: false,
			},
		},
	}

	if !bootstrap.CreateFeedDiscovererIsNilForTest(deps) {
		t.Error("expected nil feed discoverer when disabled")
	}
}

func TestCreateFrontierWorkerPool_Disabled(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{},
	}

	if !bootstrap.CreateFrontierWorkerPoolIsNilForTest(deps) {
		t.Error("expected nil worker pool when fetcher is disabled")
	}
}

func TestCreateFeedPoller_EnabledButNilFrontier(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{
			feedConfig: &config.FeedConfig{
				Enabled: true,
			},
		},
	}

	// Enabled but nil frontier submitter -> nil poller
	if !bootstrap.CreateFeedPollerIsNilForTest(deps) {
		t.Error("expected nil feed poller when frontier submitter is nil")
	}
}

func TestSetupLogService_WithSSEAndArchive(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigWithLogs{
		mockConfig: mockConfig{},
		logsConfig: &logsconfig.Config{
			Enabled:        true,
			BufferSize:     200,
			SSEEnabled:     true,
			ArchiveEnabled: true,
			RetentionDays:  30,
			MinLevel:       "info",
			MinioBucket:    "test-logs",
			RedisEnabled:   false,
		},
	}

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: cfg,
	}

	broker := bootstrap.SetupSSEForTest(deps)
	defer func() {
		_ = broker.Stop()
	}()

	result := bootstrap.SetupLogServiceForTest(deps, broker)
	if result.Service == nil {
		t.Fatal("expected non-nil log service")
	}
}

func TestSetupLogService_RedisDisabledConfig(t *testing.T) {
	t.Parallel()

	cfg := &mockConfigWithLogs{
		mockConfig: mockConfig{
			redisConfig: &config.RedisConfig{
				Enabled: false,
			},
		},
		logsConfig: &logsconfig.Config{
			Enabled:      true,
			RedisEnabled: true, // enabled in logs but Redis itself disabled
		},
	}

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: cfg,
	}

	broker := bootstrap.SetupSSEForTest(deps)
	defer func() {
		_ = broker.Stop()
	}()

	result := bootstrap.SetupLogServiceForTest(deps, broker)
	if result.Service == nil {
		t.Fatal("expected non-nil log service")
	}
	// Redis writer should be nil since Redis config returns disabled
	if result.RedisWriter != nil {
		t.Error("expected nil Redis writer when Redis config is disabled")
	}
}

// mockConfigWithLogs allows overriding the logs config.
type mockConfigWithLogs struct {
	mockConfig
	logsConfig *logsconfig.Config
}

func (m *mockConfigWithLogs) GetLogsConfig() *logsconfig.Config {
	if m.logsConfig != nil {
		return m.logsConfig
	}
	return &logsconfig.Config{}
}
