// Package feed provides RSS and Atom feed parsing for URL discovery.
package feed

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/crawler/internal/frontier"
)

// FrontierSubmitter submits URLs to the frontier queue.
type FrontierSubmitter interface {
	Submit(ctx context.Context, params SubmitParams) error
}

// SubmitParams contains the parameters for submitting a URL to the frontier.
// Mirrors database.SubmitParams to avoid import cycle with the database package.
type SubmitParams struct {
	URL      string
	URLHash  string
	Host     string
	SourceID string
	Origin   string
	Priority int
}

// FeedStateStore manages feed polling state.
type FeedStateStore interface {
	GetOrCreate(ctx context.Context, sourceID, feedURL string) (*domain.FeedState, error)
	UpdateSuccess(ctx context.Context, sourceID string, result PollResult) error
	UpdateError(ctx context.Context, sourceID, errorType, errMsg string) error
}

// PollResult contains the results of a successful feed poll.
type PollResult struct {
	ETag      *string
	Modified  *string
	ItemCount int
}

// HTTPFetcher fetches content from a URL with optional conditional GET headers.
type HTTPFetcher interface {
	Fetch(ctx context.Context, url string, etag, lastModified *string) (*FetchResponse, error)
}

// FetchResponse represents the result of an HTTP fetch.
type FetchResponse struct {
	StatusCode   int
	Body         string
	ETag         *string
	LastModified *string
}

// Logger provides structured logging for the poller.
type Logger interface {
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}

// Poller polls feeds and submits discovered URLs to the frontier.
type Poller struct {
	fetcher   HTTPFetcher
	feedState FeedStateStore
	frontier  FrontierSubmitter
	disabler  SourceFeedDisabler
	log       Logger
}

// NewPoller creates a new feed poller.
func NewPoller(
	fetcher HTTPFetcher,
	feedState FeedStateStore,
	frontierSubmitter FrontierSubmitter,
	disabler SourceFeedDisabler,
	log Logger,
) *Poller {
	return &Poller{
		fetcher:   fetcher,
		feedState: feedState,
		frontier:  frontierSubmitter,
		disabler:  disabler,
		log:       log,
	}
}

// PollFeed polls a single feed URL, parses it, and submits discovered URLs
// to the frontier. It uses conditional GET headers (If-None-Match,
// If-Modified-Since) from previous polling state to avoid re-processing
// unchanged feeds.
func (p *Poller) PollFeed(ctx context.Context, sourceID, feedURL string) error {
	state, err := p.feedState.GetOrCreate(ctx, sourceID, feedURL)
	if err != nil {
		return fmt.Errorf("poll feed get state: %w", err)
	}

	resp, fetchErr := p.fetcher.Fetch(ctx, feedURL, state.LastETag, state.LastModified)
	if fetchErr != nil {
		pollErr := ClassifyNetworkError(fetchErr, feedURL)
		p.recordError(ctx, sourceID, pollErr)
		return fmt.Errorf("poll feed fetch: %w", pollErr)
	}

	if resp.StatusCode == http.StatusNotModified {
		p.log.Info("feed not modified, skipping",
			"source_id", sourceID,
			"feed_url", feedURL,
		)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		pollErr := ClassifyHTTPStatus(resp.StatusCode, feedURL)
		p.recordError(ctx, sourceID, pollErr)
		return pollErr
	}

	return p.processResponse(ctx, sourceID, feedURL, resp)
}

// processResponse parses the feed body, submits discovered URLs, and
// updates the feed state on success.
func (p *Poller) processResponse(
	ctx context.Context,
	sourceID, feedURL string,
	resp *FetchResponse,
) error {
	items, parseErr := ParseFeed(ctx, resp.Body)
	if parseErr != nil {
		pollErr := ClassifyParseError(parseErr, feedURL)
		p.recordError(ctx, sourceID, pollErr)
		return fmt.Errorf("poll feed parse: %w", pollErr)
	}

	submitted := p.submitItems(ctx, sourceID, items)

	result := PollResult{
		ETag:      resp.ETag,
		Modified:  resp.LastModified,
		ItemCount: submitted,
	}

	if updateErr := p.feedState.UpdateSuccess(ctx, sourceID, result); updateErr != nil {
		return fmt.Errorf("poll feed update success: %w", updateErr)
	}

	// Re-enable feed if it was disabled and is now succeeding (cooldown retry)
	if p.disabler != nil {
		if enableErr := p.disabler.EnableFeed(ctx, sourceID); enableErr != nil {
			p.log.Warn("failed to re-enable feed",
				"source_id", sourceID,
				"error", enableErr.Error(),
			)
		}
	}

	p.log.Info("feed polled successfully",
		"source_id", sourceID,
		"feed_url", feedURL,
		"items_submitted", submitted,
	)

	return nil
}

// submitItems normalizes each feed item URL and submits it to the frontier.
// Items that fail normalization or submission are logged and skipped.
// Returns the count of successfully submitted items.
func (p *Poller) submitItems(ctx context.Context, sourceID string, items []FeedItem) int {
	submitted := 0

	for i := range items {
		if submitErr := p.submitSingleItem(ctx, sourceID, &items[i]); submitErr != nil {
			p.log.Error("failed to submit feed item",
				"source_id", sourceID,
				"url", items[i].URL,
				"error", submitErr.Error(),
			)
			continue
		}

		submitted++
	}

	return submitted
}

// submitSingleItem normalizes and submits a single feed item to the frontier.
func (p *Poller) submitSingleItem(ctx context.Context, sourceID string, item *FeedItem) error {
	normalized, normalizeErr := frontier.NormalizeURL(item.URL)
	if normalizeErr != nil {
		return fmt.Errorf("normalize url: %w", normalizeErr)
	}

	urlHash, hashErr := frontier.URLHash(item.URL)
	if hashErr != nil {
		return fmt.Errorf("url hash: %w", hashErr)
	}

	host, hostErr := frontier.ExtractHost(normalized)
	if hostErr != nil {
		return fmt.Errorf("extract host: %w", hostErr)
	}

	params := SubmitParams{
		URL:      normalized,
		URLHash:  urlHash,
		Host:     host,
		SourceID: sourceID,
		Origin:   domain.FrontierOriginFeed,
		Priority: domain.FrontierDefaultPriority + domain.FrontierFeedBonus,
	}

	if submitErr := p.frontier.Submit(ctx, params); submitErr != nil {
		return fmt.Errorf("submit to frontier: %w", submitErr)
	}

	return nil
}

// recordError logs a feed polling error at the appropriate severity level
// and records it in the feed state store.
func (p *Poller) recordError(ctx context.Context, sourceID string, err error) {
	var pollErr *PollError
	if errors.As(err, &pollErr) {
		p.logPollError(sourceID, pollErr)

		if updateErr := p.feedState.UpdateError(
			ctx, sourceID, string(pollErr.Type), pollErr.Error(),
		); updateErr != nil {
			p.log.Error("failed to record feed error",
				"source_id", sourceID,
				"error", updateErr.Error(),
			)
		}

		// Check auto-disable threshold (WARN-level only — ERROR needs human attention)
		if pollErr.Level == LevelWarn && p.disabler != nil {
			p.checkDisableThreshold(ctx, sourceID, pollErr.Type)
		}

		return
	}

	// Unknown error type — always ERROR level
	p.log.Error("feed poll failed",
		"source_id", sourceID,
		"error", err.Error(),
	)

	if updateErr := p.feedState.UpdateError(
		ctx, sourceID, string(ErrTypeUnexpected), err.Error(),
	); updateErr != nil {
		p.log.Error("failed to record feed error",
			"source_id", sourceID,
			"error", updateErr.Error(),
		)
	}
}

// logPollError logs a classified poll error at the appropriate severity.
func (p *Poller) logPollError(sourceID string, pollErr *PollError) {
	logFn := p.log.Warn
	if pollErr.Level == LevelError {
		logFn = p.log.Error
	}
	logFn("feed poll failed",
		"source_id", sourceID,
		"error_type", string(pollErr.Type),
		"status_code", pollErr.StatusCode,
		"error", pollErr.Error(),
	)
}

// checkDisableThreshold checks if a feed should be auto-disabled based on consecutive errors.
func (p *Poller) checkDisableThreshold(ctx context.Context, sourceID string, errType ErrorType) {
	threshold, shouldDisable := DisableThreshold(errType)
	if !shouldDisable {
		return
	}

	state, err := p.feedState.GetOrCreate(ctx, sourceID, "")
	if err != nil {
		return
	}

	if state.ConsecutiveErrors < threshold {
		return
	}

	if disableErr := p.disabler.DisableFeed(ctx, sourceID, string(errType)); disableErr != nil {
		p.log.Error("failed to disable feed",
			"source_id", sourceID,
			"error", disableErr.Error(),
		)
		return
	}

	p.log.Warn("feed auto-disabled",
		"source_id", sourceID,
		"reason", string(errType),
		"consecutive_errors", state.ConsecutiveErrors,
	)
}
