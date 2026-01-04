// Package sources manages the configuration and lifecycle of web content sources for GoCrawl.
// It handles source configuration loading and validation through a YAML-based configuration system.
package sources

import (
	"context"
	"sync"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
	storagetypes "github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// Interface defines the read-only interface for accessing sources.
type Interface interface {
	// ValidateSource validates a source configuration and returns the validated source.
	ValidateSource(
		ctx context.Context,
		sourceName string,
		indexManager storagetypes.IndexManager,
	) (*configtypes.Source, error)
	// ValidateSourceByID validates a source configuration by ID and returns the validated source.
	// This is more efficient than ValidateSource as it can fetch a single source from the API.
	ValidateSourceByID(
		ctx context.Context,
		sourceID string,
		indexManager storagetypes.IndexManager,
	) (*configtypes.Source, error)
	// GetSources retrieves all source configurations.
	GetSources() ([]Config, error)
}

// Config represents a source configuration.
type Config = types.SourceConfig

// SelectorConfig defines the CSS selectors used for content extraction.
type SelectorConfig = types.SelectorConfig

// ArticleSelectors defines the CSS selectors used for article content extraction.
type ArticleSelectors = types.ArticleSelectors

// Sources manages a collection of web content sources.
type Sources struct {
	sources  []Config
	logger   logger.Interface
	metrics  *types.SourcesMetrics
	apiURL   string       // Store API URL for reloading sources
	mu       sync.RWMutex // Mutex for thread-safe source updates
	loadOnce sync.Once    // Ensures sources are only loaded once (lazy loading)
}

// Ensure Sources implements Interface
var _ Interface = (*Sources)(nil)

// GetSources returns all sources.
func (s *Sources) GetSources() ([]Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make([]Config, len(s.sources))
	copy(sources, s.sources)
	return sources, nil
}
