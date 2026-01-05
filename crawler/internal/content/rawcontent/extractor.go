// Package rawcontent provides functionality for extracting raw content from any HTML page
// without making assumptions about content type. The classifier will handle type detection.
package rawcontent

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"encoding/json"
	"strconv"

	"github.com/PuerkitoBio/goquery"
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
	ID              string
	URL             string
	Title           string
	RawText         string
	RawHTML         string
	MetaDescription string
	MetaKeywords    string
	OGType          string
	OGTitle         string
	OGDescription   string
	OGImage         string
	OGURL           string
	CanonicalURL      string
	Author            string
	PublishedDate     *time.Time
	ArticleSection    string
	ArticleOpinion    bool
	ArticleContentTier string
	TwitterCard       string
	TwitterSite       string
	OGImageWidth      int
	OGImageHeight     int
	OGSiteName        string
	JSONLDData        map[string]any
	CreatedAt         time.Time
	UpdatedAt         time.Time
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

const (
	minHTMLContentLength = 50
	minParagraphLength   = 20
)

// extractRawHTML extracts the raw HTML content from the page
func extractRawHTML(e *colly.HTMLElement, containerSelector, bodySelector string, excludeSelectors []string) string {
	// Try container selector first
	if html := tryExtractHTMLFromSelector(e, containerSelector, excludeSelectors); html != "" {
		return html
	}

	// Try body selector
	if html := tryExtractHTMLFromSelector(e, bodySelector, excludeSelectors); html != "" {
		return html
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
		if html := tryExtractHTMLFromSelector(e, sel, excludeSelectors); html != "" {
			if len(strings.TrimSpace(html)) > minHTMLContentLength {
				return html
			}
		}
	}

	// Last resort: get body HTML (excluding common non-content areas)
	return extractBodyHTML(e)
}

// tryExtractHTMLFromSelector attempts to extract HTML from a selector
func tryExtractHTMLFromSelector(e *colly.HTMLElement, selector string, excludeSelectors []string) string {
	if selector == "" {
		return ""
	}

	container := e.DOM.Find(selector).First()
	if container.Length() == 0 {
		return ""
	}

	// Apply excludes
	applyExcludes(container, excludeSelectors)

	html, _ := container.Html()
	return html
}

// applyExcludes applies exclude selectors to a container
func applyExcludes(container *goquery.Selection, excludeSelectors []string) {
	for _, exclude := range excludeSelectors {
		if exclude != "" {
			container.Find(exclude).Remove()
		}
	}
}

// extractBodyHTML extracts HTML from body element, removing non-content areas
func extractBodyHTML(e *colly.HTMLElement) string {
	body := e.DOM.Find("body")
	if body.Length() == 0 {
		return ""
	}

	// Remove common non-content elements
	body.Find("header, footer, nav, aside, .header, .footer, .navigation, .sidebar, .menu, script, style").Remove()
	html, _ := body.Html()
	return html
}

// extractRawText extracts plain text from the page
func extractRawText(
	e *colly.HTMLElement,
	containerSelector, bodySelector string,
	excludeSelectors []string,
	rawHTML string,
) string {
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
		paraText := strings.TrimSpace(s.Text())
		if len(paraText) > minParagraphLength {
			textParts = append(textParts, paraText)
		}
	})

	if len(textParts) == 0 {
		return strings.TrimSpace(body.Text())
	}

	return strings.Join(textParts, "\n\n")
}

// extractJSONLD extracts JSON-LD structured data from script tags
func extractJSONLD(e *colly.HTMLElement) map[string]any {
	result := make(map[string]any)

	// Find all script tags with type="application/ld+json"
	e.DOM.Find("script[type='application/ld+json']").Each(func(i int, s *goquery.Selection) {
		jsonText := strings.TrimSpace(s.Text())
		if jsonText == "" {
			return
		}

		var jsonData any
		if err := json.Unmarshal([]byte(jsonText), &jsonData); err != nil {
			// Skip invalid JSON, continue processing other scripts
			return
		}

		// Handle both single objects and arrays
		var jsonObjs []any
		switch v := jsonData.(type) {
		case []any:
			jsonObjs = v
		case map[string]any:
			jsonObjs = []any{v}
		default:
			return
		}

		// Extract NewsArticle schema data
		for _, obj := range jsonObjs {
			objMap, ok := obj.(map[string]any)
			if !ok {
				continue
			}

			// Check if this is a NewsArticle or Article type
			typeVal, _ := objMap["@type"].(string)
			if typeVal != "NewsArticle" && typeVal != "Article" {
				// Store non-NewsArticle schemas in result for reference
				continue
			}

			// Extract NewsArticle fields
			if headline, ok := objMap["headline"].(string); ok && headline != "" {
				result["jsonld_headline"] = headline
			}
			if desc, ok := objMap["description"].(string); ok && desc != "" {
				result["jsonld_description"] = desc
			}
			if wordCount, ok := objMap["wordCount"].(float64); ok {
				result["jsonld_word_count"] = int(wordCount)
			}
			if section, ok := objMap["articleSection"].(string); ok && section != "" {
				result["jsonld_article_section"] = section
			}
			if url, ok := objMap["url"].(string); ok && url != "" {
				result["jsonld_url"] = url
			}
			if dateCreated, ok := objMap["dateCreated"].(string); ok && dateCreated != "" {
				result["jsonld_date_created"] = dateCreated
			}
			if dateModified, ok := objMap["dateModified"].(string); ok && dateModified != "" {
				result["jsonld_date_modified"] = dateModified
			}
			if datePublished, ok := objMap["datePublished"].(string); ok && datePublished != "" {
				result["jsonld_date_published"] = datePublished
			}
			if keywords, ok := objMap["keywords"].([]any); ok && len(keywords) > 0 {
				keywordStrs := make([]string, 0, len(keywords))
				for _, kw := range keywords {
					if kwStr, ok := kw.(string); ok {
						keywordStrs = append(keywordStrs, kwStr)
					}
				}
				if len(keywordStrs) > 0 {
					result["jsonld_keywords"] = keywordStrs
				}
			}
			if author, ok := objMap["author"].(string); ok && author != "" {
				result["jsonld_author"] = author
			} else if authorObj, ok := objMap["author"].(map[string]any); ok {
				if authorName, ok := authorObj["name"].(string); ok && authorName != "" {
					result["jsonld_author"] = authorName
				}
			}
			if publisher, ok := objMap["publisher"].(map[string]any); ok {
				if pubName, ok := publisher["name"].(string); ok && pubName != "" {
					result["jsonld_publisher_name"] = pubName
				}
			}
			if image, ok := objMap["image"].(map[string]any); ok {
				if imageURL, ok := image["url"].(string); ok && imageURL != "" {
					result["jsonld_image_url"] = imageURL
				}
			} else if imageStr, ok := objMap["image"].(string); ok && imageStr != "" {
				result["jsonld_image_url"] = imageStr
			}

			// Store full JSON-LD object for reference
			result["jsonld_raw"] = objMap
		}
	})

	return result
}

// extractArticleMeta extracts article-specific metadata
func extractArticleMeta(e *colly.HTMLElement) ArticleMeta {
	var meta ArticleMeta

	meta.Section = extractMeta(e, "article:section")
	meta.ContentTier = extractMeta(e, "article:content_tier")

	// Parse article:opinion as boolean
	opinionStr := extractMeta(e, "article:opinion")
	meta.Opinion = opinionStr == "true" || opinionStr == "1"

	return meta
}

// extractTwitterMeta extracts Twitter card metadata
func extractTwitterMeta(e *colly.HTMLElement) TwitterMeta {
	var meta TwitterMeta

	meta.Card = extractMeta(e, "twitter:card")
	meta.Site = extractMeta(e, "twitter:site")

	return meta
}

// extractExtendedOG extracts extended Open Graph metadata
func extractExtendedOG(e *colly.HTMLElement) ExtendedOG {
	var og ExtendedOG

	og.SiteName = extractMeta(e, "og:site_name")

	// Parse numeric values
	if widthStr := extractMeta(e, "og:image:width"); widthStr != "" {
		if width, err := strconv.Atoi(widthStr); err == nil {
			og.ImageWidth = width
		}
	}
	if heightStr := extractMeta(e, "og:image:height"); heightStr != "" {
		if height, err := strconv.Atoi(heightStr); err == nil {
			og.ImageHeight = height
		}
	}

	return og
}

// extractMetadata extracts Open Graph and other metadata
func extractMetadata(data *RawContentData, e *colly.HTMLElement) {
	// Extract basic meta tags (keep existing extraction for backward compatibility)
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

	// Use specialized extractors (SRP)
	jsonLDData := extractJSONLD(e)
	if len(jsonLDData) > 0 {
		data.JSONLDData = jsonLDData
	} else {
		data.JSONLDData = make(map[string]any)
	}

	articleMeta := extractArticleMeta(e)
	data.ArticleSection = articleMeta.Section
	data.ArticleOpinion = articleMeta.Opinion
	data.ArticleContentTier = articleMeta.ContentTier

	twitterMeta := extractTwitterMeta(e)
	data.TwitterCard = twitterMeta.Card
	data.TwitterSite = twitterMeta.Site

	extendedOG := extractExtendedOG(e)
	data.OGImageWidth = extendedOG.ImageWidth
	data.OGImageHeight = extendedOG.ImageHeight
	data.OGSiteName = extendedOG.SiteName
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
		childText := e.ChildText(sel)
		if childText != "" {
			return strings.TrimSpace(childText)
		}
		element := e.DOM.Find(sel).First()
		if element.Length() > 0 {
			elementText := element.Text()
			if elementText != "" {
				return strings.TrimSpace(elementText)
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
