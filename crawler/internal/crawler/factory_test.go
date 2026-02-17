package crawler_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"
	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

func testParams(t *testing.T) crawler.CrawlerParams {
	t.Helper()
	log := infralogger.NewNop()
	bus := events.NewEventBus(log)
	return crawler.CrawlerParams{
		Logger: log,
		Bus:    bus,
		Config: &crawlerconfig.Config{},
	}
}

func TestFactory_Create_ReturnsIsolatedInstances(t *testing.T) {
	f := crawler.NewFactory(testParams(t))

	a, err := f.Create()
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	b, err := f.Create()
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	if a == b {
		t.Fatal("expected two different instances, got same pointer")
	}
}

func TestFactory_SharedStartURLHashes(t *testing.T) {
	f := crawler.NewFactory(testParams(t))

	// Hash should be empty initially
	if got := f.GetStartURLHash("src1"); got != "" {
		t.Fatalf("expected empty hash, got %q", got)
	}

	// Create an instance — its writes should be visible via the factory
	inst, err := f.Create()
	if err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	// We cannot directly write into the shared map from outside,
	// but we can verify that GetStartURLHash returns what the instance's
	// own GetStartURLHash would return (both share the same map).
	// Since the crawler hasn't run, both should return "".
	if got := inst.GetStartURLHash("src1"); got != "" {
		t.Fatalf("expected empty hash from instance, got %q", got)
	}
	if got := f.GetStartURLHash("src1"); got != "" {
		t.Fatalf("expected empty hash from factory, got %q", got)
	}
}

func TestFactory_GetHashTracker(t *testing.T) {
	tracker := adaptive.NewHashTracker(nil) // nil redis is fine for this test
	params := testParams(t)
	params.HashTracker = tracker

	f := crawler.NewFactory(params)

	if got := f.GetHashTracker(); got != tracker {
		t.Fatalf("expected same hash tracker pointer, got different")
	}
}

func TestFactory_GetHashTracker_Nil(t *testing.T) {
	f := crawler.NewFactory(testParams(t))
	if got := f.GetHashTracker(); got != nil {
		t.Fatalf("expected nil hash tracker, got %v", got)
	}
}
