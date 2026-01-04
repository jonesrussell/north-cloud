package sources

import (
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/loader"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
)

// NewSources creates a new Sources instance without loading sources immediately.
// Sources will be loaded lazily when ValidateSource is first called.
// This is more efficient for systems with many sources that may not all be used.
func NewSources(cfg config.Interface, log logger.Interface) (*Sources, error) {
	crawlerCfg := cfg.GetCrawlerConfig()

	// API loader is required - no file-based fallback
	if crawlerCfg == nil || crawlerCfg.SourcesAPIURL == "" {
		return nil, errors.New("sources_api_url is required in crawler configuration. API-only mode is enabled")
	}

	// Store API URL for lazy loading (crawlerCfg is guaranteed to be non-nil here)
	apiURL := crawlerCfg.SourcesAPIURL

	return &Sources{
		sources: nil, // Sources will be loaded lazily on first ValidateSource call
		logger:  log,
		metrics: types.NewSourcesMetrics(),
		apiURL:  apiURL,
	}, nil
}

// loadSourcesFromAPI attempts to load sources from the gosources API
func loadSourcesFromAPI(apiURL string, log logger.Interface) ([]Config, error) {
	apiLoader := loader.NewAPILoader(apiURL, log)

	configs, err := apiLoader.LoadSources()
	if err != nil {
		return nil, fmt.Errorf("failed to load sources from API: %w", err)
	}

	if len(configs) == 0 {
		return nil, errors.New("no sources found from API")
	}

	// Convert loaded configs to our source type
	sourceConfigs := make([]Config, 0, len(configs))
	for i := range configs {
		sourceConfigs = append(sourceConfigs, convertLoaderConfig(configs[i]))
	}

	return sourceConfigs, nil
}

// ensureSourcesLoaded ensures sources are loaded. If sources are nil or empty, loads them from the API.
// This implements lazy loading - sources are only loaded when first needed.
// Uses sync.Once to ensure sources are only loaded once, even with concurrent calls.
func (s *Sources) ensureSourcesLoaded() error {
	var loadErr error
	s.loadOnce.Do(func() {
		// Check if sources are already loaded (double-check)
		s.mu.RLock()
		alreadyLoaded := len(s.sources) > 0
		s.mu.RUnlock()

		if alreadyLoaded {
			return // Sources already loaded
		}

		// Load sources from API
		loadErr = s.reloadSources()
	})

	return loadErr
}

// reloadSources reloads sources from the API and updates the cached sources.
func (s *Sources) reloadSources() error {
	if s.apiURL == "" {
		return errors.New("API URL not configured")
	}

	// Load sources from API
	newSources, err := loadSourcesFromAPI(s.apiURL, s.logger)
	if err != nil {
		return fmt.Errorf("failed to reload sources from API: %w", err)
	}

	// If no sources found, return an error
	if len(newSources) == 0 {
		return errors.New("no sources found")
	}

	// Update cached sources (with write lock)
	s.mu.Lock()
	s.sources = newSources
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Info("Sources loaded from API",
			"count", len(newSources),
			"url", s.apiURL)
	}

	return nil
}
