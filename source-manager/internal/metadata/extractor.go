package metadata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jonesrussell/gosources/internal/logger"
	"github.com/jonesrussell/gosources/internal/models"
)

// MetadataResponse represents suggested values from URL extraction
type MetadataResponse struct {
	Name         string                `json:"name"`
	URL          string                `json:"url"`
	ArticleIndex string                `json:"article_index"`
	PageIndex    string                `json:"page_index"`
	Selectors    models.SelectorConfig `json:"selectors"`
}

// Extractor handles metadata extraction from URLs
type Extractor struct {
	logger logger.Logger
	client *http.Client
}

// NewExtractor creates a new metadata extractor
func NewExtractor(log logger.Logger) *Extractor {
	return &Extractor{
		logger: log,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Extract fetches a URL and extracts metadata for form prefilling
func (e *Extractor) Extract(ctx context.Context, sourceURL string) (*MetadataResponse, error) {
	e.logger.Info("Extracting metadata from URL",
		logger.String("url", sourceURL),
	)

	// Validate URL
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Fetch the page
	req, err := http.NewRequestWithContext(ctx, "GET", sourceURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid bot blocking
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; North-Cloud-SourceManager/1.0)")

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract metadata
	metadata := &MetadataResponse{
		URL:          sourceURL,
		ArticleIndex: e.generateArticleIndex(parsedURL),
		PageIndex:    e.generatePageIndex(parsedURL),
		Selectors: models.SelectorConfig{
			Article: models.ArticleSelectors{},
			List:    models.ListSelectors{},
			Page:    models.PageSelectors{},
		},
	}

	// Extract name from various sources (priority order)
	metadata.Name = e.extractName(doc, parsedURL)

	// Extract OpenGraph metadata
	e.extractOGMetadata(doc, &metadata.Selectors)

	// Extract meta tags
	e.extractMetaTags(doc, &metadata.Selectors)

	// Extract structured data hints
	e.extractStructuredData(doc, &metadata.Selectors)

	e.logger.Info("Metadata extraction complete",
		logger.String("url", sourceURL),
		logger.String("name", metadata.Name),
	)

	return metadata, nil
}

// extractName extracts a suggested name from the page
func (e *Extractor) extractName(doc *goquery.Document, parsedURL *url.URL) string {
	// Try OG title first
	if ogTitle, exists := doc.Find("meta[property='og:title']").Attr("content"); exists && ogTitle != "" {
		return ogTitle
	}

	// Try OG site name
	if ogSite, exists := doc.Find("meta[property='og:site_name']").Attr("content"); exists && ogSite != "" {
		return ogSite
	}

	// Try title tag
	if title := doc.Find("title").First().Text(); title != "" {
		return strings.TrimSpace(title)
	}

	// Fall back to domain name
	return parsedURL.Host
}

// extractOGMetadata extracts OpenGraph meta tags
func (e *Extractor) extractOGMetadata(doc *goquery.Document, selectors *models.SelectorConfig) {
	// Article selectors
	if _, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
		selectors.Article.OGTitle = "meta[property='og:title']"
	}

	if _, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		selectors.Article.OGDescription = "meta[property='og:description']"
	}

	if _, exists := doc.Find("meta[property='og:image']").Attr("content"); exists {
		selectors.Article.OGImage = "meta[property='og:image']"
	}

	if _, exists := doc.Find("meta[property='og:url']").Attr("content"); exists {
		selectors.Article.OGURL = "meta[property='og:url']"
	}

	if _, exists := doc.Find("meta[property='og:type']").Attr("content"); exists {
		selectors.Article.OGType = "meta[property='og:type']"
	}

	if _, exists := doc.Find("meta[property='og:site_name']").Attr("content"); exists {
		selectors.Article.OGSiteName = "meta[property='og:site_name']"
	}

	// Page selectors
	if _, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
		selectors.Page.OGTitle = "meta[property='og:title']"
	}

	if _, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		selectors.Page.OGDescription = "meta[property='og:description']"
	}

	if _, exists := doc.Find("meta[property='og:image']").Attr("content"); exists {
		selectors.Page.OGImage = "meta[property='og:image']"
	}

	if _, exists := doc.Find("meta[property='og:url']").Attr("content"); exists {
		selectors.Page.OGURL = "meta[property='og:url']"
	}
}

// extractMetaTags extracts standard meta tags
func (e *Extractor) extractMetaTags(doc *goquery.Document, selectors *models.SelectorConfig) {
	// Description
	if _, exists := doc.Find("meta[name='description']").Attr("content"); exists {
		selectors.Article.Description = "meta[name='description']"
		selectors.Page.Description = "meta[name='description']"
	}

	// Keywords
	if _, exists := doc.Find("meta[name='keywords']").Attr("content"); exists {
		selectors.Article.Keywords = "meta[name='keywords']"
		selectors.Page.Keywords = "meta[name='keywords']"
	}

	// Canonical
	if _, exists := doc.Find("link[rel='canonical']").Attr("href"); exists {
		selectors.Article.Canonical = "link[rel='canonical']"
		selectors.Page.Canonical = "link[rel='canonical']"
	}

	// Author
	if _, exists := doc.Find("meta[name='author']").Attr("content"); exists {
		selectors.Article.Author = "meta[name='author']"
	}

	// Article published time
	if _, exists := doc.Find("meta[property='article:published_time']").Attr("content"); exists {
		selectors.Article.PublishedTime = "meta[property='article:published_time']"
	}
}

// extractStructuredData extracts structured data hints (JSON-LD)
func (e *Extractor) extractStructuredData(doc *goquery.Document, selectors *models.SelectorConfig) {
	// Check for JSON-LD
	doc.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		if i == 0 { // Only use first JSON-LD script
			selectors.Article.JSONLD = "script[type='application/ld+json']"
		}
	})

	// Look for common article structures
	if doc.Find("article").Length() > 0 {
		selectors.Article.Container = "article"

		// Common title selectors
		if doc.Find("article h1").Length() > 0 {
			selectors.Article.Title = "article h1"
		} else if doc.Find("article .article-title").Length() > 0 {
			selectors.Article.Title = "article .article-title"
		}

		// Common body selectors
		if doc.Find("article .article-body").Length() > 0 {
			selectors.Article.Body = "article .article-body"
		} else if doc.Find("article .content").Length() > 0 {
			selectors.Article.Body = "article .content"
		}

		// Common image selectors
		if doc.Find("article img").Length() > 0 {
			selectors.Article.Image = "article img"
		}
	}

	// Look for common list structures
	if doc.Find(".article-list").Length() > 0 {
		selectors.List.Container = ".article-list"
		selectors.List.ArticleList = ".article-list article"
	} else if doc.Find(".news-list").Length() > 0 {
		selectors.List.Container = ".news-list"
		selectors.List.ArticleList = ".news-list article"
	}
}

// generateArticleIndex generates an Elasticsearch index name for articles
func (e *Extractor) generateArticleIndex(parsedURL *url.URL) string {
	// Convert domain to index name (e.g., example.com -> example_com_articles)
	domain := strings.ReplaceAll(parsedURL.Host, ".", "_")
	domain = strings.ReplaceAll(domain, "-", "_")
	domain = strings.ToLower(domain)

	// Remove www prefix if present
	domain = strings.TrimPrefix(domain, "www_")

	return fmt.Sprintf("%s_articles", domain)
}

// generatePageIndex generates an Elasticsearch index name for pages
func (e *Extractor) generatePageIndex(parsedURL *url.URL) string {
	// Convert domain to index name (e.g., example.com -> example_com_pages)
	domain := strings.ReplaceAll(parsedURL.Host, ".", "_")
	domain = strings.ReplaceAll(domain, "-", "_")
	domain = strings.ToLower(domain)

	// Remove www prefix if present
	domain = strings.TrimPrefix(domain, "www_")

	return fmt.Sprintf("%s_pages", domain)
}
