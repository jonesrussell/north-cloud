package fetcher

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractedContent represents content extracted from a fetched HTML page.
type ExtractedContent struct {
	Title         string `json:"title"`
	Body          string `json:"body"`
	Description   string `json:"description,omitempty"`
	Author        string `json:"author,omitempty"`
	ContentHash   string `json:"content_hash"`
	URL           string `json:"url"`
	SourceID      string `json:"source_id"`
	OGType        string `json:"og_type,omitempty"`
	OGTitle       string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage       string `json:"og_image,omitempty"`
	CanonicalURL  string `json:"canonical_url,omitempty"`
	MetaKeywords  string `json:"meta_keywords,omitempty"`
	PublishedDate string `json:"published_date,omitempty"`
	WordCount     int    `json:"word_count"`
}

// ContentExtractor extracts article content from HTML using goquery.
type ContentExtractor struct{}

// NewContentExtractor creates a new content extractor.
func NewContentExtractor() *ContentExtractor {
	return &ContentExtractor{}
}

// Extract parses HTML and extracts article content.
// Returns extracted content with a SHA-256 hash of the body text for deduplication.
func (e *ContentExtractor) Extract(
	sourceID, pageURL string,
	body []byte,
) (*ExtractedContent, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("parse html: %w", err)
	}

	content := &ExtractedContent{
		URL:      pageURL,
		SourceID: sourceID,
	}

	content.Title = extractPageTitle(doc)
	content.Description = extractMetaDescription(doc)
	content.Author = extractMetaAuthor(doc)
	content.Body = extractBodyText(doc)
	content.ContentHash = computeHash(content.Body)
	content.WordCount = len(strings.Fields(content.Body))

	// OG metadata
	content.OGType = extractOGMeta(doc, "og:type")
	content.OGTitle = extractOGMeta(doc, "og:title")
	content.OGDescription = extractOGMeta(doc, "og:description")
	content.OGImage = extractOGMeta(doc, "og:image")

	content.CanonicalURL = extractCanonicalURL(doc)
	content.MetaKeywords = extractMetaKeywords(doc)
	content.PublishedDate = extractPublishedDate(doc)

	return content, nil
}

// extractPageTitle extracts the page title, preferring <title> then og:title fallback.
func extractPageTitle(doc *goquery.Document) string {
	if title := strings.TrimSpace(doc.Find("title").First().Text()); title != "" {
		return title
	}

	if ogTitle, exists := doc.Find("meta[property='og:title']").Attr("content"); exists {
		return strings.TrimSpace(ogTitle)
	}

	return ""
}

// extractMetaDescription extracts the description from meta tags.
func extractMetaDescription(doc *goquery.Document) string {
	if desc, exists := doc.Find("meta[name='description']").Attr("content"); exists {
		return strings.TrimSpace(desc)
	}

	if ogDesc, exists := doc.Find("meta[property='og:description']").Attr("content"); exists {
		return strings.TrimSpace(ogDesc)
	}

	return ""
}

// extractMetaAuthor extracts the author from meta tags.
func extractMetaAuthor(doc *goquery.Document) string {
	if author, exists := doc.Find("meta[name='author']").Attr("content"); exists {
		return strings.TrimSpace(author)
	}

	return ""
}

// nonContentSelectors lists elements to strip before extracting body text.
const nonContentSelectors = "script, style, nav, header, footer"

// extractBodyText extracts the main body text from the document.
// Prefers <article> content; falls back to <body> with non-content elements stripped.
func extractBodyText(doc *goquery.Document) string {
	article := doc.Find("article").First()
	if article.Length() > 0 {
		article.Find(nonContentSelectors).Remove()
		return strings.TrimSpace(article.Text())
	}

	body := doc.Find("body").First()
	if body.Length() > 0 {
		body.Find(nonContentSelectors).Remove()
		return strings.TrimSpace(body.Text())
	}

	return ""
}

// computeHash returns the hex-encoded SHA-256 digest of the given text.
func computeHash(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// extractOGMeta extracts an OpenGraph meta tag value by property name.
func extractOGMeta(doc *goquery.Document, property string) string {
	selector := fmt.Sprintf("meta[property='%s']", property)
	if val, exists := doc.Find(selector).Attr("content"); exists {
		return strings.TrimSpace(val)
	}
	return ""
}

// extractCanonicalURL extracts the canonical URL from <link rel="canonical">.
func extractCanonicalURL(doc *goquery.Document) string {
	if href, exists := doc.Find("link[rel='canonical']").Attr("href"); exists {
		return strings.TrimSpace(href)
	}
	return ""
}

// extractMetaKeywords extracts the keywords meta tag value.
func extractMetaKeywords(doc *goquery.Document) string {
	if kw, exists := doc.Find("meta[name='keywords']").Attr("content"); exists {
		return strings.TrimSpace(kw)
	}
	return ""
}

// extractPublishedDate extracts a published date from common meta tag patterns.
// Tries article:published_time (OG), then datePublished, then pubdate, then <time datetime>.
func extractPublishedDate(doc *goquery.Document) string {
	selectors := []struct {
		sel  string
		attr string
	}{
		{"meta[property='article:published_time']", "content"},
		{"meta[name='datePublished']", "content"},
		{"meta[name='pubdate']", "content"},
		{"time[datetime]", "datetime"},
	}

	for _, s := range selectors {
		if val, exists := doc.Find(s.sel).Attr(s.attr); exists {
			if trimmed := strings.TrimSpace(val); trimmed != "" {
				return trimmed
			}
		}
	}

	return ""
}
