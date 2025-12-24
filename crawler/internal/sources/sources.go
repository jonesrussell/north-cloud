// Package sources manages the configuration and lifecycle of web content sources for GoCrawl.
// It handles source configuration loading and validation through a YAML-based configuration system.
package sources

import (
	"context"
	"errors"
	"sync"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
	storagetypes "github.com/jonesrussell/north-cloud/crawler/internal/storage/types"
)

// Interface defines the read-only interface for accessing sources.
type Interface interface {
	// ListSources retrieves all sources.
	ListSources(ctx context.Context) ([]*Config, error)
	// ValidateSource validates a source configuration and returns the validated source.
	ValidateSource(
		ctx context.Context,
		sourceName string,
		indexManager storagetypes.IndexManager,
	) (*configtypes.Source, error)
	// GetMetrics returns the current metrics.
	GetMetrics() types.SourcesMetrics
	// FindByName finds a source by name. Returns nil if not found.
	FindByName(name string) *Config
	// GetSources retrieves all source configurations.
	GetSources() ([]Config, error)
}

// Params contains the parameters for creating a new source manager.
type Params struct {
	// Logger is the logger to use.
	Logger logger.Interface
}

// ErrInvalidSource is returned when a source is invalid.
var ErrInvalidSource = errors.New("invalid source")

// ErrSourceNotFound is returned when a source is not found.
var ErrSourceNotFound = errors.New("source not found")

// ErrSourceExists is returned when a source already exists.
var ErrSourceExists = errors.New("source already exists")

// ValidateParams validates the parameters for creating a new source manager.
func ValidateParams(p Params) error {
	if p.Logger == nil {
		return errors.New("logger is required")
	}
	return nil
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
	apiURL  string       // Store API URL for reloading sources
	mu      sync.RWMutex // Mutex for thread-safe source updates
}

// Ensure Sources implements Interface
var _ Interface = (*Sources)(nil)

// ListSources retrieves all sources.
func (s *Sources) ListSources(ctx context.Context) ([]*Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Config, 0, len(s.sources))
	for i := range s.sources {
		result = append(result, &s.sources[i])
	}
	return result, nil
}

// GetMetrics returns the current metrics.
func (s *Sources) GetMetrics() types.SourcesMetrics {
	return *s.metrics
}

// GetSources returns all sources.
func (s *Sources) GetSources() ([]Config, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make([]Config, len(s.sources))
	copy(sources, s.sources)
	return sources, nil
}

// FindByName finds a source by name.
func (s *Sources) FindByName(name string) *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.sources {
		if s.sources[i].Name == name {
			return &s.sources[i]
		}
	}
	return nil
}
