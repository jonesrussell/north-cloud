// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"fmt"

	colly "github.com/gocolly/colly/v2"
	configtypes "github.com/jonesrussell/gocrawl/internal/config/types"
	"github.com/jonesrussell/gocrawl/internal/content"
	"github.com/jonesrussell/gocrawl/internal/content/contenttype"
	sourcestypes "github.com/jonesrussell/gocrawl/internal/sources/types"
)

// ProcessHTML processes the HTML content.
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

	// Get source config for content type detection
	source := c.getSourceConfig()

	// Detect content type and get appropriate processor
	processor := c.selectProcessor(e)
	if processor == nil {
		contentType := c.htmlProcessor.DetectContentType(e, source)
		c.logger.Debug("No processor found for content",
			"url", e.Request.URL.String(),
			"type", contentType)
		c.state.IncrementProcessed()
		return
	}

	// Process the content
	err := processor.Process(c.state.Context(), e)
	if err != nil {
		contentType := c.htmlProcessor.DetectContentType(e, source)
		// If the error is "not implemented", log at debug level since this is expected
		// until the feature is implemented
		if err.Error() == "not implemented" {
			c.logger.Debug("Content processing not implemented",
				"url", e.Request.URL.String(),
				"type", contentType)
		} else {
			c.logger.Error("Failed to process content",
				"error", err,
				"url", e.Request.URL.String(),
				"type", contentType)
			c.state.IncrementError()
		}
	} else {
		contentType := c.htmlProcessor.DetectContentType(e, source)
		c.logger.Debug("Successfully processed content",
			"url", e.Request.URL.String(),
			"type", contentType)
	}

	c.state.IncrementProcessed()
}

// getSourceConfig gets the source configuration for the current source
func (c *Crawler) getSourceConfig() *configtypes.Source {
	sourceName := c.state.CurrentSource()

	c.logger.Debug("Getting source configuration",
		"source_name", sourceName,
		"sources_manager_nil", c.sources == nil)

	if sourceName == "" {
		c.logger.Debug("Source name is empty, cannot get source configuration")
		return nil
	}

	if c.sources == nil {
		c.logger.Debug("Sources manager is nil, cannot get source configuration",
			"source_name", sourceName)
		return nil
	}

	sourceConfig := c.sources.FindByName(sourceName)
	if sourceConfig == nil {
		c.logger.Debug("Source not found by name",
			"source_name", sourceName,
			"search_method", "FindByName")
		return nil
	}

	c.logger.Debug("Source found by name",
		"source_name", sourceName,
		"source_url", sourceConfig.URL,
		"has_article_body_selector", func() bool {
			config := sourcestypes.ConvertToConfigSource(sourceConfig)
			return config != nil && config.Selectors.Article.Body != ""
		}())

	// Convert to configtypes.Source
	return sourcestypes.ConvertToConfigSource(sourceConfig)
}

// selectProcessor selects the appropriate processor for the given HTML element
func (c *Crawler) selectProcessor(e *colly.HTMLElement) content.Processor {
	// Get URL for logging
	pageURL := ""
	if e.Request != nil && e.Request.URL != nil {
		pageURL = e.Request.URL.String()
	}

	source := c.getSourceConfig()

	c.logger.Debug("Selecting processor for HTML element",
		"url", pageURL,
		"source_found_by_name", source != nil,
		"current_source_name", c.state.CurrentSource(),
		"source_name", func() string {
			if source != nil {
				return source.Name
			}
			return nilString
		}())

	// If source not found by name, try to find it by URL
	if source == nil && e.Request != nil && e.Request.URL != nil {
		sourceURL := e.Request.URL.String()
		// Use HTMLProcessor's findSourceByURL method via DetectContentType fallback
		// The DetectContentType method will handle finding source by URL if source is nil
		c.logger.Debug("Source not found by name, will try URL-based lookup in DetectContentType",
			"url", sourceURL,
			"current_source_name", c.state.CurrentSource())
	}

	contentType := c.htmlProcessor.DetectContentType(e, source)

	c.logger.Debug("Content type detected",
		"content_type", contentType,
		"url", pageURL,
		"source_name", func() string {
			if source != nil {
				return source.Name
			}
			return nilString
		}())

	// Try to get a processor for the specific content type
	processor := c.getProcessorForType(contentType)
	if processor != nil {
		c.logger.Debug("Processor found for content type",
			"content_type", contentType,
			"processor_type", fmt.Sprintf("%T", processor),
			"url", pageURL)
		return processor
	}

	// Fallback: Try additional processors
	for _, p := range c.processors {
		if p.CanProcess(contentType) {
			c.logger.Debug("Fallback processor found",
				"content_type", contentType,
				"processor_type", fmt.Sprintf("%T", p),
				"url", pageURL)
			return p
		}
	}

	c.logger.Debug("No processor found for content type",
		"content_type", contentType,
		"url", pageURL)

	return nil
}

// getProcessorForType returns a processor for the given content type
func (c *Crawler) getProcessorForType(contentType contenttype.Type) content.Processor {
	switch contentType {
	case contenttype.Article:
		return c.articleProcessor
	case contenttype.Page:
		return c.pageProcessor
	case contenttype.Video, contenttype.Image, contenttype.HTML, contenttype.Job:
		// Try to find a processor for the specific content type
		for _, p := range c.processors {
			if p.CanProcess(contentType) {
				return p
			}
		}
	}
	return nil
}

// GetProcessor returns a processor for the given content type.
func (c *Crawler) GetProcessor(contentType contenttype.Type) (content.Processor, error) {
	for _, p := range c.processors {
		if p.CanProcess(contentType) {
			return p, nil
		}
	}

	if contentType == contenttype.Article {
		return c.articleProcessor, nil
	}

	if contentType == contenttype.Page {
		return c.pageProcessor, nil
	}

	return nil, fmt.Errorf("no processor found for content type: %s", contentType)
}
