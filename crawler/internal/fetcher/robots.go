// Package fetcher provides HTTP fetching utilities for the crawler,
// including robots.txt compliance checking with per-host caching.
package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// Default cache TTL for robots.txt entries.
const defaultRobotsCacheTTL = 24 * time.Hour

// robotsTxtPath is the well-known path for robots.txt files.
const robotsTxtPath = "/robots.txt"

// maxRobotsBodyBytes limits the size of robots.txt responses we will read.
const maxRobotsBodyBytes = 512 * 1024 // 512 KB

// RobotsChecker checks and caches robots.txt rules per host.
type RobotsChecker struct {
	httpClient *http.Client
	userAgent  string
	cache      map[string]*robotsCacheEntry // keyed by host
	mu         sync.RWMutex
	cacheTTL   time.Duration
}

// robotsCacheEntry stores the parsed robots.txt data and metadata for a host.
type robotsCacheEntry struct {
	data      *robotstxt.RobotsData
	fetchedAt time.Time
	allowAll  bool // true if robots.txt was missing/404 or errored (allow all)
}

// NewRobotsChecker creates a new RobotsChecker.
func NewRobotsChecker(
	httpClient *http.Client,
	userAgent string,
	cacheTTL time.Duration,
) *RobotsChecker {
	if cacheTTL == 0 {
		cacheTTL = defaultRobotsCacheTTL
	}

	return &RobotsChecker{
		httpClient: httpClient,
		userAgent:  userAgent,
		cache:      make(map[string]*robotsCacheEntry),
		cacheTTL:   cacheTTL,
	}
}

// IsAllowed checks if the given URL is allowed by the host's robots.txt.
// It fetches and caches robots.txt if not cached or stale.
// Returns true if allowed, false if disallowed.
// Missing or errored robots.txt results in allow all (standard practice).
func (r *RobotsChecker) IsAllowed(ctx context.Context, rawURL string) (bool, error) {
	parsed, parseErr := url.Parse(rawURL)
	if parseErr != nil {
		return false, fmt.Errorf("robots: parse url: %w", parseErr)
	}

	host := strings.ToLower(parsed.Host)
	if host == "" {
		return false, fmt.Errorf("robots: empty host in url %q", rawURL)
	}

	entry, fetchErr := r.getOrFetchEntry(ctx, host, parsed.Scheme)
	if fetchErr != nil {
		return false, fetchErr
	}

	if entry.allowAll {
		return true, nil
	}

	return entry.data.TestAgent(parsed.Path, r.userAgent), nil
}

// CrawlDelay returns the crawl-delay for the host, if specified in robots.txt.
// Returns 0 if no crawl-delay is set or robots.txt is not cached.
func (r *RobotsChecker) CrawlDelay(host string) time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.cache[strings.ToLower(host)]
	if !ok || entry.allowAll || entry.data == nil {
		return 0
	}

	group := entry.data.FindGroup(r.userAgent)
	if group == nil {
		return 0
	}

	return group.CrawlDelay
}

// getOrFetchEntry returns a cached entry if fresh, otherwise fetches robots.txt.
func (r *RobotsChecker) getOrFetchEntry(
	ctx context.Context,
	host string,
	scheme string,
) (*robotsCacheEntry, error) {
	if entry, ok := r.getCachedEntry(host); ok {
		return entry, nil
	}

	return r.fetchAndCache(ctx, host, scheme)
}

// getCachedEntry returns a cached entry if it exists and is not stale.
func (r *RobotsChecker) getCachedEntry(host string) (*robotsCacheEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, ok := r.cache[host]
	if !ok {
		return nil, false
	}

	if time.Since(entry.fetchedAt) > r.cacheTTL {
		return nil, false
	}

	return entry, true
}

// fetchAndCache fetches robots.txt for the host and caches the result.
func (r *RobotsChecker) fetchAndCache(
	ctx context.Context,
	host string,
	scheme string,
) (*robotsCacheEntry, error) {
	if scheme == "" {
		scheme = "https"
	}

	robotsURL := scheme + "://" + host + robotsTxtPath

	body, statusCode, fetchErr := r.doFetch(ctx, robotsURL)
	if fetchErr != nil {
		// Fetch failures are treated as allow-all (graceful degradation).
		return r.cacheAllowAll(host), nil //nolint:nilerr // intentional: fetch error => allow all
	}

	entry := r.parseAndBuildEntry(body, statusCode)

	r.mu.Lock()
	r.cache[host] = entry
	r.mu.Unlock()

	return entry, nil
}

// doFetch performs the HTTP GET request for a robots.txt URL.
func (r *RobotsChecker) doFetch(
	ctx context.Context,
	robotsURL string,
) (body []byte, statusCode int, err error) {
	req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL, http.NoBody)
	if reqErr != nil {
		return nil, 0, fmt.Errorf("robots: create request: %w", reqErr)
	}

	req.Header.Set("User-Agent", r.userAgent)

	resp, doErr := r.httpClient.Do(req) //nolint:gosec // G704: URL from crawl target
	if doErr != nil {
		return nil, 0, fmt.Errorf("robots: fetch: %w", doErr)
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxRobotsBodyBytes)

	body, readErr := io.ReadAll(limited)
	if readErr != nil {
		return nil, resp.StatusCode, fmt.Errorf("robots: read body: %w", readErr)
	}

	return body, resp.StatusCode, nil
}

// parseAndBuildEntry parses a robots.txt response body with status code.
// Only 2xx responses are parsed; all others are treated as allow-all
// for graceful degradation (standard crawling practice).
func (r *RobotsChecker) parseAndBuildEntry(
	body []byte,
	statusCode int,
) *robotsCacheEntry {
	if !isSuccessStatus(statusCode) {
		return &robotsCacheEntry{
			fetchedAt: time.Now(),
			allowAll:  true,
		}
	}

	robots, parseErr := robotstxt.FromBytes(body)
	if parseErr != nil {
		return &robotsCacheEntry{
			fetchedAt: time.Now(),
			allowAll:  true,
		}
	}

	return &robotsCacheEntry{
		data:      robots,
		fetchedAt: time.Now(),
	}
}

// statusSuccessLow is the lower bound (inclusive) for HTTP success status codes.
const statusSuccessLow = 200

// statusSuccessHigh is the upper bound (exclusive) for HTTP success status codes.
const statusSuccessHigh = 300

// isSuccessStatus returns true if the HTTP status code is in the 2xx range.
func isSuccessStatus(statusCode int) bool {
	return statusCode >= statusSuccessLow && statusCode < statusSuccessHigh
}

// cacheAllowAll stores an allow-all entry for the host and returns it.
func (r *RobotsChecker) cacheAllowAll(host string) *robotsCacheEntry {
	entry := &robotsCacheEntry{
		fetchedAt: time.Now(),
		allowAll:  true,
	}

	r.mu.Lock()
	r.cache[host] = entry
	r.mu.Unlock()

	return entry
}
