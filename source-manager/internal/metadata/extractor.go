package metadata

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/jonesrussell/north-cloud/source-manager/internal/models"
	infrahttp "github.com/north-cloud/infrastructure/http"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests
	defaultHTTPTimeout = 30 * time.Second
)

// isPrivateIP checks if an IP address is in a private/reserved range
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for private ranges
	if ip.IsPrivate() {
		return true
	}

	// Check for link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for unspecified (0.0.0.0 or ::)
	if ip.IsUnspecified() {
		return true
	}

	return false
}

// errInvalidURLScheme is returned when URL scheme is not http or https
var errInvalidURLScheme = errors.New("invalid URL scheme: only http and https are allowed")

// validateURL checks if a URL is safe to fetch (prevents SSRF attacks)
func validateURL(ctx context.Context, rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errInvalidURLScheme
	}

	// Block localhost and common internal hostnames
	hostname := strings.ToLower(parsedURL.Hostname())
	blockedHostnames := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"metadata.google.internal",
		"169.254.169.254", // AWS/GCP metadata service
	}
	for _, blocked := range blockedHostnames {
		if hostname == blocked {
			return fmt.Errorf("blocked hostname: %s", hostname)
		}
	}

	// Resolve hostname and check for private IPs
	resolver := &net.Resolver{}
	ips, err := resolver.LookupIPAddr(ctx, hostname)
	if err != nil {
		// If we can't resolve, allow it (might be a valid external domain)
		// The HTTP request will fail if unreachable
		return nil //nolint:nilerr // intentional: DNS failure is acceptable, HTTP will fail if unreachable
	}

	for _, ipAddr := range ips {
		if isPrivateIP(ipAddr.IP) {
			return fmt.Errorf("hostname resolves to private IP: %s", ipAddr.IP.String())
		}
	}

	return nil
}

// MetadataResponse represents suggested values from URL extraction
type MetadataResponse struct {
	Name      string                `json:"name"`
	URL       string                `json:"url"`
	Selectors models.SelectorConfig `json:"selectors"`
}

// Extractor handles metadata extraction from URLs
type Extractor struct {
	logger infralogger.Logger
	client *http.Client
}

// NewExtractor creates a new metadata extractor
func NewExtractor(log infralogger.Logger) *Extractor {
	return &Extractor{
		logger: log,
		client: infrahttp.NewClient(&infrahttp.ClientConfig{
			Timeout: defaultHTTPTimeout,
		}),
	}
}

// Extract fetches a URL and extracts metadata for form prefilling
func (e *Extractor) Extract(ctx context.Context, sourceURL string) (*MetadataResponse, error) {
	e.logger.Info("Extracting metadata from URL",
		infralogger.String("url", sourceURL),
	)

	// Validate URL for SSRF protection
	if err := validateURL(ctx, sourceURL); err != nil {
		return nil, fmt.Errorf("URL validation failed: %w", err)
	}

	// Parse URL for metadata extraction
	parsedURL, err := url.Parse(sourceURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Fetch the page
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, http.NoBody)
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
		URL: sourceURL,
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
		infralogger.String("url", sourceURL),
		infralogger.String("name", metadata.Name),
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

	// Extract article selectors
	e.extractArticleSelectors(doc, selectors)

	// Extract list selectors
	e.extractListSelectors(doc, selectors)
}

// extractArticleSelectors extracts article-related selectors from the document
func (e *Extractor) extractArticleSelectors(doc *goquery.Document, selectors *models.SelectorConfig) {
	articleSelection := doc.Find("article")
	if articleSelection.Length() == 0 {
		return
	}

	selectors.Article.Container = "article"

	// Common title selectors
	e.extractArticleTitle(doc, selectors)

	// Common body selectors
	e.extractArticleBody(doc, selectors)

	// Common image selectors
	if doc.Find("article img").Length() > 0 {
		selectors.Article.Image = "article img"
	}
}

// extractArticleTitle extracts title selector from article
func (e *Extractor) extractArticleTitle(doc *goquery.Document, selectors *models.SelectorConfig) {
	if doc.Find("article h1").Length() > 0 {
		selectors.Article.Title = "article h1"
		return
	}

	if doc.Find("article .article-title").Length() > 0 {
		selectors.Article.Title = "article .article-title"
	}
}

// extractArticleBody extracts body selector from article
func (e *Extractor) extractArticleBody(doc *goquery.Document, selectors *models.SelectorConfig) {
	if doc.Find("article .article-body").Length() > 0 {
		selectors.Article.Body = "article .article-body"
		return
	}

	if doc.Find("article .content").Length() > 0 {
		selectors.Article.Body = "article .content"
	}
}

// extractListSelectors extracts list-related selectors from the document
func (e *Extractor) extractListSelectors(doc *goquery.Document, selectors *models.SelectorConfig) {
	if doc.Find(".article-list").Length() > 0 {
		selectors.List.Container = ".article-list"
		selectors.List.ArticleList = ".article-list article"
		return
	}

	if doc.Find(".news-list").Length() > 0 {
		selectors.List.Container = ".news-list"
		selectors.List.ArticleList = ".news-list article"
	}
}
