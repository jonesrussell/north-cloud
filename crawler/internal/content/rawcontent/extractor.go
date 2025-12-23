// Package rawcontent provides functionality for extracting raw content from any HTML page
// without making assumptions about content type. The classifier will handle type detection.
package rawcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// RawContentData represents extracted raw content from any HTML page
type RawContentData struct {
	ID            string
	URL           string
	Title         string
	RawText       string
	RawHTML       string
	MetaDescription string
	MetaKeywords  string
	OGType        string
	OGTitle       string
	OGDescription string
	OGImage       string
	OGURL         string
	CanonicalURL  string
	Author        string
	PublishedDate *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ExtractRawContent extracts raw content from any HTML element without type assumptions.
// Uses available selectors if provided, but falls back to generic extraction strategies.
func ExtractRawContent(e *colly.HTMLElement, sourceURL string, titleSelector, bodySelector, containerSelector string, excludeSelectors []string) *RawContentData {
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
	
	// Try OG title
	ogTitle := extractMeta(e, "og:title")
	if ogTitle != "" {
		return ogTitle
	}
	
	// Try title tag
	title := e.DOM.Find("title").First().Text()
	if title != "" {
		return strings.TrimSpace(title)
	}
	
	// Try h1 as fallback
	h1 := e.DOM.Find("h1").First().Text()
	if h1 != "" {
		return strings.TrimSpace(h1)
	}
	
	return ""
}

// extractRawHTML extracts the raw HTML content from the page
func extractRawHTML(e *colly.HTMLElement, containerSelector, bodySelector string, excludeSelectors []string) string {
	// Try container selector first
	if containerSelector != "" {
		container := e.DOM.Find(containerSelector).First()
		if container.Length() > 0 {
			// Apply excludes
			for _, exclude := range excludeSelectors {
				if exclude != "" {
					container.Find(exclude).Remove()
				}
			}
			html, _ := container.Html()
			if html != "" {
				return html
			}
		}
	}
	
	// Try body selector
	if bodySelector != "" {
		body := e.DOM.Find(bodySelector).First()
		if body.Length() > 0 {
			// Apply excludes
			for _, exclude := range excludeSelectors {
				if exclude != "" {
					body.Find(exclude).Remove()
				}
			}
			html, _ := body.Html()
			if html != "" {
				return html
			}
		}
	}
	
	// Fallback: try common content containers
	fallbackSelectors := []string{
		"article",
		"main",
		".content",
		".post-content",
		".entry-content",
		"[role='main']",
		"[role='article']",
	}
	
	for _, sel := range fallbackSelectors {
		container := e.DOM.Find(sel).First()
		if container.Length() > 0 {
			// Apply excludes
			for _, exclude := range excludeSelectors {
				if exclude != "" {
					container.Find(exclude).Remove()
				}
			}
			html, _ := container.Html()
			if html != "" && len(strings.TrimSpace(html)) > 50 {
				return html
			}
		}
	}
	
	// Last resort: get body HTML (excluding common non-content areas)
	body := e.DOM.Find("body")
	if body.Length() > 0 {
		// Remove common non-content elements
		body.Find("header, footer, nav, aside, .header, .footer, .navigation, .sidebar, .menu, script, style").Remove()
		html, _ := body.Html()
		return html
	}
	
	return ""
}

// extractRawText extracts plain text from the page
func extractRawText(e *colly.HTMLElement, containerSelector, bodySelector string, excludeSelectors []string, rawHTML string) string {
	// If we have raw HTML, extract text from it
	if rawHTML != "" {
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
		if err == nil {
			text := doc.Text()
			cleaned := strings.TrimSpace(text)
			if cleaned != "" {
				return cleaned
			}
		}
	}
	
	// Try container selector
	if containerSelector != "" {
		text := extractTextFromContainer(e, containerSelector, excludeSelectors)
		if text != "" {
			return text
		}
	}
	
	// Try body selector
	if bodySelector != "" {
		text := extractText(e, bodySelector)
		if text != "" {
			return text
		}
	}
	
	// Fallback: try common content containers
	fallbackSelectors := []string{
		"article",
		"main",
		".content",
		".post-content",
		".entry-content",
		"[role='main']",
		"[role='article']",
	}
	
	for _, sel := range fallbackSelectors {
		text := extractTextFromContainer(e, sel, excludeSelectors)
		if text != "" && len(strings.TrimSpace(text)) > 50 {
			return text
		}
	}
	
	// Last resort: extract from body paragraphs
	return extractFromBodyParagraphs(e, excludeSelectors)
}

// extractFromBodyParagraphs extracts text from body paragraphs
func extractFromBodyParagraphs(e *colly.HTMLElement, excludeSelectors []string) string {
	body := e.DOM.Find("body")
	if body.Length() == 0 {
		return ""
	}
	
	// Remove common non-content elements
	body.Find("header, footer, nav, aside, .header, .footer, .navigation, .sidebar, .menu, script, style").Remove()
	
	// Apply excludes
	for _, exclude := range excludeSelectors {
		if exclude != "" {
			body.Find(exclude).Remove()
		}
	}
	
	// Get all paragraphs
	paragraphs := body.Find("p")
	if paragraphs.Length() == 0 {
		// If no paragraphs, just get all text
		return strings.TrimSpace(body.Text())
	}
	
	var textParts []string
	paragraphs.Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if len(text) > 20 {
			textParts = append(textParts, text)
		}
	})
	
	if len(textParts) == 0 {
		return strings.TrimSpace(body.Text())
	}
	
	return strings.Join(textParts, "\n\n")
}

// extractMetadata extracts Open Graph and other metadata
func extractMetadata(data *RawContentData, e *colly.HTMLElement) {
	data.MetaDescription = extractMeta(e, "description")
	data.MetaKeywords = extractMeta(e, "keywords")
	data.OGType = extractMeta(e, "og:type")
	data.OGTitle = extractMeta(e, "og:title")
	data.OGDescription = extractMeta(e, "og:description")
	data.OGImage = extractMeta(e, "og:image")
	data.OGURL = extractMeta(e, "og:url")
	data.CanonicalURL = extractAttr(e, "link[rel='canonical']", "href")
	data.Author = extractMeta(e, "author")
	
	// Try to extract published date from meta tags
	if dateStr := extractMeta(e, "article:published_time"); dateStr != "" {
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			data.PublishedDate = &t
		}
	}
	if data.PublishedDate == nil {
		if dateStr := extractMeta(e, "article:published"); dateStr != "" {
			if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
				data.PublishedDate = &t
			}
		}
	}
}

// extractText extracts text from the first element matching the selector
func extractText(e *colly.HTMLElement, selector string) string {
	if selector == "" {
		return ""
	}
	selectors := strings.Split(selector, ",")
	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}
		text := e.ChildText(sel)
		if text != "" {
			return strings.TrimSpace(text)
		}
		element := e.DOM.Find(sel).First()
		if element.Length() > 0 {
			text := element.Text()
			if text != "" {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

// extractTextFromContainer extracts text from a container element
func extractTextFromContainer(e *colly.HTMLElement, containerSelector string, excludeSelectors []string) string {
	if containerSelector == "" {
		return ""
	}
	
	selectors := strings.Split(containerSelector, ",")
	for _, sel := range selectors {
		sel = strings.TrimSpace(sel)
		if sel == "" {
			continue
		}
		
		container := e.DOM.Find(sel).First()
		if container.Length() == 0 {
			continue
		}
		
		// Apply excludes
		for _, exclude := range excludeSelectors {
			if exclude != "" {
				container.Find(exclude).Remove()
			}
		}
		
		text := container.Text()
		if text != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// extractMeta extracts a meta tag value
func extractMeta(e *colly.HTMLElement, property string) string {
	// Try property attribute first (for og: tags)
	selector := fmt.Sprintf("meta[property='%s']", property)
	value := e.DOM.Find(selector).AttrOr("content", "")
	if value != "" {
		return value
	}
	
	// Try name attribute (for standard meta tags)
	selector = fmt.Sprintf("meta[name='%s']", property)
	value = e.DOM.Find(selector).AttrOr("content", "")
	return value
}

// extractAttr extracts an attribute from an element
func extractAttr(e *colly.HTMLElement, selector, attr string) string {
	element := e.DOM.Find(selector).First()
	if element.Length() > 0 {
		return element.AttrOr(attr, "")
	}
	return ""
}

// generateID generates a unique ID from the URL
func generateID(url string) string {
	hash := sha256.Sum256([]byte(url))
	return hex.EncodeToString(hash[:])
}

