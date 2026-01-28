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
	infralogger "github.com/north-cloud/infrastructure/logger"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests
	defaultHTTPTimeout = 30 * time.Second
	// dialTimeout is the timeout for establishing connections
	dialTimeout = 10 * time.Second
	// maxIdleConns is the maximum number of idle connections across all hosts
	maxIdleConns = 10
	// maxRedirects is the maximum number of redirects to follow (matches net/http default)
	maxRedirects = 10
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

// errPrivateIPBlocked is returned when connection to private IP is attempted
var errPrivateIPBlocked = errors.New("connection to private/internal IP address is not allowed")

// safeDialContext creates a DialContext function that blocks connections to private IPs.
// This prevents SSRF attacks including DNS rebinding by validating IPs at connection time.
func safeDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Parse host and port
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	// Resolve the hostname to IP addresses
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("DNS resolution failed: %w", err)
	}

	// Check all resolved IPs - block if any are private
	for _, ipAddr := range ips {
		if isPrivateIP(ipAddr.IP) {
			return nil, fmt.Errorf("%w: %s resolves to %s", errPrivateIPBlocked, host, ipAddr.IP)
		}
	}

	// All IPs are safe, proceed with connection using the first IP
	if len(ips) == 0 {
		return nil, fmt.Errorf("no IP addresses found for host: %s", host)
	}

	// Connect to the first safe IP
	dialer := &net.Dialer{Timeout: dialTimeout}
	target := net.JoinHostPort(ips[0].IP.String(), port)

	return dialer.DialContext(ctx, network, target)
}

// newSSRFSafeClient creates an HTTP client that prevents SSRF attacks
// by validating destination IPs at connection time.
func newSSRFSafeClient() *http.Client {
	transport := &http.Transport{
		DialContext:           safeDialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          maxIdleConns,
		IdleConnTimeout:       defaultHTTPTimeout,
		TLSHandshakeTimeout:   dialTimeout,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   defaultHTTPTimeout,
		// Don't follow redirects automatically - validate each redirect URL
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			// The safeDialContext will validate the redirect destination
			return nil
		},
	}
}

// validateURLScheme checks if a URL has a valid scheme (http/https only)
func validateURLScheme(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errInvalidURLScheme
	}

	// Block obviously dangerous hostnames before even attempting DNS
	hostname := strings.ToLower(parsedURL.Hostname())
	blockedHostnames := []string{
		"localhost",
		"metadata.google.internal",
		"169.254.169.254", // AWS/GCP/Azure metadata service
	}
	for _, blocked := range blockedHostnames {
		if hostname == blocked {
			return fmt.Errorf("blocked hostname: %s", hostname)
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

// NewExtractor creates a new metadata extractor with SSRF protection.
// The client validates destination IPs at connection time to prevent
// SSRF attacks including DNS rebinding.
func NewExtractor(log infralogger.Logger) *Extractor {
	return &Extractor{
		logger: log,
		client: newSSRFSafeClient(),
	}
}

// Extract fetches a URL and extracts metadata for form prefilling
func (e *Extractor) Extract(ctx context.Context, sourceURL string) (*MetadataResponse, error) {
	e.logger.Info("Extracting metadata from URL",
		infralogger.String("url", sourceURL),
	)

	// Validate URL scheme (SSRF protection is enforced at connection time by safeDialContext)
	if err := validateURLScheme(sourceURL); err != nil {
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
