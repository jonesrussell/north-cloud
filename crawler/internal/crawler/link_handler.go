// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
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
	crawler  *Crawler
	linkRepo *database.QueuedLinkRepository
	saveLinks bool
}

// NewLinkHandler creates a new link handler.
func NewLinkHandler(c *Crawler, linkRepo *database.QueuedLinkRepository, saveLinks bool) *LinkHandler {
	return &LinkHandler{
		crawler:   c,
		linkRepo:  linkRepo,
		saveLinks: saveLinks,
	}
}

// HandleLink processes a single link from an HTML element.
func (h *LinkHandler) HandleLink(e *colly.HTMLElement) {
	link := e.Attr("href")
	if link == "" {
		return
	}

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

	// Save link to database instead of immediately visiting if enabled
	if h.saveLinks && h.linkRepo != nil {
		// Get context from crawler state (same pattern as ProcessHTML)
		ctx := h.crawler.state.Context()
		if ctx == nil {
			// Context is nil, crawler has been stopped/reset - abort
			h.crawler.logger.Debug("Cannot save link - crawler context is nil", "url", absLink)
			return
		}

		parentURL := e.Request.URL.String()
		if err := h.saveLinkToQueue(ctx, absLink, parentURL, e.Request.Depth); err != nil {
			h.crawler.logger.Warn("Failed to save link to queue, falling back to immediate visit",
				"url", absLink,
				"error", err)
			// Fallback to immediate visit if save fails
			h.visitWithRetries(e, absLink)
		} else {
			h.crawler.logger.Debug("Saved link to queue", "url", absLink, "depth", e.Request.Depth)
		}
	} else {
		// Log why we're not saving (for debugging)
		if !h.saveLinks {
			h.crawler.logger.Debug("Link saving disabled, visiting immediately", "url", absLink)
		} else if h.linkRepo == nil {
			h.crawler.logger.Debug("Link repository not available, visiting immediately", "url", absLink)
		}
		// Original behavior: visit immediately
		h.visitWithRetries(e, absLink)
	}
}

// shouldSkipLink determines if a link should be skipped based on its scheme or prefix.
func (h *LinkHandler) shouldSkipLink(link string) bool {
	skipPrefixes := []string{"#", "javascript:", "mailto:", "tel:"}

	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(link, prefix) {
			return true
		}
	}

	return false
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

// saveLinkToQueue saves a discovered link to the database for later processing.
func (h *LinkHandler) saveLinkToQueue(ctx context.Context, url, parentURL string, depth int) error {
	sourceName := h.crawler.state.CurrentSource()
	if sourceName == "" {
		return fmt.Errorf("no current source")
	}

	// Use source name as source ID (can be enhanced later to get actual ID from sources service)
	sourceID := sourceName

	queuedLink := &domain.QueuedLink{
		SourceID:   sourceID,
		SourceName: sourceName,
		URL:        url,
		ParentURL:  &parentURL,
		Depth:      depth + 1, // Increment depth (colly's depth is 0-indexed from start URL)
		Status:     "pending",
		Priority:   0, // Can be configured based on URL patterns in the future
	}

	return h.linkRepo.CreateOrUpdate(ctx, queuedLink)
}
