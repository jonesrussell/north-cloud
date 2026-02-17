package crawler

import (
	"fmt"
	"sync"

	"github.com/jonesrussell/north-cloud/crawler/internal/adaptive"
)

// FactoryInterface creates isolated Crawler instances that share hash state.
// Each call to Create() returns a fresh Crawler with its own collector, state,
// lifecycle, and signals — safe for concurrent use by the scheduler.
type FactoryInterface interface {
	// Create returns a new, isolated Crawler instance.
	Create() (Interface, error)
	// GetStartURLHash returns the hash captured for a specific source's start URL.
	GetStartURLHash(sourceID string) string
	// GetHashTracker returns the shared hash tracker for adaptive scheduling.
	GetHashTracker() *adaptive.HashTracker
}

// Factory implements FactoryInterface.
type Factory struct {
	params CrawlerParams

	// Shared across all instances created by this factory.
	startURLHashes   map[string]string
	startURLHashesMu *sync.RWMutex
}

var _ FactoryInterface = (*Factory)(nil)

// NewFactory creates a Factory that produces isolated Crawler instances.
// All instances share the same CrawlerParams (immutable) and startURLHash state.
func NewFactory(params CrawlerParams) *Factory {
	return &Factory{
		params:           params,
		startURLHashes:   make(map[string]string),
		startURLHashesMu: &sync.RWMutex{},
	}
}

// Create returns a new Crawler instance with its own mutable state
// but sharing the factory's hash map and mutex.
func (f *Factory) Create() (Interface, error) {
	result, err := NewCrawlerWithParams(f.params)
	if err != nil {
		return nil, fmt.Errorf("factory create: %w", err)
	}

	c, ok := result.Crawler.(*Crawler)
	if !ok {
		return nil, fmt.Errorf("factory create: unexpected crawler type %T", result.Crawler)
	}

	// Inject shared hash state so all instances read/write the same map.
	c.startURLHashes = f.startURLHashes
	c.startURLHashesMu = f.startURLHashesMu

	return c, nil
}

// GetStartURLHash returns the hash captured for a specific source's start URL.
func (f *Factory) GetStartURLHash(sourceID string) string {
	f.startURLHashesMu.RLock()
	defer f.startURLHashesMu.RUnlock()
	return f.startURLHashes[sourceID]
}

// GetHashTracker returns the shared hash tracker for adaptive scheduling.
func (f *Factory) GetHashTracker() *adaptive.HashTracker {
	return f.params.HashTracker
}
