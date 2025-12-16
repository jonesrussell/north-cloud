// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"net/url"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// LinkHandler handles link processing for the crawler.
type LinkHandler struct {
	crawler *Crawler
}

// NewLinkHandler creates a new link handler.
func NewLinkHandler(c *Crawler) *LinkHandler {
	return &LinkHandler{
		crawler: c,
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

	h.visitWithRetries(e, absLink)
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

	for attempt := 0; attempt < h.crawler.cfg.MaxRetries; attempt++ {
		err := e.Request.Visit(absLink)
		if err == nil {
			h.crawler.logger.Debug("Successfully visited link", "url", absLink)
			return
		}

		if h.isNonRetryableError(err) {
			h.crawler.logger.Debug("Skipping non-retryable link",
				"url", absLink,
				"error", err.Error())
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
