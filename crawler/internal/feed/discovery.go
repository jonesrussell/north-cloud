package feed

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// SourceFeedUpdater persists a discovered feed URL for a source.
type SourceFeedUpdater interface {
	UpdateFeedURL(ctx context.Context, sourceID, feedURL string) error
}

// UndiscoveredSource is a source without a feed_url.
type UndiscoveredSource struct {
	SourceID string
	BaseURL  string
}

// Discoverer discovers RSS/Atom feeds for sources that lack a feed_url.
type Discoverer struct {
	fetcher       HTTPFetcher
	sourceUpdater SourceFeedUpdater
	log           Logger
	attempted     sync.Map
	retryAfter    time.Duration
}

// NewDiscoverer creates a feed discoverer.
func NewDiscoverer(
	fetcher HTTPFetcher,
	sourceUpdater SourceFeedUpdater,
	log Logger,
	retryAfter time.Duration,
) *Discoverer {
	return &Discoverer{
		fetcher:       fetcher,
		sourceUpdater: sourceUpdater,
		log:           log,
		retryAfter:    retryAfter,
	}
}

// commonFeedPaths are well-known paths to probe when HTML link discovery fails.
var commonFeedPaths = []string{
	"/feed",
	"/rss",
	"/feed.xml",
	"/rss.xml",
	"/atom.xml",
	"/index.xml",
}

// rssXMLType and atomXMLType are the MIME type substrings for feed link detection.
const (
	rssXMLType  = "rss+xml"
	atomXMLType = "atom+xml"
)

// DiscoverFeed attempts to discover a feed URL for a source.
// Returns the discovered feed URL or empty string if none found.
func (d *Discoverer) DiscoverFeed(ctx context.Context, sourceID, baseURL string) string {
	if d.recentlyAttempted(sourceID) {
		return ""
	}
	defer d.recordAttempt(sourceID)

	feedURL := d.discoverFromHTML(ctx, baseURL)
	if feedURL != "" {
		return feedURL
	}

	return d.discoverFromCommonPaths(ctx, baseURL)
}

// RunDiscoveryLoop runs feed discovery on a fixed interval.
// It blocks until ctx is cancelled and returns nil on clean shutdown.
func (d *Discoverer) RunDiscoveryLoop(
	ctx context.Context,
	interval time.Duration,
	listUndiscovered func(ctx context.Context) ([]UndiscoveredSource, error),
) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	d.discoverUndiscoveredFeeds(ctx, listUndiscovered)

	for {
		select {
		case <-ctx.Done():
			d.log.Info("feed discovery loop stopped")
			return nil
		case <-ticker.C:
			d.discoverUndiscoveredFeeds(ctx, listUndiscovered)
		}
	}
}

// discoverUndiscoveredFeeds lists sources without feed URLs and attempts discovery.
func (d *Discoverer) discoverUndiscoveredFeeds(
	ctx context.Context,
	listUndiscovered func(ctx context.Context) ([]UndiscoveredSource, error),
) {
	sources, err := listUndiscovered(ctx)
	if err != nil {
		d.log.Error("failed to list undiscovered sources", "error", err.Error())
		return
	}

	if len(sources) == 0 {
		return
	}

	d.log.Info("discovering feeds for sources", "count", len(sources))

	for i := range sources {
		d.discoverAndUpdate(ctx, sources[i].SourceID, sources[i].BaseURL)
	}
}

// discoverAndUpdate runs discovery for a single source and updates if found.
func (d *Discoverer) discoverAndUpdate(ctx context.Context, sourceID, baseURL string) {
	feedURL := d.DiscoverFeed(ctx, sourceID, baseURL)
	if feedURL == "" {
		return
	}

	if err := d.sourceUpdater.UpdateFeedURL(ctx, sourceID, feedURL); err != nil {
		d.log.Error("failed to update feed URL",
			"source_id", sourceID,
			"feed_url", feedURL,
			"error", err.Error(),
		)
		return
	}

	d.log.Info("discovered feed URL",
		"source_id", sourceID,
		"feed_url", feedURL,
	)
}

// discoverFromHTML fetches the base URL and looks for <link rel="alternate"> feed tags.
// Each candidate is validated by fetching and parsing it.
func (d *Discoverer) discoverFromHTML(ctx context.Context, baseURL string) string {
	resp, err := d.fetcher.Fetch(ctx, baseURL, nil, nil)
	if err != nil {
		d.log.Error("failed to fetch base URL for discovery",
			"url", baseURL,
			"error", err.Error(),
		)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	candidates := extractFeedLinkCandidates(baseURL, resp.Body)
	for _, candidate := range candidates {
		if d.isValidFeed(ctx, candidate) {
			return candidate
		}
	}

	return ""
}

// extractFeedLinkCandidates parses HTML and returns all feed URL candidates from link tags.
func extractFeedLinkCandidates(baseURL, body string) []string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var candidates []string

	doc.Find(`link[rel="alternate"]`).Each(func(_ int, s *goquery.Selection) {
		linkType, _ := s.Attr("type")
		if !isFeedType(linkType) {
			return
		}

		href, exists := s.Attr("href")
		if !exists || href == "" {
			return
		}

		resolved := resolveURL(baseURL, href)
		if resolved != "" {
			candidates = append(candidates, resolved)
		}
	})

	return candidates
}

// discoverFromCommonPaths probes well-known feed paths and validates each.
func (d *Discoverer) discoverFromCommonPaths(ctx context.Context, baseURL string) string {
	for _, path := range commonFeedPaths {
		candidate := resolveURL(baseURL, path)
		if candidate == "" {
			continue
		}

		if d.isValidFeed(ctx, candidate) {
			return candidate
		}
	}

	return ""
}

// isValidFeed fetches a URL and checks if it parses as a valid feed.
func (d *Discoverer) isValidFeed(ctx context.Context, feedURL string) bool {
	resp, err := d.fetcher.Fetch(ctx, feedURL, nil, nil)
	if err != nil {
		return false
	}

	if resp.StatusCode != http.StatusOK {
		return false
	}

	items, parseErr := ParseFeed(ctx, resp.Body)
	return parseErr == nil && len(items) > 0
}

// recentlyAttempted checks if a source was attempted within the retryAfter window.
func (d *Discoverer) recentlyAttempted(sourceID string) bool {
	val, ok := d.attempted.Load(sourceID)
	if !ok {
		return false
	}

	attemptedAt, isTime := val.(time.Time)
	if !isTime {
		return false
	}

	return time.Since(attemptedAt) < d.retryAfter
}

// recordAttempt records when a source was last attempted.
func (d *Discoverer) recordAttempt(sourceID string) {
	d.attempted.Store(sourceID, time.Now())
}

// isFeedType checks if a link type attribute indicates an RSS or Atom feed.
func isFeedType(linkType string) bool {
	return strings.Contains(linkType, rssXMLType) || strings.Contains(linkType, atomXMLType)
}

// resolveURL resolves a potentially relative href against a base URL.
func resolveURL(baseURL, href string) string {
	base, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	ref, err := url.Parse(href)
	if err != nil {
		return ""
	}

	resolved := base.ResolveReference(ref)
	return resolved.String()
}
