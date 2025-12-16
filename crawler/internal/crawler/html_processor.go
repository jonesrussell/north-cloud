// Package crawler provides the core crawling functionality for the application.
package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/jonesrussell/gocrawl/internal/config/types"
	"github.com/jonesrussell/gocrawl/internal/constants"
	"github.com/jonesrussell/gocrawl/internal/content"
	"github.com/jonesrussell/gocrawl/internal/content/contenttype"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/sources"
	sourcestypes "github.com/jonesrussell/gocrawl/internal/sources/types"
)

// HTMLProcessor processes HTML content and delegates to appropriate content processors.
type HTMLProcessor struct {
	logger       logger.Interface
	processors   []content.Processor
	unknownTypes map[contenttype.Type]int
	sources      sources.Interface
}

// NewHTMLProcessor creates a new HTMLProcessor.
func NewHTMLProcessor(log logger.Interface, sourcesManager sources.Interface) *HTMLProcessor {
	return &HTMLProcessor{
		logger:       log,
		processors:   make([]content.Processor, 0, DefaultProcessorsCapacity), // Pre-allocate for article and page processors
		unknownTypes: make(map[contenttype.Type]int),
		sources:      sourcesManager,
	}
}

// CanProcess returns whether the processor can handle the given content type.
func (p *HTMLProcessor) CanProcess(contentType contenttype.Type) bool {
	return contentType == contenttype.HTML
}

// ContentType returns the content type this processor handles.
func (p *HTMLProcessor) ContentType() contenttype.Type {
	return contenttype.HTML
}

// Start initializes the processor.
func (p *HTMLProcessor) Start(ctx context.Context) error {
	return nil
}

// Stop stops the processor.
func (p *HTMLProcessor) Stop(ctx context.Context) error {
	return nil
}

// ValidateJob validates a job before processing.
func (p *HTMLProcessor) ValidateJob(job *content.Job) error {
	if job == nil {
		return errors.New("job cannot be nil")
	}
	return nil
}

// GetProcessor returns a processor for the given content type.
func (p *HTMLProcessor) GetProcessor(contentType contenttype.Type) (content.ContentProcessor, error) {
	for _, processor := range p.processors {
		if processor.CanProcess(contentType) {
			return processor, nil
		}
	}
	return nil, fmt.Errorf("no processor found for content type: %s", contentType)
}

// RegisterProcessor registers a new processor.
func (p *HTMLProcessor) RegisterProcessor(processor content.Processor) {
	p.processors = append(p.processors, processor)
}

// ProcessContent processes content using the appropriate processor.
func (p *HTMLProcessor) ProcessContent(ctx context.Context, ct contenttype.Type, contentData any) error {
	proc, err := p.GetProcessor(ct)
	if err != nil {
		return err
	}
	return proc.Process(ctx, contentData)
}

// findSourceByURL attempts to find a source configuration by matching the URL domain.
// This is a helper method that finds sources by URL when the source name lookup fails.
func (p *HTMLProcessor) findSourceByURL(pageURL string) *types.Source {
	p.logger.Debug("Starting source lookup by URL", "url", pageURL)

	if p.sources == nil {
		p.logger.Debug("Source lookup failed: sources manager is nil")
		return nil
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		p.logger.Debug("Source lookup failed: URL parsing error", "url", pageURL, "error", err)
		return nil
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		p.logger.Debug("Source lookup failed: empty hostname", "url", pageURL)
		return nil
	}

	p.logger.Debug("Extracted hostname from URL", "hostname", hostname, "url", pageURL)

	// Get all sources and try to match by domain
	sourceConfigs, err := p.sources.GetSources()
	if err != nil {
		p.logger.Debug("Source lookup failed: error getting sources", "error", err)
		return nil
	}

	p.logger.Debug("Checking sources for domain match",
		"hostname", hostname,
		"total_sources", len(sourceConfigs))

	for i := range sourceConfigs {
		source := &sourceConfigs[i]
		if matched := p.matchSourceByDomain(source, hostname); matched != nil {
			return matched
		}
		if matched := p.matchSourceByURL(source, hostname); matched != nil {
			return matched
		}
	}

	p.logger.Debug("No source found matching URL", "hostname", hostname, "checked_sources", len(sourceConfigs))
	return nil
}

// matchSourceByDomain checks if a source matches by allowed domains.
func (p *HTMLProcessor) matchSourceByDomain(source *sourcestypes.SourceConfig, hostname string) *types.Source {
	p.logger.Debug("Checking source",
		"source_name", source.Name,
		"allowed_domains", source.AllowedDomains,
		"source_url", source.URL)

	for _, allowedDomain := range source.AllowedDomains {
		exactMatch := allowedDomain == hostname
		wildcardMatch := allowedDomain == "*."+hostname

		if exactMatch || wildcardMatch {
			matchType := "wildcard"
			if exactMatch {
				matchType = "exact"
			}
			p.logger.Debug("Domain match found",
				"source_name", source.Name,
				"matched_domain", allowedDomain,
				"hostname", hostname,
				"match_type", matchType)
			return sourcestypes.ConvertToConfigSource(source)
		}
	}
	return nil
}

// matchSourceByURL checks if a source matches by source URL hostname.
func (p *HTMLProcessor) matchSourceByURL(source *sourcestypes.SourceConfig, hostname string) *types.Source {
	sourceParsedURL, parseErr := url.Parse(source.URL)
	if parseErr != nil {
		return nil
	}

	sourceHostname := sourceParsedURL.Hostname()
	if sourceHostname == hostname {
		p.logger.Debug("Source URL hostname match found",
			"source_name", source.Name,
			"source_hostname", sourceHostname,
			"target_hostname", hostname)
		return sourcestypes.ConvertToConfigSource(source)
	}
	return nil
}

// tryFindSourceByURL attempts to find a source by URL from the HTML element.
func (p *HTMLProcessor) tryFindSourceByURL(e *colly.HTMLElement) *types.Source {
	if e.Request == nil || e.Request.URL == nil {
		return nil
	}

	sourceURL := e.Request.URL.String()
	source := p.findSourceByURL(sourceURL)
	if source != nil {
		p.logger.Debug("Found source by URL for content type detection",
			"url", sourceURL,
			"source_name", source.Name,
			"has_article_body_selector", source.Selectors.Article.Body != "",
			"article_body_selector", source.Selectors.Article.Body)
	} else {
		p.logger.Debug("No source found by URL lookup", "url", sourceURL)
	}
	return source
}

// DetectContentType detects the content type of the given HTML element using selector-based detection.
func (p *HTMLProcessor) DetectContentType(e *colly.HTMLElement, source *types.Source) contenttype.Type {
	// e.DOM is a goquery.Selection, and since OnHTML("html") is used,
	// e.DOM represents the html element, so Find() searches the entire document

	// Get URL for logging
	pageURL := ""
	if e.Request != nil && e.Request.URL != nil {
		pageURL = e.Request.URL.String()
	}

	p.logger.Debug("Starting content type detection",
		"url", pageURL,
		"source_provided", source != nil,
		"source_name", func() string {
			if source != nil {
				return source.Name
			}
			return nilString
		}())

	// Strategy 1: Check Open Graph type metadata
	ogType := e.DOM.Find("meta[property='og:type']").AttrOr("content", "")
	if ogType == "article" {
		p.logger.Debug("Detected article via og:type metadata", "url", pageURL)
		return contenttype.Article
	}

	// Strategy 2: Use article selectors to detect content
	// If source is nil, try to find it by URL
	if source == nil {
		source = p.tryFindSourceByURL(e)
		if source == nil {
			p.logger.Debug("Defaulting to page: source is nil after lookup attempts",
				"url", pageURL,
				"reason", "source_nil")
			return contenttype.Page
		}
	}

	// Check if source has article body selector
	if source.Selectors.Article.Body == "" {
		p.logger.Debug("Defaulting to page: article body selector is empty",
			"url", pageURL,
			"source_name", source.Name,
			"reason", "empty_article_body_selector",
			"has_list_container_selector", source.Selectors.List.Container != "",
			"has_article_title_selector", source.Selectors.Article.Title != "")
		return contenttype.Page
	}

	// Log source configuration that will be used
	p.logger.Debug("Using source configuration for content type detection",
		"source_name", source.Name,
		"article_body_selector", source.Selectors.Article.Body,
		"article_title_selector", source.Selectors.Article.Title,
		"list_container_selector", source.Selectors.List.Container,
		"url", pageURL)

	// Get article body using the source's body selector
	bodySelector := source.Selectors.Article.Body
	articleBody := e.DOM.Find(bodySelector)
	if articleBody.Length() == 0 {
		p.logger.Debug("No article body found with selector",
			"selector", bodySelector,
			"source_name", source.Name,
			"url", pageURL,
			"reason", "selector_no_match")
		return contenttype.Page
	}

	// Verify it has substantial content (articles typically have >200 chars)
	bodyText := strings.TrimSpace(articleBody.Text())
	if len(bodyText) < constants.MinArticleBodyLength {
		p.logger.Debug("Body content too short, treating as page",
			"length", len(bodyText),
			"min_required", constants.MinArticleBodyLength,
			"source_name", source.Name,
			"url", pageURL,
			"reason", "body_too_short")
		return contenttype.Page
	}

	// Strategy 3: Verify title exists (articles should have titles)
	titleSelector := source.Selectors.Article.Title
	if titleSelector != "" {
		articleTitle := e.DOM.Find(titleSelector)
		if articleTitle.Length() == 0 {
			p.logger.Debug("No article title found, treating as page",
				"title_selector", titleSelector,
				"source_name", source.Name,
				"url", pageURL,
				"reason", "title_selector_no_match")
			return contenttype.Page
		}

		titleText := strings.TrimSpace(articleTitle.Text())
		if titleText == "" {
			p.logger.Debug("Empty article title, treating as page",
				"title_selector", titleSelector,
				"source_name", source.Name,
				"url", pageURL,
				"reason", "empty_title")
			return contenttype.Page
		}
	}

	// If we got here, it has body + title with substantial content
	p.logger.Debug("Detected article via selectors",
		"body_length", len(bodyText),
		"source_name", source.Name,
		"url", pageURL,
		"detection_method", "selector_based")
	return contenttype.Article
}

// GetUnknownTypes returns a map of content types that have no registered processor.
func (p *HTMLProcessor) GetUnknownTypes() map[contenttype.Type]int {
	return p.unknownTypes
}
