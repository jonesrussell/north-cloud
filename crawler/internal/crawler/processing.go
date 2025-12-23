// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"errors"

	colly "github.com/gocolly/colly/v2"
	"github.com/jonesrussell/gocrawl/internal/content"
	"github.com/jonesrussell/gocrawl/internal/content/contenttype"
)

// ProcessHTML processes the HTML content as raw content for classification.
// All content is extracted and indexed to raw_content indexes without type detection.
func (c *Crawler) ProcessHTML(e *colly.HTMLElement) {
	// Check if context is cancelled before processing
	ctx := c.state.Context()
	select {
	case <-ctx.Done():
		// Context cancelled, abort this request
		e.Request.Abort()
		return
	default:
		// Continue processing
	}

	// Always use raw content processor to extract raw content
	// The classifier will handle content type classification later
	processor := c.rawContentProcessor
	if processor == nil {
		c.logger.Debug("Raw content processor not available, skipping content",
			"url", e.Request.URL.String())
		c.state.IncrementProcessed()
		return
	}

	// Process the content as raw content
	err := processor.Process(c.state.Context(), e)
	if err != nil {
		// If the error is "not implemented", log at debug level since this is expected
		// until the feature is implemented
		if err.Error() == "not implemented" {
			c.logger.Debug("Content processing not implemented",
				"url", e.Request.URL.String())
		} else {
			c.logger.Error("Failed to process raw content",
				"error", err,
				"url", e.Request.URL.String())
			c.state.IncrementError()
		}
	} else {
		c.logger.Debug("Successfully processed raw content",
			"url", e.Request.URL.String())
	}

	c.state.IncrementProcessed()
}

// GetProcessor returns a processor for the given content type.
// All content types are processed as raw content - the classifier handles type detection
func (c *Crawler) GetProcessor(contentType contenttype.Type) (content.Processor, error) {
	// Always use raw content processor for all content types
	// The classifier will handle content type classification
	if c.rawContentProcessor == nil {
		return nil, errors.New("raw content processor not initialized")
	}
	return c.rawContentProcessor, nil
}
