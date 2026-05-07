package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
)

// backfillLimit is the maximum number of feed items processed per source
// during a backfill run (TC-011, PR-002).
const backfillLimit = 20

// Backfill runs once over each enabled source and emits "created" lifecycle
// events for the top N (backfillLimit) most recent items. Idempotent: items
// already in the catalogue with the same content hash are skipped (no events,
// no ES writes).
//
// Differences from RunSource:
//   - No conditional GET (forces a fresh fetch, ignoring cached ETag/Last-Modified).
//   - No RescindAbsent step (catalogue may be empty; we are seeding it).
//   - Hard-capped at backfillLimit items per source.
//
// Per TC-011 and PR-002.
func (r *Runner) Backfill(ctx context.Context) error {
	for i := range r.deps.Sources {
		src := r.deps.Sources[i]
		if !src.Enabled {
			continue
		}
		if srcErr := r.backfillSource(ctx, src); srcErr != nil {
			r.deps.Metrics.RecordSourceError(src.ID, srcErr)
			// Continue to next source rather than abort the whole run.
		}
	}
	return nil
}

func (r *Runner) backfillSource(ctx context.Context, src domain.AlertSource) error {
	pollStartedAt := r.deps.Now().UTC()

	// Force fresh fetch: pass empty ETag and Last-Modified so upstream
	// cannot respond with 304 Not Modified.
	out, fetchErr := r.deps.Fetch.Fetch(ctx, rss.FetchInput{
		Source:       src,
		LastETag:     "",
		LastModified: "",
	})
	if fetchErr != nil {
		r.deps.Metrics.RecordPoll(src.ID, "error", time.Since(pollStartedAt))
		return fmt.Errorf("backfill fetch: %w", fetchErr)
	}

	feed, parseErr := rss.ParseFeed(out.Body)
	if parseErr != nil {
		r.deps.Metrics.RecordParseFailure(src.ID, "feed")
		return fmt.Errorf("parse feed: %w", parseErr)
	}

	items := feed.Channel.Items
	if len(items) > backfillLimit {
		items = items[:backfillLimit]
	}

	for i := range items {
		// processBackfillItem records its own metrics; continue to next item on error.
		_ = r.processBackfillItem(ctx, src, items[i])
	}

	// Save checkpoint with the fresh ETag so a subsequent normal poll can
	// use conditional GET and receive 304 if the feed hasn't changed.
	cp := catalogue.PollCheckpoint{
		SourceID:     src.ID,
		FeedURL:      src.FeedURL,
		LastPolledAt: pollStartedAt,
		LastEtag:     out.ETag,
		LastModified: out.LastModified,
		LastStatus:   out.StatusCode,
	}
	if cpErr := r.deps.Store.SaveCheckpoint(ctx, cp); cpErr != nil {
		return fmt.Errorf("save checkpoint: %w", cpErr)
	}

	r.deps.Metrics.RecordPoll(src.ID, "ok", time.Since(pollStartedAt))
	return nil
}

func (r *Runner) processBackfillItem(
	ctx context.Context,
	src domain.AlertSource,
	item rss.Item,
) error {
	alert, parseErr := rss.ParseItem(item, src)
	if parseErr != nil {
		r.deps.Metrics.RecordParseFailure(src.ID, "item")
		return parseErr
	}

	// Enrich with severity, scope, and expiry.
	alert.Severity = r.deps.SevInfer(alert.Hazard)
	alert.Scope = r.deps.Resolver.Resolve(src, item.Title)
	if src.DefaultExpiry > 0 {
		e := alert.IssuedAt.Add(src.DefaultExpiry)
		alert.ExpiresAt = &e
	}

	hash := contentHash(alert)

	existing, lookupErr := r.deps.Store.LookupAlert(ctx, src.ID, alert.ID)
	if lookupErr == nil && existing != nil && existing.ContentHash == hash {
		// Idempotent: already in catalogue with same hash — skip.
		return nil
	}

	// Write to ES first (ES is canonical).
	if indexErr := r.deps.Indexer.Index(ctx, alert); indexErr != nil {
		r.deps.Metrics.RecordESWriteFailure(src.ID, "create")
		return indexErr
	}

	// Update catalogue.
	if markErr := r.deps.Store.MarkSeen(ctx, catalogue.CatalogEntry{
		SourceID:    src.ID,
		AlertID:     alert.ID,
		LastSeenAt:  r.deps.Now().UTC(),
		IsActive:    true,
		ContentHash: hash,
	}); markErr != nil {
		return markErr
	}

	// Publish created lifecycle event (best-effort; do not roll back ES on failure).
	event := domain.NewLifecycleEvent(domain.EventCreated, alert)
	if pubErr := r.deps.Pub.Publish(ctx, event); pubErr != nil {
		r.deps.Metrics.RecordRedisPublishFailure(src.ID, string(domain.EventCreated))
	}

	r.deps.Metrics.RecordCreated(src.ID, string(alert.Category), string(alert.Severity))
	return nil
}
