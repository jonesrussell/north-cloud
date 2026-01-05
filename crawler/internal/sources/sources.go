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
	sources []Config
	logger  logger.Interface
	metrics *types.SourcesMetrics
	apiURL  string       // Store API URL for loading sources
	mu      sync.RWMutex // Mutex for thread-safe source updates
}

// Ensure Sources implements Interface
var _ Interface = (*Sources)(nil)

// GetSources returns all sources. Loads sources lazily on first call if not already loaded.
func (s *Sources) GetSources() ([]Config, error) {
	// Check if sources need to be loaded
	s.mu.RLock()
	needLoad := len(s.sources) == 0
	s.mu.RUnlock()

	if needLoad {
		// Load sources from API (double-checked locking pattern)
		s.mu.Lock()
		// Check again after acquiring write lock
		if len(s.sources) == 0 {
			newSources, err := loadSourcesFromAPI(s.apiURL, s.logger)
			if err != nil {
				s.mu.Unlock()
				return nil, fmt.Errorf("failed to load sources from API: %w", err)
			}
			if len(newSources) == 0 {
				s.mu.Unlock()
				return nil, errors.New("no sources found from API")
			}
			s.sources = newSources
			if s.logger != nil {
				s.logger.Info("Sources loaded from API",
					"count", len(newSources),
					"url", s.apiURL)
			}
		}
		s.mu.Unlock()
	}

	// Return copy of sources (with read lock)
	s.mu.RLock()
	defer s.mu.RUnlock()
	sources := make([]Config, len(s.sources))
	copy(sources, s.sources)
	return sources, nil
}
