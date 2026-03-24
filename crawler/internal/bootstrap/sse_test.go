package bootstrap_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/bootstrap"
	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

func TestSetupSSE(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{},
	}

	broker := bootstrap.SetupSSEForTest(deps)
	if broker == nil {
		t.Fatal("expected non-nil SSE broker")
	}

	// Clean up
	if err := broker.Stop(); err != nil {
		t.Errorf("failed to stop broker: %v", err)
	}
}

func TestSetupSSE_Full(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{},
	}

	result := bootstrap.SetupSSEFullForTest(deps)
	if result.Broker == nil {
		t.Fatal("expected non-nil SSE broker")
	}
	if result.Handler == nil {
		t.Fatal("expected non-nil SSE handler")
	}
	if result.Publisher == nil {
		t.Fatal("expected non-nil SSE publisher")
	}

	// Clean up
	if err := result.Broker.Stop(); err != nil {
		t.Errorf("failed to stop broker: %v", err)
	}
}

func TestStartBackgroundWorkers_AllNilComponents(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{},
	}

	sc := &bootstrap.ServiceComponents{}

	// Should not panic with all nil service components
	bg := bootstrap.StartBackgroundWorkersForTest(deps, sc)
	_ = bg
}

func TestSetupMigrator(t *testing.T) {
	t.Parallel()

	deps := &bootstrap.CommandDeps{
		Logger: infralogger.NewNop(),
		Config: &mockConfig{
			sourceManagerConfig: &config.SourceManagerConfig{
				URL: "http://localhost:8050",
			},
		},
	}

	migrator := bootstrap.SetupMigratorForTest(deps)
	if migrator == nil {
		t.Fatal("expected non-nil migrator")
	}
}
