package sources

import (
	"errors"
	"fmt"

	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/loader"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
)

// LoadSources creates a new Sources instance by loading sources from either the
// gosources API or a YAML file specified in the crawler config. Returns an error if no sources are found.
// The logger parameter is optional and can be nil.
func LoadSources(cfg config.Interface, log logger.Interface) (*Sources, error) {
	var sources []Config
	var err error

	crawlerCfg := cfg.GetCrawlerConfig()

	// API loader is required - no file-based fallback
	if crawlerCfg == nil || crawlerCfg.SourcesAPIURL == "" {
		return nil, errors.New("sources_api_url is required in crawler configuration. API-only mode is enabled")
	}

	if log != nil {
		log.Info("Loading sources from API", "url", crawlerCfg.SourcesAPIURL)
	}
	sources, err = loadSourcesFromAPI(crawlerCfg.SourcesAPIURL, log)
	if err != nil {
		return nil, fmt.Errorf("failed to load sources from API: %w", err)
	}

	// If no sources found, return an error
	if len(sources) == 0 {
		return nil, errors.New("no sources found")
	}

	// Store API URL for dynamic reloading (crawlerCfg is guaranteed to be non-nil here)
	apiURL := crawlerCfg.SourcesAPIURL

	return &Sources{
		sources: sources,
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

	// Update cached sources (with write lock)
	s.mu.Lock()
	s.sources = newSources
	s.mu.Unlock()

	if s.logger != nil {
		s.logger.Info("Sources reloaded from API",
			"count", len(newSources))
	}

	return nil
}

