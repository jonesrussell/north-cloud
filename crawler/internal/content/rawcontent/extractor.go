// Package rawcontent provides functionality for extracting raw content from any HTML page
// without making assumptions about content type. The classifier will handle type detection.
package rawcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
)

// ArticleMeta contains article-specific metadata
type ArticleMeta struct {
	Section     string
	Opinion     bool
	ContentTier string
}

// TwitterMeta contains Twitter card metadata
type TwitterMeta struct {
	Card string
	Site string
}

// ExtendedOG contains extended Open Graph metadata
type ExtendedOG struct {
	ImageWidth  int
	ImageHeight int
	SiteName    string
}

// RawContentData represents extracted raw content from any HTML page
type RawContentData struct {
	ID                 string
	URL                string
	Title              string
	RawText            string
	RawHTML            string
	MetaDescription    string
	MetaKeywords       string
	OGType             string
	OGTitle            string
	OGDescription      string
	OGImage            string
	OGURL              string
	CanonicalURL       string
	Author             string
	PublishedDate      *time.Time
	ArticleSection     string
	ArticleOpinion     bool
	ArticleContentTier string
	TwitterCard        string
	TwitterSite        string
	OGImageWidth       int
	OGImageHeight      int
	OGSiteName         string
	JSONLDData         map[string]any
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// ExtractRawContent extracts raw content from any HTML element without type assumptions.
// Uses available selectors if provided, but falls back to generic extraction strategies.
func ExtractRawContent(
	e *colly.HTMLElement,
	sourceURL, titleSelector, bodySelector, containerSelector string,
	excludeSelectors []string,
) *RawContentData {
	data := &RawContentData{
		URL:       sourceURL,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract title - try selector first, then OG, then fallback
	data.Title = extractTitle(e, titleSelector)

	// Extract raw HTML - preserve original HTML for classifier
	data.RawHTML = extractRawHTML(e, containerSelector, bodySelector, excludeSelectors)

	// Extract raw text - from HTML or direct extraction
	data.RawText = extractRawText(e, containerSelector, bodySelector, excludeSelectors, data.RawHTML)

	// Extract metadata
	extractMetadata(data, e)

	// Generate ID from URL
	data.ID = generateID(sourceURL)

	return data
}

// extractTitle extracts the page title using multiple strategies
func extractTitle(e *colly.HTMLElement, selector string) string {
	// Try selector if provided
	if selector != "" {
		title := extractText(e, selector)
		if title != "" {
			return title
		}
	}

	// Try JSON-LD headline (often cleanest title on news sites)
	jsonldTitle := extractJSONLDHeadline(e)
	if jsonldTitle != "" {
		return jsonldTitle
	}

	// Try OG title
	ogTitle := extractMeta(e, "og:title")
	if ogTitle != "" {
		return ogTitle
	}

	// Try title tag
	title := e.ChildText("title")
	if title != "" {
		return strings.TrimSpace(title)
	}

	// Try h1 as fallback
	h1 := e.ChildText("h1")
	if h1 != "" {
		return strings.TrimSpace(h1)
	}

	return ""
}

// generateID generates a unique ID from the URL
func generateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}
