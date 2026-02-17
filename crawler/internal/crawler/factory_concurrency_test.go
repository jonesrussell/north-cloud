package crawler_test

import (
	"sync"
	"testing"

	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const concurrentJobCount = 10

// TestFactory_ConcurrentJobs_NoInterference creates 10 crawler instances
// concurrently from the same factory and verifies no race conditions.
// Run with: go test -race ./crawler/internal/crawler/...
func TestFactory_ConcurrentJobs_NoInterference(t *testing.T) {
	log := infralogger.NewNop()
	bus := events.NewEventBus(log)

	f := crawler.NewFactory(crawler.CrawlerParams{
		Logger: log,
		Bus:    bus,
		Config: &crawlerconfig.Config{},
	})

	var wg sync.WaitGroup
	errs := make(chan error, concurrentJobCount)
	instances := make(chan crawler.Interface, concurrentJobCount)

	for range concurrentJobCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inst, err := f.Create()
			if err != nil {
				errs <- err
				return
			}
			instances <- inst
		}()
	}

	wg.Wait()
	close(errs)
	close(instances)

	for err := range errs {
		t.Fatalf("concurrent Create() error: %v", err)
	}

	// Collect all instances and verify they're distinct
	seen := make(map[crawler.Interface]struct{})
	for inst := range instances {
		if _, exists := seen[inst]; exists {
			t.Fatal("factory returned duplicate instance pointer")
		}
		seen[inst] = struct{}{}
	}

	if len(seen) != concurrentJobCount {
		t.Fatalf("expected %d unique instances, got %d", concurrentJobCount, len(seen))
	}
}
