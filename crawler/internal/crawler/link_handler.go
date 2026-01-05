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
		return
	}

	absLink := e.Request.AbsoluteURL(link)
	if absLink == "" {
		h.crawler.logger.Debug("Failed to make absolute URL", "url", link)
		return
	}

	if err := h.validateURL(absLink); err != nil {
		h.crawler.logger.Debug("Invalid URL", "url", absLink, "error", err)
		return
	}

	// Save external links to database for tracking (if enabled), but always visit immediately
	if h.shouldSaveLink() && h.isExternalLink(absLink) {
		h.trySaveLink(absLink, e) // Save for tracking - errors are logged but don't block visiting
	}

	// Always visit the link (normal crawling behavior)
	h.visitWithRetries(e, absLink)
}

// shouldSkipLink determines if a link should be skipped based on its scheme or prefix.
func (h *LinkHandler) shouldSkipLink(link string) bool {
	if link == "" {
		return true
	}

	u, err := url.Parse(link)
	if err != nil || u.Scheme == "" {
		return true
	}

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
			h.crawler.logger.Debug("Successfully visited link", "url", absLink)
			return
		}

		if h.isNonRetryableError(err) {
			return
		}

		lastErr = err
		h.crawler.logger.Debug("Failed to visit link, retrying",
			"url", absLink,
			"error", err,
			"attempt", attempt+1,
			"max_retries", h.crawler.cfg.MaxRetries)

		time.Sleep(h.crawler.cfg.RetryDelay)
	}

	h.crawler.logger.Error("Failed to visit link after retries",
		"url", absLink,
		"error", lastErr,
		"max_retries", h.crawler.cfg.MaxRetries)
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
		h.crawler.logger.Debug("Cannot save link - crawler context is nil", "url", absLink)
		return false
	}

	parentURL := e.Request.URL.String()
	if err := h.saveLinkToQueue(ctx, absLink, parentURL, e.Request.Depth); err != nil {
		h.crawler.logger.Warn("Failed to save discovered link, continuing with visit",
			"url", absLink,
			"error", err)
		return false
	}

	h.crawler.logger.Debug("Saved discovered link", "url", absLink, "depth", e.Request.Depth)
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
