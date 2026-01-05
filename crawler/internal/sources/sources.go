// Package sources manages the configuration and lifecycle of web content sources for the crawler.
// It handles source configuration loading and validation through API-based configuration.
package sources

import (
	"context"
	"errors"
	"fmt"
	"sync"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/loader"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
)

// Interface defines the read-only interface for accessing sources.
type Interface interface {
	// ValidateSourceByID validates a source configuration by ID and returns the validated source.
	// Fetches the source directly from the API.
	ValidateSourceByID(
		ctx context.Context,
		sourceID string,
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
	sources   []Config
	logger    logger.Interface
	metrics   *types.SourcesMetrics
	apiURL    string       // Store API URL for loading sources
	jwtSecret string       // JWT secret for service-to-service authentication
	mu        sync.RWMutex // Mutex for thread-safe source updates
}

// Ensure Sources implements Interface
var _ Interface = (*Sources)(nil)

// GetSources returns all sources. Loads sources lazily on first call if not already loaded.
func (s *Sources) GetSources() ([]Config, error) {
	// Check if sources need to be loaded
	s.mu.RLock()
	needLoad := len(s.sources) == 0
	s.mu.RUnlock()

	if !needLoad {
		return s.copySources()
	}

	// Load sources from API (double-checked locking pattern)
	if err := s.loadSourcesIfNeeded(); err != nil {
		return nil, err
	}

	return s.copySources()
}

// loadSourcesIfNeeded loads sources from API if they haven't been loaded yet.
// Uses double-checked locking pattern for thread safety.
func (s *Sources) loadSourcesIfNeeded() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check again after acquiring write lock
	if len(s.sources) > 0 {
		return nil
	}

	apiLoader := loader.NewAPILoader(s.apiURL, s.logger)
	configs, err := apiLoader.LoadSources()
	if err != nil {
		return fmt.Errorf("failed to load sources from API: %w", err)
	}
	if len(configs) == 0 {
		return errors.New("no sources found from API")
	}

	// Convert loaded configs to our source type
	sourceConfigs := make([]Config, 0, len(configs))
	for i := range configs {
		sourceConfigs = append(sourceConfigs, convertLoaderConfig(configs[i]))
	}

	s.sources = sourceConfigs
	if s.logger != nil {
		s.logger.Info("Sources loaded from API",
			"count", len(sourceConfigs),
			"url", s.apiURL)
	}
	return nil
}

// copySources returns a copy of the cached sources (must be called with read lock held).
func (s *Sources) copySources() ([]Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make([]Config, len(s.sources))
	copy(sources, s.sources)
	return sources, nil
}
