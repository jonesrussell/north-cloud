// Package page provides functionality for processing and managing web pages.
package page

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/gocolly/colly/v2"
	configtypes "github.com/jonesrussell/gocrawl/internal/config/types"
	"github.com/jonesrussell/gocrawl/internal/domain"
	"github.com/jonesrussell/gocrawl/internal/logger"
	"github.com/jonesrussell/gocrawl/internal/sources"
	sourcestypes "github.com/jonesrussell/gocrawl/internal/sources/types"
	storagetypes "github.com/jonesrussell/gocrawl/internal/storage/types"
)

// Interface defines the contract for page processing services.
type Interface interface {
	Process(e *colly.HTMLElement) error
}

// ContentService implements the Interface for page processing.
type ContentService struct {
	logger        logger.Interface
	storage       storagetypes.Interface
	indexName     string
	sources       sources.Interface
	sourceManager SourceManager
}

// SourceManager defines the interface for managing sources.
type SourceManager interface {
	FindSourceByURL(rawURL string) *configtypes.Source
}

// NewContentService creates a new page content service.
func NewContentService(log logger.Interface, storage storagetypes.Interface, indexName string) Interface {
	return &ContentService{
		logger:    log,
		storage:   storage,
		indexName: indexName,
	}
}

// NewContentServiceWithSources creates a new page content service with sources access.
func NewContentServiceWithSources(
	log logger.Interface,
	storage storagetypes.Interface,
	indexName string,
	sourcesManager sources.Interface,
) Interface {
	return &ContentService{
		logger:        log,
		storage:       storage,
		indexName:     indexName,
		sources:       sourcesManager,
		sourceManager: &sourceWrapper{sources: sourcesManager},
	}
}

// sourceWrapper wraps sources.Interface to implement SourceManager.
type sourceWrapper struct {
	sources sources.Interface
}

// FindSourceByURL implements SourceManager.
func (s *sourceWrapper) FindSourceByURL(rawURL string) *configtypes.Source {
	if s.sources == nil {
		return nil
	}
	allSources, err := s.sources.GetSources()
	if err != nil {
		return nil
	}
	for i := range allSources {
		source := &allSources[i]
		// A more robust check might involve parsing domains or using a more sophisticated matching logic
		if strings.Contains(rawURL, source.URL) {
			return sourcestypes.ConvertToConfigSource(source)
		}
	}
	return nil
}

// Process implements the Interface.
func (s *ContentService) Process(e *colly.HTMLElement) error {
	if e == nil {
		return errors.New("nil HTML element")
	}

	sourceURL := e.Request.URL.String()

	// Get source configuration and determine index name
	// Use local variable to avoid data race when Process() is called concurrently
	indexName := s.getPageIndexName(sourceURL)
	selectors := GetSelectorsForURL(s.sourceManager, sourceURL)

	// Extract page data using Colly methods with selectors
	pageData := extractPage(e, selectors, sourceURL)

	// Convert to domain.Page
	page := &domain.Page{
		ID:            pageData.ID,
		URL:           pageData.URL,
		Title:         pageData.Title,
		Content:       pageData.Content,
		Description:   pageData.Description,
		Keywords:      pageData.Keywords,
		OgTitle:       pageData.OgTitle,
		OgDescription: pageData.OgDescription,
		OgImage:       pageData.OgImage,
		OgURL:         pageData.OgURL,
		CanonicalURL:  pageData.CanonicalURL,
		CreatedAt:     pageData.CreatedAt,
		UpdatedAt:     pageData.UpdatedAt,
	}

	// Index the page to Elasticsearch
	s.logger.Debug("Processing page with index",
		"index_name", indexName,
		"page_id", page.ID,
		"url", page.URL)
	if err := s.storage.IndexDocument(context.Background(), indexName, page.ID, page); err != nil {
		s.logger.Error("Failed to index page",
			"error", err,
			"pageID", page.ID,
			"url", page.URL,
			"index", indexName)
		return fmt.Errorf("failed to index page: %w", err)
	}

	s.logger.Debug("Page indexed successfully",
		"pageID", page.ID,
		"url", page.URL,
		"index", indexName,
		"title", page.Title)

	return nil
}

// getPageIndexName determines the index name for a page based on source configuration.
func (s *ContentService) getPageIndexName(sourceURL string) string {
	indexName := s.indexName

	if s.sources == nil {
		s.logger.Debug("No sources manager available, using default index",
			"default_index", indexName,
			"url", sourceURL)
		return indexName
	}

	sourceConfig := s.findSourceByURL(sourceURL)
	if sourceConfig == nil {
		s.logger.Debug("Source not found for URL, using default index",
			"url", sourceURL,
			"default_index", indexName)
		return indexName
	}

	s.logger.Debug("Source found by URL, using source-specific index",
		"url", sourceURL,
		"source_name", sourceConfig.Name,
		"page_index", sourceConfig.PageIndex,
		"default_index", indexName)

	// Prefer PageIndex, fallback to Index for backward compatibility
	if sourceConfig.PageIndex != "" {
		indexName = sourceConfig.PageIndex
		s.logger.Debug("Using source-specific page index",
			"index_name", indexName,
			"source_name", sourceConfig.Name,
			"url", sourceURL)
	} else if sourceConfig.Index != "" {
		indexName = sourceConfig.Index
		s.logger.Debug("Using source index (backward compatibility)",
			"index_name", indexName,
			"source_name", sourceConfig.Name,
			"url", sourceURL)
	} else {
		s.logger.Debug("Source found but PageIndex is empty, using default index",
			"default_index", indexName,
			"source_name", sourceConfig.Name,
			"url", sourceURL)
	}

	return indexName
}

// findSourceByURL attempts to find a source configuration by matching the URL domain.
// This is a helper method that returns sources.Config (which has PageIndex field).
func (s *ContentService) findSourceByURL(pageURL string) *sources.Config {
	if s.sources == nil {
		return nil
	}

	parsedURL, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	hostname := parsedURL.Hostname()
	if hostname == "" {
		return nil
	}

	// Get all sources and try to match by domain
	sourceConfigs, err := s.sources.GetSources()
	if err != nil {
		return nil
	}

	for i := range sourceConfigs {
		source := &sourceConfigs[i]
		// Check if domain matches any allowed domain
		for _, allowedDomain := range source.AllowedDomains {
			if allowedDomain == hostname || allowedDomain == "*."+hostname {
				return source
			}
		}
		// Also check source URL
		if sourceParsedURL, parseErr := url.Parse(source.URL); parseErr == nil {
			if sourceParsedURL.Hostname() == hostname {
				return source
			}
		}
	}

	return nil
}
