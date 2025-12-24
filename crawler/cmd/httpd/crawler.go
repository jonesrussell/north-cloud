package httpd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	crawlerconfig "github.com/jonesrussell/north-cloud/crawler/internal/config/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/constants"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler"
	"github.com/jonesrussell/north-cloud/crawler/internal/crawler/events"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
	infracontext "github.com/north-cloud/infrastructure/context"
)

// createCrawlerForJobs creates a crawler instance for job execution.
func createCrawlerForJobs(
	deps *CommandDeps,
	storageResult *StorageResult,
) (crawler.Interface, error) {
	// Create event bus
	bus := events.NewEventBus(deps.Logger)

	// Get crawler config
	crawlerCfg := deps.Config.GetCrawlerConfig()

	// Load source manager
	sourceManager, err := loadSourceManager(deps)
	if err != nil {
		return nil, err
	}

	// Ensure raw content indexes (non-fatal if it fails)
	if err := ensureRawContentIndexes(deps, storageResult, sourceManager); err != nil {
		deps.Logger.Warn("Failed to ensure raw content indexes", "error", err)
		// Continue - not fatal
	}

	// Create crawler
	return createCrawler(deps, bus, crawlerCfg, storageResult, sourceManager)
}

// loadSourceManager loads sources using the API loader.
func loadSourceManager(deps *CommandDeps) (sources.Interface, error) {
	sourceManager, err := sources.LoadSources(deps.Config, deps.Logger)
	if err != nil {
		return nil, fmt.Errorf("failed to load sources: %w", err)
	}
	return sourceManager, nil
}

// ensureRawContentIndexes ensures raw content indexes exist for all sources.
func ensureRawContentIndexes(
	deps *CommandDeps,
	storageResult *StorageResult,
	sourceManager sources.Interface,
) error {
	rawIndexer := storage.NewRawContentIndexer(storageResult.Storage, deps.Logger)
	allSources, err := sourceManager.GetSources()
	if err != nil {
		return fmt.Errorf("failed to get sources: %w", err)
	}

	ctx, cancel := createTimeoutContext(constants.DefaultShutdownTimeout)
	defer cancel()

	for i := range allSources {
		// Extract hostname from source URL for index naming
		sourceHostname := extractHostnameFromURL(allSources[i].URL)
		if sourceHostname == "" {
			// Fallback to source name if URL parsing fails
			sourceHostname = allSources[i].Name
		}
		if indexErr := rawIndexer.EnsureRawContentIndex(ctx, sourceHostname); indexErr != nil {
			deps.Logger.Warn("Failed to ensure raw content index",
				"source", allSources[i].Name,
				"source_url", allSources[i].URL,
				"hostname", sourceHostname,
				"error", indexErr)
			// Continue with other sources - not fatal
		}
	}
	return nil
}

// createCrawler creates a crawler instance with the given parameters.
func createCrawler(
	deps *CommandDeps,
	bus *events.EventBus,
	crawlerCfg *crawlerconfig.Config,
	storageResult *StorageResult,
	sourceManager sources.Interface,
) (crawler.Interface, error) {
	crawlerResult, err := crawler.NewCrawlerWithParams(crawler.CrawlerParams{
		Logger:       deps.Logger,
		Bus:          bus,
		IndexManager: storageResult.IndexManager,
		Sources:      sourceManager,
		Config:       crawlerCfg,
		Storage:      storageResult.Storage,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create crawler: %w", err)
	}
	return crawlerResult.Crawler, nil
}

// extractHostnameFromURL extracts the hostname from a URL for use in index naming.
// Example: "https://www.sudbury.com/article" → "www.sudbury.com"
func extractHostnameFromURL(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return ""
	}
	return hostname
}

// createTimeoutContext creates a context with timeout.
func createTimeoutContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	return infracontext.WithTimeout(timeout)
}
