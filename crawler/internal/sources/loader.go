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
// Sources will be loaded lazily when GetSources() is first called.
func NewSources(cfg config.Interface, log logger.Interface) (*Sources, error) {
	crawlerCfg := cfg.GetCrawlerConfig()

	// API loader is required - no file-based fallback
	if crawlerCfg == nil || crawlerCfg.SourcesAPIURL == "" {
		return nil, errors.New("sources_api_url is required in crawler configuration. API-only mode is enabled")
	}

	// Store API URL for lazy loading
	apiURL := crawlerCfg.SourcesAPIURL

	return &Sources{
		sources: nil, // Sources will be loaded lazily on first GetSources() call
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
