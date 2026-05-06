// Package runner orchestrates one poll cycle per alert source:
// fetch → parse → score → dedup → index → publish → rescind absent.
//
// Layer: L2. Imports: L1 packages (adapter/rss, catalogue, elasticsearch,
// redis, scope, severity, observability) and domain (L0) only.
package runner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jonesrussell/north-cloud/alert-crawler/internal/adapter/rss"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/catalogue"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/domain"
	"github.com/jonesrussell/north-cloud/alert-crawler/internal/observability"
)

// fetcher is the rss.Client seam.
type fetcher interface {
	Fetch(ctx context.Context, in rss.FetchInput) (*rss.FetchOutput, error)
}

// store is the catalogue.Store seam.
type store interface {
	LoadCheckpoint(ctx context.Context, sourceID, feedURL string) (*catalogue.PollCheckpoint, error)
	SaveCheckpoint(ctx context.Context, c catalogue.PollCheckpoint) error
	IncrementConsecutiveFailures(ctx context.Context, sourceID, feedURL string) error
	ResetConsecutiveFailures(ctx context.Context, sourceID, feedURL string) error
	LookupAlert(ctx context.Context, sourceID, alertID string) (*catalogue.CatalogEntry, error)
	MarkSeen(ctx context.Context, e catalogue.CatalogEntry) error
	RescindAbsent(ctx context.Context, sourceID string, pollStartedAt time.Time) ([]string, error)
	MarkRescinded(ctx context.Context, sourceID, alertID string) error
}

// indexer is the elasticsearch.Indexer seam.
type indexer interface {
	Index(ctx context.Context, alert domain.Alert) error
	MarkRescinded(ctx context.Context, alertID string, at time.Time, reason string) error
}

// publisher is the redis.Publisher seam.
type publisher interface {
	Publish(ctx context.Context, event domain.LifecycleEvent) error
}

// resolver is the scope.Resolver seam.
type resolver interface {
	Resolve(src domain.AlertSource, hint string) []string
}

// consecutiveFailureWarnThreshold is the inclusive count at which
// RecordConsecutiveFailures emits WARN (NFR-005).
const consecutiveFailureWarnThreshold = 6

// Dependencies bundles all external collaborators for a Runner.
// Production wires concrete types; tests wire mocks.
type Dependencies struct {
	Fetch         fetcher
	Store         store
	Indexer       indexer
	Pub           publisher
	Resolver      resolver
	SevInfer      func(domain.Hazard) domain.Severity // closure over severity.Table
	Metrics       *observability.Metrics
	Sources       []domain.AlertSource
	DefaultExpiry time.Duration
	Now           func() time.Time // injectable for tests; defaults to time.Now
}

// Runner iterates over configured sources on each Run call.
type Runner struct {
	deps Dependencies
}

// New constructs a Runner. If deps.Now is nil it defaults to time.Now.
func New(deps Dependencies) *Runner {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	return &Runner{deps: deps}
}

// Run polls all enabled sources. Per-source errors are recorded via Metrics
// and do not abort the cycle.
func (r *Runner) Run(ctx context.Context) error {
	for i := range r.deps.Sources {
		src := r.deps.Sources[i]
		if !src.Enabled {
			continue
		}
		if err := r.RunSource(ctx, src); err != nil {
			r.deps.Metrics.RecordSourceError(src.ID, err)
		}
	}
	return nil
}

// RunSource executes one complete poll cycle for a single source.
func (r *Runner) RunSource(ctx context.Context, src domain.AlertSource) error {
	pollStartedAt := r.deps.Now().UTC()

	cp, err := r.loadOrInitCheckpoint(ctx, src)
	if err != nil {
		return err
	}

	out, fetchErr := r.deps.Fetch.Fetch(ctx, rss.FetchInput{
		Source:       src,
		LastETag:     cp.LastEtag,
		LastModified: cp.LastModified,
	})
	if fetchErr != nil {
		return r.handleFetchError(ctx, src, cp, pollStartedAt, fetchErr)
	}

	feed, parseErr := rss.ParseFeed(out.Body)
	if parseErr != nil {
		r.deps.Metrics.RecordParseFailure(src.ID, "feed")
		r.deps.Metrics.RecordPoll(src.ID, "error", time.Since(pollStartedAt))
		return fmt.Errorf("parse feed: %w", parseErr)
	}

	for i := range feed.Channel.Items {
		if itemErr := r.processItem(ctx, src, feed.Channel.Items[i]); itemErr != nil {
			// processItem records its own metrics; continue to next item.
			continue
		}
	}

	absentIDs, rescindErr := r.deps.Store.RescindAbsent(ctx, src.ID, pollStartedAt)
	if rescindErr != nil {
		return fmt.Errorf("RescindAbsent: %w", rescindErr)
	}
	for _, id := range absentIDs {
		r.rescind(ctx, src, id)
	}

	return r.saveSuccessCheckpoint(ctx, src, cp, out, pollStartedAt)
}

// loadOrInitCheckpoint returns the stored checkpoint or a zero-value one.
func (r *Runner) loadOrInitCheckpoint(ctx context.Context, src domain.AlertSource) (*catalogue.PollCheckpoint, error) {
	cp, err := r.deps.Store.LoadCheckpoint(ctx, src.ID, src.FeedURL)
	if err != nil && !errors.Is(err, catalogue.ErrNotFound) {
		return nil, fmt.Errorf("load checkpoint: %w", err)
	}
	if cp == nil {
		cp = &catalogue.PollCheckpoint{SourceID: src.ID, FeedURL: src.FeedURL}
	}
	return cp, nil
}

// saveSuccessCheckpoint persists cache headers, resets failure counter, and
// records an "ok" poll metric.
func (r *Runner) saveSuccessCheckpoint(
	ctx context.Context,
	src domain.AlertSource,
	cp *catalogue.PollCheckpoint,
	out *rss.FetchOutput,
	pollStartedAt time.Time,
) error {
	cp.LastPolledAt = pollStartedAt
	cp.LastEtag = out.ETag
	cp.LastModified = out.LastModified
	cp.LastStatus = out.StatusCode
	cp.ConsecutiveFailures = 0

	if err := r.deps.Store.SaveCheckpoint(ctx, *cp); err != nil {
		return fmt.Errorf("save checkpoint: %w", err)
	}
	// Non-fatal: best-effort reset.
	_ = r.deps.Store.ResetConsecutiveFailures(ctx, src.ID, src.FeedURL)

	r.deps.Metrics.RecordPoll(src.ID, "ok", time.Since(pollStartedAt))
	return nil
}

// handleFetchError classifies the fetch error and records appropriate metrics.
func (r *Runner) handleFetchError(
	ctx context.Context,
	src domain.AlertSource,
	cp *catalogue.PollCheckpoint,
	pollStartedAt time.Time,
	fetchErr error,
) error {
	switch {
	case errors.Is(fetchErr, rss.ErrNotModified):
		cp.LastPolledAt = pollStartedAt
		if saveErr := r.deps.Store.SaveCheckpoint(ctx, *cp); saveErr != nil {
			return fmt.Errorf("save checkpoint after 304: %w", saveErr)
		}
		r.deps.Metrics.RecordPoll(src.ID, "not_modified", time.Since(pollStartedAt))
		return nil

	case errors.Is(fetchErr, rss.ErrTransient):
		r.recordTransientFailure(ctx, src)
		r.deps.Metrics.RecordPoll(src.ID, "error", time.Since(pollStartedAt))
		return fetchErr

	default:
		r.deps.Metrics.RecordPoll(src.ID, "error", time.Since(pollStartedAt))
		return fetchErr
	}
}

// recordTransientFailure increments the failure counter and emits the count metric.
func (r *Runner) recordTransientFailure(ctx context.Context, src domain.AlertSource) {
	if err := r.deps.Store.IncrementConsecutiveFailures(ctx, src.ID, src.FeedURL); err != nil {
		return
	}
	newCP, lerr := r.deps.Store.LoadCheckpoint(ctx, src.ID, src.FeedURL)
	if lerr != nil || newCP == nil {
		return
	}
	r.deps.Metrics.RecordConsecutiveFailures(src.ID, newCP.ConsecutiveFailures)
}

// processItem converts one RSS item to a domain.Alert and writes it through
// the index+catalogue+publish pipeline.
func (r *Runner) processItem(ctx context.Context, src domain.AlertSource, item rss.Item) error {
	alert, err := rss.ParseItem(item, src)
	if err != nil {
		r.deps.Metrics.RecordParseFailure(src.ID, "item")
		return err
	}

	alert.Severity = r.deps.SevInfer(alert.Hazard)
	alert.Scope = r.deps.Resolver.Resolve(src, item.Title)

	if src.DefaultExpiry > 0 {
		exp := alert.IssuedAt.Add(src.DefaultExpiry)
		alert.ExpiresAt = &exp
	}

	hash := contentHash(alert)

	existing, lookupErr := r.deps.Store.LookupAlert(ctx, src.ID, alert.ID)
	isCreate := lookupErr != nil && errors.Is(lookupErr, catalogue.ErrNotFound)
	isUpdate := existing != nil && existing.ContentHash != hash
	isUnchanged := existing != nil && existing.ContentHash == hash

	if isUnchanged {
		return r.deps.Store.MarkSeen(ctx, catalogue.CatalogEntry{
			SourceID:    src.ID,
			AlertID:     alert.ID,
			LastSeenAt:  r.deps.Now().UTC(),
			IsActive:    true,
			ContentHash: hash,
		})
	}

	if indexErr := r.deps.Indexer.Index(ctx, alert); indexErr != nil {
		op := "create"
		if isUpdate {
			op = "update"
		}
		r.deps.Metrics.RecordESWriteFailure(src.ID, op)
		return indexErr
	}

	if markErr := r.deps.Store.MarkSeen(ctx, catalogue.CatalogEntry{
		SourceID:    src.ID,
		AlertID:     alert.ID,
		LastSeenAt:  r.deps.Now().UTC(),
		IsActive:    true,
		ContentHash: hash,
	}); markErr != nil {
		return markErr
	}

	eventType := domain.EventCreated
	if isUpdate {
		eventType = domain.EventUpdated
	}
	if pubErr := r.deps.Pub.Publish(ctx, domain.NewLifecycleEvent(eventType, alert)); pubErr != nil {
		r.deps.Metrics.RecordRedisPublishFailure(src.ID, string(eventType))
		// ES is canonical; do not roll back.
	}

	if isCreate {
		r.deps.Metrics.RecordCreated(src.ID, string(alert.Category), string(alert.Severity))
	} else {
		r.deps.Metrics.RecordUpdated(src.ID, string(alert.Category), string(alert.Severity))
	}
	return nil
}

// rescind marks one alert rescinded in ES, catalogue, and Redis.
func (r *Runner) rescind(ctx context.Context, src domain.AlertSource, alertID string) {
	now := r.deps.Now().UTC()
	if err := r.deps.Indexer.MarkRescinded(ctx, alertID, now, "absent from upstream feed"); err != nil {
		r.deps.Metrics.RecordESWriteFailure(src.ID, "rescind")
		return
	}
	if err := r.deps.Store.MarkRescinded(ctx, src.ID, alertID); err != nil {
		_ = err // ES is canonical; log only via non-nil return is best-effort.
	}
	event := domain.LifecycleEvent{
		EventType: domain.EventRescinded,
		EventAt:   now,
		AlertID:   alertID,
	}
	if err := r.deps.Pub.Publish(ctx, event); err != nil {
		r.deps.Metrics.RecordRedisPublishFailure(src.ID, string(domain.EventRescinded))
	}
	r.deps.Metrics.RecordRescinded(src.ID)
}

// hashInput is the stable subset of Alert fields used for content change detection.
type hashInput struct {
	Title        string              `json:"title"`
	Severity     domain.Severity     `json:"severity"`
	Substances   []string            `json:"substances,omitempty"`
	Composition  []domain.Substance  `json:"composition,omitempty"`
	Summary      string              `json:"summary"`
	ParseQuality domain.ParseQuality `json:"parse_quality"`
}

// contentHash returns a hex-encoded SHA-256 of the alert's mutable content fields.
func contentHash(a domain.Alert) string {
	in := hashInput{
		Title:        a.Title,
		Severity:     a.Severity,
		Summary:      a.Summary,
		ParseQuality: a.ParseQuality,
	}
	if a.Hazard.HarmReduction != nil {
		in.Substances = a.Hazard.HarmReduction.Substances
		in.Composition = a.Hazard.HarmReduction.Composition
	}
	b, marshalErr := json.Marshal(in)
	if marshalErr != nil {
		// Fallback: hash the title alone so we never return an empty hash.
		b = []byte(a.Title)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
