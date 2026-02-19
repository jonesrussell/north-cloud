package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/fetcher"
	"github.com/jonesrussell/north-cloud/crawler/internal/sources/apiclient"
	"github.com/jonesrussell/north-cloud/crawler/internal/storage"
)

// === frontierClaimerAdapter ===

// frontierClaimerAdapter bridges fetcher.FrontierClaimer to database.FrontierRepository.
type frontierClaimerAdapter struct {
	repo *database.FrontierRepository
}

func (a *frontierClaimerAdapter) Claim(ctx context.Context) (*domain.FrontierURL, error) {
	url, err := a.repo.Claim(ctx)
	if err != nil {
		if errors.Is(err, database.ErrNoURLAvailable) {
			return nil, fetcher.ErrNoURLAvailable
		}
		return nil, err
	}
	return url, nil
}

func (a *frontierClaimerAdapter) UpdateFetched(ctx context.Context, id string, params fetcher.FetchedParams) error {
	return a.repo.UpdateFetched(ctx, id, database.FetchedParams{
		ContentHash:  params.ContentHash,
		ETag:         params.ETag,
		LastModified: params.LastModified,
	})
}

func (a *frontierClaimerAdapter) UpdateFetchedWithFinalURL(ctx context.Context, id, finalURL string, params fetcher.FetchedParams) error {
	return a.repo.UpdateFetchedWithFinalURL(ctx, id, finalURL, database.FetchedParams{
		ContentHash:  params.ContentHash,
		ETag:         params.ETag,
		LastModified: params.LastModified,
	})
}

func (a *frontierClaimerAdapter) UpdateFailed(ctx context.Context, id, lastError string, maxRetries int) error {
	return a.repo.UpdateFailed(ctx, id, lastError, maxRetries)
}

func (a *frontierClaimerAdapter) UpdateDead(ctx context.Context, id, reason string) error {
	return a.repo.UpdateDead(ctx, id, reason)
}

// === hostUpdaterAdapter ===

// hostUpdaterAdapter bridges fetcher.HostUpdater to database.HostStateRepository.
// It ensures the host row exists before updating, since UpdateLastFetch requires it.
type hostUpdaterAdapter struct {
	repo *database.HostStateRepository
}

func (a *hostUpdaterAdapter) UpdateLastFetch(ctx context.Context, host string) error {
	if _, err := a.repo.GetOrCreate(ctx, host); err != nil {
		return fmt.Errorf("ensure host state: %w", err)
	}
	return a.repo.UpdateLastFetch(ctx, host)
}

// === contentIndexerAdapter ===

// contentIndexerAdapter bridges fetcher.ContentIndexer to storage.RawContentIndexer.
// It resolves source IDs to source names via the source-manager API with a local cache.
type contentIndexerAdapter struct {
	indexer     *storage.RawContentIndexer
	apiClient   *apiclient.Client
	sourceCache sync.Map // map[string]string (sourceID → sourceName)
}

func (a *contentIndexerAdapter) Index(ctx context.Context, content *fetcher.ExtractedContent) error {
	sourceName, err := a.resolveSourceName(ctx, content.SourceID)
	if err != nil {
		return fmt.Errorf("resolve source name for %s: %w", content.SourceID, err)
	}

	if ensureErr := a.indexer.EnsureRawContentIndex(ctx, sourceName); ensureErr != nil {
		return fmt.Errorf("ensure index for %s: %w", sourceName, ensureErr)
	}

	rawContent := mapExtractedToRawContent(content, sourceName)
	return a.indexer.IndexRawContent(ctx, rawContent)
}

// resolveSourceName looks up the source name for a source ID, using a cache.
func (a *contentIndexerAdapter) resolveSourceName(ctx context.Context, sourceID string) (string, error) {
	if cached, ok := a.sourceCache.Load(sourceID); ok {
		name, _ := cached.(string)
		return name, nil
	}

	source, err := a.apiClient.GetSource(ctx, sourceID)
	if err != nil {
		return "", fmt.Errorf("get source %s: %w", sourceID, err)
	}

	a.sourceCache.Store(sourceID, source.Name)
	return source.Name, nil
}

// mapExtractedToRawContent converts fetcher.ExtractedContent to storage.RawContent.
func mapExtractedToRawContent(content *fetcher.ExtractedContent, sourceName string) *storage.RawContent {
	return &storage.RawContent{
		ID:                   content.ContentHash,
		URL:                  content.URL,
		SourceName:           sourceName,
		Title:                content.Title,
		RawText:              content.Body,
		MetaDescription:      content.Description,
		Author:               content.Author,
		ClassificationStatus: "pending",
		CrawledAt:            time.Now(),
	}
}
