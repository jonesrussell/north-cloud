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

// convertLoaderConfig converts a loader.Config to a types.SourceConfig
func convertLoaderConfig(cfg loader.Config) Config {
	// Parse rate limit duration
	var rateLimit time.Duration
	if cfg.RateLimit != nil {
		switch v := cfg.RateLimit.(type) {
		case string:
			var err error
			rateLimit, err = time.ParseDuration(v)
			if err != nil {
				// If parsing fails, use a default value
				rateLimit = time.Second
			}
		case int, int64, float64:
			// Convert numeric value to duration in seconds
			switch val := v.(type) {
			case int:
				rateLimit = time.Duration(val) * time.Second
			case int64:
				rateLimit = time.Duration(val) * time.Second
			case float64:
				rateLimit = time.Duration(val) * time.Second
			default:
				rateLimit = time.Second
			}
		default:
			// Use default value for unknown types
			rateLimit = time.Second
		}
	} else {
		// Default to 1 second if not specified
		rateLimit = time.Second
	}

	// Parse URL to get domain
	u, err := url.Parse(cfg.URL)
	if err != nil {
		// If URL parsing fails, use the URL as is
		return Config{
			Name:           cfg.Name,
			URL:            cfg.URL,
			AllowedDomains: []string{cfg.URL},
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

	// Get the domain from the URL
	domain := u.Hostname()
	if domain == "" {
		domain = cfg.URL
	}

	return Config{
		Name:           cfg.Name,
		URL:            cfg.URL,
		AllowedDomains: []string{domain},
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
