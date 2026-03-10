package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jonesrussell/north-cloud/infrastructure/indigenous"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
)

// IndigenousSource represents a single source entry from the global indigenous sources JSON file.
type IndigenousSource struct {
	Name       string `json:"name"`
	Homepage   string `json:"homepage"`
	RSS        string `json:"rss"`
	Region     string `json:"region"`
	Country    string `json:"country"`
	Language   string `json:"language"`
	RenderMode string `json:"render_mode"`
}

// Import configuration constants.
const (
	indigenousDefaultRateLimit        = "10s"
	indigenousDynamicRateLimit        = "12s"
	indigenousDefaultMaxDepth         = 2
	indigenousDynamicMaxDepth         = 1
	indigenousDefaultIngestionMode    = "standard"
	indigenousFeedIngestionMode       = "feed"
	indigenousDefaultFeedPollInterval = 60
)

// ParseIndigenousSources parses the JSON array of indigenous sources from a reader.
func ParseIndigenousSources(r io.Reader) ([]IndigenousSource, error) {
	var sources []IndigenousSource
	if err := json.NewDecoder(r).Decode(&sources); err != nil {
		return nil, fmt.Errorf("decode indigenous sources JSON: %w", err)
	}
	return sources, nil
}

// ValidateIndigenousSource validates a single indigenous source entry.
// Returns an error message or empty string if valid.
func ValidateIndigenousSource(src IndigenousSource) string {
	if strings.TrimSpace(src.Name) == "" {
		return "name is required"
	}
	if strings.TrimSpace(src.Homepage) == "" {
		return "homepage is required"
	}
	if !strings.HasPrefix(src.Homepage, "http://") && !strings.HasPrefix(src.Homepage, "https://") {
		return "homepage must start with http:// or https://"
	}
	if strings.TrimSpace(src.Region) == "" {
		return "region is required"
	}
	if _, err := indigenous.NormalizeRegionSlug(src.Region); err != nil {
		return fmt.Sprintf("invalid region %q: %s", src.Region, err.Error())
	}
	if src.RenderMode != "static" && src.RenderMode != "dynamic" {
		return fmt.Sprintf("render_mode must be 'static' or 'dynamic', got %q", src.RenderMode)
	}
	return ""
}

// IndigenousSourceToModel converts a validated IndigenousSource to a models.Source.
func IndigenousSourceToModel(src IndigenousSource) (*models.Source, error) {
	regionSlug, err := indigenous.NormalizeRegionSlug(src.Region)
	if err != nil {
		return nil, fmt.Errorf("normalize region: %w", err)
	}

	rateLimit := indigenousDefaultRateLimit
	maxDepth := indigenousDefaultMaxDepth
	if src.RenderMode == "dynamic" {
		rateLimit = indigenousDynamicRateLimit
		maxDepth = indigenousDynamicMaxDepth
	}

	ingestionMode := indigenousDefaultIngestionMode
	var feedURL *string
	if strings.TrimSpace(src.RSS) != "" {
		ingestionMode = indigenousFeedIngestionMode
		rss := strings.TrimSpace(src.RSS)
		feedURL = &rss
	}

	return &models.Source{
		Name:                    src.Name,
		URL:                     src.Homepage,
		RateLimit:               rateLimit,
		MaxDepth:                maxDepth,
		Enabled:                 true,
		FeedURL:                 feedURL,
		IngestionMode:           ingestionMode,
		FeedPollIntervalMinutes: indigenousDefaultFeedPollInterval,
		RenderMode:              src.RenderMode,
		Type:                    "indigenous",
		IndigenousRegion:        &regionSlug,
	}, nil
}
