package sources

import (
	"errors"
	"os"

	"github.com/jonesrussell/north-cloud/crawler/internal/config"
	"github.com/jonesrussell/north-cloud/crawler/internal/logger"
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

	// Get JWT secret from environment for service-to-service authentication
	jwtSecret := os.Getenv("AUTH_JWT_SECRET")

	return &Sources{
		sources:   nil, // Sources will be loaded lazily on first GetSources() call
		logger:    log,
		metrics:   types.NewSourcesMetrics(),
		apiURL:    apiURL,
		jwtSecret: jwtSecret,
	}, nil
}
