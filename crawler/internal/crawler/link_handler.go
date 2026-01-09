// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/north-cloud/crawler/internal/database"
	"github.com/jonesrussell/north-cloud/crawler/internal/domain"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// LinkHandler handles link processing for the crawler.
type LinkHandler struct {
	crawler   *Crawler
	linkRepo  *database.DiscoveredLinkRepository
	saveLinks bool
}

// NewLinkHandler creates a new link handler.
func NewLinkHandler(c *Crawler, linkRepo *database.DiscoveredLinkRepository, saveLinks bool) *LinkHandler {
	return &LinkHandler{
		crawler:   c,
		linkRepo:  linkRepo,
		saveLinks: saveLinks,
	}
}

// HandleLink processes a single link from an HTML element.
func (h *LinkHandler) HandleLink(e *colly.HTMLElement) {
	link := e.Attr("href")
	if h.shouldSkipLink(link) {
		h.crawler.logger.Debug("Skipping link",
			infralogger.String("url", link),
			infralogger.String("reason", "invalid scheme or empty"),
			infralogger.String("page_url", e.Request.URL.String()),
		)
		return
	}

	absLink := e.Request.AbsoluteURL(link)
	if absLink == "" {
		h.crawler.logger.Debug("Skipping link",
			infralogger.String("url", link),
			infralogger.String("reason", "failed to make absolute URL"),
			infralogger.String("page_url", e.Request.URL.String()),
		)
		return
	}

	if err := h.validateURL(absLink); err != nil {
		h.crawler.logger.Debug("Skipping link",
			infralogger.String("url", absLink),
			infralogger.String("reason", "invalid URL"),
			infralogger.Error(err),
			infralogger.String("page_url", e.Request.URL.String()),
		)
		return
	}

	h.crawler.logger.Debug("Discovered link",
		infralogger.String("url", absLink),
		infralogger.String("page_url", e.Request.URL.String()),
		infralogger.Int("depth", e.Request.Depth),
	)

	// Save external links to database for tracking (if enabled), but always visit immediately
	if h.shouldSaveLink() && h.isExternalLink(absLink) {
		h.trySaveLink(absLink, e) // Save for tracking - errors are logged but don't block visiting
	}

	// Always visit the link (normal crawling behavior)
	h.visitWithRetries(e, absLink)
}

// shouldSkipLink determines if a link should be skipped based on its scheme or prefix.
// Relative URLs (no scheme) are allowed because they can be made absolute by AbsoluteURL().
func (h *LinkHandler) shouldSkipLink(link string) bool {
	if link == "" {
		return true
	}

	u, err := url.Parse(link)
	if err != nil {
		return true
	}

	// Allow relative URLs (no scheme) - they will be converted to absolute URLs
	if u.Scheme == "" {
		return false
	}

	// Only allow http and https schemes
	switch u.Scheme {
	case "http", "https":
		return false
	default:
		return true
	}
}

// validateURL validates a URL if validation is enabled in configuration.
func (h *LinkHandler) validateURL(absLink string) error {
	if !h.crawler.cfg.ValidateURLs {
		return nil
	}

	if _, err := url.Parse(absLink); err != nil {
		return ErrInvalidURL
	}

	return nil
}

// visitWithRetries attempts to visit a URL with configured retry logic.
func (h *LinkHandler) visitWithRetries(e *colly.HTMLElement, absLink string) {
	var lastErr error

	for attempt := range h.crawler.cfg.MaxRetries {
		err := e.Request.Visit(absLink)
		if err == nil {
			h.crawler.logger.Debug("Successfully queued link for visiting", infralogger.String("url", absLink))
			return
		}

		if h.isNonRetryableError(err) {
			h.crawler.logger.Debug("Skipping link visit due to non-retryable error",
				infralogger.String("url", absLink),
				infralogger.Error(err),
				infralogger.String("reason", h.getErrorReason(err)),
			)
			return
		}

		lastErr = err
		h.crawler.logger.Debug("Failed to visit link, retrying",
			infralogger.String("url", absLink),
			infralogger.Error(err),
			infralogger.Int("attempt", attempt+1),
			infralogger.Int("max_retries", h.crawler.cfg.MaxRetries),
		)

		time.Sleep(h.crawler.cfg.RetryDelay)
	}

	h.crawler.logger.Error("Failed to visit link after retries",
		infralogger.String("url", absLink),
		infralogger.Error(lastErr),
		infralogger.Int("max_retries", h.crawler.cfg.MaxRetries),
		infralogger.String("page_url", e.Request.URL.String()),
	)
}

// getErrorReason extracts a human-readable reason from an error message.
func (h *LinkHandler) getErrorReason(err error) string {
	if err == nil {
		return "unknown"
	}

	errMsg := strings.ToLower(err.Error())
	if strings.Contains(errMsg, "forbidden domain") {
		return "forbidden domain"
	}
	if strings.Contains(errMsg, "max depth") || strings.Contains(errMsg, "maximum depth") {
		return "max depth reached"
	}
	if strings.Contains(errMsg, "already visited") {
		return "already visited"
	}
	if strings.Contains(errMsg, "missing url") {
		return "missing URL"
	}

	return "unknown error"
}

// isNonRetryableError checks if an error should not trigger a retry.
func (h *LinkHandler) isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	nonRetryablePatterns := []string{
		"forbidden domain",
		"max depth",
		"maximum depth",
		"already visited",
		"missing url",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// shouldSaveLink checks if link saving is enabled and repository is available.
func (h *LinkHandler) shouldSaveLink() bool {
	return h.saveLinks && h.linkRepo != nil
}

// isExternalLink checks if a link points to an external domain (not in the source's allowed domains).
func (h *LinkHandler) isExternalLink(linkURL string) bool {
	parsedLink, err := url.Parse(linkURL)
	if err != nil {
		return false
	}

	linkHostname := parsedLink.Hostname()
	if linkHostname == "" {
		return false
	}

	// Get source configuration to check allowed domains
	sourceID := h.crawler.state.CurrentSource()
	if sourceID == "" {
		return false
	}

	ctx := h.crawler.state.Context()
	if ctx == nil {
		return false
	}

	source, err := h.crawler.sources.ValidateSourceByID(ctx, sourceID)
	if err != nil {
		// If we can't get source config, don't save the link
		return false
	}

	// Check if link's domain matches any allowed domain
	for _, allowedDomain := range source.AllowedDomains {
		// Exact match
		if allowedDomain == linkHostname {
			return false // Internal link
		}
		// Check wildcard domains (e.g., "*.example.com" matches "www.example.com")
		if strings.HasPrefix(allowedDomain, "*.") {
			domainSuffix := strings.TrimPrefix(allowedDomain, "*.")
			if strings.HasSuffix(linkHostname, "."+domainSuffix) || linkHostname == domainSuffix {
				return false // Internal link
			}
		}
	}

	// Link doesn't match any allowed domain, so it's external
	return true
}

// trySaveLink attempts to save a link to the database queue for tracking purposes.
// This does not prevent the link from being visited - it's for discovery tracking only.
// Returns true if link was saved successfully, false otherwise.
func (h *LinkHandler) trySaveLink(absLink string, e *colly.HTMLElement) bool {
	// Get context from crawler state (same pattern as ProcessHTML)
	ctx := h.crawler.state.Context()
	if ctx == nil {
		// Context is nil, crawler has been stopped/reset - abort
		h.crawler.logger.Debug("Cannot save link - crawler context is nil", infralogger.String("url", absLink))
		return false
	}

	parentURL := e.Request.URL.String()
	if err := h.saveLinkToQueue(ctx, absLink, parentURL, e.Request.Depth); err != nil {
		h.crawler.logger.Warn("Failed to save discovered link, continuing with visit",
			infralogger.String("url", absLink),
			infralogger.Error(err),
		)
		return false
	}

	h.crawler.logger.Debug("Saved discovered link",
		infralogger.String("url", absLink),
		infralogger.Int("depth", e.Request.Depth),
	)
	return true
}

// saveLinkToQueue saves a discovered link to the database for tracking and discovery purposes.
// This allows tracking of discovered links even though they are also visited immediately.
func (h *LinkHandler) saveLinkToQueue(ctx context.Context, linkURL, parentURL string, depth int) error {
	sourceID := h.crawler.state.CurrentSource()
	if sourceID == "" {
		return errors.New("no current source")
	}

	// Get source config to get the source name
	source, err := h.crawler.sources.ValidateSourceByID(ctx, sourceID)
	if err != nil {
		return fmt.Errorf("failed to get source config: %w", err)
	}

	discoveredLink := &domain.DiscoveredLink{
		SourceID:   sourceID,
		SourceName: source.Name,
		URL:        linkURL,
		ParentURL:  &parentURL,
		Depth:      depth + 1, // Increment depth (colly's depth is 0-indexed from start URL)
		Status:     "pending",
		Priority:   0, // Can be configured based on URL patterns in the future
	}

	return h.linkRepo.CreateOrUpdate(ctx, discoveredLink)
}
