package sources

import (
	"net/url"
	"time"

	configtypes "github.com/jonesrussell/north-cloud/crawler/internal/config/types"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/converter"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/loader"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/types"
)

// convertArticleSelectors converts article selectors from various types to types.ArticleSelectors.
// Uses generic converter to eliminate manual field copying.
func convertArticleSelectors(s any) types.ArticleSelectors {
	result, err := converter.ConvertValue[types.ArticleSelectors](s)
	if err != nil {
		// Return empty struct on conversion error
		// This maintains backward compatibility with the original implementation
		return types.ArticleSelectors{}
	}
	return result
}

// convertListSelectors converts list selectors from various types to types.ListSelectors.
// Uses generic converter to eliminate manual field copying.
func convertListSelectors(s any) types.ListSelectors {
	result, err := converter.ConvertValue[types.ListSelectors](s)
	if err != nil {
		// Return empty struct on conversion error
		return types.ListSelectors{}
	}
	return result
}

// convertPageSelectors converts page selectors from various types to types.PageSelectors.
// Uses generic converter to eliminate manual field copying.
func convertPageSelectors(s any) types.PageSelectors {
	result, err := converter.ConvertValue[types.PageSelectors](s)
	if err != nil {
		// Return empty struct on conversion error
		return types.PageSelectors{}
	}
	return result
}

// createSelectorConfig creates a new SelectorConfig from the given selectors.
func createSelectorConfig(selectors any) types.SelectorConfig {
	var articleSelectors types.ArticleSelectors
	var listSelectors types.ListSelectors
	var pageSelectors types.PageSelectors

	switch s := selectors.(type) {
	case configtypes.SourceSelectors:
		articleSelectors = convertArticleSelectors(s.Article)
		listSelectors = convertListSelectors(s.List)
		pageSelectors = convertPageSelectors(s.Page)
	case configtypes.ArticleSelectors:
		articleSelectors = convertArticleSelectors(s)
	case loader.SourceSelectors:
		articleSelectors = convertArticleSelectors(s.Article)
		listSelectors = convertListSelectors(s.List)
		pageSelectors = convertPageSelectors(s.Page)
	case loader.ArticleSelectors:
		articleSelectors = convertArticleSelectors(s)
	default:
		// Return empty selectors for unknown types
		articleSelectors = types.ArticleSelectors{}
		listSelectors = types.ListSelectors{}
		pageSelectors = types.PageSelectors{}
	}

	return types.SelectorConfig{
		Article: articleSelectors,
		List:    listSelectors,
		Page:    pageSelectors,
	}
}

// createConfigFromLoader creates a Config struct from a loader.Config.
// This helper eliminates duplicate Config creation code.
func createConfigFromLoader(cfg loader.Config, rateLimit time.Duration, allowedDomains []string) Config {
	return Config{
		ID:             cfg.ID,
		Name:           cfg.Name,
		URL:            cfg.URL,
		AllowedDomains: allowedDomains,
		StartURLs:      []string{cfg.URL},
		RateLimit:      rateLimit,
		MaxDepth:       cfg.MaxDepth,
		Time:           cfg.Time,
		Index:          cfg.Index,
		ArticleIndex:   cfg.ArticleIndex,
		PageIndex:      cfg.PageIndex,
		Selectors:      createSelectorConfig(cfg.Selectors),
		Rules:          configtypes.Rules{},
	}
}

// parseRateLimit parses the rate limit from various types (string, int, float64).
func parseRateLimit(rateLimit any) time.Duration {
	if rateLimit == nil {
		return time.Second
	}

	switch v := rateLimit.(type) {
	case string:
		duration, err := time.ParseDuration(v)
		if err != nil {
			return time.Second
		}
		return duration
	case int:
		return time.Duration(v) * time.Second
	case int64:
		return time.Duration(v) * time.Second
	case float64:
		return time.Duration(v) * time.Second
	default:
		return time.Second
	}
}

// convertLoaderConfig converts a loader.Config to a types.SourceConfig
func convertLoaderConfig(cfg loader.Config) Config {
	rateLimit := parseRateLimit(cfg.RateLimit)

	// Parse URL to get domain
	u, err := url.Parse(cfg.URL)
	if err != nil {
		// If URL parsing fails, use the URL as is
		return createConfigFromLoader(cfg, rateLimit, []string{cfg.URL})
	}

	// Get the domain from the URL
	domain := u.Hostname()
	if domain == "" {
		domain = cfg.URL
	}

	return createConfigFromLoader(cfg, rateLimit, []string{domain})
}
