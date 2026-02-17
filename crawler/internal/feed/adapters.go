package feed

import (
	"context"

	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
)

// FrontierRepoAdapter adapts database.FrontierRepository to the
// FrontierSubmitter interface expected by the feed poller.
type FrontierRepoAdapter struct {
	repo *database.FrontierRepository
}

// NewFrontierRepoAdapter creates a new adapter wrapping a FrontierRepository.
func NewFrontierRepoAdapter(repo *database.FrontierRepository) *FrontierRepoAdapter {
	return &FrontierRepoAdapter{repo: repo}
}

// Submit converts feed.SubmitParams to database.SubmitParams and delegates
// to the underlying repository.
func (a *FrontierRepoAdapter) Submit(ctx context.Context, params SubmitParams) error {
	return a.repo.Submit(ctx, database.SubmitParams{
		URL:      params.URL,
		URLHash:  params.URLHash,
		Host:     params.Host,
		SourceID: params.SourceID,
		Origin:   params.Origin,
		Priority: params.Priority,
	})
}

// FeedStateRepoAdapter adapts database.FeedStateRepository to the
// FeedStateStore interface expected by the feed poller.
type FeedStateRepoAdapter struct {
	repo *database.FeedStateRepository
}

// NewFeedStateRepoAdapter creates a new adapter wrapping a FeedStateRepository.
func NewFeedStateRepoAdapter(repo *database.FeedStateRepository) *FeedStateRepoAdapter {
	return &FeedStateRepoAdapter{repo: repo}
}

// GetOrCreate delegates to the underlying repository. The returned
// domain.FeedState is used directly since the poller already depends
// on the domain package.
func (a *FeedStateRepoAdapter) GetOrCreate(
	ctx context.Context,
	sourceID, feedURL string,
) (*domain.FeedState, error) {
	return a.repo.GetOrCreate(ctx, sourceID, feedURL)
}

// UpdateSuccess converts feed.PollResult to database.FeedPollResult and
// delegates to the underlying repository.
func (a *FeedStateRepoAdapter) UpdateSuccess(
	ctx context.Context,
	sourceID string,
	result PollResult,
) error {
	return a.repo.UpdateSuccess(ctx, sourceID, database.FeedPollResult{
		ETag:      result.ETag,
		Modified:  result.Modified,
		ItemCount: result.ItemCount,
	})
}

// UpdateError delegates to the underlying repository.
func (a *FeedStateRepoAdapter) UpdateError(
	ctx context.Context,
	sourceID, errMsg string,
) error {
	return a.repo.UpdateError(ctx, sourceID, errMsg)
}
